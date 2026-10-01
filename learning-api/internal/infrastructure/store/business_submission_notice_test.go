package store

import (
	"os"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"sync"
	"testing"
	"time"
)

func submissionNoticeFixture(t *testing.T, enabled bool) (*MemoryStore, learning.Principal) {
	t.Helper()
	s := NewMemoryStore()
	p, err := s.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	s.guardians = []learning.Guardian{{ID: "submission-parent", Name: "家长", UnionID: "submission-union", AccountStatus: "正常"}}
	s.guardianStudents = []learning.GuardianStudent{{GuardianID: "submission-parent", StudentID: p.StudentID, Status: learning.GuardianStudentActive}}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "submission-openid", UnionID: "submission-union", Subscribed: true}}
	content := "课程名称：{{thing6.DATA}}\n作业名称：{{thing2.DATA}}\n作业提交时间：{{time4.DATA}}"
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "submission-template", Title: "作业提交成功通知", Status: "启用", Content: content, Fields: parseOfficialTemplateFields(content)})
	if _, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeHomeworkSubmitted, TemplateID: "submission-template", Enabled: enabled}); err != nil {
		t.Fatal(err)
	}
	return s, p
}
func submissionNoticeRequest(id string) learning.SubmissionRequest {
	return learning.SubmissionRequest{RequestID: id, HomeworkID: "hw-g05-english-s1-q1", Answers: []learning.SubmissionAnswer{{QuestionID: "q1", Choice: "A"}, {QuestionID: "q2", Text: "我学会了找中心句。"}}}
}
func TestBusinessSubmissionNoticeIdempotentRetryAndTrueResubmission(t *testing.T) {
	s, p := submissionNoticeFixture(t, true)
	req := submissionNoticeRequest("attempt-1")
	first, err := s.CreateSubmission("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := s.CreateSubmission("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != repeat.ID || len(s.businessNoticeEvents) != 1 || len(s.businessNoticeTasks) != 1 {
		t.Fatal("retry created another submission or notice")
	}
	event := s.businessNoticeEvents[0]
	task := s.businessNoticeTasks[0]
	if event.Kind != learning.NoticeHomeworkSubmitted || event.RelatedID != first.ID || task.Values["time4"] != first.CreatedAt || task.Values["thing2"] != first.TaskTitle || task.Values["thing6"] == "" {
		t.Fatalf("notice does not describe actual submission: %#v", task)
	}
	detail, err := s.BusinessNoticeDetail(learning.Principal{StudentID: p.StudentID, GuardianID: "submission-parent"}, event.ID)
	if err != nil || detail.Event.RelatedID != first.ID {
		t.Fatalf("wrong child result detail: %v", err)
	}
	req.Answers[0].Choice = "B"
	if _, err := s.CreateSubmission("test", p, req); err == nil {
		t.Fatal("reused request ID accepted different payload")
	}
	req.RequestID = "attempt-2"
	second, err := s.CreateSubmission("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID || len(s.businessNoticeEvents) != 2 || len(s.businessNoticeTasks) != 2 {
		t.Fatal("intentional resubmission was suppressed")
	}
}
func TestBusinessSubmissionDefaultOffAndFailedSave(t *testing.T) {
	s, p := submissionNoticeFixture(t, false)
	invalid := submissionNoticeRequest("invalid")
	invalid.Answers = nil
	if _, err := s.CreateSubmission("test", p, invalid); err == nil {
		t.Fatal("empty submission saved")
	}
	if len(s.businessNoticeEvents) != 0 {
		t.Fatal("failed save generated notice")
	}
	if _, err := s.CreateSubmission("test", p, submissionNoticeRequest("valid")); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != 1 || len(s.businessNoticeTasks) != 0 {
		t.Fatal("default off emitted external task or lost station record")
	}
	for _, notice := range s.notices {
		if notice.RelatedID == s.businessNoticeEvents[0].ID && notice.Type != "练" {
			t.Fatal("submission categorized as course")
		}
	}
}
func TestBusinessSubmissionNoticeRevalidatesHomeworkAccess(t *testing.T) {
	s, p := submissionNoticeFixture(t, true)
	if _, err := s.CreateSubmission("test", p, submissionNoticeRequest("access")); err != nil {
		t.Fatal(err)
	}
	task := s.businessNoticeTasks[0]
	for i := range s.homework {
		if s.homework[i].ID == "hw-g05-english-s1-q1" {
			s.homework[i].Status = string(learning.StatusDraft)
		}
	}
	if s.businessTaskValidity(task, time.Now()) == "" {
		t.Fatal("revoked homework access did not invalidate queued notice")
	}
}

func TestBusinessSubmissionMySQLAtomicRetryAndReload(t *testing.T) {
	dsn := os.Getenv("STARLINE_BUSINESS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated MySQL DSN not configured")
	}
	if !strings.Contains(dsn, "/starline_business_test?") {
		t.Fatal("requires disposable starline_business_test")
	}
	base := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := base.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer base.db.Close()

	// Keep each run independent; startup deliberately preserves historical outbox rows.
	for _, table := range []string{"business_notice_tasks", "business_notice_events", "business_notice_receipts", "business_schedule_snapshots"} {
		if _, err := base.db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	s, p := submissionNoticeFixture(t, true)
	s.guardians[0].Phone = "17900000001"
	for i := range s.grants {
		s.grants[i].OpenedAt = "2026-10-01 08:00:00"
	}
	s.db = base.db
	if err := s.bootstrapPersistAll(); err != nil {
		t.Fatal(err)
	}
	req := submissionNoticeRequest("mysql-attempt")
	first, err := s.CreateSubmission("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.loadSubmissionsFromDB(); err != nil {
		t.Fatal(err)
	}
	if err := s.loadBusinessNoticesFromDB(); err != nil {
		t.Fatal(err)
	}
	repeat, err := s.CreateSubmission("test", p, req)
	if err != nil || first.ID != repeat.ID {
		t.Fatalf("reloaded retry: %v", err)
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM student_submission_results WHERE student_id=? AND request_id=?", p.StudentID, req.RequestID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate submissions: %d %v", count, err)
	}

	// Two independent API stores start from the same stale state.
	a, b := s.cloneForMutation(), s.cloneForMutation()
	a.db, b.db = s.db, s.db
	concurrentReq := submissionNoticeRequest("mysql-concurrent")
	var wg sync.WaitGroup
	results := make(chan learning.Submission, 2)
	errs := make(chan error, 2)
	for _, instance := range []*MemoryStore{a, b} {
		wg.Add(1)
		go func(instance *MemoryStore) {
			defer wg.Done()
			result, err := instance.CreateSubmission("test", p, concurrentReq)
			results <- result
			errs <- err
		}(instance)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var saved learning.Submission
	for result := range results {
		if saved.ID != "" && (result.ID != saved.ID || result.CreatedAt != saved.CreatedAt) {
			t.Fatal("concurrent retry returned a different record")
		}
		saved = result
	}
	for _, check := range []struct {
		query string
		id    string
	}{
		{"SELECT COUNT(*) FROM student_submission_results WHERE id=?", saved.ID},
		{"SELECT COUNT(*) FROM pending_reviews WHERE submission_id=?", saved.ID},
		{"SELECT COUNT(*) FROM business_notice_events WHERE id=?", businessNoticeHash(learning.NoticeHomeworkSubmitted, saved.ID)},
		{"SELECT COUNT(*) FROM business_notice_tasks WHERE event_id=?", businessNoticeHash(learning.NoticeHomeworkSubmitted, saved.ID)},
	} {
		if err := s.db.QueryRow(check.query, check.id).Scan(&count); err != nil || count != 1 {
			t.Fatalf("concurrent outbox count: %d %v (%s)", count, err, check.query)
		}
	}
	// A fresh instance carrying a conflicting payload must not overwrite the winner.
	stale := s.cloneForMutation()
	stale.db = s.db
	conflict := submissionNoticeRequest("mysql-concurrent")
	conflict.Answers[0].Choice = "B"
	if _, err := stale.CreateSubmission("test", p, conflict); err == nil {
		t.Fatal("stale instance overwrote request ID")
	}
	if err := s.loadSubmissionsFromDB(); err != nil {
		t.Fatal(err)
	}
	if s.submissions[saved.ID].Answers[0].Choice != "A" {
		t.Fatal("original answer overwritten")
	}

	upperReq, lowerReq := submissionNoticeRequest("mysql-Case"), submissionNoticeRequest("mysql-case")
	upper, err := s.CreateSubmission("test", p, upperReq)
	if err != nil {
		t.Fatal(err)
	}
	lower, err := s.CreateSubmission("test", p, lowerReq)
	if err != nil {
		t.Fatal(err)
	}
	if upper.ID == lower.ID {
		t.Fatal("case-sensitive request IDs collapsed")
	}

	if _, err := s.db.Exec("DROP TRIGGER IF EXISTS business_test_fail"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("CREATE TRIGGER business_test_fail BEFORE INSERT ON business_notice_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced failure'"); err != nil {
		t.Fatal(err)
	}
	defer s.db.Exec("DROP TRIGGER IF EXISTS business_test_fail")
	req.RequestID = "mysql-rollback"
	if _, err := s.CreateSubmission("test", p, req); err == nil {
		t.Fatal("outbox failure accepted submission")
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM student_submission_results WHERE request_id=?", req.RequestID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("submission persisted despite rollback: %d %v", count, err)
	}
	for _, sub := range s.submissions {
		if sub.RequestID == req.RequestID {
			t.Fatal("failed submission published in memory")
		}
	}

	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	finalReq := learning.ReviewCompleteRequest{Score: 90, TeacherComment: "复核完成", FinalStatus: "已批改"}
	if _, err := s.CompleteReview("test", teacher, "rev-001", finalReq); err == nil {
		t.Fatal("failed completion event committed review")
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pending_reviews WHERE id='rev-001'").Scan(&count); err != nil || count != 1 {
		t.Fatal("review removed despite failed outbox")
	}
	if _, err := s.db.Exec("DROP TRIGGER IF EXISTS business_test_fail"); err != nil {
		t.Fatal(err)
	}
	completed, err := s.CompleteReview("test", teacher, "rev-001", finalReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.loadBusinessNoticesFromDB(); err != nil {
		t.Fatal(err)
	}
	eventID := businessNoticeHash(learning.NoticeReviewCompleted, completed.ID)
	if event, ok := s.businessEvent(eventID); !ok || event.Kind != learning.NoticeReviewCompleted || event.RelatedID != completed.ID {
		t.Fatal("completion event lost after reload")
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM notices WHERE related_id=? AND recipient_student_id=? AND channel='站内通知'", eventID, completed.StudentID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("completion station record lost: %d %v", count, err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pending_reviews WHERE id='rev-001'").Scan(&count); err != nil || count != 0 {
		t.Fatal("completed review not removed")
	}
}
