package store

import (
	"os"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func familyTestCurriculum(prefix string) []learning.CurriculumNode {
	return []learning.CurriculumNode{
		{ID: prefix + "-unit", Type: learning.CurriculumUnit, Name: "Unit 1", SortOrder: 1},
		{ID: prefix + "-chapter", ParentID: prefix + "-unit", Type: learning.CurriculumChapter, Name: "Chapter 1", SortOrder: 1},
		{ID: prefix + "-lesson", ParentID: prefix + "-chapter", Type: learning.CurriculumLesson, Name: "Lesson 1", SortOrder: 1},
	}
}

func TestCourseFamilyMySQLRestart(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated MySQL test database required")
	}
	s := NewMemoryStore()
	for index := range s.grants {
		if s.grants[index].OpenedAt == "" {
			s.grants[index].OpenedAt = "2026-09-27 09:00:00"
		}
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	admin, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	name := "共享目录重启测试 " + time.Now().Format("20060102150405.000000000")
	family, err := s.CreateCourseFamily("测试", admin, learning.CourseFamilyCreateRequest{Name: name, LearningSpaceIDs: []string{"space-g05-english-s1-q1-splus", "space-g05-english-s1-q1-h"}, Curriculum: familyTestCurriculum("mysql-family-" + time.Now().Format("150405"))})
	if err != nil {
		t.Fatal(err)
	}
	var storedFamilyID string
	if err := s.db.QueryRow(`SELECT id FROM course_families WHERE id = ?`, family.ID).Scan(&storedFamilyID); err != nil || storedFamilyID != family.ID {
		t.Fatalf("family row: %q %v", storedFamilyID, err)
	}
	var nodeCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM course_curriculum_nodes WHERE course_id IN (?, ?)`, family.Courses[0].ID, family.Courses[1].ID).Scan(&nodeCount); err != nil || nodeCount != 0 {
		t.Fatalf("shared nodes duplicated: %d %v", nodeCount, err)
	}
	_ = s.db.Close()
	restored := NewMemoryStore()
	if err := restored.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer restored.db.Close()
	admin, err = restored.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range restored.CourseFamilies(admin) {
		if item.ID == family.ID {
			if len(item.Courses) != 2 || item.Courses[0].Curriculum[2].ID != item.Courses[1].Curriculum[2].ID {
				t.Fatalf("restored family = %#v", item)
			}
			return
		}
	}
	t.Fatal("family missing after restart")
}

func TestCourseFamilySharesCurriculumAcrossDynamicLevels(t *testing.T) {
	s := NewMemoryStore()
	admin, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	family, err := s.CreateCourseFamily("超级管理员", admin, learning.CourseFamilyCreateRequest{
		Name: "五年级英文阅读系列", LearningSpaceIDs: []string{"space-g05-english-s1-q1-splus", "space-g05-english-s1-q1-h"}, Curriculum: familyTestCurriculum("shared"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(family.Courses) != 2 || family.Courses[0].Curriculum[2].ID != family.Courses[1].Curriculum[2].ID {
		t.Fatalf("curriculum not shared: %#v", family)
	}
	added, err := s.AddCourseFamilyCourse("超级管理员", admin, family.ID, learning.CourseFamilyAddCourseRequest{LearningSpaceID: "space-g05-english-s1-q1"})
	if err != nil {
		t.Fatal(err)
	}
	if added.Curriculum[2].ID != family.Curriculum[2].ID {
		t.Fatal("added level did not reuse the shared lesson")
	}
	if _, err := s.AddCourseFamilyCourse("超级管理员", admin, family.ID, learning.CourseFamilyAddCourseRequest{LearningSpaceID: "space-g05-english-s1-q1"}); err == nil {
		t.Fatal("duplicate level accepted")
	}
	rows := s.CourseFamilies(admin)
	if len(rows) != 1 || len(rows[0].Courses) != 3 {
		t.Fatalf("family list = %#v", rows)
	}

	// The same lesson ID is safe: materials remain scoped by course ID.
	s.materials = append(s.materials, learning.Material{ID: "family-splus-material", CourseID: family.Courses[0].ID, LessonID: family.Curriculum[2].ID, Title: "S+ 专属讲义"})
	updated := familyTestCurriculum("shared")
	updated[2].Name = "Lesson 1 revised"
	result, err := s.UpdateCourseFamily("超级管理员", admin, family.ID, learning.CourseFamilyUpdateRequest{Name: family.Name, Curriculum: updated})
	if err != nil {
		t.Fatal(err)
	}
	if result.Curriculum[2].Name != "Lesson 1 revised" {
		t.Fatalf("update = %#v", result)
	}
	for _, course := range s.Courses(admin) {
		if course.FamilyID == family.ID && course.Curriculum[2].Name != "Lesson 1 revised" {
			t.Fatalf("stale member curriculum: %#v", course)
		}
	}
	if s.materials[len(s.materials)-1].Curriculum.Lesson != "Lesson 1 revised" {
		t.Fatal("material path did not follow shared curriculum")
	}
	_, err = s.UpdateCourseFamily("超级管理员", admin, family.ID, learning.CourseFamilyUpdateRequest{Name: family.Name, Curriculum: updated[:2]})
	if err == nil || !strings.Contains(err.Error(), "讲义") {
		t.Fatalf("bound lesson removal should fail, got %v", err)
	}
}

func TestCourseFamilyImportRetainsCoursesAndMapsExistingContent(t *testing.T) {
	s := NewMemoryStore()
	admin, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.CreateCourse("超级管理员", admin, learning.CourseUpsertRequest{Name: "导入测试 S", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: familyTestCurriculum("one")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateCourse("超级管理员", admin, learning.CourseUpsertRequest{Name: "导入测试 S+", LearningSpaceID: "space-g05-english-s1-q1-splus", Curriculum: familyTestCurriculum("two")})
	if err != nil {
		t.Fatal(err)
	}
	s.materials = append(s.materials, learning.Material{ID: "import-material", CourseID: second.ID, LessonID: "two-lesson", Title: "S+ 讲义"})
	s.homework = append(s.homework, learning.Homework{ID: "import-homework", CourseID: second.ID, LessonID: "two-lesson", Title: "S+ 练习"})
	family, err := s.ImportCourseFamily("超级管理员", admin, learning.CourseFamilyImportRequest{Name: "导入测试系列", CourseIDs: []string{first.ID, second.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(family.Courses) != 2 || family.Courses[1].ID != second.ID || s.materials[len(s.materials)-1].LessonID != "one-lesson" || s.homework[len(s.homework)-1].LessonID != "one-lesson" {
		t.Fatalf("import did not retain IDs/bindings: %#v", family)
	}
	if s.materials[len(s.materials)-1].CourseID != second.ID {
		t.Fatal("material course ownership changed")
	}

	third, err := s.CreateCourse("超级管理员", admin, learning.CourseUpsertRequest{Name: "导入测试 H", LearningSpaceID: "space-g05-english-s1-q1-h", Curriculum: []learning.CurriculumNode{{ID: "other-unit", Type: learning.CurriculumUnit, Name: "Other Unit", SortOrder: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ImportCourseFamily("超级管理员", admin, learning.CourseFamilyImportRequest{Name: "应拒绝", CourseIDs: []string{second.ID, third.ID}})
	if err == nil {
		t.Fatal("mismatched or already linked course accepted")
	}
}
