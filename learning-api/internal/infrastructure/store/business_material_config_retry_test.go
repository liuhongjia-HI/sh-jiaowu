package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func TestMaterialNoticeCanRetryDefiniteConfigurationRejectionAfterRepair(t *testing.T) {
	s, p, req := materialBusinessFixture(t, true)
	if _, err := s.CreateMaterial("test", p, req); err != nil {
		t.Fatal(err)
	}
	attachMaterialParent(s, s.notices[0].RecipientStudentID)
	if _, err := s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
		t.Fatal(err)
	}
	stationBefore := len(s.notices)
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) {
		return "", &officialSendError{reason: "小程序链接生成失败（40165）", configuration: true}
	}
	if err := s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	rejected := s.businessNoticeTasks[0]
	if rejected.Status != "配置错误" || !s.businessTaskCanRetry(rejected, time.Now()) {
		t.Fatalf("definite rejection cannot be repaired: %+v", rejected)
	}
	for _, status := range []string{"结果待确认", "微信已受理", "已送达"} {
		prior := rejected
		prior.Status = status
		if s.businessTaskCanRetry(prior, time.Now()) {
			t.Fatalf("unsafe retry for %s", status)
		}
	}
	prior := rejected
	prior.MessageID = "already-accepted"
	if s.businessTaskCanRetry(prior, time.Now()) {
		t.Fatal("accepted message retried as configuration rejection")
	}
	if _, err := s.RetryBusinessNotice("test", rejected.ID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.officialMessageSender = func(r learning.OfficialMessageRequest) (string, error) { calls++; return "repaired-message", nil }
	if err := s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || s.businessNoticeTasks[0].Status != "微信已受理" || len(s.notices) != stationBefore {
		t.Fatalf("retry duplicated station notice or delivery: %+v", s.businessNoticeTasks)
	}
}
