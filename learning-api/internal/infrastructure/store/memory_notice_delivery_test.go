package store

import (
	"database/sql"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestNoticeSenderPanicBecomesPersistedFailureAndCanRetry(t *testing.T) {
	mutationDriverState.reset(false)
	db, err := sql.Open(mutationTestDriverName, "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	store := NewMemoryStore()
	store.db = db
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("load ops: %v", err)
	}
	store.officialNoticeSender = func(learning.Notice) error { panic("sender unavailable") }

	notice, err := store.CreateNotice("通知测试", ops, learning.NoticeCreateRequest{
		Type: "通知", Title: "panic 发送", Target: "测试学生", Summary: "测试",
		Channel: "公众号模板消息", RecipientOpenID: "openid-panic",
	})
	if err != nil {
		t.Fatalf("business request must succeed when sender panics: %v", err)
	}
	if notice.Status != "发送失败" || !strings.Contains(notice.FailureReason, "通知发送异常") {
		t.Fatalf("sender panic was not recorded as a retryable failure: %#v", notice)
	}
	mutationDriverState.mu.Lock()
	commits := mutationDriverState.commits
	mutationDriverState.mu.Unlock()
	if commits != 2 {
		t.Fatalf("panic outcome was not persisted in a second transaction: commits=%d", commits)
	}

	store.officialNoticeSender = func(learning.Notice) error { return nil }
	retried, err := store.RetryNotice("通知测试", ops, notice.ID)
	if err != nil {
		t.Fatalf("retry panic failure: %v", err)
	}
	if retried.Status != "已发送" || retried.FailureReason != "" {
		t.Fatalf("retry did not recover failed delivery: %#v", retried)
	}
}

func TestNoticeOutcomePersistenceFailureKeepsBusinessSuccessAndPendingRetry(t *testing.T) {
	mutationDriverState.reset(false)
	db, err := sql.Open(mutationTestDriverName, "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	store := NewMemoryStore()
	store.db = db
	beforeNoticeCount := len(store.notices)
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("load ops: %v", err)
	}
	sendCount := 0
	store.officialNoticeSender = func(learning.Notice) error {
		sendCount++
		// The first transaction has committed. Fail only the second transaction
		// that records the delivery outcome.
		if sendCount == 1 {
			mutationDriverState.reset(true)
		}
		return nil
	}

	notice, err := store.CreateNotice("通知测试", ops, learning.NoticeCreateRequest{
		Type: "通知", Title: "结果落库失败", Target: "测试学生", Summary: "测试",
		Channel: "公众号模板消息", RecipientOpenID: "openid-persist-failure",
	})
	if err != nil {
		t.Fatalf("committed business request must not fail because outcome persistence failed: %v", err)
	}
	if notice.ID == "" || notice.Status != "发送中" {
		t.Fatalf("expected first-phase result to be returned, got %#v", notice)
	}
	if len(store.notices) != beforeNoticeCount+2 || len(store.pendingNoticeDeliveries) != 1 || store.pendingNoticeDeliveries[0].ID != notice.ID {
		t.Fatalf("delivery was silently lost after outcome persistence failure: notices=%#v pending=%#v", store.notices, store.pendingNoticeDeliveries)
	}

	mutationDriverState.reset(false)
	retried, err := store.RetryNotice("通知测试", ops, notice.ID)
	if err != nil {
		t.Fatalf("retry after outcome persistence failure: %v", err)
	}
	if retried.Status != "已发送" || retried.FailureReason != "" {
		t.Fatalf("retry did not persist a recovered delivery: %#v", retried)
	}
	if len(store.notices) != beforeNoticeCount+2 {
		t.Fatalf("retry duplicated the original business data: notices=%#v", store.notices)
	}
	if len(store.pendingNoticeDeliveries) != 0 {
		t.Fatalf("recovered deliveries should not remain pending: %#v", store.pendingNoticeDeliveries)
	}
	if sendCount != 2 {
		t.Fatalf("expected original send plus one deduplicated retry, got %d", sendCount)
	}
}

func TestSendingNoticeOutboxIsRestoredAfterSecondPhasePersistenceFailure(t *testing.T) {
	store := NewMemoryStore()
	store.notices = []learning.Notice{
		{ID: "notice-restart", Channel: "公众号模板消息", RecipientOpenID: "openid-restart", Status: "发送中"},
		{ID: "notice-restart-station", Channel: "站内通知", Status: "已发送"},
		{ID: "notice-failed", Channel: "公众号模板消息", RecipientOpenID: "openid-failed", Status: "发送失败"},
	}
	sent := 0
	store.officialNoticeSender = func(learning.Notice) error { sent++; return nil }

	store.restorePendingNoticeDeliveries()
	store.restorePendingNoticeDeliveries()
	if len(store.pendingNoticeDeliveries) != 1 || store.pendingNoticeDeliveries[0].ID != "notice-restart" {
		t.Fatalf("startup outbox must restore each send-in-progress notice once, got %#v", store.pendingNoticeDeliveries)
	}

	store.drainPendingNoticeDeliveries()
	if sent != 1 || store.notices[0].Status != "已发送" || store.notices[0].FailureReason != "" {
		t.Fatalf("restored outbox was not delivered and finalized: sent=%d notices=%#v", sent, store.notices)
	}
}

func TestRetryNoticeAllowsSendInProgressRecord(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("load ops: %v", err)
	}
	store.notices = append([]learning.Notice{{
		ID: "notice-sending", Channel: "公众号模板消息", RecipientOpenID: "openid-sending", Status: "发送中", Target: "测试学生", Title: "测试", Summary: "测试",
	}}, store.notices...)
	store.officialNoticeSender = func(learning.Notice) error { return nil }

	notice, err := store.RetryNotice("通知测试", ops, "notice-sending")
	if err != nil {
		t.Fatalf("retry send-in-progress notice: %v", err)
	}
	if notice.Status != "已发送" || notice.RetryCount != 1 {
		t.Fatalf("retry should explicitly recover a send-in-progress notice: %#v", notice)
	}
}
