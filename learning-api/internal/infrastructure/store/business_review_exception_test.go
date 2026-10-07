package store

import (
	"os"
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

func reviewExceptionFixture(t *testing.T) (*MemoryStore, learning.Principal) {
	t.Helper()
	s, p := homeworkNotificationsFixture(t, learning.NoticeReviewCompleted)
	completed := s.bindingForBusinessKind(learning.NoticeReviewCompleted)
	completed.Enabled = false
	if _, err := s.UpdateBusinessNoticeBinding("test", completed); err != nil {
		t.Fatal(err)
	}
	content := "作业：{{thing7.DATA}}\n班级：{{thing5.DATA}}\n异常原因：{{const2.DATA}}"
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "exception-fixture", Title: "隔离批改异常通知", Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
	_, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeReviewException, TemplateID: "exception-fixture", Enabled: true, ApprovedReasons: []string{"作业缺页", "图片不清"}, StudentIDs: []string{"stu-001"}, GuardianIDs: []string{"content-parent"}})
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}
func TestBusinessReviewExceptionExplicitActionDedupeAndResolution(t *testing.T) {
	s, p := reviewExceptionFixture(t)
	original := cloneBusinessNoticeValue(s.submissions)
	req := learning.ReviewExceptionRequest{RequestID: "incident-1", ClassName: "隔离测试班", Reason: "作业缺页"}
	review, err := s.MarkReviewException("test", p, "rev-001", req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.submissions, original) {
		t.Fatal("exception fabricated a result or changed scores")
	}
	if review.Status != "批改异常" || len(s.businessNoticeTasks) != 1 {
		t.Fatal("explicit exception did not queue exactly one target")
	}
	task := s.businessNoticeTasks[0]
	if task.GuardianID != "content-parent" || task.Values["thing5"] != req.ClassName || task.Values["const2"] != req.Reason {
		t.Fatal("incorrect target or semantic values")
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "" {
		t.Fatal(reason)
	}
	if _, err = s.MarkReviewException("test", p, "rev-001", req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatal("repeated exception duplicated task")
	}
	if _, err = s.BusinessNoticeDetail(learning.Principal{GuardianID: "content-parent"}, review.ExceptionEventID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BusinessNoticeDetail(learning.Principal{StudentID: "other-child"}, review.ExceptionEventID); err == nil {
		t.Fatal("another child saw exception")
	}
	_, err = s.CompleteReview("test", p, "rev-001", learning.ReviewCompleteRequest{Score: 80, TeacherComment: "待复核反馈", FinalStatus: "待复核"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 || s.reviews[0].ExceptionEventID != "" {
		t.Fatal("recheck pushed an exception or failed to clear it")
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason == "" {
		t.Fatal("resolved exception remains deliverable")
	}
	if _, err = s.BusinessNoticeDetail(learning.Principal{GuardianID: "content-parent"}, review.ExceptionEventID); err == nil {
		t.Fatal("resolved link not invalidated")
	}
	if _, err = s.MarkReviewException("test", p, "rev-001", req); err == nil {
		t.Fatal("old request reactivated resolved exception")
	}
	req.RequestID = "incident-2"
	if _, err = s.MarkReviewException("test", p, "rev-001", req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 2 || s.businessNoticeTasks[1].EventID == task.EventID {
		t.Fatal("new incident reused resolved event")
	}
}
func TestBusinessReviewExceptionRejectsBadReasonScopeAndChangedIncident(t *testing.T) {
	s, p := reviewExceptionFixture(t)
	req := learning.ReviewExceptionRequest{RequestID: "incident-1", ClassName: "隔离测试班", Reason: "作业缺页"}
	bad := req
	bad.Reason = "未审核原因"
	if _, err := s.MarkReviewException("test", p, "rev-001", bad); err == nil {
		t.Fatal("unapproved reason accepted")
	}
	bad = req
	bad.ClassName = ""
	if _, err := s.MarkReviewException("test", p, "rev-001", bad); err == nil {
		t.Fatal("invented missing class")
	}
	outsider := p
	outsider.UserID = "other-user"
	if _, err := s.MarkReviewException("test", outsider, "rev-001", req); err == nil {
		t.Fatal("teacher accessed unassigned review")
	}
	if len(s.businessNoticeEvents) != 0 {
		t.Fatal("invalid operation created event")
	}
	review, err := s.MarkReviewException("test", p, "rev-001", req)
	if err != nil {
		t.Fatal(err)
	}
	old := s.businessNoticeTasks[0]
	changed := req
	changed.Reason = "图片不清"
	if _, err = s.MarkReviewException("test", p, "rev-001", changed); err == nil {
		t.Fatal("same request ID changed content")
	}
	req.RequestID = "incident-2"
	req.Reason = "图片不清"
	if _, err = s.MarkReviewException("test", p, "rev-001", req); err != nil {
		t.Fatal(err)
	}
	if reason := s.businessTaskValidity(old, time.Now()); reason == "" {
		t.Fatal("superseded incident still valid")
	}
	if _, valid := s.currentReviewException(learning.BusinessNoticeEvent{ID: review.ExceptionEventID, RelatedID: review.ID, StudentID: review.StudentID}); valid {
		t.Fatal("old version remains current")
	}
	binding := s.bindingForBusinessKind(learning.NoticeReviewException)
	binding.ApprovedReasons = []string{"作业缺页"}
	if _, err = s.UpdateBusinessNoticeBinding("test", binding); err != nil {
		t.Fatal(err)
	}
	if reason := s.businessTaskValidity(s.businessNoticeTasks[1], time.Now()); reason == "" {
		t.Fatal("withdrawn approved reason still sends")
	}
	sends := 0
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sends++; return "unexpected", nil }
	if err = s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 0 {
		t.Fatal("invalid incident sent")
	}
}
func TestBusinessReviewExceptionMySQLReloadAndFailedWrite(t *testing.T) {
	dsn := os.Getenv("STARLINE_BUSINESS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("isolated database not configured")
	}
	if !strings.Contains(dsn, "/starline_business_test?") {
		t.Fatal("dedicated database required")
	}
	base := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	err := base.ConnectDatabase(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer base.db.Close()
	s, p := reviewExceptionFixture(t)
	s.guardians[0].Phone = "17700000001"
	s.guardians[1].Phone = "17700000002"
	for i := range s.grants {
		s.grants[i].OpenedAt = "2026-10-08 09:00:00"
	}
	s.db = base.db
	if err = s.bootstrapPersistAll(); err != nil {
		t.Fatal(err)
	}
	req := learning.ReviewExceptionRequest{RequestID: "incident-1", ClassName: "隔离测试班", Reason: "作业缺页"}
	review, err := s.MarkReviewException("test", p, "rev-001", req)
	if err != nil {
		t.Fatal(err)
	}
	restored := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	err = restored.ConnectDatabase(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.db.Close()
	current, valid := restored.currentReviewException(restored.businessNoticeEvents[0])
	if !valid || current.ExceptionEventID != review.ExceptionEventID || current.ExceptionReason != req.Reason || current.ExceptionClass != req.ClassName {
		t.Fatal("exception metadata did not reload")
	}
	count := len(restored.businessNoticeTasks)
	if _, err = restored.MarkReviewException("test", p, "rev-001", req); err != nil {
		t.Fatal(err)
	}
	if len(restored.businessNoticeTasks) != count {
		t.Fatal("reload allowed duplicate")
	}
	_, err = restored.db.Exec("CREATE TRIGGER reject_exception_update BEFORE UPDATE ON pending_reviews FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='isolated failure'")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.db.Exec("DROP TRIGGER IF EXISTS reject_exception_update")
	req.RequestID = "incident-2"
	req.Reason = "图片不清"
	if _, err = restored.MarkReviewException("test", p, "rev-001", req); err == nil {
		t.Fatal("failed write accepted")
	}
	if len(restored.businessNoticeTasks) != count || restored.reviews[0].ExceptionReason != "作业缺页" {
		t.Fatal("failed write leaked event or review")
	}
}
