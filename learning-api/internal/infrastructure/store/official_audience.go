package store

import (
	"errors"
	"regexp"
	"sort"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) reachableOfficialGuardian(id string) (officialAudienceTarget, string) {
	var guardian *learning.Guardian
	for i := range s.guardians {
		if s.guardians[i].ID == id {
			guardian = &s.guardians[i]
			break
		}
	}
	if guardian == nil {
		return officialAudienceTarget{}, "家长账号不存在"
	}
	if guardian.AccountStatus != "正常" {
		return officialAudienceTarget{}, "家长账号已停用"
	}
	if guardian.UnionID == "" {
		return officialAudienceTarget{}, "缺少小程序 UnionID，请重新登录小程序"
	}
	var matched []learning.OfficialFollower
	for _, follower := range s.officialFollowers {
		if follower.UnionID == guardian.UnionID && follower.Subscribed && follower.OpenID != "" {
			matched = append(matched, follower)
		}
	}
	if len(matched) == 0 {
		return officialAudienceTarget{}, "未关注当前服务号或关注身份未同步"
	}
	if len(matched) != 1 {
		return officialAudienceTarget{}, "公众号身份匹配不唯一"
	}
	for _, other := range s.guardians {
		if other.ID != guardian.ID && other.AccountStatus == "正常" && other.UnionID == guardian.UnionID {
			return officialAudienceTarget{}, "小程序身份关联多个家长账号"
		}
	}
	names := []string{}
	for _, relation := range s.guardianStudents {
		if relation.GuardianID != id || relation.Status != learning.GuardianStudentActive {
			continue
		}
		student, ok := s.findStudent(relation.StudentID)
		if ok && student.AccountStatus == "正常" {
			names = append(names, student.Name)
		}
	}
	if len(names) == 0 {
		return officialAudienceTarget{}, "没有有效的学生绑定关系"
	}
	return officialAudienceTarget{GuardianID: id, GuardianName: firstNonEmpty(guardian.Name, guardian.Nickname, "家长"), OpenID: matched[0].OpenID, StudentNames: uniqueStrings(names)}, ""
}

func (s *MemoryStore) LookupOfficialRecipient(phone string) (learning.OfficialRecipientLookup, error) {
	phone = strings.TrimSpace(phone)
	if !regexp.MustCompile(`^1[0-9]{10}$`).MatchString(phone) {
		return learning.OfficialRecipientLookup{}, errors.New("请输入完整手机号")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for _, guardian := range s.guardians {
		if guardian.Phone == phone {
			ids = append(ids, guardian.ID)
		}
	}
	if len(ids) != 1 {
		return learning.OfficialRecipientLookup{}, errors.New("未找到唯一家长账号，请先完成小程序登录绑定")
	}
	target, reason := s.reachableOfficialGuardian(ids[0])
	return learning.OfficialRecipientLookup{GuardianID: ids[0], Name: target.GuardianName, Phone: phone, StudentNames: target.StudentNames, Reachable: reason == "", Reason: reason}, nil
}

func (s *MemoryStore) officialSelectedAudienceUnlocked(req learning.OfficialAudiencePreviewRequest) (learning.OfficialAudiencePreview, []officialAudienceTarget, error) {
	mode := strings.TrimSpace(req.RecipientMode)
	if mode == "" {
		mode = "grades"
	}
	selected := map[string]bool{}
	if mode == "specified" {
		ids := compactStrings(req.GuardianIDs)
		if len(ids) != 1 {
			return learning.OfficialAudiencePreview{}, nil, errors.New("指定账号模式必须选择一位家长")
		}
		found := false
		for _, guardian := range s.guardians {
			if guardian.ID == ids[0] {
				found = true
			}
		}
		if !found {
			return learning.OfficialAudiencePreview{}, nil, errors.New("指定家长不存在")
		}
		selected[ids[0]] = true
	} else if mode != "grades" || len(req.GuardianIDs) > 0 {
		return learning.OfficialAudiencePreview{}, nil, errors.New("接收模式与指定账号不一致")
	}
	gradeSet := map[string]bool{}
	for _, grade := range compactStrings(req.Grades) {
		gradeSet[grade] = true
	}
	if mode == "grades" && len(gradeSet) == 0 {
		return learning.OfficialAudiencePreview{}, nil, errors.New("请至少选择一个年级")
	}
	students := map[string]learning.Student{}
	for _, raw := range s.students {
		student := s.decorateStudent(raw)
		if student.AccountStatus == "正常" && (mode == "specified" || gradeSet[student.Grade]) {
			students[student.ID] = student
		}
	}
	namesByGuardian := map[string][]string{}
	studentSet := map[string]bool{}
	for _, relation := range s.guardianStudents {
		if relation.Status != learning.GuardianStudentActive || (mode == "specified" && !selected[relation.GuardianID]) {
			continue
		}
		student, ok := students[relation.StudentID]
		if !ok {
			continue
		}
		namesByGuardian[relation.GuardianID] = append(namesByGuardian[relation.GuardianID], student.Name)
		studentSet[student.ID] = true
	}
	preview := learning.OfficialAudiencePreview{Grades: compactStrings(req.Grades), StudentCount: len(students)}
	if mode == "specified" {
		preview.StudentCount = len(studentSet)
		if len(namesByGuardian) == 0 {
			for id := range selected {
				namesByGuardian[id] = nil
			}
		}
	}
	targets := []officialAudienceTarget{}
	seen := map[string]bool{}
	for id, names := range namesByGuardian {
		preview.GuardianCount++
		target, reason := s.reachableOfficialGuardian(id)
		if reason != "" {
			preview.UnmatchedCount++
			preview.UnreachableReasons = append(preview.UnreachableReasons, reason)
			continue
		}
		if seen[target.OpenID] {
			preview.DuplicateCount++
			continue
		}
		seen[target.OpenID] = true
		target.StudentNames = uniqueStrings(names)
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].GuardianID < targets[j].GuardianID })
	preview.ReachableCount = len(targets)
	preview.UnreachableCount = preview.GuardianCount - preview.ReachableCount
	preview.UnreachableReasons = uniqueStrings(preview.UnreachableReasons)
	return preview, targets, nil
}
