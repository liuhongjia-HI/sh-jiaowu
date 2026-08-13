package store

import "starline/learning-api/internal/domain/learning"

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
		s.pendingNoticeDeliveries = nil
		s.mu.Unlock()
		var zero T
		return zero, err
	}
	jobs := append([]learning.Notice(nil), s.pendingNoticeDeliveries...)
	s.pendingNoticeDeliveries = nil
	sender := s.officialNoticeSender
	s.mu.Unlock()

	outcomes := make([]noticeDeliveryOutcome, 0, len(jobs))
	for _, notice := range jobs {
		outcome := noticeDeliveryOutcome{id: notice.ID, status: "已发送"}
		if sender == nil {
			outcome.status = "待配置"
			outcome.reason = "需配置 WECHAT_OFFICIAL_ACCOUNT_APPID、WECHAT_OFFICIAL_ACCOUNT_SECRET、WECHAT_OFFICIAL_ACCOUNT_TEMPLATE_ID。"
		} else if err := sender(notice); err != nil {
			outcome.status = "发送失败"
			outcome.reason = err.Error()
		}
		outcomes = append(outcomes, outcome)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(outcomes) > 0 {
		if err := persistentMutationError(s, func(work *MemoryStore) error {
			work.applyNoticeDeliveryOutcomes(outcomes)
			return nil
		}); err != nil {
			var zero T
			return zero, err
		}
	}
	if refresh != nil {
		result = refresh(s, result)
	}
	return result, nil
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
