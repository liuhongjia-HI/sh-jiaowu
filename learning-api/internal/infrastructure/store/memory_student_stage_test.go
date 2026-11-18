package store

import (
	"encoding/json"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func useS1Q1Calendar(t *testing.T, s *MemoryStore) {
	t.Helper()
	now := time.Now().In(studentCalendarZone)
	terms := []academicCalendarTerm{{
		AcademicYear: currentAcademicYear(), Semester: "S1",
		StartDate:   now.AddDate(0, 0, -30).Format(time.DateOnly),
		MidtermDate: now.AddDate(0, 0, 30).Format(time.DateOnly),
		EndDate:     now.AddDate(0, 0, 60).Format(time.DateOnly),
	}}
	raw, err := json.Marshal(terms)
	if err != nil {
		t.Fatal(err)
	}
	s.settings["academicCalendar"] = string(raw)
}

func TestStudentCourseStageCalendarBoundaries(t *testing.T) {
	s := &MemoryStore{settings: map[string]string{"academicCalendar": `[
		{"academicYear":"2026.2027学年","semester":"S1 第一学期","startDate":"2026-09-01","midtermDate":"2026-11-10","endDate":"2027-01-15"},
		{"academicYear":"2026.2027学年","semester":"S2 第二学期","startDate":"2027-02-01","midtermDate":"2027-04-30","endDate":"2027-07-15"}
	]`}}
	for _, test := range []struct {
		date string
		want studentCourseStage
	}{
		{"2026-09-01", studentCourseStage{current: 1}},
		{"2026-11-10", studentCourseStage{current: 1}},
		{"2026-11-11", studentCourseStage{current: 2, past: 1}},
		{"2027-01-15", studentCourseStage{current: 2, past: 1}},
		{"2027-01-16", studentCourseStage{past: 2}},
		{"2027-02-01", studentCourseStage{current: 3, past: 2}},
		{"2027-04-30", studentCourseStage{current: 3, past: 2}},
		{"2027-05-01", studentCourseStage{current: 4, past: 3}},
		{"2027-07-15", studentCourseStage{current: 4, past: 3}},
	} {
		t.Run(test.date, func(t *testing.T) {
			if got := s.studentCourseStageOn(test.date); got.current != test.want.current || got.past != test.want.past {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
	if stage := s.studentCourseStageOn("2026-08-31"); stage.isCurrent(1) {
		t.Fatal("Q1 should start on the configured opening date")
	}
	s.settings["academicCalendar"] = `[{"semester":"S1","startDate":"2026-09-01","endDate":"2027-01-15"}]`
	if got := s.studentCourseStageOn("2026-11-11"); got != (studentCourseStage{}) {
		t.Fatalf("missing midterm should preserve existing display, got %+v", got)
	}
}

func TestStudentCourseStageSwitchesHomeStudyAndPreview(t *testing.T) {
	s := NewMemoryStore()
	principal, err := s.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	s.grants, s.spaceAccess = nil, nil
	now := time.Now().In(studentCalendarZone)
	date := func(offset int) string { return now.AddDate(0, 0, offset).Format(time.DateOnly) }
	setMidterm := func(offset int) {
		t.Helper()
		terms := []academicCalendarTerm{{AcademicYear: currentAcademicYear(), Semester: "S1 第一学期", StartDate: date(-7), MidtermDate: date(offset), EndDate: date(7)}}
		raw, err := json.Marshal(terms)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpdateSetting("test", learning.SettingUpdateRequest{Key: "academicCalendar", Value: string(raw)}); err != nil {
			t.Fatal(err)
		}
	}
	open := func(spaceID string) {
		t.Helper()
		if _, err := s.CreateDirectGrant("test", learning.DirectGrantCreateRequest{
			StudentID: principal.StudentID, LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"course"},
			StartsAt: date(-3), EndsAt: date(3),
		}); err != nil {
			t.Fatal(err)
		}
	}
	const oldCourse = "course-g05-english-s1-q1"
	const currentCourse = "course-g05-english-s1-q2"
	const oldSpace = "space-g05-english-s1-q1"
	const currentSpace = "space-g05-english-s1-q2"
	setMidterm(0)
	open(oldSpace)

	checkBoards := func(currentID, historyID, previewID string) {
		t.Helper()
		home, err := s.StudentHome(principal)
		if err != nil || home.ContinueCourse.ID != currentID {
			t.Fatalf("home course = %q, want %q, error = %v", home.ContinueCourse.ID, currentID, err)
		}
		study, err := s.StudentStudy(principal)
		if err != nil {
			t.Fatal(err)
		}
		checkCards := func(cards []learning.StudentCourseCard, want string) {
			t.Helper()
			if want == "" && len(cards) != 0 || want != "" && (len(cards) != 1 || cards[0].ID != want) {
				t.Fatalf("want %q, got %v cards", want, len(cards))
			}
		}
		checkCards(home.Courses, currentID)
		checkCards(study.Courses, currentID)
		checkCards(study.HistoryCourses, historyID)
		for _, subject := range study.Subjects {
			if subject.Subject == "英文" {
				if subject.PreviewCourseID != previewID {
					t.Fatalf("preview course = %q, want %q", subject.PreviewCourseID, previewID)
				}
				return
			}
		}
		t.Fatal("English subject is missing")
	}

	checkBoards(oldCourse, "", "")
	setMidterm(-1)
	checkBoards("", oldCourse, currentCourse)
	if detail, err := s.StudentCourseDetail(principal, currentCourse); err != nil || len(detail.Materials) == 0 {
		t.Fatalf("current preview should open with first lesson material: %v", err)
	}
	if detail, err := s.StudentCourseDetail(principal, oldCourse); err != nil || len(detail.Materials) == 0 {
		t.Fatalf("authorized historical course should still have readable material: %v", err)
	} else if _, err := s.StudentMaterial(principal, detail.Materials[0].ID); err != nil {
		t.Fatalf("authorized historical material must remain accessible: %v", err)
	}
	open(currentSpace)
	checkBoards(currentCourse, oldCourse, "")
	for index := range s.grants {
		if containsString(s.learningSpaceIDsForGrant(s.grants[index].ID), oldSpace) {
			s.grants[index].EndsAt = date(-1)
		}
	}
	checkBoards(currentCourse, "", "")
	if _, err := s.StudentCourseDetail(principal, oldCourse); err == nil {
		t.Fatal("expired old course must not reopen as a preview")
	}
}

func TestStudentCourseStageKeepsPriorYearEntitlementInHistory(t *testing.T) {
	s := NewMemoryStore()
	s.settings["academicCalendar"] = `[
		{"academicYear":"2025.2026学年","semester":"S2","startDate":"2026-02-01","midtermDate":"2026-05-01","endDate":"2026-07-15"},
		{"academicYear":"2026.2027学年","semester":"S1","startDate":"2026-09-01","midtermDate":"2026-11-10","endDate":"2027-01-15"}
	]`
	s.grants, s.spaceAccess = nil, nil
	const studentID = "stu-001"
	pastSpace, futureSpace := "space-g05-english-s2-q2", "space-g05-english-s1-q2"
	for _, spaceID := range []string{pastSpace, futureSpace} {
		if _, err := s.CreateDirectGrant("test", learning.DirectGrantCreateRequest{
			StudentID: studentID, LearningSpaceIDs: []string{spaceID}, ContentTypeCodes: []string{"course"},
			StartsAt: "2026-05-01", EndsAt: "2099-12-31",
		}); err != nil {
			t.Fatal(err)
		}
	}
	for index := range s.packages {
		if s.packages[index].ID == directGrantPackageID(studentID, pastSpace) {
			s.packages[index].AcademicYear = "2025.2026学年"
		}
		if s.packages[index].ID == directGrantPackageID(studentID, futureSpace) {
			s.packages[index].AcademicYear = "2026.2027学年"
		}
	}
	current, history := s.splitStudentCourses(studentID, s.studentCourseStageOn("2026-09-02"))
	if len(current) != 0 || len(history) != 1 || history[0].LearningSpaceID != pastSpace {
		t.Fatalf("want only prior-year course in history, got %d current and %d history", len(current), len(history))
	}
}
