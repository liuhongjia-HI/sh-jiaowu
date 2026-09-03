package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestFirstPendingLessonPerCourseAdvancesInCurriculumOrder(t *testing.T) {
	store := NewMemoryStore()
	store.courses = []learning.Course{{
		ID: "course-1",
		Curriculum: []learning.CurriculumNode{
			{ID: "lesson-111", Type: learning.CurriculumLesson, Name: "1.1.1", SortOrder: 1},
			{ID: "lesson-112", Type: learning.CurriculumLesson, Name: "1.1.2", SortOrder: 2},
			{ID: "lesson-121", Type: learning.CurriculumLesson, Name: "1.2.1", SortOrder: 3},
		},
	}}
	items := []learning.Homework{
		{ID: "hw-112", CourseID: "course-1", LessonID: "lesson-112", SortOrder: 2},
		{ID: "hw-111", CourseID: "course-1", LessonID: "lesson-111", SortOrder: 1},
		{ID: "hw-121", CourseID: "course-1", LessonID: "lesson-121", SortOrder: 3},
	}
	got := store.firstPendingLessonPerCourse(items)
	if len(got) != 1 || got[0].ID != "hw-111" {
		t.Fatalf("expected first lesson homework, got %#v", got)
	}
	got = store.firstPendingLessonPerCourse(items[:1])
	if len(got) != 1 || got[0].ID != "hw-112" {
		t.Fatalf("expected next lesson homework after first completion, got %#v", got)
	}
}
