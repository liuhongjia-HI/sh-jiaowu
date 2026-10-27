package store

import (
	"errors"
	"fmt"
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
	if target == nil && !req.IncludeAll && len(nodes) == 0 || len(nodes) > 2000 {
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
	submissionCounts := map[string]int{}
	for _, submission := range s.submissions {
		submissionCounts[submission.HomeworkID]++
	}

	add := func(id, kind, title, courseID, lessonID, status string, blocking bool) {
		item, exists := courses[courseID]
		if !exists || !(target != nil || req.IncludeAll || nodes[lessonID]) {
			return
		}
		path, invalid := "未关联目录", false
		if lessonID != "" {
			path, invalid = referenceNodePath(item.Curriculum, lessonID)
		}
		count := 0
		if kind == "练习" {
			count = submissionCounts[id]
		}
		if status == "" {
			status = "未标注"
		}
		out = append(out, learning.CurriculumReference{ID: id, Kind: kind, Title: title, CourseID: courseID, CourseName: item.Name, LessonID: lessonID, Status: status, Path: path, InvalidBinding: invalid, SubmissionCount: count, Blocking: blocking})
	}
	for _, item := range s.teachingPlans {
		blocking := target == nil || target.Grade != item.Grade || !subjectsMatch(target.Subject, item.Subject) || target.Semester != item.Semester || target.Phase != item.Phase
		add(item.ID, "教案", item.Title, item.CourseID, item.LessonID, "已上传", blocking)
	}
	for _, item := range s.materials {
		add(item.ID, "讲义", item.Title, item.CourseID, item.LessonID, string(item.Status), target == nil)
	}
	for _, item := range s.homework {
		add(item.ID, "练习", item.Title, item.CourseID, item.LessonID, item.Status, target == nil)
	}
	return out, nil
}

func referenceNodePath(nodes []learning.CurriculumNode, id string) (string, bool) {
	byID := map[string]learning.CurriculumNode{}
	for _, node := range nodes {
		byID[node.ID] = node
	}
	parts, seen := []string{}, map[string]bool{}
	current := id
	for current != "" {
		node, ok := byID[current]
		if !ok || seen[current] {
			return "目录关联已失效（" + id + "）", true
		}
		seen[current] = true
		label := strings.TrimSpace(node.Name)
		if label == "" {
			label = fmt.Sprintf("%s %d", node.Type, node.SortOrder)
		}
		parts = append([]string{label}, parts...)
		current = node.ParentID
	}
	return strings.Join(parts, " / "), !curriculumLeaf(nodes, id)
}

// Called only on an isolated mutation copy. Save validation rejects references
// added after preview, and any failure discards all moves and directory changes.
func (s *MemoryStore) moveCurriculumReferences(p learning.Principal, courses map[string]learning.Course, moves []learning.CurriculumReferenceMove) error {
	if len(moves) > 10000 {
		return errors.New("一次迁移的资料过多，请分批处理")
	}
	seen := map[string]bool{}
	for _, move := range moves {
		course, ok := courses[move.CourseID]
		if !ok || !canSeeCourse(p, course) {
			return errors.New("不能迁移其他课程或未负责班型的资料")
		}
		targetExists := false
		for _, node := range course.Curriculum {
			if node.ID == move.ToLessonID {
				targetExists = true
			}
		}
		if !targetExists || !curriculumLeaf(course.Curriculum, move.ToLessonID) || move.FromLessonID == move.ToLessonID {
			return errors.New("迁移目标必须是保留的叶子课节，且不能与原课节相同")
		}
		key := move.Kind + ":" + move.ID
		if seen[key] {
			return errors.New("同一资料不能重复迁移")
		}
		seen[key] = true
		found := false
		switch move.Kind {
		case "讲义":
			if !canUploadHandout(p) {
				return errors.New("当前账号没有迁移讲义权限")
			}
			for i := range s.materials {
				item := &s.materials[i]
				if item.ID == move.ID && item.CourseID == move.CourseID && item.LessonID == move.FromLessonID {
					item.LessonID = move.ToLessonID
					found = true
					break
				}
			}
		case "练习":
			if !canUploadQuestion(p) {
				return errors.New("当前账号没有迁移练习权限")
			}
			for i := range s.homework {
				item := &s.homework[i]
				if item.ID == move.ID && item.CourseID == move.CourseID && item.LessonID == move.FromLessonID {
					item.LessonID = move.ToLessonID
					found = true
					break
				}
			}
		case "教案":
			if !s.canUploadPlan(p, course.Grade, course.Subject) {
				return errors.New("当前账号没有迁移教案权限")
			}
			for i := range s.teachingPlans {
				item := &s.teachingPlans[i]
				if item.ID == move.ID && item.CourseID == move.CourseID && item.LessonID == move.FromLessonID {
					item.LessonID = move.ToLessonID
					path, _ := curriculumPathForLesson(course, move.ToLessonID)
					item.Chapter = planChapterLabel(path)
					found = true
					break
				}
			}
		default:
			return errors.New("不支持的关联资料类型")
		}
		if !found {
			return fmt.Errorf("%s关联已变化或资料已删除，请刷新后重新选择迁移目标", move.Kind)
		}
	}
	return nil
}

func referenceMovesAudit(moves []learning.CurriculumReferenceMove) string {
	if len(moves) == 0 {
		return ""
	}
	return "; 资料迁移：" + mustJSON(moves)
}
