package store

import (
	"fmt"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"time"
)

func (s *MemoryStore) homeworkNotificationValues(binding learning.BusinessNoticeBinding, event learning.BusinessNoticeEvent) map[string]string {
	id := event.RelatedID
	var submission learning.Submission
	if event.Kind == learning.NoticeReviewCompleted {
		submission = s.submissions[id]
		id = submission.HomeworkID
	}
	homework, ok := s.findHomework(id)
	if !ok {
		return nil
	}
	courseName := homework.Course
	if courseName == "" {
		if course, found := s.findCourse(homework.CourseID); found {
			courseName = course.Name
		}
	}
	values := map[string]string{}
	for key, source := range binding.FieldMappings {
		switch source {
		case "course_name":
			values[key] = businessShortName(courseName)
		case "homework_title":
			values[key] = businessShortName(homework.Title)
		case "student_name":
			values[key] = businessShortName(event.StudentName)
		case "score":
			values[key] = fmt.Sprint(submission.FinalScore)
		case "published_at", "reviewed_at":
			values[key] = notificationTimeField(key, parseBusinessTime(event.CreatedAt))
		case "deadline_at":
			if deadline, ok := homeworkNotificationDeadline(homework); ok {
				values[key] = notificationTimeField(key, deadline)
			}
		}
	}
	return values
}
func homeworkNotificationDeadline(homework learning.Homework) (time.Time, bool) {
	deadline, err := time.Parse(time.RFC3339, homework.DeadlineAt)
	if err != nil && homework.Deadline != "" {
		deadline, err = time.ParseInLocation("2006-01-02", homework.Deadline, businessNoticeLocation)
		deadline = deadline.Add(24*time.Hour - time.Second)
	}
	return deadline, err == nil
}
func notificationTimeField(key string, t time.Time) string {
	format := "2006-01-02 15:04:05"
	if strings.HasPrefix(key, "date") {
		format = "2006-01-02"
	}
	return t.In(businessNoticeLocation).Format(format)
}
