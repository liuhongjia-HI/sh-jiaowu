package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func cancellationFixture(t *testing.T) *MemoryStore {
	t.Helper()
	s := businessFixture(t)
	content := "课程：{{thing8.DATA}}\n学生：{{thing9.DATA}}\n上课时间：{{time10.DATA}}\n取消时间：{{time11.DATA}}\n次数：{{number12.DATA}}"
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "cancel-fixture", Title: "课程取消测试模板", Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
	b := learning.BusinessNoticeBinding{Kind: learning.NoticeScheduleCancelled, TemplateID: "cancel-fixture", Enabled: true, StudentIDs: []string{"s1"}, GuardianIDs: []string{"g1"}, FieldMappings: map[string]string{"thing8": "course_name", "thing9": "student_name", "time10": "lesson_time", "time11": "cancelled_at", "number12": "lesson_count"}}
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err != nil {
		t.Fatal(err)
	}
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("cancel-lesson", "cancel-series")}
	})
	s.planBusinessReminders(time.Now())
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].Status = "已取消" })
	return s
}
func TestBusinessCancellationConfiguredFieldsScopeAndNoDuplicate(t *testing.T) {
	s := cancellationFixture(t)
	cancels := []learning.BusinessNoticeTask{}
	for _, task := range s.businessNoticeTasks {
		if task.Kind == learning.NoticeScheduleCancelled {
			cancels = append(cancels, task)
		}
	}
	if len(cancels) != 1 || cancels[0].GuardianID != "g1" || cancels[0].StudentID != "s1" {
		t.Fatal("cancellation broadened target")
	}
	task := cancels[0]
	if task.Values["thing8"] != "五年级英文" || task.Values["thing9"] != "孩子甲" || task.Values["number12"] != "1" || task.Values["time10"] != businessLessonTime(s.scheduleClasses[0]) || task.Values["time11"] == "" {
		t.Fatal("incorrect original lesson fields")
	}
	if err := validateOfficialMessageValues(task.Values); err != nil {
		t.Fatal(err)
	}
	count := len(s.businessNoticeTasks)
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].Status = "已取消" })
	if len(s.businessNoticeTasks) != count {
		t.Fatal("duplicate cancellation generated notification")
	}
	sends := 0
	s.officialMessageSender = func(req learning.OfficialMessageRequest) (string, error) {
		sends++
		if req.TemplateID != "cancel-fixture" || req.OpenID != "o1" {
			t.Fatal("cancelled course still sent old reminder or wrong target")
		}
		return "cancel-msg", nil
	}
	if err := s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatalf("sends=%d want 1", sends)
	}
}
func TestBusinessCancellationRestoredLessonAndChangedMappingBlock(t *testing.T) {
	s := cancellationFixture(t)
	var task learning.BusinessNoticeTask
	for _, item := range s.businessNoticeTasks {
		if item.Kind == learning.NoticeScheduleCancelled {
			task = item
		}
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "" {
		t.Fatal(reason)
	}
	s.scheduleClasses[0].Status = "已确认"
	if reason := s.businessTaskValidity(task, time.Now()); reason == "" {
		t.Fatal("restored lesson retained cancellation")
	}
	s.scheduleClasses[0].Status = "已取消"
	b := s.bindingForBusinessKind(learning.NoticeScheduleCancelled)
	b.FieldMappings["time10"] = "cancelled_at"
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err != nil {
		t.Fatal(err)
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "取消通知字段映射已变更" {
		t.Fatal("changed mapping not detected")
	}
	b.FieldMappings["time10"] = "course_name"
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err == nil {
		t.Fatal("invalid time mapping accepted")
	}
}
