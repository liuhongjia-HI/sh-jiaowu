package store

import (
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func TestRestoreCancelledLessonChecksCurrentSlotAndPreservesSeriesIdentity(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lessons := seriesLessons(s, series)
	original := lessons[0]
	if _, err := s.CancelScheduleClassScope("教务", p, original.ID, learning.EditScopeThis); err != nil {
		t.Fatal(err)
	}
	before := seriesLessons(s, series)
	preview, err := s.PreviewRestoreScheduleClass(p, original.ID)
	if err != nil || !preview.CanSave || !reflect.DeepEqual(before, seriesLessons(s, series)) {
		t.Fatalf("preview mutated or failed: %#v %v", preview, err)
	}
	eventsBefore := len(s.businessNoticeEvents)
	restored, err := s.RestoreScheduleClass("教务", p, original.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != original.Status || restored.ID != original.ID || restored.SeriesID != original.SeriesID || restored.Detached != original.Detached || restored.CreatedAt != original.CreatedAt || restored.AcademicYear != original.AcademicYear {
		t.Fatalf("restore changed identity: %#v", restored)
	}
	if len(s.businessNoticeEvents) <= eventsBefore {
		t.Fatal("restored lesson did not produce confirmation event")
	}
	after := seriesLessons(s, series)
	if !reflect.DeepEqual(before[1:], after[1:]) {
		t.Fatal("restore touched other cancelled or active lessons")
	}
	if _, err := s.RestoreScheduleClass("教务", p, original.ID, true); err == nil {
		t.Fatal("active lesson restored twice")
	}
	if _, err := s.CancelScheduleClassScope("教务", p, original.ID, learning.EditScopeThis); err != nil {
		t.Fatal(err)
	}
	req := lessonUpdateRequest(original)
	req.IgnoreWarnings = true
	if _, err := s.CreateScheduleClass("教务", p, req); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PreviewRestoreScheduleClass(p, original.ID)
	if err != nil || preview.CanSave || len(preview.Lessons[0].Errors) == 0 {
		t.Fatalf("conflict preview missed: %#v %v", preview, err)
	}
	before = append([]learning.ScheduleClass(nil), s.scheduleClasses...)
	if _, err := s.RestoreScheduleClass("教务", p, original.ID, true); err == nil {
		t.Fatal("restored occupied slot")
	}
	if !reflect.DeepEqual(before, s.scheduleClasses) {
		t.Fatal("failed restore mutated state")
	}
}

func TestRestoreRejectsRevokedTeacherPastLessonAndUnprivilegedActor(t *testing.T) {
	for _, scenario := range []string{"teaching-scope", "disabled", "past", "teacher"} {
		t.Run(scenario, func(t *testing.T) {
			s, p, series := seedWeeklySeries(t)
			id := seriesLessons(s, series)[0].ID
			if _, err := s.CancelScheduleClassScope("教务", p, id, learning.EditScopeThis); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "teaching-scope", "disabled":
				for i := range s.users {
					if s.users[i].ID == "user-teacher" {
						if scenario == "disabled" {
							s.users[i].AccountStatus = "停用"
						} else {
							s.users[i].LearningSpaceIDs = nil
						}
					}
				}
			case "past":
				for i := range s.scheduleClasses {
					if s.scheduleClasses[i].ID == id {
						s.scheduleClasses[i].LessonDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
					}
				}
			case "teacher":
				p, _ = s.PrincipalByUserID("user-teacher")
			}
			preview, err := s.PreviewRestoreScheduleClass(p, id)
			if err != nil || preview.CanSave {
				t.Fatalf("invalid preview accepted: %#v %v", preview, err)
			}
			if _, err := s.RestoreScheduleClass("教务", p, id, true); err == nil {
				t.Fatal("invalid restore accepted")
			}
		})
	}
}
