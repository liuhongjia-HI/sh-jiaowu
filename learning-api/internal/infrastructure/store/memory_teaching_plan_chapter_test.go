package store

import (
	"os"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func chapterPlanFixture(t *testing.T, s *MemoryStore) (learning.Principal, learning.Course, learning.TeachingPlan) {
	t.Helper()
	p, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	spaceID := ""
	for _, space := range s.learningSpaces {
		if space.Grade == "五年级" && subjectsMatch(space.Subject, "english") && space.Status == learning.StatusEnabled {
			spaceID = space.ID
			break
		}
	}
	stamp := time.Now().Format("150405.000000000")
	course, err := s.CreateCourse("管理员", p, learning.CourseUpsertRequest{Name: "教案章节验收" + stamp, LearningSpaceID: spaceID, Status: learning.StatusEnabled, Curriculum: []learning.CurriculumNode{{ID: "plan-test-u1-" + stamp, Type: learning.CurriculumUnit, Name: "第一课", SortOrder: 1}, {ID: "plan-test-u2-" + stamp, Type: learning.CurriculumUnit, Name: "第二课", SortOrder: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreateTeachingPlan("管理员", p, learning.TeachingPlanUploadRequest{Grade: course.Grade, Subject: course.Subject, CourseID: course.ID, LessonID: course.Curriculum[0].ID, File: learning.FileAsset{ID: "plan-chapter-file-" + stamp, FileName: "备课.pdf", OriginalPath: "/tmp/plan-chapter-test.pdf", PreviewStatus: "待转换"}})
	if err != nil {
		t.Fatal(err)
	}
	return p, course, plan
}

func TestTeachingPlanChapterRenameProtectionAndReassignment(t *testing.T) {
	s := NewMemoryStore()
	p, course, plan := chapterPlanFixture(t, s)
	if plan.Chapter == "" || plan.Semester == "" || plan.Phase == "" {
		t.Fatalf("missing scope/path: %#v", plan)
	}
	course.Curriculum[0].Name = "新版第一课"
	req := learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: course.LearningSpaceID, Status: course.Status, Curriculum: course.Curriculum}
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.TeachingPlan(p, plan.ID)
	if !strings.Contains(updated.Chapter, "新版第一课") {
		t.Fatalf("renamed path stale: %#v", updated)
	}
	req.Curriculum = req.Curriculum[1:]
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err == nil || !strings.Contains(err.Error(), "教案") {
		t.Fatalf("referenced chapter removed: %v", err)
	}
	if err := s.DeleteCourse("管理员", p, course.ID); err == nil || !strings.Contains(err.Error(), "教案") {
		t.Fatalf("referenced directory removed: %v", err)
	}
	reader, _ := s.PrincipalByUserID("user-teacher")
	reader.CanUploadHandout = false
	reader.TeacherLibrary = &learning.TeacherLibraryPolicy{}
	if _, err := s.UpdateTeachingPlanChapter("只读教师", reader, plan.ID, learning.TeachingPlanChapterRequest{}); err == nil {
		t.Fatal("read-only teacher changed chapter")
	}
	if _, err := s.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{CourseID: course.ID, LessonID: course.Curriculum[1].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err != nil {
		t.Fatalf("unused chapter cannot be removed after reassignment: %v", err)
	}
	if _, err := s.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCourse("管理员", p, course.ID); err != nil {
		t.Fatal(err)
	}
	legacy, err := s.TeachingPlan(p, plan.ID)
	if err != nil || legacy.CourseID != "" || legacy.Chapter != "" {
		t.Fatalf("unclassified plan lost: %#v %v", legacy, err)
	}
}

func TestTeachingPlanChapterMySQLReload(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	s := NewMemoryStore()
	for i := range s.grants {
		s.grants[i].OpenedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	p, course, plan := chapterPlanFixture(t, s)
	restart := NewMemoryStore()
	if err := restart.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer restart.db.Close()
	reloaded, err := restart.TeachingPlan(p, plan.ID)
	if err != nil || reloaded.CourseID != plan.CourseID || reloaded.LessonID != plan.LessonID || reloaded.Semester != plan.Semester || reloaded.Phase != plan.Phase || reloaded.Chapter != plan.Chapter {
		t.Fatalf("chapter scope did not persist: %#v %v", reloaded, err)
	}
	if _, err := restart.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{CourseID: course.ID, LessonID: course.Curriculum[1].ID}); err != nil {
		t.Fatal(err)
	}
	if err := restart.loadTeachingPlansFromDB(); err != nil {
		t.Fatal(err)
	}
	reassigned, err := restart.TeachingPlan(p, plan.ID)
	if err != nil || reassigned.LessonID != course.Curriculum[1].ID {
		t.Fatalf("chapter reassignment did not persist: %#v %v", reassigned, err)
	}
}
