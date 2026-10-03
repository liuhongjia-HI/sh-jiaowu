package store

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"time"
)

const teachingPlanBatchKind = "teaching_plan_upload_batch"

func teachingPlanBatchID(userID, batchID string) string {
	return businessNoticeHash(teachingPlanBatchKind, userID, batchID)
}

func (s *MemoryStore) registerTeachingPlanNoticeBatch(p learning.Principal, batchID string, plan learning.TeachingPlan) error {
	legacy := batchID == ""
	if legacy {
		batchID = plan.ID
	}
	id := teachingPlanBatchID(p.UserID, batchID)
	found := false
	for i := range s.businessNoticeEvents {
		if s.businessNoticeEvents[i].ID == id {
			s.businessNoticeEvents[i].ResourceIDs = appendUnique(s.businessNoticeEvents[i].ResourceIDs, plan.ID)
			found = true
			break
		}
	}
	if !found {
		s.businessNoticeEvents = append(s.businessNoticeEvents, learning.BusinessNoticeEvent{ID: id, Kind: teachingPlanBatchKind, UploaderID: p.UserID, BatchID: batchID, ResourceIDs: []string{plan.ID}, CreatedAt: businessTime(time.Now())})
	}
	if legacy {
		_, err := s.completeTeachingPlanNoticeBatchUnlocked(p, batchID)
		return err
	}
	return nil
}

func (s *MemoryStore) PendingTeachingPlanNoticeBatches(p learning.Principal) []learning.PendingTeachingPlanNoticeBatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []learning.PendingTeachingPlanNoticeBatch{}
	if !canUploadHandout(p) {
		return rows
	}
	for _, e := range s.businessNoticeEvents {
		if e.Kind != teachingPlanBatchKind || e.UploaderID != p.UserID || (e.BatchCompleted && e.CompletedResourceCount == len(e.ResourceIDs)) {
			continue
		}
		if _, err := s.teachingPlanBatchResources(p, e.ResourceIDs, true); err == nil {
			rows = append(rows, learning.PendingTeachingPlanNoticeBatch{BatchID: e.BatchID, ResourceCount: len(e.ResourceIDs)})
		}
	}
	return rows
}

func (s *MemoryStore) teachingPlanBatchResources(p learning.Principal, ids []string, upload bool) ([]learning.TeachingPlan, error) {
	plans := []learning.TeachingPlan{}
	for _, id := range ids {
		plan, err := s.planUnlocked(p, id)
		if err != nil || (upload && !s.canUploadPlan(p, plan.Grade, plan.Subject)) {
			return nil, errors.New("教案不存在或访问权限已失效")
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func teachingPlanNoticeURL(b learning.BusinessNoticeBinding, e learning.BusinessNoticeEvent) string {
	if !validTeacherNoticeOrigin(b.WebOrigin) || len(e.ResourceIDs) == 0 {
		return ""
	}
	target, _ := url.Parse(b.WebOrigin)
	target.Path = "/teaching-plans"
	target.RawPath = ""
	target.RawQuery = url.Values{"plan": []string{e.ResourceIDs[0]}}.Encode()
	return target.String()
}

func teachingPlanNoticeValues(b learning.BusinessNoticeBinding, e learning.BusinessNoticeEvent, plans []learning.TeachingPlan) map[string]string {
	values := map[string]string{}
	if len(plans) == 0 {
		return values
	}
	scopes := []string{}
	for _, p := range plans {
		scopes = appendUnique(scopes, p.Grade+" · "+p.Subject)
	}
	for key, source := range b.FieldMappings {
		switch source {
		case "resource_title":
			values[key] = businessShortName(plans[0].Title)
		case "resource_count":
			values[key] = fmt.Sprint(len(plans))
		case "teacher_name":
			values[key] = businessShortName(e.RecipientName)
		case "teaching_scope":
			values[key] = businessShortName(strings.Join(scopes, "、"))
		case "published_at":
			format := "2006-01-02 15:04:05"
			if strings.HasPrefix(key, "date") {
				format = "2006-01-02"
			}
			values[key] = parseBusinessTime(e.CreatedAt).In(businessNoticeLocation).Format(format)
		}
	}
	return values
}

func (s *MemoryStore) teacherNoticeOpenID(userID, uploaderID string, plans []learning.TeachingPlan) (string, string) {
	for _, row := range s.teachingPlanNoticeAudienceUnlocked(uploaderID, plans) {
		if row.UserID == userID {
			if !row.Reachable {
				return "", row.Reason
			}
			if len(row.PlanIDs) != len(plans) {
				return "", "教案访问权限已失效"
			}
			for _, user := range s.users {
				if user.ID == userID {
					for _, f := range s.officialFollowers {
						if f.Subscribed && f.UnionID == user.UnionID && f.OpenID != "" {
							return f.OpenID, ""
						}
					}
				}
			}
		}
	}
	return "", "教师账号、授课范围或关注身份已失效"
}

func (s *MemoryStore) CompleteTeachingPlanNoticeBatch(operator string, p learning.Principal, batchID string) (learning.MaterialNoticeBatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.MaterialNoticeBatchResult, error) {
		result, err := work.completeTeachingPlanNoticeBatchUnlocked(p, batchID)
		if err == nil && !result.AlreadyCompleted {
			work.prependLog(operator, "汇总教案通知", batchID)
		}
		return result, err
	})
}

func (s *MemoryStore) completeTeachingPlanNoticeBatchUnlocked(p learning.Principal, batchID string) (learning.MaterialNoticeBatchResult, error) {
	result := learning.MaterialNoticeBatchResult{}
	if batchID == "" || len(batchID) > 64 || batchID != strings.TrimSpace(batchID) || !canUploadHandout(p) {
		return result, errors.New("没有权限汇总此教案批次")
	}
	registryID := teachingPlanBatchID(p.UserID, batchID)
	index := -1
	for i, e := range s.businessNoticeEvents {
		if e.ID == registryID && e.Kind == teachingPlanBatchKind {
			index = i
			break
		}
	}
	if index < 0 {
		return result, errors.New("上传批次不存在或不属于当前账号")
	}
	registry := s.businessNoticeEvents[index]
	plans, err := s.teachingPlanBatchResources(p, registry.ResourceIDs, true)
	if err != nil {
		return result, err
	}
	result.ResourceCount, result.AlreadyCompleted = len(plans), registry.BatchCompleted
	binding := s.bindingForBusinessKind(learning.NoticeTeachingPlansUploaded)
	now := time.Now()
	for _, recipient := range s.teachingPlanNoticeAudienceUnlocked(p.UserID, plans) {
		id := businessNoticeHash(registryID, recipient.UserID)
		e := learning.BusinessNoticeEvent{ID: id, Kind: learning.NoticeTeachingPlansUploaded, BatchID: registryID, UploaderID: p.UserID, RecipientUserID: recipient.UserID, RecipientName: recipient.Name, ResourceIDs: recipient.PlanIDs, Title: "教案已更新", Summary: fmt.Sprintf("共%d份教案", len(recipient.PlanIDs)), CreatedAt: registry.CreatedAt, ExpiresAt: businessTime(parseBusinessTime(registry.CreatedAt).Add(7 * 24 * time.Hour))}
		current, _ := s.principalByUserIDUnlocked(recipient.UserID)
		visible, err := s.teachingPlanBatchResources(current, e.ResourceIDs, false)
		if err != nil {
			continue
		}
		e.Values = teachingPlanNoticeValues(binding, e, visible)
		existed := false
		for i, old := range s.businessNoticeEvents {
			if old.ID == id {
				s.businessNoticeEvents[i] = e
				existed = true
				break
			}
		}
		if !existed {
			if registry.BatchCompleted {
				continue
			}
			s.businessNoticeEvents = append(s.businessNoticeEvents, e)
		}
		result.RecipientCount++
		included := len(binding.TeacherIDs) == 0 || containsString(binding.TeacherIDs, recipient.UserID)
		if !binding.Enabled || !binding.Ready || !included {
			continue
		}
		openID, reason := s.teacherNoticeOpenID(recipient.UserID, p.UserID, visible)
		taskIndex := -1
		for i, task := range s.businessNoticeTasks {
			if task.EventID == id {
				taskIndex = i
				break
			}
		}
		if taskIndex < 0 && existed {
			continue
		} // Never replay a batch completed while disabled.
		if taskIndex >= 0 {
			old := s.businessNoticeTasks[taskIndex]
			if old.Status != "待发送" && (old.Status != "不可触达" || old.Attempts != 0) {
				continue
			}
		}
		taskID := businessNoticeHash(id, recipient.UserID)
		task := learning.BusinessNoticeTask{ID: taskID, EventID: id, Kind: e.Kind, RecipientUserID: recipient.UserID, RecipientName: recipient.Name, OpenID: openID, TemplateID: binding.TemplateID, Values: cloneMap(e.Values), URL: teachingPlanNoticeURL(binding, e), ClientMessageID: taskID, Status: "待发送", DueAt: businessTime(now), ExpiresAt: e.ExpiresAt, CreatedAt: businessTime(now)}
		if reason != "" {
			task.Status = "不可触达"
			task.FailureReason = reason
		}
		if taskIndex >= 0 {
			task.CreatedAt = s.businessNoticeTasks[taskIndex].CreatedAt
			task.FirstAttemptAt = s.businessNoticeTasks[taskIndex].FirstAttemptAt
			task.Attempts = s.businessNoticeTasks[taskIndex].Attempts
			s.businessNoticeTasks[taskIndex] = task
		} else {
			s.businessNoticeTasks = append(s.businessNoticeTasks, task)
		}
	}
	s.businessNoticeEvents[index].BatchCompleted = true
	s.businessNoticeEvents[index].CompletedResourceCount = len(registry.ResourceIDs)
	return result, nil
}

func (s *MemoryStore) teacherNoticeTaskValidity(task learning.BusinessNoticeTask, b learning.BusinessNoticeBinding) string {
	if task.RecipientUserID == "" || (len(b.TeacherIDs) > 0 && !containsString(b.TeacherIDs, task.RecipientUserID)) {
		return "教师不在试运行范围"
	}
	e, ok := s.businessEvent(task.EventID)
	if !ok || e.Kind != learning.NoticeTeachingPlansUploaded || e.RecipientUserID != task.RecipientUserID {
		return "教案通知事件不存在或归属已变化"
	}
	p, err := s.principalByUserIDUnlocked(task.RecipientUserID)
	if err != nil || !hasRole(p.Roles, learning.RoleTeacher) {
		return "教师账号已失效"
	}
	plans, err := s.teachingPlanBatchResources(p, e.ResourceIDs, false)
	if err != nil || len(plans) == 0 {
		return "教案访问权限已失效"
	}
	openID, reason := s.teacherNoticeOpenID(task.RecipientUserID, e.UploaderID, plans)
	if reason != "" {
		return reason
	}
	if openID != task.OpenID {
		return "教师关注身份已变更"
	}
	if !reflect.DeepEqual(task.Values, teachingPlanNoticeValues(b, e, plans)) || task.URL != teachingPlanNoticeURL(b, e) {
		return "教案模板映射或网页入口已变更"
	}
	return ""
}
