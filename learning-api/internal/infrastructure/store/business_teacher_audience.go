package store

import (
	"errors"
	"sort"
	"starline/learning-api/internal/domain/learning"
	"strings"
)

func (s *MemoryStore) TeachingPlanNotificationAudience(p learning.Principal, req learning.TeachingPlanAudienceRequest) ([]learning.TeachingPlanNoticeRecipient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(req.PlanIDs) == 0 || len(req.PlanIDs) > 500 {
		return nil, errors.New("请选择 1 至 500 份教案")
	}
	plans := []learning.TeachingPlan{}
	seen := map[string]bool{}
	for _, id := range req.PlanIDs {
		id = strings.TrimSpace(id)
		if seen[id] {
			continue
		}
		seen[id] = true
		plan, err := s.planUnlocked(p, id)
		if err != nil || !s.canUploadPlan(p, plan.Grade, plan.Subject) {
			return nil, errors.New("没有权限预览此教案的提醒接收范围")
		}
		plans = append(plans, plan)
	}
	return s.teachingPlanNoticeAudienceUnlocked(p.UserID, plans), nil
}

func (s *MemoryStore) teachingPlanNoticeAudienceUnlocked(uploaderID string, plans []learning.TeachingPlan) []learning.TeachingPlanNoticeRecipient {
	rows := []learning.TeachingPlanNoticeRecipient{}
	identities := map[string]int{}
	for _, user := range s.users {
		if user.AccountStatus == "正常" && hasRole(user.Roles, learning.RoleTeacher) && user.UnionID != "" {
			identities[user.UnionID]++
		}
	}
	for _, user := range s.users {
		if user.ID == uploaderID || user.AccountStatus != "正常" || !hasRole(user.Roles, learning.RoleTeacher) {
			continue
		}
		p, err := s.principalByUserIDUnlocked(user.ID)
		if err != nil {
			continue
		}
		row := learning.TeachingPlanNoticeRecipient{UserID: user.ID, Name: user.Name, PlanIDs: []string{}}
		for _, plan := range plans {
			if s.canViewPlan(p, plan.Grade, plan.Subject) {
				row.PlanIDs = append(row.PlanIDs, plan.ID)
			}
		}
		if len(row.PlanIDs) == 0 {
			continue
		}
		switch {
		case user.UnionID == "":
			row.Reason = "未关联微信身份"
		case identities[user.UnionID] != 1:
			row.Reason = "微信身份关联多个教师账号"
		default:
			openIDs := map[string]bool{}
			for _, follower := range s.officialFollowers {
				if follower.Subscribed && follower.UnionID == user.UnionID && follower.OpenID != "" {
					openIDs[follower.OpenID] = true
				}
			}
			if len(openIDs) == 0 {
				row.Reason = "未关注公众号或身份未匹配"
			} else if len(openIDs) > 1 {
				row.Reason = "公众号关注身份不唯一"
			} else {
				row.Reachable = true
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].UserID < rows[j].UserID })
	return rows
}
