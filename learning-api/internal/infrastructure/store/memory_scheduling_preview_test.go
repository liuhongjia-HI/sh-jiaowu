package store

import (
	"os"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

func TestSchedulePreviewCollectsEveryConflictWithoutMutation(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lessons := seriesLessons(s, series)
	req := lessonUpdateRequest(lessons[0])
	req.Repeat = &learning.ScheduleRepeat{Freq: "weekly", Interval: 1, Count: 4}
	beforeClasses, beforeNotices, beforeLogs := len(s.scheduleClasses), len(s.notices), len(s.logs)
	result, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ScheduleClassCreateRequest: req})
	if err != nil {
		t.Fatal(err)
	}
	if result.CanSave || len(result.Lessons) != 4 {
		t.Fatalf("expected four blocked dates: %#v", result)
	}
	for _, lesson := range result.Lessons {
		if len(lesson.Errors) == 0 {
			t.Fatalf("missing conflict for %s", lesson.Date)
		}
	}
	if len(s.scheduleClasses) != beforeClasses || len(s.notices) != beforeNotices || len(s.logs) != beforeLogs {
		t.Fatal("preview changed business state")
	}
}

func TestSchedulePreviewSeriesMoveExcludesItsOwnLessons(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lessons := seriesLessons(s, series)
	req := lessonUpdateRequest(lessons[0])
	req.StartDate = lessons[1].LessonDate
	req.IgnoreWarnings = true
	req.EditScope = learning.EditScopeAll
	result, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ID: lessons[0].ID, ScheduleClassCreateRequest: req})
	if err != nil || !result.CanSave || len(result.Lessons) != 4 {
		t.Fatalf("preview should allow batch shift: %#v %v", result, err)
	}
	if s.scheduleClasses[0].LessonDate != lessons[0].LessonDate {
		t.Fatal("preview wrote shifted date")
	}
	_, err = s.UpdateScheduleClass("教务", p, lessons[0].ID, req)
	if err != nil {
		t.Fatalf("save disagrees with preview: %v", err)
	}
}

func TestCancelScheduleScopePreservesHistoryAndDetached(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lessons := seriesLessons(s, series)
	for i := range s.scheduleClasses {
		if s.scheduleClasses[i].ID == lessons[0].ID {
			s.scheduleClasses[i].LessonDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		}
		if s.scheduleClasses[i].ID == lessons[2].ID {
			s.scheduleClasses[i].Detached = true
		}
	}
	if _, err := s.CancelScheduleClassScope("教务", p, lessons[1].ID, learning.EditScopeAll); err != nil {
		t.Fatal(err)
	}
	for _, lesson := range seriesLessons(s, series) {
		expected := lesson.ID == lessons[1].ID || lesson.ID == lessons[3].ID
		if (lesson.Status == "已取消") != expected {
			t.Fatalf("wrong cancellation scope: %#v", lesson)
		}
	}
}

func TestCancelScheduleScopeChecksAllPermissionsBeforeMutation(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lessons := seriesLessons(s, series)
	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	for i := range s.scheduleClasses {
		if s.scheduleClasses[i].SeriesID == series {
			s.scheduleClasses[i].AuditStatus = learning.AuditPending
		}
	}
	for i := range s.scheduleClasses {
		if s.scheduleClasses[i].ID == lessons[3].ID {
			s.scheduleClasses[i].AuditStatus = learning.AuditApproved
		}
	}
	if _, err := s.CancelScheduleClassScope("教师", teacher, lessons[0].ID, learning.EditScopeAll); err == nil {
		t.Fatal("teacher cancelled approved lesson")
	}
	for _, lesson := range seriesLessons(s, series) {
		if lesson.Status == "已取消" {
			t.Fatal("partial cancellation on permission failure")
		}
	}
	if _, err := s.CancelScheduleClassScope("教务", p, lessons[0].ID, ""); err == nil {
		t.Fatal("missing series scope must be rejected")
	}
}

func TestTemporaryUnavailabilityOverridesCoverageAndPreviewIsSoft(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lesson := seriesLessons(s, series)[0]
	slots := s.ownerAvailability("teacher", lesson.TeacherID)
	slots = append(slots, learning.AvailabilitySlot{DayOfWeek: lesson.DayOfWeek, StartTime: lesson.StartTime, EndTime: lesson.EndTime, StartDate: lesson.LessonDate, EndDate: lesson.LessonDate, Unavailable: true})
	if _, err := s.SaveAvailability("教务", p, learning.AvailabilityUpsertRequest{OwnerType: "teacher", OwnerID: lesson.TeacherID, Slots: slots}); err != nil {
		t.Fatal(err)
	}
	start, _ := parseClock(lesson.StartTime)
	end, _ := parseClock(lesson.EndTime)
	if s.teacherAvailable(lesson.TeacherID, lesson.DayOfWeek, start, end, lesson.LessonDate, lesson.LessonDate) {
		t.Fatal("leave must override weekly availability")
	}
	req := lessonUpdateRequest(lesson)
	req.EditScope = learning.EditScopeThis
	result, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ID: lesson.ID, ScheduleClassCreateRequest: req})
	if err != nil || !result.CanSave || len(result.Lessons[0].Warnings) == 0 {
		t.Fatalf("leave should warn before coordinated override: %#v %v", result, err)
	}
	req.IgnoreWarnings = false
	if _, err := s.UpdateScheduleClass("教务", p, lesson.ID, req); err == nil || !strings.Contains(err.Error(), "可上课") {
		t.Fatalf("must require explicit override: %v", err)
	}
}

func TestMySQLCalendarAvailabilityAndSeriesSurviveReload(t *testing.T) {
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
	p, err := s.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatal(err)
	}
	targetDate := time.Now().AddDate(0, 0, 45)
	for {
		occupied := false
		date := targetDate.Format("2006-01-02")
		for _, item := range s.scheduleClasses {
			if item.TeacherID == "user-teacher" && item.LessonDate == date && item.Status != "已取消" && item.StartTime < "07:00" && item.EndTime > "06:00" {
				occupied = true
			}
		}
		if !occupied {
			break
		}
		targetDate = targetDate.AddDate(0, 0, 1)
	}
	date := targetDate.Format("2006-01-02")
	// Pick a unique time to avoid any seed lesson. Isolated test DB only.
	req := learning.ScheduleClassCreateRequest{CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main", ClassType: "1V1", DurationMinutes: 60, StartTime: "06:00", EndTime: "07:00", StartDate: date, StudentIDs: []string{"stu-001"}, IgnoreWarnings: true, Repeat: &learning.ScheduleRepeat{Freq: "weekly", Interval: 1, Count: 3}}
	first, err := s.CreateScheduleClass("测试教务", p, req)
	if err != nil {
		t.Fatal(err)
	}
	slots := s.ownerAvailability("teacher", req.TeacherID)
	slots = append(slots, learning.AvailabilitySlot{DayOfWeek: first.DayOfWeek, StartTime: "06:00", EndTime: "07:00", StartDate: date, EndDate: date, Unavailable: true})
	if _, err := s.SaveAvailability("测试教务", p, learning.AvailabilityUpsertRequest{OwnerType: "teacher", OwnerID: req.TeacherID, Slots: slots}); err != nil {
		t.Fatal(err)
	}
	reload := NewMemoryStore()
	if err := reload.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer reload.db.Close()
	found := false
	for _, slot := range reload.ownerAvailability("teacher", req.TeacherID) {
		if slot.Unavailable && slot.StartDate == date {
			found = true
		}
	}
	if !found {
		t.Fatal("temporary unavailability lost on reload")
	}
	lessons := seriesLessons(reload, first.SeriesID)
	if len(lessons) != 3 {
		t.Fatalf("lost series: %d", len(lessons))
	}
	if _, err := reload.CancelScheduleClassScope("测试教务", p, lessons[1].ID, learning.EditScopeThisAndFuture); err != nil {
		t.Fatal(err)
	}
	third := NewMemoryStore()
	if err := third.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer third.db.Close()
	lessons = seriesLessons(third, first.SeriesID)
	defer third.CancelScheduleClassScope("测试清理", p, first.ID, learning.EditScopeAll)
	if lessons[0].Status == "已取消" || lessons[1].Status != "已取消" || lessons[2].Status != "已取消" {
		t.Fatalf("scope did not persist: %#v", lessons)
	}
}
