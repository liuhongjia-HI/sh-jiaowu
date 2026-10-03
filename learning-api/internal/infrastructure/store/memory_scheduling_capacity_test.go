package store

import (
	"os"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestScheduleCapacityPreviewAndSaveAgree(t *testing.T) {
	for _, tc := range []struct {
		name, classType, errorText, status string
		students                           []string
		plan, expected                     int
	}{
		{name: "one optional", classType: "1V1", students: []string{"stu-001"}, expected: 1, status: "已确认"},
		{name: "two same names optional", classType: "1V2", students: []string{"stu-001", "stu-002"}, expected: 2, status: "已确认"},
		{name: "selected count exceeds plan", classType: "1V4", students: []string{"stu-001", "stu-002"}, plan: 1, expected: 2, status: "已确认"},
		{name: "underfilled can schedule", classType: "1V4", students: []string{"stu-001"}, expected: 1, status: "已确认"},
		{name: "empty reservation optional", classType: "1V2", expected: 0, status: "已确认"},
		{name: "too many students", classType: "1V1", students: []string{"stu-001", "stu-002"}, errorText: "学生人数超过班型容量"},
		{name: "too large plan", classType: "1V2", students: []string{"stu-001"}, plan: 3, errorText: "预计人数不能超过班型容量"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewMemoryStore()
			// Same display names must retain distinct student identities.
			for i := range s.students {
				if s.students[i].ID == "stu-001" || s.students[i].ID == "stu-002" {
					s.students[i].Name = "同名学生"
				}
			}
			p, err := s.PrincipalByUserID("user-super")
			if err != nil {
				t.Fatal(err)
			}
			req := teacherLessonRequest()
			req.ClassType, req.StudentIDs, req.ExpectedStudentCount = tc.classType, tc.students, tc.plan
			req.IgnoreWarnings = true
			before := len(s.scheduleClasses)
			preview, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ScheduleClassCreateRequest: req})
			if err != nil {
				t.Fatal(err)
			}
			if len(s.scheduleClasses) != before {
				t.Fatal("preview wrote lessons")
			}
			created, saveErr := s.CreateScheduleClass("测试管理员", p, req)
			if tc.errorText != "" {
				if preview.CanSave || len(preview.Lessons) != 1 || !strings.Contains(strings.Join(preview.Lessons[0].Errors, ";"), tc.errorText) {
					t.Fatalf("preview: %#v", preview)
				}
				if saveErr == nil || !strings.Contains(saveErr.Error(), tc.errorText) {
					t.Fatalf("save: %v", saveErr)
				}
				if len(s.scheduleClasses) != before {
					t.Fatal("rejected save wrote lessons")
				}
				return
			}
			if !preview.CanSave || saveErr != nil {
				t.Fatalf("preview=%#v save=%v", preview, saveErr)
			}
			if created.ExpectedStudentCount != tc.expected || created.Status != tc.status || len(created.Students) != len(tc.students) {
				t.Fatalf("created: %#v", created)
			}
			for i, id := range tc.students {
				if created.Students[i].ID != id {
					t.Fatalf("student identity changed: %#v", created.Students)
				}
			}
		})
	}
}

func TestScheduleCapacityMySQLReload(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated capacity database required")
	}
	if !strings.Contains(dsn, "/starline_schedule_capacity_test?") {
		t.Fatal("requires dedicated starline_schedule_capacity_test database")
	}
	s := NewMemoryStore()
	for i := range s.grants {
		s.grants[i].OpenedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer func() { s.db.Close() }()
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
	p, err := s.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	req := teacherLessonRequest()
	req.ClassType, req.StudentIDs, req.ExpectedStudentCount = "1V2", []string{"stu-001", "stu-002"}, 0
	req.StartDate, req.IgnoreWarnings = time.Now().AddDate(0, 0, 30).Format("2006-01-02"), true
	created, err := s.CreateScheduleClass("测试管理员", p, req)
	if err != nil {
		t.Fatal(err)
	}
	reload()
	assertSaved := func() {
		t.Helper()
		for _, lesson := range s.ScheduleClasses(p) {
			if lesson.ID != created.ID {
				continue
			}
			if lesson.ClassType != "1V2" || lesson.Capacity != 2 || lesson.ExpectedStudentCount != 2 || len(lesson.Students) != 2 || lesson.Students[0].ID != "stu-001" || lesson.Students[1].ID != "stu-002" {
				t.Fatalf("reload changed class: %#v", lesson)
			}
			return
		}
		t.Fatal("saved class missing")
	}
	assertSaved()
	before := len(s.ScheduleClasses(p))
	req.StartDate = time.Now().AddDate(0, 0, 31).Format("2006-01-02")
	req.ClassType = "1V1"
	if _, err := s.CreateScheduleClass("测试管理员", p, req); err == nil || !strings.Contains(err.Error(), "学生人数超过班型容量") {
		t.Fatalf("over-capacity save: %v", err)
	}
	reload()
	if len(s.ScheduleClasses(p)) != before {
		t.Fatal("rejected capacity request persisted a class")
	}
	assertSaved()
}
