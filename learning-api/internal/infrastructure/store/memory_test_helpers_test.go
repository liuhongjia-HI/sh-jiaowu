package store

import (
	"starline/learning-api/internal/domain/learning"
)

func firstLessonID(course learning.Course) string {
	for _, node := range course.Curriculum {
		if node.Type == learning.CurriculumLesson {
			return node.ID
		}
	}
	return ""
}

func testCurriculum(prefix string) []learning.CurriculumNode {
	return []learning.CurriculumNode{
		{ID: prefix + "-unit", Type: learning.CurriculumUnit, Name: "Unit 1"},
		{ID: prefix + "-chapter", ParentID: prefix + "-unit", Type: learning.CurriculumChapter, Name: "Chapter 1"},
		{ID: prefix + "-lesson", ParentID: prefix + "-chapter", Type: learning.CurriculumLesson, Name: "Lesson 1"},
	}
}

func noticeListContains(notices []learning.Notice, id string) bool {
	for _, notice := range notices {
		if notice.ID == id {
			return true
		}
	}
	return false
}

func findStoreStudentTodo(todos []learning.StudentTodo, todoType string) (learning.StudentTodo, bool) {
	for _, todo := range todos {
		if todo.Type == todoType {
			return todo, true
		}
	}
	return learning.StudentTodo{}, false
}

func favoriteListContains(favorites []learning.Favorite, id string) bool {
	for _, favorite := range favorites {
		if favorite.ID == id {
			return true
		}
	}
	return false
}

func learningRecordContains(records []learning.StudentLearningRecord, id string) bool {
	for _, record := range records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func materialVisible(items []learning.Material, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func materialHasLearningDimensions(items []learning.Material, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return item.Grade != "" && item.Semester != "" && item.Subject != ""
		}
	}
	return false
}

func enabledPackageIDForGrade(store *MemoryStore, grade string) string {
	for _, pkg := range store.packages {
		if pkg.Grade == grade && pkg.Status == learning.StatusEnabled {
			return pkg.ID
		}
	}
	return ""
}

// TestStudentAverageScoreIsDerivedFromGradedSubmissions 锁定“平均分”不再是一个
// 可以手工填、永远不更新的静态字段：没有任何已批改记录时诚实显示 0（前端据此展示
// “完成后生成”），批改后自动反映真实分数，不需要任何人手工同步。
