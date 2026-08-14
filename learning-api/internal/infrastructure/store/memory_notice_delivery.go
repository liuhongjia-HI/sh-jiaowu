package store

import (
	"fmt"

	"starline/learning-api/internal/domain/learning"
)

type noticeDeliveryOutcome struct {
	id     string
	status string
	reason string
}

// noticeMutation keeps network delivery outside the MemoryStore mutex. The
// first transaction records the business change and durable "发送中" state;
// the second records the sender outcome. A sender callback may safely reenter
// any public Store method.
func noticeMutation[T any](s *MemoryStore, change func(*MemoryStore) (T, error), refresh func(*MemoryStore, T) T) (T, error) {
	s.mu.Lock()
	result, err := change(s)
	if err != nil {
		s.mu.Unlock()
		var zero T
		return zero, err
	}
	s.mu.Unlock()

	s.drainPendingNoticeDeliveries()

	s.mu.Lock()
	defer s.mu.Unlock()
	if refresh != nil {
		result = refresh(s, result)
	}
	return result, nil
}

// drainPendingNoticeDeliveries performs the external side effect after the
// "发送中" outbox record is committed. It never holds s.mu while invoking a
// sender, and a second persistence failure leaves the durable outbox state
// unchanged so a later retry or process restart can safely try again.
func (s *MemoryStore) drainPendingNoticeDeliveries() {
	s.mu.Lock()
	jobs := append([]learning.Notice(nil), s.pendingNoticeDeliveries...)
	s.pendingNoticeDeliveries = nil
	sender := s.officialNoticeSender
	s.mu.Unlock()

	outcomes := make([]noticeDeliveryOutcome, 0, len(jobs))
	for _, notice := range jobs {
		outcomes = append(outcomes, sendNoticeSafely(sender, notice))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(outcomes) > 0 {
		if err := persistentMutationError(s, func(work *MemoryStore) error {
			work.applyNoticeDeliveryOutcomes(outcomes)
			return nil
		}); err != nil {
			// The business transaction has already committed. Keep every delivery
			// job for an at-least-once retry instead of making callers repeat the
			// business request (which could create duplicate data).
			s.pendingNoticeDeliveries = mergePendingNoticeDeliveries(s.pendingNoticeDeliveries, jobs)
			return
		}
	}
}

// restorePendingNoticeDeliveries turns persisted "发送中" official-account
// notices back into the in-memory outbox during startup. A process has one
// ConnectDatabase lifecycle, so this cannot create concurrent startup senders.
func (s *MemoryStore) restorePendingNoticeDeliveries() {
	if s.officialNoticeSender == nil {
		return
	}
	recovered := make([]learning.Notice, 0)
	for _, notice := range s.notices {
		if notice.Status == "发送中" && notice.Channel == "公众号模板消息" && notice.RecipientOpenID != "" {
			recovered = append(recovered, notice)
		}
	}
	s.pendingNoticeDeliveries = mergePendingNoticeDeliveries(s.pendingNoticeDeliveries, recovered)
}

// sendNoticeSafely converts an integration panic into a normal failed delivery
// outcome, so one bad sender implementation cannot lose the rest of the outbox.
func sendNoticeSafely(sender func(learning.Notice) error, notice learning.Notice) (outcome noticeDeliveryOutcome) {
	outcome = noticeDeliveryOutcome{id: notice.ID, status: "已发送"}
	defer func() {
		if recovered := recover(); recovered != nil {
			outcome.status = "发送失败"
			outcome.reason = fmt.Sprintf("通知发送异常：%v", recovered)
		}
	}()
	if sender == nil {
		outcome.status = "待配置"
		outcome.reason = "需配置 WECHAT_OFFICIAL_ACCOUNT_APPID、WECHAT_OFFICIAL_ACCOUNT_SECRET、WECHAT_OFFICIAL_ACCOUNT_TEMPLATE_ID。"
		return outcome
	}
	if err := sender(notice); err != nil {
		outcome.status = "发送失败"
		outcome.reason = err.Error()
	}
	return outcome
}

func mergePendingNoticeDeliveries(existing, additions []learning.Notice) []learning.Notice {
	seen := make(map[string]bool, len(existing)+len(additions))
	merged := make([]learning.Notice, 0, len(existing)+len(additions))
	for _, notices := range [][]learning.Notice{existing, additions} {
		for _, notice := range notices {
			if notice.ID == "" || seen[notice.ID] {
				continue
			}
			seen[notice.ID] = true
			merged = append(merged, notice)
		}
	}
	return merged
}

func (s *MemoryStore) applyNoticeDeliveryOutcomes(outcomes []noticeDeliveryOutcome) {
	byID := make(map[string]noticeDeliveryOutcome, len(outcomes))
	for _, outcome := range outcomes {
		byID[outcome.id] = outcome
	}
	for index := range s.notices {
		if outcome, ok := byID[s.notices[index].ID]; ok {
			s.notices[index].Status = outcome.status
			s.notices[index].FailureReason = outcome.reason
		}
	}
	for index := range s.parentNotices {
		if outcome, ok := byID[s.parentNotices[index].NoticeID]; ok {
			s.parentNotices[index].Status = outcome.status
			s.parentNotices[index].FailureReason = outcome.reason
		}
	}
}

func refreshNotice(s *MemoryStore, value learning.Notice) learning.Notice {
	for _, notice := range s.notices {
		if notice.ID == value.ID {
			return notice
		}
	}
	return value
}

func refreshParentNotice(s *MemoryStore, value learning.ParentNotice) learning.ParentNotice {
	for _, notice := range s.parentNotices {
		if notice.ID == value.ID {
			return notice
		}
	}
	return value
}

func refreshStudentReminder(s *MemoryStore, value learning.StudentRemindResult) learning.StudentRemindResult {
	for _, notice := range s.notices {
		if notice.ID != value.NoticeID {
			continue
		}
		if notice.Status == "已发送" {
			value.Message = "已发送学习提醒"
		} else {
			value.Message = "已创建学习提醒，待完成通知配置后补发"
		}
		break
	}
	return value
}
