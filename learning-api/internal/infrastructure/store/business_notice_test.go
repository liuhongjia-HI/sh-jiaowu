package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func businessFixture(t *testing.T) *MemoryStore {
	t.Helper()
	s := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	s.students = []learning.Student{{ID: "s1", Name: "孩子甲", AccountStatus: "正常"}, {ID: "s2", Name: "孩子乙", AccountStatus: "正常"}}
	s.guardians = []learning.Guardian{{ID: "g1", Name: "妈妈", UnionID: "union1", AccountStatus: "正常"}, {ID: "g2", Name: "爸爸", UnionID: "union2", AccountStatus: "正常"}}
	s.guardianStudents = []learning.GuardianStudent{{GuardianID: "g1", StudentID: "s1", Status: learning.GuardianStudentActive}, {GuardianID: "g1", StudentID: "s2", Status: learning.GuardianStudentActive}, {GuardianID: "g2", StudentID: "s1", Status: learning.GuardianStudentActive}}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "o1", UnionID: "union1", Subscribed: true}, {OpenID: "o2", UnionID: "union2", Subscribed: true}}
	for _, binding := range defaultBusinessNoticeBindings() {
		if len(binding.RequiredFields) == 0 || binding.Kind == learning.NoticeReviewException || binding.Kind == learning.NoticeHomeworkSubmitted {
			continue
		}
		content := ""
		for key, label := range binding.RequiredFields {
			content += label + "：{{" + key + ".DATA}}\n"
		}
		s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: binding.Kind, Title: binding.Title, Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
		if _, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: binding.Kind, TemplateID: binding.Kind, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func futureBusinessClass(id, series string) learning.ScheduleClass {
	date := time.Now().In(businessNoticeLocation).AddDate(0, 0, 2).Format("2006-01-02")
	return learning.ScheduleClass{ID: id, SeriesID: series, CourseID: "course", CourseName: "五年级英文", Name: "英文班", LessonDate: date, StartDate: date, EndDate: date, StartTime: "16:00", EndTime: "17:00", Status: "已确认", AuditStatus: learning.AuditApproved, Students: []learning.CandidateStudent{{ID: "s1", Name: "孩子甲"}, {ID: "s2", Name: "孩子乙"}}}
}
func mutateBusiness(t *testing.T, s *MemoryStore, change func(*MemoryStore)) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := persistentMutationError(s, func(work *MemoryStore) error { change(work); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessNoticeApprovedSnapshotAndSecondMove(t *testing.T) {
	s := businessFixture(t)
	item := futureBusinessClass("lesson", "series")
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses = []learning.ScheduleClass{item} })
	if len(s.businessNoticeEvents) != 2 || len(s.businessNoticeTasks) != 3 {
		t.Fatalf("first confirmation must fan out to correct parents: events=%d tasks=%d", len(s.businessNoticeEvents), len(s.businessNoticeTasks))
	}
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses[0].AuditStatus = learning.AuditPending
		work.scheduleClasses[0].StartTime = "17:00"
	})
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].StartTime = "18:00" })
	if len(s.businessNoticeEvents) != 2 {
		t.Fatal("pending changes generated notifications")
	}
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].AuditStatus = learning.AuditApproved })
	event := s.businessNoticeEvents[2]
	if event.Kind != learning.NoticeScheduleChanged || event.Lessons[0].Before.StartTime != "16:00" || event.Lessons[0].After.StartTime != "18:00" {
		t.Fatalf("must compare final approval against last effective arrangement: %#v", event)
	}
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].StartTime = "19:00" })
	if len(s.businessNoticeEvents) != 6 {
		t.Fatal("a second real adjustment must generate another event")
	}
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].ReservationNote = "内部备注" })
	if len(s.businessNoticeEvents) != 6 {
		t.Fatal("notes must not generate adjustments")
	}
}

func TestBusinessNoticeSeriesCombinesOnlyWithinStudentAndSeries(t *testing.T) {
	s := businessFixture(t)
	mutateBusiness(t, s, func(work *MemoryStore) {
		for i := 0; i < 16; i++ {
			item := futureBusinessClass(fmt.Sprintf("lesson-%02d", i), "series")
			item.LessonDate = time.Now().AddDate(0, 0, i+2).Format("2006-01-02")
			work.scheduleClasses = append(work.scheduleClasses, item)
		}
	})
	if len(s.businessNoticeEvents) != 2 || len(s.businessNoticeTasks) != 3 {
		t.Fatalf("series must not flood guardians: %d events / %d tasks", len(s.businessNoticeEvents), len(s.businessNoticeTasks))
	}
	for _, event := range s.businessNoticeEvents {
		if len(event.Lessons) != 16 {
			t.Fatal("detail lost affected lessons")
		}
	}
	if !strings.Contains(s.businessNoticeTasks[0].PagePath, s.businessNoticeTasks[0].EventID) {
		t.Fatal("jump must identify exact event")
	}
}

func TestBusinessNoticeRestartLeaseReceiptAndNoDuplicateDelivery(t *testing.T) {
	s := businessFixture(t)
	item := futureBusinessClass("lesson", "series")
	item.Students = item.Students[:1]
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses = []learning.ScheduleClass{item} })
	sent := 0
	s.officialMessageSender = func(request learning.OfficialMessageRequest) (string, error) {
		sent++
		id := fmt.Sprint(900 + sent)
		if err := s.HandleOfficialDeliveryReceipt(request.OpenID, id, "success"); err != nil {
			t.Fatal(err)
		}
		return id, nil
	}
	now := time.Now()
	if err := s.ProcessBusinessNotices(now); err != nil {
		t.Fatal(err)
	}
	for _, task := range s.businessNoticeTasks {
		if task.Kind == learning.NoticeScheduleConfirmed && task.Status != "已送达" {
			t.Fatalf("early receipt was lost: %#v", task)
		}
	}
	if sent != 2 {
		t.Fatalf("expected two parents, got %d", sent)
	}
	if err := s.ProcessBusinessNotices(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if sent != 2 {
		t.Fatal("accepted messages sent twice")
	}
	for _, task := range s.BusinessNoticeTasks() {
		if task.OpenID != "" {
			t.Fatal("admin API leaked recipient openid")
		}
	}
}

func TestBusinessNoticeCancellationInvalidatesReminderAndIdentity(t *testing.T) {
	s := businessFixture(t)
	item := futureBusinessClass("lesson", "series")
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{item}
		work.planBusinessReminders(time.Now())
	})
	if len(s.businessNoticeTasks) < 6 {
		t.Fatal("missing reminder tasks")
	}
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses[0].Status = "已取消" })
	if err := s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, task := range s.businessNoticeTasks {
		if task.Kind == learning.NoticeScheduleReminder && task.Status != "已失效" {
			t.Fatalf("cancelled lesson retained active reminder: %#v", task)
		}
	}
	id := s.businessNoticeEvents[0].ID
	principal := learning.Principal{StudentID: "s2", GuardianID: "g1"}
	detail, err := s.BusinessNoticeDetail(principal, id)
	if err != nil || detail.Event.StudentID != "s1" || !detail.CanSwitch {
		t.Fatalf("guardian should resolve exact child: %#v %v", detail, err)
	}
	mutateBusiness(t, s, func(work *MemoryStore) { work.guardianStudents[0].Status = learning.GuardianStudentInactive })
	if _, err := s.BusinessNoticeDetail(principal, id); err == nil {
		t.Fatal("old link remained available after unlink")
	}
}

func TestBusinessNoticeUncertainResultDoesNotRetryBeyondWechatWindow(t *testing.T) {
	s := businessFixture(t)
	item := futureBusinessClass("lesson", "series")
	item.Students = item.Students[:1]
	s.officialFollowers = s.officialFollowers[:1]
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses = []learning.ScheduleClass{item} })
	sent := 0
	s.officialMessageSender = func(request learning.OfficialMessageRequest) (string, error) {
		sent++
		return "", &officialSendError{reason: "timeout", uncertain: true}
	}
	now := time.Now()
	if err := s.ProcessBusinessNotices(now); err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Fatal("missing initial attempt")
	}
	if err := s.ProcessBusinessNotices(now.Add(11 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if sent != 1 || s.businessNoticeTasks[0].Status != "结果待确认" {
		t.Fatal("unknown result blindly retried beyond deduplication window")
	}
	if _, err := s.RetryBusinessNotice("test", s.businessNoticeTasks[0].ID); err == nil {
		t.Fatal("uncertain task must not allow normal manual retry")
	}
}

func TestBusinessReminderQuietWindow(t *testing.T) {
	for _, tc := range []struct{ start, want string }{{"2030-10-03 00:30", "2030-10-02 20:00"}, {"2030-10-03 09:00", "2030-10-02 20:00"}, {"2030-10-03 14:00", "2030-10-03 12:00"}, {"2030-10-03 23:30", "2030-10-02 20:00"}} {
		start, _ := time.ParseInLocation("2006-01-02 15:04", tc.start, businessNoticeLocation)
		got := businessReminderDue(start).Format("2006-01-02 15:04")
		if got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.start, got, tc.want)
		}
	}
}

func TestBusinessNoticeFieldValidationAndAtomicFailure(t *testing.T) {
	s := businessFixture(t)
	if _, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeScheduleChanged, TemplateID: learning.NoticeScheduleConfirmed, Enabled: true}); err == nil {
		t.Fatal("mismatched template accepted")
	}
	if err := validateOfficialMessageValues(map[string]string{"thing1": strings.Repeat("中", 21)}); err == nil {
		t.Fatal("oversized field accepted")
	}
	if err := validateOfficialMessageValues(map[string]string{"time4": "2030-10-03 16:00~17:00"}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	_, err := persistentMutation(s, func(work *MemoryStore) (bool, error) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("lesson", "series")}
		return false, errors.New("save failed")
	})
	s.mu.Unlock()
	if err == nil || len(s.businessNoticeEvents) != 0 || len(s.scheduleClasses) != 0 {
		t.Fatal("failed operation published business change or notification")
	}
}

func TestBusinessNoticeDetailDoesNotExposeOtherChildrenOrDisabledGuardian(t *testing.T) {
	s := businessFixture(t)
	item := futureBusinessClass("privacy", "")
	item.ReservationNote = "private staff note"
	mutateBusiness(t, s, func(work *MemoryStore) { work.scheduleClasses = []learning.ScheduleClass{item} })
	detail, err := s.BusinessNoticeDetail(learning.Principal{GuardianID: "g1", StudentID: "s2"}, s.businessNoticeEvents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range append(detail.CurrentLessons, detail.Event.Lessons[0].After) {
		if len(item.Students) != 1 || item.Students[0].ID != detail.Event.StudentID || item.ReservationNote != "" {
			t.Fatalf("other child's data exposed: %#v", item)
		}
	}
	s.guardians[0].AccountStatus = "停用"
	if _, err := s.BusinessNoticeDetail(learning.Principal{GuardianID: "g1"}, detail.Event.ID); err == nil {
		t.Fatal("disabled guardian accessed details")
	}
}

func TestBusinessNoticeManualRetryRestartsDefiniteFailureAndRedactsIdentity(t *testing.T) {
	s := businessFixture(t)
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("retry", "")}
	})
	task := &s.businessNoticeTasks[0]
	task.Status = "发送失败"
	task.Retryable = true
	task.Attempts = 4
	task.FirstAttemptAt = businessTime(time.Now().Add(-time.Hour))
	listed := s.BusinessNoticeTasks()
	if !listed[0].Retryable || listed[0].OpenID != "" {
		t.Fatal("retry eligibility or identity redaction incorrect")
	}
	if _, err := s.RetryBusinessNotice("test", task.ID); err != nil {
		t.Fatal(err)
	}
	if s.businessNoticeTasks[0].Attempts != 0 || s.businessNoticeTasks[0].FirstAttemptAt != "" {
		t.Fatal("manual retry remained exhausted")
	}
}

func TestBusinessReminderRecoveryRespectsQuietHours(t *testing.T) {
	s := businessFixture(t)
	now, _ := time.ParseInLocation("2006-01-02 15:04", "2030-10-03 22:00", businessNoticeLocation)
	lesson := futureBusinessClass("late-recovery", "")
	lesson.LessonDate = "2030-10-03"
	lesson.StartTime = "23:30"
	lesson.EndTime = "23:59"
	s.scheduleClasses = []learning.ScheduleClass{lesson}
	s.planBusinessReminders(now)
	for _, task := range s.businessNoticeTasks {
		if !parseBusinessTime(task.DueAt).After(now) {
			t.Fatal("missed reminder scheduled during quiet hours")
		}
	}
	sent := 0
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sent++; return "unexpected", nil }
	if err := s.ProcessBusinessNotices(now); err != nil {
		t.Fatal(err)
	}
	if sent != 0 {
		t.Fatal("recovery pushed ordinary reminder outside allowed window")
	}
}

func TestBusinessUncertainRecoveryNeverAutomaticallyResends(t *testing.T) {
	s := businessFixture(t)
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("dedup-window", "")}
	})
	// Keep one recipient and disable reminder planning to isolate uncertain retries.
	s.businessNoticeTasks = s.businessNoticeTasks[:1]
	binding := s.bindingForBusinessKind(learning.NoticeScheduleReminder)
	if _, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: binding.Kind, TemplateID: binding.TemplateID, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	sent := 0
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) {
		sent++
		return "", &officialSendError{reason: "timeout", uncertain: true}
	}
	now := time.Now()
	for _, elapsed := range []time.Duration{0, 15 * time.Second, time.Minute, 75 * time.Second, 6 * time.Minute, 375 * time.Second, 21 * time.Minute} {
		if err := s.ProcessBusinessNotices(now.Add(elapsed)); err != nil {
			t.Fatal(err)
		}
	}
	if sent != 1 || s.businessNoticeTasks[0].Status != "结果待确认" {
		t.Fatalf("unknown response was automatically retried: sent=%d task=%#v", sent, s.businessNoticeTasks[0])
	}
}

func TestBusinessNoticePermanentFailureAndUnknownFailureCannotRetry(t *testing.T) {
	for _, sendErr := range []error{&officialSendError{reason: "invalid recipient"}, errors.New("unclassified failure")} {
		s := businessFixture(t)
		mutateBusiness(t, s, func(work *MemoryStore) {
			work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("permanent-retry", "")}
		})
		calls := 0
		s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { calls++; return "", sendErr }
		if err := s.ProcessBusinessNotices(time.Now()); err != nil {
			t.Fatal(err)
		}
		initial := calls
		if err := s.ProcessBusinessNotices(time.Now().Add(20 * time.Minute)); err != nil {
			t.Fatal(err)
		}
		if calls != initial {
			t.Fatal("non-retryable error automatically resent")
		}
		for _, task := range s.businessNoticeTasks {
			if task.Status == "发送失败" {
				if task.Retryable {
					t.Fatal("non-retryable failure flagged eligible")
				}
				if _, err := s.RetryBusinessNotice("test", task.ID); err == nil {
					t.Fatal("permanent or unknown failure allowed manual retry")
				}
			}
		}
	}
}
