package store

import (
	"starline/learning-api/internal/domain/learning"
	"strings"
)

func reminderNoticeValues(binding learning.BusinessNoticeBinding, event learning.BusinessNoticeEvent) map[string]string {
	if len(binding.FieldMappings) == 0 {
		return businessNoticeValues(event)
	}
	values := map[string]string{}
	if len(event.Lessons) == 0 {
		return values
	}
	lesson := event.Lessons[0].After
	for key, source := range binding.FieldMappings {
		switch source {
		case "course_name":
			values[key] = businessShortName(firstNonEmpty(lesson.CourseName, lesson.Name))
		case "student_name":
			values[key] = businessShortName(event.StudentName)
		case "lesson_time":
			values[key] = businessLessonTime(lesson)
			if strings.HasPrefix(key, "date") {
				values[key] = firstNonEmpty(lesson.LessonDate, lesson.StartDate)
			}
		}
	}
	return values
}
