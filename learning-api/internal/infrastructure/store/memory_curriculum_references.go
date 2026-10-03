package store

import (
	"errors"
	"starline/learning-api/internal/domain/learning"
	"strings"
)

// Shared directories affect every class type, including unpublished resources.
// This is a read-only preview; the eventual save still checks current bindings.
func (s *MemoryStore) CurriculumReferences(p learning.Principal, courseID string, req learning.CurriculumReferencesRequest) ([]learning.CurriculumReference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	course, ok := s.findCourse(strings.TrimSpace(courseID))
	if !ok || !p.CanMaintainCourses() || !canSeeCourse(p, course) {
		return nil, errors.New("没有权限查看该课程的目录引用")
	}
	nodes := map[string]bool{}
	var target *learningSpace
	if targetID := strings.TrimSpace(req.TargetLearningSpaceID); targetID != "" {
		space, exists := s.findLearningSpace(targetID)
		candidate := course
		candidate.LearningSpaceID = targetID
		candidate.Grade, candidate.Subject = space.Grade, space.Subject
		if !exists || space.Status != learning.StatusEnabled || !canSeeCourse(p, candidate) {
			return nil, errors.New("目标教学范围不可维护")
		}
		if course.FamilyID != "" {
			return nil, errors.New("共享目录课程不能单独切换范围")
		}
		target = &space
	}
	for _, id := range req.NodeIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			nodes[id] = true
		}
	}
	if target == nil && len(nodes) == 0 || len(nodes) > 2000 {
		return nil, errors.New("请选择有效目录节点")
	}
	courses := map[string]learning.Course{course.ID: course}
	if course.FamilyID != "" {
		for _, item := range s.courses {
			if item.FamilyID == course.FamilyID {
				if !canSeeCourse(p, item) {
					return nil, errors.New("当前账号不能查看该系列的全部班型")
				}
				courses[item.ID] = item
			}
		}
	}
	out := []learning.CurriculumReference{}
	add := func(id, kind, title, courseID, lessonID string, blocking bool) {
		if item, exists := courses[courseID]; exists && (target != nil || nodes[lessonID]) {
			out = append(out, learning.CurriculumReference{ID: id, Kind: kind, Title: title, CourseID: courseID, CourseName: item.Name, LessonID: lessonID, Blocking: blocking})
		}
	}
	for _, item := range s.teachingPlans {
		blocking := target == nil || target.Grade != item.Grade || !subjectsMatch(target.Subject, item.Subject) || target.Semester != item.Semester || target.Phase != item.Phase
		add(item.ID, "教案", item.Title, item.CourseID, item.LessonID, blocking)
	}
	for _, item := range s.materials {
		add(item.ID, "讲义", item.Title, item.CourseID, item.LessonID, target == nil)
	}
	for _, item := range s.homework {
		add(item.ID, "练习", item.Title, item.CourseID, item.LessonID, target == nil)
	}
	return out, nil
}
