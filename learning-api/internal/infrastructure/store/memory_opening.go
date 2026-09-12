package store

import (
	"starline/learning-api/internal/domain/learning"
	"strings"
)

// openingSubject is shared by the picker and every grant write path. Existing
// access checks deliberately do not use it: disabling a subject stops new sales.
func (s *MemoryStore) openingSubject(spaceID, grade string) (learning.SubjectMetadata, string) {
	space, ok := s.findLearningSpace(spaceID)
	if !ok {
		return learning.SubjectMetadata{}, "课程范围不存在：" + spaceID
	}
	var meta learning.SubjectMetadata
	for _, item := range s.subjects {
		if subjectsMatch(item.Name, space.Subject) {
			meta = item
			break
		}
	}
	if meta.ID == "" {
		return meta, "学科不存在：" + space.Subject
	}
	if meta.Status != "启用" {
		return meta, "学科已停用：" + meta.Name
	}
	if space.Status != learning.StatusEnabled {
		return meta, "课程范围已停用：" + space.Name
	}
	if space.Grade != grade {
		return meta, "不能给" + grade + "学生开通" + space.Grade + "课程"
	}
	index := gradeIndexOf(grade)
	if index < 0 {
		return meta, "课程年级不可用：" + grade
	}
	levels := extraSubjectLevels(index)
	for _, core := range demoSubjects {
		if subjectsMatch(core, meta.Name) {
			levels = levelsForGradeSubject(index, core)
			break
		}
	}
	level := strings.TrimSpace(space.Level)
	if level == "" {
		level = "S"
	}
	if !containsString(levels, level) {
		return meta, "该年级不开放此学科或等级：" + space.Name
	}
	return meta, ""
}

func (s *MemoryStore) packageOpeningBlockedReason(packageID, grade string) string {
	ids := s.learningSpaceIDsForPackage(packageID)
	if len(ids) == 0 {
		return "套餐没有可开通的课程范围"
	}
	for _, id := range ids {
		if _, reason := s.openingSubject(id, grade); reason != "" {
			return reason
		}
	}
	return ""
}

func sameContentTypes(left, right []string) bool {
	left, right = uniqueTrimmed(left), uniqueTrimmed(right)
	if len(left) != len(right) {
		return false
	}
	for _, code := range left {
		if !containsString(right, code) {
			return false
		}
	}
	return true
}

// Include future grants, but never let an expired or revoked grant be revived
// by an old client's unchanged selection list.
func (s *MemoryStore) protectedDirectGrant(studentID, spaceID string) (packageGrant, bool) {
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !isDirectGrantPackage(grant.PackageID) || grantPermissionState(grant) == "已到期" {
			continue
		}
		ids := s.learningSpaceIDsForPackage(grant.PackageID)
		if len(ids) == 1 && ids[0] == spaceID {
			return grant, true
		}
	}
	return packageGrant{}, false
}
