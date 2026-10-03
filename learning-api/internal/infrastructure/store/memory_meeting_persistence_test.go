package store

import (
	"os"
	"reflect"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestMeetingSchedulingAndDisabledTeacherMySQLReload(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated test database required")
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
	p, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	req := teacherLessonRequest()
	req.IgnoreWarnings = true
	date := func(days int) string { return time.Now().AddDate(0, 0, days).Format("2006-01-02") }
	req.StartDate = date(30)
	req.Repeat = &learning.ScheduleRepeat{Freq: "custom", Dates: []learning.ScheduleCustomDate{
		{Date: date(35), StartTime: "14:00", EndTime: "15:00"},
		{Date: date(32)},
	}}
	first, err := s.CreateScheduleClass("教务", p, req)
	if err != nil {
		t.Fatal(err)
	}
	original := seriesLessons(s, first.SeriesID)
	assertLessons := func(expected []learning.ScheduleClass) {
		t.Helper()
		actual := seriesLessons(s, first.SeriesID)
		if len(actual) != len(expected) {
			t.Fatalf("lesson count changed: %d != %d", len(actual), len(expected))
		}
		for i, want := range expected {
			got := actual[i]
			if got.ID != want.ID || got.SeriesID != want.SeriesID || got.LessonDate != want.LessonDate || got.StartTime != want.StartTime || got.EndTime != want.EndTime || got.DurationMinutes != want.DurationMinutes || got.Status != want.Status || got.AuditStatus != want.AuditStatus || got.Detached != want.Detached || got.CreatedAt != want.CreatedAt || !reflect.DeepEqual(got.Students, want.Students) {
				t.Fatalf("lesson identity/time/status/student data changed on reload: %#v != %#v", got, want)
			}
		}
	}
	reload()
	assertLessons(original)
	if len(original) != 3 || original[1].StartTime != "19:00" || original[2].StartTime != "14:00" || original[2].DurationMinutes != 60 {
		t.Fatalf("custom date overrides incorrect: %#v", original)
	}
	// Cancel two lessons, then restore only one: the second must remain cancelled.
	for _, lesson := range original[:2] {
		if _, err := s.CancelScheduleClassScope("教务", p, lesson.ID, learning.EditScopeThis); err != nil {
			t.Fatal(err)
		}
	}
	cancelled := seriesLessons(s, first.SeriesID)
	reload()
	assertLessons(cancelled)
	if _, err := s.RestoreScheduleClass("教务", p, original[0].ID, true); err != nil {
		t.Fatal(err)
	}
	restored := seriesLessons(s, first.SeriesID)
	if restored[0].ID != original[0].ID || restored[0].Status != original[0].Status || restored[1].Status != "已取消" {
		t.Fatalf("single restore changed cancelled sibling: %#v", restored)
	}
	reload()
	assertLessons(restored)
	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	update := learning.TeacherUpsertRequest{Name: teacher.Name, Phone: teacher.Phone, CampusID: teacher.CampusID, LearningSpaceIDs: teacher.LearningSpaceIDs, TeacherLibrary: teacher.TeacherLibrary, CanUploadHandout: teacher.CanUploadHandout, CanUploadQuestion: teacher.CanUploadQuestion, CanReview: teacher.CanReview, AccountStatus: "停用"}
	if _, err := s.UpdateTeacher("教务", p, teacher.UserID, update); err != nil {
		t.Fatal(err)
	}
	reload()
	assertLessons(restored)
	if _, err := s.PrincipalByUserID(teacher.UserID); err == nil {
		t.Fatal("disabled teacher became available after reload")
	}
	for _, user := range s.users {
		if user.ID == teacher.UserID && (user.TokenVersion != teacher.TokenVersion+1 || user.AccountStatus != "停用") {
			t.Fatal("revocation version/status did not persist")
		}
	}
	update.AccountStatus = "正常"
	if _, err := s.UpdateTeacher("教务", p, teacher.UserID, update); err != nil {
		t.Fatal(err)
	}
	reload()
	assertLessons(restored)
	enabled, err := s.PrincipalByUserID(teacher.UserID)
	if err != nil || enabled.TokenVersion != teacher.TokenVersion+1 || !reflect.DeepEqual(enabled.LearningSpaceIDs, teacher.LearningSpaceIDs) {
		t.Fatalf("reenabling reverted revocation or teaching scope: %#v %v", enabled, err)
	}
}
