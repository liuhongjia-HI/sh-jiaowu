package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func customLessonRequest() learning.ScheduleClassCreateRequest {
	req := teacherLessonRequest()
	req.IgnoreWarnings = true
	req.Repeat = &learning.ScheduleRepeat{Freq: "custom", Dates: []learning.ScheduleCustomDate{
		{Date: "2026-06-12", StartTime: "14:00", EndTime: "15:00"},
		{Date: "2026-06-08"},
	}}
	return req
}

func TestCustomSchedulePreviewAndCreateUsePerDateTimes(t *testing.T) {
	s := NewMemoryStore()
	p, err := s.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatal(err)
	}
	req := customLessonRequest()
	beforeClasses, beforeNotices, beforeLogs := len(s.scheduleClasses), len(s.notices), len(s.logs)
	preview, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ScheduleClassCreateRequest: req})
	if err != nil || !preview.CanSave || len(preview.Lessons) != 3 {
		t.Fatalf("preview: %#v %v", preview, err)
	}
	if len(s.scheduleClasses) != beforeClasses || len(s.notices) != beforeNotices || len(s.logs) != beforeLogs {
		t.Fatal("preview mutated state")
	}
	first, err := s.CreateScheduleClass("教务", p, req)
	if err != nil {
		t.Fatal(err)
	}
	lessons := seriesLessons(s, first.SeriesID)
	if len(lessons) != 3 {
		t.Fatalf("created %d lessons", len(lessons))
	}
	for i, lesson := range lessons {
		day := preview.Lessons[i]
		if lesson.LessonDate != day.Date || lesson.StartTime != day.StartTime || lesson.EndTime != day.EndTime {
			t.Fatalf("preview/save differ: %#v %#v", day, lesson)
		}
	}
	if lessons[1].StartTime != "19:00" || lessons[1].DurationMinutes != 90 || lessons[2].StartTime != "14:00" || lessons[2].DurationMinutes != 60 {
		t.Fatalf("incorrect overrides/defaults: %#v", lessons)
	}
}

func TestCustomScheduleConflictAndInvalidTimeRejectWholeBatch(t *testing.T) {
	for _, invalidTime := range []bool{false, true} {
		t.Run(map[bool]string{false: "conflict", true: "invalid-time"}[invalidTime], func(t *testing.T) {
			s := NewMemoryStore()
			p, _ := s.PrincipalByUserID("user-ops")
			req := customLessonRequest()
			if invalidTime {
				req.Repeat.Dates[0].EndTime = "13:00"
			} else {
				occupied := scheduleRequestForDate(req, *req.Repeat, "2026-06-12")
				occupied.StartDate, occupied.Repeat = "2026-06-12", nil
				if _, err := s.CreateScheduleClass("教务", p, occupied); err != nil {
					t.Fatal(err)
				}
			}
			beforeClasses, beforeNotices, beforeLogs := len(s.scheduleClasses), len(s.notices), len(s.logs)
			preview, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ScheduleClassCreateRequest: req})
			if err != nil || preview.CanSave || len(preview.Lessons) != 3 || len(preview.Lessons[2].Errors) == 0 {
				t.Fatalf("missing per-date failure: %#v %v", preview, err)
			}
			if _, err := s.CreateScheduleClass("教务", p, req); err == nil {
				t.Fatal("invalid batch accepted")
			}
			if len(s.scheduleClasses) != beforeClasses || len(s.notices) != beforeNotices || len(s.logs) != beforeLogs {
				t.Fatal("failed batch partially committed")
			}
		})
	}
}

func TestCustomScheduleRejectsDuplicateAndOutOfOrderFirstDate(t *testing.T) {
	for _, dates := range [][]learning.ScheduleCustomDate{
		{{Date: "2026-06-03"}}, {{Date: "2026-06-02"}}, {{Date: "2026-06-08"}, {Date: "2026-06-08"}}, {{Date: "2026-02-30"}},
	} {
		repeat, err := normalizeRepeat(&learning.ScheduleRepeat{Freq: "custom", Dates: dates})
		if err == nil {
			_, err = expandRepeatDates(repeat, "2026-06-03")
		}
		if err == nil {
			t.Fatalf("invalid dates accepted: %#v", dates)
		}
	}
	dates := make([]learning.ScheduleCustomDate, maxGeneratedLessons)
	if _, err := normalizeRepeat(&learning.ScheduleRepeat{Freq: "custom", Dates: dates}); err == nil {
		t.Fatal("more than 200 lessons accepted")
	}
}
