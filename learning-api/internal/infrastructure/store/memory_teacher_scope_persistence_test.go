package store

import (
	"os"
	"reflect"
	"sort"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

func TestTeacherScopeMySQLGroupedReadAndLegacyPrecision(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated teacher scope database required")
	}
	if !strings.Contains(dsn, "/starline_teacher_scope_test?") {
		t.Fatal("requires dedicated starline_teacher_scope_test database")
	}
	s := NewMemoryStore()
	for i := range s.grants {
		s.grants[i].OpenedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	reload := func() {
		t.Helper()
		if err := s.db.Close(); err != nil {
			t.Fatal(err)
		}
		s = NewMemoryStore()
		if err := s.ConnectDatabase(dsn); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { s.db.Close() }()
	admin, _ := s.PrincipalByUserID("user-super")
	teaching := []string{}
	for _, space := range s.learningSpaces {
		if space.Status == learning.StatusEnabled && space.Grade == "五年级" && (subjectsMatch(space.Subject, "english") || subjectsMatch(space.Subject, "math")) {
			teaching = append(teaching, space.ID)
		}
	}
	sort.Strings(teaching)
	if len(teaching) < 3 {
		t.Fatal("fixture lacks multiple teaching stages")
	}
	req := learning.TeacherUpsertRequest{Name: "范围持久化老师", Phone: "18000011114", LearningSpaceIDs: teaching, TeacherLibrary: &learning.TeacherLibraryPolicy{SpaceIDs: teaching, Scopes: []learning.TeacherLibraryScope{{Grade: "六年级", Subject: "english"}}, CanManageCourses: true, CanDownload: false}, CanUploadHandout: true}
	teacher, err := s.CreateTeacher("test", admin, req)
	if err != nil {
		t.Fatal(err)
	}
	id := teacher.ID
	expectedPolicy := cloneTeacherLibrary(teacher.TeacherLibrary)
	reload()
	p, err := s.PrincipalByUserID(id)
	actualIDs := append([]string(nil), p.LearningSpaceIDs...)
	sort.Strings(actualIDs)
	if err != nil || !reflect.DeepEqual(actualIDs, teaching) || !reflect.DeepEqual(p.TeacherLibrary, expectedPolicy) || !p.CanUploadHandout || p.CanUploadQuestion || p.CanReview {
		t.Fatal("grouped teaching/read permissions or capabilities changed after reload")
	}
	if !s.canViewPlan(p, "六年级", "english") || s.canUploadPlan(p, "六年级", "english") {
		t.Fatal("extra read became upload or lost read access")
	}
	// Even with an explicit course-maintenance capability, extra read is not a teaching grant.
	var extraID string
	for _, space := range s.learningSpaces {
		if space.Grade == "六年级" && subjectsMatch(space.Subject, "english") && space.Status == learning.StatusEnabled {
			extraID = space.ID
			break
		}
	}
	if extraID == "" {
		t.Fatal("fixture missing extra-read space")
	}
	if _, err = s.CreateCourse("test", p, learning.CourseUpsertRequest{Name: "额外范围不得创建", LearningSpaceID: extraID, Curriculum: []learning.CurriculumNode{{ID: "extra-u1", Type: learning.CurriculumUnit, Name: "第一课", SortOrder: 1}}}); err == nil {
		t.Fatal("extra read granted course maintenance")
	}
	// The same scope update used by inline scheduling must persist alongside the resulting lesson.
	for _, space := range s.learningSpaces {
		if space.Grade == "六年级" && subjectsMatch(space.Subject, "english") && space.Status == learning.StatusEnabled {
			req.LearningSpaceIDs = appendUnique(req.LearningSpaceIDs, space.ID)
			req.TeacherLibrary.SpaceIDs = appendUnique(req.TeacherLibrary.SpaceIDs, space.ID)
		}
	}
	if _, err = s.UpdateTeacher("排课就地范围", admin, id, req); err != nil {
		t.Fatal(err)
	}
	lessonReq := teacherLessonRequest()
	lessonReq.TeacherID = id
	lessonReq.StartDate = time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	lessonReq.ExpectedStudentCount = 0
	lessonReq.IgnoreWarnings = true
	lesson, err := s.CreateScheduleClass("test", admin, lessonReq)
	if err != nil {
		t.Fatal(err)
	}
	assertLesson := func() {
		t.Helper()
		for _, current := range s.ScheduleClasses(admin) {
			if current.ID != lesson.ID {
				continue
			}
			if current.TeacherID != id || current.CourseID != lessonReq.CourseID || current.LessonDate != lessonReq.StartDate || current.StartTime != lessonReq.StartTime || current.EndTime != lessonReq.EndTime || current.ExpectedStudentCount != 1 || len(current.Students) != 1 || current.Students[0].ID != "stu-001" {
				t.Fatal("scope edit and schedule draft did not survive together")
			}
			return
		}
		t.Fatal("created lesson missing after reload")
	}
	reload()
	expanded, err := s.PrincipalByUserID(id)
	if err != nil || !containsString(expanded.LearningSpaceIDs, extraID) || expanded.TeacherLibrary.CanDownload {
		t.Fatal("inline scope/capabilities lost after reload")
	}
	assertLesson()
	// Saving a legacy precise stage must not expand it to the other stages.
	req.LearningSpaceIDs = []string{"space-g05-english-s1-q1"}
	req.TeacherLibrary = &learning.TeacherLibraryPolicy{SpaceIDs: []string{"space-g05-english-s1-q1"}, Scopes: []learning.TeacherLibraryScope{{Grade: "六年级", Subject: "english"}}, CanManageCourses: true, CanDownload: false}
	updated, err := s.UpdateTeacher("test", admin, id, req)
	if err != nil {
		t.Fatal(err)
	}
	expectedPolicy = cloneTeacherLibrary(updated.TeacherLibrary)
	reload()
	p, err = s.PrincipalByUserID(id)
	if err != nil || !reflect.DeepEqual(p.LearningSpaceIDs, req.LearningSpaceIDs) || !reflect.DeepEqual(p.TeacherLibrary, expectedPolicy) {
		t.Fatal("precise stage expanded or reading configuration changed on reload")
	}
	if s.canUploadPlan(p, "五年级", "math") {
		t.Fatal("removed math teaching grant remained active")
	}
	// Force an update failure and verify both memory and a new connection retain the old policy.
	if _, err = s.db.Exec("CREATE TRIGGER teacher_scope_fail BEFORE INSERT ON teacher_learning_space_access FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced teacher update failure'"); err != nil {
		t.Fatal(err)
	}
	req.LearningSpaceIDs = teaching
	req.Name = "失败不得保存"
	_, err = s.UpdateTeacher("test", admin, id, req)
	if _, dropErr := s.db.Exec("DROP TRIGGER teacher_scope_fail"); dropErr != nil {
		t.Fatal(dropErr)
	}
	if err == nil {
		t.Fatal("forced persistence error was accepted")
	}
	current, _ := s.PrincipalByUserID(id)
	if !reflect.DeepEqual(current.LearningSpaceIDs, []string{"space-g05-english-s1-q1"}) || current.Name != teacher.Name {
		t.Fatal("failed save contaminated memory")
	}
	reload()
	assertLesson()
	current, _ = s.PrincipalByUserID(id)
	if !reflect.DeepEqual(current.LearningSpaceIDs, []string{"space-g05-english-s1-q1"}) || current.TeacherLibrary.CanDownload || current.Name != teacher.Name {
		t.Fatal("failed save persisted widened grant")
	}
}
