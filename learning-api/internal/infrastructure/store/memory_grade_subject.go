package store

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

const gradeSubjectCatalogSetting = "gradeSubjectCatalog"

func defaultGradeSubjectCatalog() []learning.GradeSubjectMetadata {
	items := make([]learning.GradeSubjectMetadata, 0)
	for gradeIndex, subjects := range demoGradeSubjectLevels {
		grade := demoGrades[gradeIndex]
		for subject, levels := range subjects {
			if len(levels) == 0 {
				continue
			}
			items = append(items, learning.GradeSubjectMetadata{
				ID:          "g" + itoa(gradeIndex+1) + "-" + subjectSlug(subject),
				GradeCode:   "G" + itoa(gradeIndex+1),
				Grade:       grade,
				Subject:     subject,
				DisplayName: subject,
				SortOrder:   subjectCatalogSort(subject),
				Status:      "启用",
			})
		}
	}
	sortGradeSubjects(items)
	return items
}

func subjectCatalogSort(subject string) int {
	for _, item := range defaultSubjectMetadata() {
		if item.Name == subject {
			return item.SortOrder
		}
	}
	return 999
}

func (s *MemoryStore) gradeSubjectCatalogUnlocked() []learning.GradeSubjectMetadata {
	raw := strings.TrimSpace(s.settings[gradeSubjectCatalogSetting])
	if raw == "" {
		return defaultGradeSubjectCatalog()
	}
	var items []learning.GradeSubjectMetadata
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return defaultGradeSubjectCatalog()
	}
	out := make([]learning.GradeSubjectMetadata, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Grade) == "" || strings.TrimSpace(item.Subject) == "" {
			continue
		}
		if item.DisplayName == "" {
			item.DisplayName = item.Subject
		}
		if item.GradeCode == "" {
			item.GradeCode = gradeCode(item.Grade)
		}
		if item.Status == "" {
			item.Status = "启用"
		}
		out = append(out, item)
	}
	sortGradeSubjects(out)
	return out
}

func sortGradeSubjects(items []learning.GradeSubjectMetadata) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].GradeCode != items[j].GradeCode {
			return items[i].GradeCode < items[j].GradeCode
		}
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].DisplayName < items[j].DisplayName
	})
}

func (s *MemoryStore) gradeSubjectsUnlocked() []learning.GradeSubjectMetadata {
	return s.gradeSubjectCatalogUnlocked()
}

func (s *MemoryStore) updateGradeSubjectsUnlocked(operator string, req learning.GradeSubjectCatalogUpdateRequest) ([]learning.GradeSubjectMetadata, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) ([]learning.GradeSubjectMetadata, error) {
			return work.updateGradeSubjectsUnlocked(operator, req)
		})
	}
	if len(req.Items) == 0 {
		return nil, errors.New("请至少保留一门年级学科")
	}
	seen := map[string]bool{}
	items := make([]learning.GradeSubjectMetadata, 0, len(req.Items))
	for _, item := range req.Items {
		item.ID = strings.TrimSpace(item.ID)
		item.Grade = strings.TrimSpace(item.Grade)
		item.Subject = strings.TrimSpace(item.Subject)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.ImageURL = strings.TrimSpace(item.ImageURL)
		item.Summary = strings.TrimSpace(item.Summary)
		item.PreviewCourseID = strings.TrimSpace(item.PreviewCourseID)
		if item.ID == "" || item.Grade == "" || item.Subject == "" {
			return nil, errors.New("年级、学科和目录编号不能为空")
		}
		if item.Status != "启用" && item.Status != "停用" {
			return nil, errors.New("目录状态只能为启用或停用")
		}
		key := item.Grade + "|" + item.Subject
		if seen[key] {
			return nil, errors.New("同一年级不能重复配置同一学科")
		}
		seen[key] = true
		if item.DisplayName == "" {
			item.DisplayName = item.Subject
		}
		item.GradeCode = gradeCode(item.Grade)
		items = append(items, item)
	}
	sortGradeSubjects(items)
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, errors.New("年级课程目录保存失败")
	}
	s.settings[gradeSubjectCatalogSetting] = string(raw)
	s.prependLog(operator, "更新年级课程目录", "已更新 "+itoa(len(items))+" 个年级学科")
	return items, nil
}

func (s *MemoryStore) studentSubjectCards(student learning.Student) []learning.StudentSubjectCard {
	cards := make([]learning.StudentSubjectCard, 0)
	for _, meta := range s.gradeSubjectCatalogUnlocked() {
		if meta.Grade != student.Grade || meta.Status != "启用" {
			continue
		}
		card := learning.StudentSubjectCard{GradeSubjectMetadata: meta, AccessState: "locked", AccessLabel: "暂未开通"}
		courseIDs := s.courseIDsForGradeSubject(meta.Grade, meta.Subject)
		fullMaterials, fullHomework := s.subjectContentCounts(student.ID, courseIDs)
		if s.hasActiveSubjectLearningAccess(student.ID, meta.Grade, meta.Subject) {
			card.AccessState, card.AccessLabel, card.CanOpen = "full", "可学习", true
			card.MaterialNum, card.HomeworkNum = fullMaterials, fullHomework
			card.EntryCourseID = s.firstAccessibleCourseID(student.ID, meta.Grade, meta.Subject)
		} else if len(courseIDs) == 0 {
			card.AccessState, card.AccessLabel = "pending", "内容准备中"
		}
		cards = append(cards, card)
	}
	return cards
}

func (s *MemoryStore) firstAccessibleCourseID(studentID, grade, subject string) string {
	for _, course := range s.coursesForStudent(studentID) {
		if course.Grade == grade && subjectsMatch(course.Subject, subject) {
			return course.ID
		}
	}
	return ""
}

func (s *MemoryStore) courseIDsForGradeSubject(grade, subject string) []string {
	ids := make([]string, 0)
	for _, course := range s.courses {
		if course.Status == learning.StatusEnabled && course.Grade == grade && subjectsMatch(course.Subject, subject) {
			ids = append(ids, course.ID)
		}
	}
	return ids
}

func (s *MemoryStore) hasActiveSubjectLearningAccess(studentID, grade, subject string) bool {
	return s.hasActiveSubjectContent(studentID, grade, subject)
}

func (s *MemoryStore) hasActiveSubjectContent(studentID, grade, subject string) bool {
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) {
			continue
		}
		for _, spaceID := range s.learningSpaceIDsForGrant(grant.ID) {
			if space, ok := s.findLearningSpace(spaceID); ok && space.Grade == grade && subjectsMatch(space.Subject, subject) &&
				(containsString(s.contentTypesForPackage(grant.PackageID), "handout") || containsString(s.contentTypesForPackage(grant.PackageID), "question")) {
				return true
			}
		}
	}
	return false
}

func (s *MemoryStore) subjectContentCounts(studentID string, courseIDs []string) (int, int) {
	ids := map[string]bool{}
	for _, id := range courseIDs {
		ids[id] = true
	}
	materialCount, homeworkCount := 0, 0
	for _, item := range s.materialsForStudent(studentID) {
		if ids[item.CourseID] {
			materialCount++
		}
	}
	for _, item := range s.homeworkForStudent(studentID) {
		if ids[item.CourseID] {
			homeworkCount++
		}
	}
	return materialCount, homeworkCount
}

func (s *MemoryStore) previewCourseForGradeSubject(studentID string, meta learning.GradeSubjectMetadata) (learning.Course, bool) {
	for _, course := range s.previewCoursesForStudent(studentID) {
		if course.Grade == meta.Grade && subjectsMatch(course.Subject, meta.Subject) && (meta.PreviewCourseID == "" || meta.PreviewCourseID == course.ID) {
			return course, true
		}
	}
	return learning.Course{}, false
}

func (s *MemoryStore) previewContentCounts(course learning.Course) (int, int) {
	lessonID, ok := s.previewLessonForCourse(course)
	if !ok {
		return 0, 0
	}
	materials, homework := 0, 0
	for _, item := range s.materials {
		if item.CourseID == course.ID && item.LessonID == lessonID && materialPublished(item.Status) {
			materials++
		}
	}
	for _, item := range s.homework {
		if item.CourseID == course.ID && item.LessonID == lessonID && homeworkVisible(item.Status) {
			homework++
		}
	}
	return materials, homework
}
