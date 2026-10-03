package store

import (
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

func TestDisabledTeacherPreservesHistoryAndAllowsExplicitFutureHandoff(t *testing.T) {
	s, _, series := seedWeeklySeries(t)
	p, _ := s.PrincipalByUserID("user-super")
	lessons := seriesLessons(s, series)
	teacher, _ := s.PrincipalByUserID("user-teacher")
	before := append([]learning.ScheduleClass(nil), lessons...)
	studentBefore := append([]learning.Student(nil), s.students...)
	disabled, err := s.UpdateTeacher("教务", p, teacher.UserID, learning.TeacherUpsertRequest{Name: teacher.Name, Phone: teacher.Phone, CampusID: teacher.CampusID, LearningSpaceIDs: teacher.LearningSpaceIDs, CanUploadHandout: teacher.CanUploadHandout, CanReview: teacher.CanReview, AccountStatus: "停用"})
	if err != nil {
		t.Fatal(err)
	}
	if disabled.AccountStatus != "停用" || !reflect.DeepEqual(before, seriesLessons(s, series)) || !reflect.DeepEqual(studentBefore, s.students) {
		t.Fatal("disable rewrote lessons or student data")
	}
	if _, err := s.PrincipalByUserID(teacher.UserID); err == nil {
		t.Fatal("disabled account still resolved")
	}
	req := lessonUpdateRequest(lessons[0])
	req.StartDate = seriesDatePlusDays(5, 1)
	req.IgnoreWarnings = true
	if _, err := s.CreateScheduleClass("教务", p, req); err == nil || !strings.Contains(err.Error(), "停用") {
		t.Fatalf("new class allowed: %v", err)
	}
	replacement, err := s.CreateTeacher("教务", p, learning.TeacherUpsertRequest{Name: "接课老师", Phone: "13977889911", CampusID: teacher.CampusID, LearningSpaceIDs: teacher.LearningSpaceIDs})
	if err != nil {
		t.Fatal(err)
	}
	req = lessonUpdateRequest(lessons[1])
	req.TeacherID = replacement.ID
	req.EditScope = learning.EditScopeThisAndFuture
	req.IgnoreWarnings = true
	preview, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ID: lessons[1].ID, ScheduleClassCreateRequest: req})
	if err != nil || !preview.CanSave {
		t.Fatalf("handoff preview: %#v %v", preview, err)
	}
	if _, err := s.UpdateScheduleClass("教务", p, lessons[1].ID, req); err != nil {
		t.Fatal(err)
	}
	after := seriesLessons(s, series)
	if after[0].TeacherID != teacher.UserID || after[0].ID != before[0].ID {
		t.Fatal("handoff changed earlier lesson ownership")
	}
	for i := 1; i < len(after); i++ {
		if after[i].TeacherID != replacement.ID || after[i].ID != before[i].ID || after[i].LessonDate != before[i].LessonDate {
			t.Fatalf("future handoff lost lesson identity: %#v", after[i])
		}
	}
	if len(s.students) != len(studentBefore) {
		t.Fatal("handoff changed students")
	}
}

func TestTeacherOutstandingClassCountUsesIndividualLessonDate(t *testing.T) {
	s := NewMemoryStore()
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	s.scheduleClasses = []learning.ScheduleClass{
		{TeacherID: "teacher", LessonDate: yesterday, EndDate: tomorrow},
		{TeacherID: "teacher", LessonDate: tomorrow, EndDate: tomorrow},
		{TeacherID: "teacher", LessonDate: tomorrow, Status: "已取消"},
		{TeacherID: "teacher", LessonDate: today, EndTime: "00:00"},
		{TeacherID: "teacher", EndDate: yesterday},
		{TeacherID: "teacher", EndDate: tomorrow},
		{TeacherID: "other", LessonDate: tomorrow},
	}
	if count := s.activeClassCountForTeacher("teacher"); count != 2 {
		t.Fatalf("wrong outstanding count: %d", count)
	}
}
