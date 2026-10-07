package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func TestBusinessReminderRejectsReservationTemplateDespiteMatchingFields(t *testing.T) {
	s := businessFixture(t)
	for i := range s.officialTemplates {
		if s.officialTemplates[i].ID == learning.NoticeScheduleReminder {
			s.officialTemplates[i].Title = "课程预约结果通知"
		}
	}
	b := s.bindingForBusinessKind(learning.NoticeScheduleReminder)
	if b.Ready || b.Reason == "" {
		t.Fatal("reservation result masqueraded as reminder")
	}
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err == nil {
		t.Fatal("invalid reminder enabled")
	}
}
func TestBusinessReminderConfiguredFieldKeysAndScope(t *testing.T) {
	s := businessFixture(t)
	content := "课程：{{thing8.DATA}}\n学生：{{thing9.DATA}}\n上课时间：{{time10.DATA}}"
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "reminder-fixture", Title: "上课提醒", Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
	b := s.bindingForBusinessKind(learning.NoticeScheduleReminder)
	b.TemplateID = "reminder-fixture"
	b.FieldMappings = map[string]string{"thing8": "course_name", "thing9": "student_name", "time10": "lesson_time"}
	b.StudentIDs = []string{"s1"}
	b.GuardianIDs = []string{"g1"}
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err != nil {
		t.Fatal(err)
	}
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("reminder-mapped", "")}
		work.planBusinessReminders(time.Now())
	})
	var task learning.BusinessNoticeTask
	count := 0
	for _, item := range s.businessNoticeTasks {
		if item.Kind == learning.NoticeScheduleReminder {
			count++
			task = item
		}
	}
	if count != 1 || task.GuardianID != "g1" || task.Values["thing8"] != "五年级英文" || task.Values["time10"] != businessLessonTime(s.scheduleClasses[0]) {
		t.Fatal("incorrect reminder fields or broadened scope")
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "" {
		t.Fatal(reason)
	}
	b.FieldMappings["thing8"] = "student_name"
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err != nil {
		t.Fatal(err)
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "课前提醒字段映射已变更" {
		t.Fatal("old mapped reminder remained valid")
	}
}

func TestBusinessReminderSuppressesAcceptedOrDefersUncertainConfirmation(t *testing.T) {
	s := businessFixture(t)
	lesson := futureBusinessClass("suppression-lesson", "")
	start := businessLessonStart(lesson)
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{lesson}
		work.planBusinessReminders(time.Now())
	})
	var reminder learning.BusinessNoticeTask
	for _, task := range s.businessNoticeTasks {
		if task.Kind == learning.NoticeScheduleReminder && task.StudentID == "s1" && task.GuardianID == "g1" {
			reminder = task
		}
	}
	if reminder.ID == "" {
		t.Fatal("missing reminder fixture")
	}
	for i := range s.businessNoticeTasks {
		task := &s.businessNoticeTasks[i]
		if task.Kind == learning.NoticeScheduleConfirmed && task.StudentID == "s1" && task.GuardianID == "g1" {
			task.Status = "微信已受理"
			task.AcceptedAt = businessTime(start.Add(-time.Hour))
		}
	}
	if suppress, deferSend := s.businessReminderSuppression(reminder, start.Add(-time.Hour)); !suppress || deferSend {
		t.Fatal("recent accepted confirmation did not suppress reminder")
	}
	for i := range s.businessNoticeTasks {
		task := &s.businessNoticeTasks[i]
		if task.Kind == learning.NoticeScheduleConfirmed && task.StudentID == "s1" && task.GuardianID == "g1" {
			task.Status = "结果待确认"
			task.AcceptedAt = ""
		}
	}
	if suppress, deferSend := s.businessReminderSuppression(reminder, start.Add(-time.Hour)); suppress || !deferSend {
		t.Fatal("uncertain confirmation did not defer overlapping reminder")
	}
}
