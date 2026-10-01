package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"starline/learning-api/internal/domain/learning"
)

type officialSendError struct {
	reason        string
	temporary     bool
	uncertain     bool
	configuration bool
}

func (e *officialSendError) Error() string { return e.reason }

func (s *MemoryStore) businessEvent(id string) (learning.BusinessNoticeEvent, bool) {
	for _, event := range s.businessNoticeEvents {
		if event.ID == id {
			return event, true
		}
	}
	return learning.BusinessNoticeEvent{}, false
}

func (s *MemoryStore) businessTaskValidity(task learning.BusinessNoticeTask, now time.Time) string {
	if !parseBusinessTime(task.ExpiresAt).After(now) {
		return "通知已过期"
	}
	binding := s.bindingForBusinessKind(task.Kind)
	if !binding.Enabled || !bindingStudentIncluded(binding, task.StudentID) {
		return "自动通知已关闭或不在试运行范围"
	}
	if !binding.Ready {
		return "自动通知配置不可用：" + binding.Reason
	}
	if binding.TemplateID != task.TemplateID {
		return "通知模板已变更"
	}
	reachable := false
	for _, target := range s.businessRecipients(task.StudentID) {
		if target.OpenID == task.OpenID && target.GuardianID == task.GuardianID {
			reachable = true
		}
	}
	if !reachable {
		return "家长已取消关注、解绑或账号不可用"
	}
	event, ok := s.businessEvent(task.EventID)
	if !ok {
		return "业务事件不存在"
	}
	if task.Kind == learning.NoticeHomeworkSubmitted {
		submission, found := s.submissions[event.RelatedID]
		if !found || submission.StudentID != task.StudentID {
			return "提交记录已不存在或归属已变化"
		}
		if _, err := s.studentHomeworkUnlocked(learning.Principal{StudentID: task.StudentID}, submission.HomeworkID); err != nil {
			return "作业访问权限已失效"
		}
	}

	if len(event.Lessons) > 0 && task.Kind != learning.NoticeScheduleCancelled {
		for _, change := range event.Lessons {
			found := false
			for _, current := range s.scheduleClasses {
				if current.ID != change.After.ID {
					continue
				}
				if businessLessonEffective(current) && businessLessonVersion(current) == businessLessonVersion(change.After) {
					for _, student := range current.Students {
						if student.ID == task.StudentID {
							found = true
						}
					}
				}
			}
			if !found {
				return "课程安排已失效或已被新版本替代"
			}
		}
	}
	return ""
}

func (s *MemoryStore) businessTaskCanRetry(task learning.BusinessNoticeTask, now time.Time) bool {
	return task.Status == "发送失败" && s.businessTaskValidity(task, now) == "" && task.OpenID != ""
}

func (s *MemoryStore) RetryBusinessNotice(operator, id string) (learning.BusinessNoticeTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.BusinessNoticeTask, error) {
		for i, task := range work.businessNoticeTasks {
			if task.ID != id {
				continue
			}
			if !work.businessTaskCanRetry(task, time.Now()) {
				return task, errors.New("该通知不能补发，请检查发送结果和业务状态")
			}
			task.Status = "待发送"
			task.Attempts = 0
			task.FirstAttemptAt = ""
			task.ClaimedAt = ""
			task.DueAt = businessTime(time.Now())
			task.FailureReason = ""
			work.businessNoticeTasks[i] = task
			work.prependLog(operator, "补发自动通知", id)
			task.OpenID = ""
			return task, nil
		}
		return learning.BusinessNoticeTask{}, errors.New("通知任务不存在")
	})
}

func (s *MemoryStore) businessReminderSuppression(task learning.BusinessNoticeTask, now time.Time) (bool, bool) {
	if task.Kind != learning.NoticeScheduleReminder {
		return false, false
	}
	event, ok := s.businessEvent(task.EventID)
	if !ok || len(event.Lessons) == 0 {
		return false, false
	}
	version := businessLessonVersion(event.Lessons[0].After)
	start := businessLessonStart(event.Lessons[0].After)
	for _, other := range s.businessNoticeTasks {
		if other.OpenID != task.OpenID || other.StudentID != task.StudentID || (other.Kind != learning.NoticeScheduleChanged && other.Kind != learning.NoticeScheduleConfirmed) {
			continue
		}
		previous, ok := s.businessEvent(other.EventID)
		if !ok {
			continue
		}
		matches := false
		for _, change := range previous.Lessons {
			if businessLessonVersion(change.After) == version {
				matches = true
			}
		}
		if !matches {
			continue
		}
		accepted := parseBusinessTime(other.AcceptedAt)
		if !accepted.IsZero() && !accepted.Before(start.Add(-2*time.Hour)) {
			return true, false
		}
		if other.Status == "发送中" || other.Status == "结果待确认" || (other.Status == "待发送" && !parseBusinessTime(other.DueAt).After(now)) {
			return false, true
		}
	}
	return false, false
}

// A database row lock is the authoritative lease when more than one worker is
// present. JSON is storage payload; API listings redact recipient identifiers.
func (s *MemoryStore) claimBusinessTask(index int, now time.Time) (learning.BusinessNoticeTask, bool, error) {
	task := s.businessNoticeTasks[index]
	claim := func(current learning.BusinessNoticeTask) (learning.BusinessNoticeTask, bool) {
		if current.Status != "待发送" && current.Status != "发送失败" {
			return current, false
		}
		if current.Kind == learning.NoticeScheduleReminder && businessNextReminderWindow(now).After(now) {
			return current, false
		}
		if parseBusinessTime(current.DueAt).After(now) {
			return current, false
		}
		if current.Attempts >= 4 {
			return current, false
		}
		current.Status = "发送中"
		current.ClaimedAt = businessTime(now)
		if current.FirstAttemptAt == "" {
			current.FirstAttemptAt = businessTime(now)
		}
		current.Attempts++
		return current, true
	}
	if s.db == nil {
		task, ok := claim(task)
		if ok {
			s.businessNoticeTasks[index] = task
		}
		return task, ok, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return task, false, err
	}
	defer tx.Rollback()
	var payload []byte
	if err := tx.QueryRow(`SELECT payload FROM business_notice_tasks WHERE id=? FOR UPDATE`, task.ID).Scan(&payload); err != nil {
		return task, false, err
	}
	if err := json.Unmarshal(payload, &task); err != nil {
		return task, false, err
	}
	task, ok := claim(task)
	if !ok {
		s.businessNoticeTasks[index] = task
		return task, false, nil
	}
	if _, err := tx.Exec(`UPDATE business_notice_tasks SET status=?, due_at=?, payload=? WHERE id=?`, task.Status, businessSQLTime(task.DueAt), mustJSON(task), task.ID); err != nil {
		return task, false, err
	}
	if err := tx.Commit(); err != nil {
		return task, false, err
	}
	s.businessNoticeTasks[index] = task
	return task, true, nil
}

func businessSQLTime(value string) any {
	t := parseBusinessTime(value)
	if t.IsZero() {
		return nil
	}
	return t.In(businessNoticeLocation).Format("2006-01-02 15:04:05")
}

func (s *MemoryStore) ProcessBusinessNotices(now time.Time) error {
	if !s.businessWorkerMu.TryLock() {
		return nil
	}
	defer s.businessWorkerMu.Unlock()
	// Serialize preparation and delivery across API processes as well as within one process.
	// MySQL row leases remain durable; a named lock prevents a stale replica from overwriting them.
	if s.db != nil {
		conn, err := s.db.Conn(context.Background())
		if err != nil {
			return err
		}
		defer conn.Close()
		var acquired int
		if err := conn.QueryRowContext(context.Background(), "SELECT GET_LOCK(CONCAT(DATABASE(), ':business-notices'), 0)").Scan(&acquired); err != nil {
			return err
		}
		if acquired != 1 {
			return nil
		}
		defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(CONCAT(DATABASE(), ':business-notices'))")
		s.mu.Lock()
		err = s.loadBusinessNoticesFromDB()
		s.mu.Unlock()
		if err != nil {
			return err
		}
	}
	s.mu.Lock()
	err := persistentMutationError(s, func(work *MemoryStore) error {
		work.planBusinessReminders(now)
		for i, task := range work.businessNoticeTasks {
			if task.Status == "微信已受理" || task.Status == "已送达" || task.Status == "已失效" {
				continue
			}
			if reason := work.businessTaskValidity(task, now); reason != "" {
				if task.OpenID != "" {
					task.Status = "已失效"
					task.FailureReason = reason
					work.businessNoticeTasks[i] = task
				}
				continue
			}
			if task.Status == "发送中" && now.Sub(parseBusinessTime(task.ClaimedAt)) > 30*time.Second {
				task.Status = "结果待确认"
				task.FailureReason = "发送进程中断，微信接收结果待确认"
				task.DueAt = businessTime(now)
				work.businessNoticeTasks[i] = task
			}
			if task.Status == "结果待确认" && !parseBusinessTime(task.DueAt).After(now) && now.Sub(parseBusinessTime(task.FirstAttemptAt)) < 10*time.Minute {
				task.Status = "待发送"
				work.businessNoticeTasks[i] = task
			}
		}
		return nil
	})
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	for sent := 0; sent < 100; sent++ {
		s.mu.Lock()
		index := -1
		for i, task := range s.businessNoticeTasks {
			if (task.Status == "待发送" || task.Status == "发送失败") && task.Attempts < 4 && !parseBusinessTime(task.DueAt).After(now) && s.businessTaskValidity(task, now) == "" {
				index = i
				break
			}
		}
		if index < 0 {
			s.mu.Unlock()
			break
		}
		task := s.businessNoticeTasks[index]
		suppressed, waiting := s.businessReminderSuppression(task, now)
		if suppressed || waiting {
			err := persistentMutationError(s, func(work *MemoryStore) error {
				if suppressed {
					work.businessNoticeTasks[index].Status = "已失效"
					work.businessNoticeTasks[index].FailureReason = "临近开课确认或调课消息已提醒"
				} else {
					work.businessNoticeTasks[index].DueAt = businessTime(now.Add(time.Minute))
				}
				return nil
			})
			s.mu.Unlock()
			if err != nil {
				return err
			}
			continue
		}
		task, claimed, err := s.claimBusinessTask(index, now)
		sender := s.officialMessageSender
		s.mu.Unlock()
		if err != nil {
			return err
		}
		if !claimed {
			continue
		}
		messageID, sendErr := sendBusinessTemplateSafely(sender, learning.OfficialMessageRequest{TemplateID: task.TemplateID, OpenID: task.OpenID, Values: task.Values, PagePath: task.PagePath, ClientMessageID: task.ClientMessageID})
		s.mu.Lock()
		err = persistentMutationError(s, func(work *MemoryStore) error {
			for i, current := range work.businessNoticeTasks {
				if current.ID != task.ID {
					continue
				}
				current.MessageID = messageID
				current.ClaimedAt = ""
				current.FailureReason = ""
				if sendErr == nil {
					current.Status = "微信已受理"
					current.AcceptedAt = businessTime(now)
					for _, receipt := range work.businessNoticeReceipts {
						if receipt.MessageID == messageID && receipt.OpenID == current.OpenID {
							applyBusinessReceipt(&current, receipt)
						}
					}
				} else {
					current.Status = "发送失败"
					current.FailureReason = sendErr.Error()
					var typed *officialSendError
					if errors.As(sendErr, &typed) {
						if typed.configuration {
							current.Status = "配置错误"
						}
						if typed.uncertain {
							current.Status = "结果待确认"
						}
						if !typed.temporary && !typed.uncertain {
							current.Attempts = 4
						}
					}
					if current.Attempts < 4 {
						delay := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}[current.Attempts-1]
						current.DueAt = businessTime(now.Add(delay))
					}
				}
				work.businessNoticeTasks[i] = current
				break
			}
			return nil
		})
		s.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func sendBusinessTemplateSafely(sender func(learning.OfficialMessageRequest) (string, error), request learning.OfficialMessageRequest) (id string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &officialSendError{reason: "发送服务异常，结果待确认", uncertain: true}
		}
	}()
	if sender == nil {
		return "", &officialSendError{reason: "公众号发送配置不可用", configuration: true}
	}
	return sender(request)
}

func applyBusinessReceipt(task *learning.BusinessNoticeTask, receipt learning.OfficialDeliveryReceipt) {
	if task.Status == "已送达" {
		return
	}
	if receipt.Status == "success" {
		task.Status = "已送达"
		task.DeliveredAt = receipt.ReceivedAt
		task.FailureReason = ""
	} else {
		task.Status = "发送失败"
		task.FailureReason = "微信发送回执：" + receipt.Status
		task.Attempts = 4
	}
}

func (s *MemoryStore) HandleOfficialDeliveryReceipt(openID, messageID, status string) error {
	if len(messageID) == 0 || len(messageID) > 64 || len(openID) > 191 || len(status) > 100 {
		return errors.New("发送回执格式无效")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		receipt := learning.OfficialDeliveryReceipt{MessageID: messageID, OpenID: openID, Status: status, ReceivedAt: businessTime(time.Now())}
		found := false
		for i, old := range work.businessNoticeReceipts {
			if old.MessageID == messageID {
				if old.OpenID != openID {
					return errors.New("回执接收人不匹配")
				}
				if old.Status == "success" {
					receipt = old
				}
				work.businessNoticeReceipts[i] = receipt
				found = true
			}
		}
		if !found {
			work.businessNoticeReceipts = append(work.businessNoticeReceipts, receipt)
		}
		for i := range work.businessNoticeTasks {
			if work.businessNoticeTasks[i].MessageID == messageID && work.businessNoticeTasks[i].OpenID == openID {
				applyBusinessReceipt(&work.businessNoticeTasks[i], receipt)
			}
		}

		for i := range work.officialCampaignRecipients {
			recipient := &work.officialCampaignRecipients[i]
			if recipient.MessageID == messageID && recipient.OpenID == openID {
				applyCampaignReceipt(recipient, receipt)
				work.updateOfficialCampaignDelivery(recipient.CampaignID)
			}
		}
		return nil
	})
}

func (s *MemoryStore) RunBusinessNoticeWorker(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	followers := time.NewTicker(24 * time.Hour)
	defer followers.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.ProcessBusinessNotices(now); err != nil && onError != nil {
				onError(fmt.Errorf("automatic notification worker: %w", err))
			}
		case <-followers.C:
			s.mu.Lock()
			configured := s.officialFollowerSyncer != nil
			s.mu.Unlock()
			if configured {
				if _, err := s.SyncOfficialFollowers("系统定期同步"); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}
}

// Enrichment runs outside the store mutex and never delays the callback reply.
func (s *MemoryStore) RefreshOfficialFollower(openID string) error {
	s.mu.Lock()
	resolve := s.officialFollowerInfo
	s.mu.Unlock()
	if resolve == nil {
		return nil
	}
	follower, err := resolve(openID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		for i, old := range work.officialFollowers {
			if old.OpenID == openID {
				if !old.Subscribed {
					return nil
				}
				work.officialFollowers[i] = follower
				return nil
			}
		}
		return nil
	})
}
