package store

import (
	"fmt"
	"starline/learning-api/internal/domain/learning"
	"strings"
)

// Field keys come from the configured template; values come from the last
// effective lesson snapshot, never a synthetic cancellation or a new schedule.
func cancellationNoticeValues(binding learning.BusinessNoticeBinding, event learning.BusinessNoticeEvent) map[string]string {
	values := map[string]string{}
	if len(event.Lessons) == 0 {
		return values
	}
	lesson := event.Lessons[0].After
	if event.Lessons[0].Before != nil {
		lesson = *event.Lessons[0].Before
	}
	for key, source := range binding.FieldMappings {
		switch source {
		case "course_name":
			values[key] = businessShortName(firstNonEmpty(lesson.CourseName, lesson.Name))
		case "student_name":
			values[key] = businessShortName(event.StudentName)
		case "lesson_count":
			values[key] = fmt.Sprint(len(event.Lessons))
		case "lesson_time":
			values[key] = businessLessonTime(lesson)
			if strings.HasPrefix(key, "date") {
				values[key] = firstNonEmpty(lesson.LessonDate, lesson.StartDate)
			}
		case "cancelled_at":
			format := "2006-01-02 15:04:05"
			if strings.HasPrefix(key, "date") {
				format = "2006-01-02"
			}
			values[key] = parseBusinessTime(event.CreatedAt).In(businessNoticeLocation).Format(format)
		}
	}
	return values
}
