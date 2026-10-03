package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestSchedulingRejectsReadOnlyTeacherRangeAndPreservesExistingSeries(t *testing.T) {
	s, p, series := seedWeeklySeries(t)
	lessons := seriesLessons(s, series)
	for i := range s.users {
		if s.users[i].ID == "user-teacher" {
			ids := append([]string(nil), s.users[i].LearningSpaceIDs...)
			s.users[i].LearningSpaceIDs = nil
			s.users[i].TeacherLibrary = &learning.TeacherLibraryPolicy{SpaceIDs: ids, CanManageCourses: true}
		}
	}
	create := lessonUpdateRequest(lessons[0])
	create.StartDate = seriesDatePlusDays(3, 1)
	create.Repeat, create.IgnoreWarnings = nil, true
	if _, err := s.CreateScheduleClass("教务", p, create); err == nil || !strings.Contains(err.Error(), "授课范围") {
		t.Fatalf("read-only scope became teaching scope: %v", err)
	}
	update := lessonUpdateRequest(lessons[0])
	update.StartDate, update.EditScope, update.IgnoreWarnings = seriesDatePlusDays(0, 1), learning.EditScopeAll, true
	preview, err := s.PreviewScheduleClass(p, learning.SchedulePreviewRequest{ID: lessons[0].ID, ScheduleClassCreateRequest: update})
	if err != nil || !preview.CanSave {
		t.Fatalf("historical series lost edit access: %#v %v", preview, err)
	}
	if _, err := s.UpdateScheduleClass("教务", p, lessons[0].ID, update); err != nil {
		t.Fatalf("save disagrees with historical preview: %v", err)
	}
	for i, item := range seriesLessons(s, series) {
		if item.LessonDate != seriesDatePlusDays(i, 1) {
			t.Fatalf("series not moved: %#v", item)
		}
	}
}
