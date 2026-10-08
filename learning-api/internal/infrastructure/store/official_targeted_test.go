package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/infrastructure/config"
	"starline/learning-api/internal/infrastructure/logger"
	"starline/learning-api/internal/interfaces/http/middleware"
	"starline/learning-api/internal/interfaces/http/router"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func targetedOfficialFixture() *MemoryStore {
	s := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	s.settings["academicYear"] = "2026.2027学年"
	s.students = []learning.Student{{ID: "s1", Name: "测试学生", EnrollmentAcademicYear: "2025.2026学年", EnrollmentGrade: "四年级", AccountStatus: "正常"}}
	s.guardians = []learning.Guardian{{ID: "g1", Name: "测试家长一", Phone: "17900000001", UnionID: "u1", AccountStatus: "正常"}, {ID: "g2", Name: "测试家长二", Phone: "17900000002", UnionID: "u2", AccountStatus: "正常"}}
	s.guardianStudents = []learning.GuardianStudent{{GuardianID: "g1", StudentID: "s1", Status: learning.GuardianStudentActive}, {GuardianID: "g2", StudentID: "s1", Status: learning.GuardianStudentActive}}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "o1", UnionID: "u1", Subscribed: true}, {OpenID: "o2", UnionID: "u2", Subscribed: true}}
	s.officialTemplates = []learning.OfficialTemplate{{ID: "fixture", Title: "测试通知", Content: "内容:{{thing1.DATA}}", Fields: parseOfficialTemplateFields("内容:{{thing1.DATA}}"), Status: "启用"}}
	return s
}
func targetedOfficialRequest(id string) learning.OfficialCampaignCreateRequest {
	return learning.OfficialCampaignCreateRequest{RecipientMode: "specified", GuardianIDs: []string{"g1"}, TemplateID: "fixture", Values: map[string]string{"thing1": "推送联调测试"}, PagePath: "pages/notices/index", RequestID: id}
}

func TestOfficialTargetedAudienceIsolationAndDerivedGrade(t *testing.T) {
	s := targetedOfficialFixture()
	preview, targets, err := s.officialAudienceUnlocked([]string{"五年级"})
	if err != nil || preview.StudentCount != 1 || len(targets) != 2 {
		t.Fatal("derived grade preview failed")
	}
	preview, targets, err = s.officialSelectedAudienceUnlocked(learning.OfficialAudiencePreviewRequest{RecipientMode: "specified", GuardianIDs: []string{"g1"}})
	if err != nil || preview.ReachableCount != 1 || targets[0].GuardianID != "g1" {
		t.Fatal("single guardian selection broadened")
	}
	if s.students[0].Grade != "" {
		t.Fatal("preview wrote derived grade into raw record")
	}
	for _, req := range []learning.OfficialAudiencePreviewRequest{{RecipientMode: "specified"}, {RecipientMode: "specified", GuardianIDs: []string{"g1", "g2"}}, {RecipientMode: "specified", GuardianIDs: []string{"unknown"}}, {Grades: []string{"五年级"}, GuardianIDs: []string{"g1"}}, {RecipientMode: "unknown", Grades: []string{"五年级"}}} {
		if _, _, err = s.officialSelectedAudienceUnlocked(req); err == nil {
			t.Fatal("invalid selection silently accepted")
		}
	}
	lookup, err := s.LookupOfficialRecipient("17900000001")
	if err != nil || !lookup.Reachable || lookup.GuardianID != "g1" {
		t.Fatal("phone lookup failed")
	}
	s.officialFollowers[0].Subscribed = false
	lookup, err = s.LookupOfficialRecipient("17900000001")
	if err != nil || lookup.Reachable || lookup.Reason == "" {
		t.Fatal("unsubscribed account shown ready")
	}
	s.officialFollowers[0].Subscribed = true
	s.officialFollowers = append(s.officialFollowers, learning.OfficialFollower{OpenID: "other", UnionID: "u1", Subscribed: true})
	if _, reason := s.reachableOfficialGuardian("g1"); !strings.Contains(reason, "不唯一") {
		t.Fatal("ambiguous identity accepted")
	}
}

func TestOfficialTargetedRequestDedupeAndClaimRecovery(t *testing.T) {
	s := targetedOfficialFixture()
	req := targetedOfficialRequest("same-request")
	campaign, err := s.createOfficialCampaignUnlocked("test", req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.createOfficialCampaignUnlocked("test", req)
	if err != nil || again.ID != campaign.ID || len(s.officialCampaignRecipients) != 1 {
		t.Fatal("request replay duplicated targets")
	}
	req.Values["thing1"] = "changed"
	if _, err = s.createOfficialCampaignUnlocked("test", req); err == nil {
		t.Fatal("same request id allowed changed content")
	}
	sends := 0
	s.officialMessageSender = func(request learning.OfficialMessageRequest) (string, error) {
		sends++
		if request.OpenID != "o1" {
			t.Fatal("wrong guardian received")
		}
		return "", &officialSendError{reason: "timeout", uncertain: true}
	}
	if err = s.processOfficialCampaign(campaign.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = s.processOfficialCampaign(campaign.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if sends != 1 || s.officialCampaignRecipients[0].Status != "结果待确认" {
		t.Fatal("uncertain result resent")
	}
	if _, err = s.RetryOfficialCampaign("test", campaign.ID); err == nil {
		t.Fatal("uncertain campaign retry allowed")
	}
	s.officialCampaignRecipients[0].Status = "发送中"
	s.officialCampaignRecipients[0].ClaimedAt = businessTime(time.Now().Add(-2 * time.Minute))
	if err = s.processOfficialCampaign(campaign.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 1 || s.officialCampaignRecipients[0].Status != "结果待确认" {
		t.Fatal("abandoned claim blindly resent")
	}
}

func TestOfficialTargetedEarlyReceiptAndRevokedTarget(t *testing.T) {
	s := targetedOfficialFixture()
	c, err := s.createOfficialCampaignUnlocked("test", targetedOfficialRequest("receipt"))
	if err != nil {
		t.Fatal(err)
	}
	s.officialMessageSender = func(request learning.OfficialMessageRequest) (string, error) {
		if err := s.HandleOfficialDeliveryReceipt(request.OpenID, "fixture-message", "success"); err != nil {
			t.Fatal(err)
		}
		return "fixture-message", nil
	}
	if err = s.processOfficialCampaign(c.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if s.officialCampaignRecipients[0].Status != "已送达" {
		t.Fatal("early receipt lost")
	}
	if err = s.HandleOfficialDeliveryReceipt("o1", "fixture-message", "failed:user block"); err != nil {
		t.Fatal(err)
	}
	if s.officialCampaignRecipients[0].Status != "已送达" {
		t.Fatal("old failure replaced delivered status")
	}
	if err = s.HandleOfficialDeliveryReceipt("o2", "fixture-message", "success"); err == nil {
		t.Fatal("mismatched receipt accepted")
	}
	c, err = s.createOfficialCampaignUnlocked("test", targetedOfficialRequest("revoked"))
	if err != nil {
		t.Fatal(err)
	}
	s.officialFollowers[0].Subscribed = false
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { t.Fatal("revoked target sent"); return "", nil }
	if err = s.processOfficialCampaign(c.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if s.officialCampaignRecipients[1].Status != "发送失败" {
		t.Fatal("revoked target not blocked")
	}
}

func TestOfficialTargetedMySQLAtomicReloadAndConcurrentDelivery(t *testing.T) {
	dsn := os.Getenv("STARLINE_BUSINESS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated test database not configured")
	}
	if !strings.Contains(dsn, "/starline_business_test?") {
		t.Fatal("requires isolated database")
	}
	base := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := base.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer base.db.Close()
	s := targetedOfficialFixture()
	s.db = base.db
	if err := s.bootstrapPersistAll(); err != nil {
		t.Fatal(err)
	}
	operator := middleware.AuditOperatorLabel("测试教务", "test-admin", "127.0.0.1", strings.Repeat("Chrome browser user agent ", 20))
	c, err := s.createOfficialCampaignUnlocked(operator, targetedOfficialRequest("mysql"))
	if err != nil {
		t.Fatal(err)
	}
	copyStore := targetedOfficialFixture()
	copyStore.db = s.db
	if err = copyStore.loadOfficialMessagingFromDB(); err != nil {
		t.Fatal(err)
	}
	again, err := copyStore.createOfficialCampaignUnlocked(middleware.AuditOperatorLabel("测试教务", "test-admin", "127.0.0.2", "different browser"), targetedOfficialRequest("mysql"))
	if err != nil || again.ID != c.ID || again.CreatedBy != "测试教务" || again.RecipientMode != "specified" || again.RequestDigest == "" || len(copyStore.officialCampaignRecipients) != 1 {
		t.Fatal("selection or dedupe metadata lost on reload")
	}
	var sends atomic.Int32
	sender := func(req learning.OfficialMessageRequest) (string, error) {
		if req.OpenID != "o1" {
			return "", errors.New("wrong target")
		}
		sends.Add(1)
		return "mysql-message", nil
	}
	s.officialMessageSender = sender
	copyStore.officialMessageSender = sender
	var wg sync.WaitGroup
	for _, store := range []*MemoryStore{s, copyStore} {
		wg.Add(1)
		go func(store *MemoryStore) {
			defer wg.Done()
			if err := store.processOfficialCampaign(c.ID, time.Now()); err != nil {
				t.Error(err)
			}
		}(store)
	}
	wg.Wait()
	if sends.Load() != 1 {
		t.Fatal("concurrent processes duplicated send")
	}
	if err = copyStore.loadOfficialMessagingFromDB(); err != nil {
		t.Fatal(err)
	}
	if copyStore.officialCampaignRecipients[0].MessageID != "mysql-message" || copyStore.officialCampaignRecipients[0].Status != "微信已受理" {
		t.Fatal("delivery result did not persist")
	}
	if _, err = s.db.Exec("CREATE TRIGGER official_notice_test_fail BEFORE INSERT ON official_message_campaigns FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced rollback'"); err != nil {
		t.Fatal(err)
	}
	defer s.db.Exec("DROP TRIGGER IF EXISTS official_notice_test_fail")
	s.mu.Lock()
	_, err = s.createOfficialCampaignUnlocked("test", targetedOfficialRequest("rollback"))
	s.mu.Unlock()
	if err == nil {
		t.Fatal("forced persistence failure ignored")
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM official_message_campaigns").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || sends.Load() != 1 {
		t.Fatal("failed creation published task or sent")
	}
}

func TestOfficialTargetedLocalHTTPDeliveryAndReceipt(t *testing.T) {
	s := targetedOfficialFixture()
	s.users = NewMemoryStore().users
	var sends atomic.Int32
	s.officialMessageSender = func(req learning.OfficialMessageRequest) (string, error) {
		if req.OpenID != "o1" {
			return "", errors.New("wrong target")
		}
		sends.Add(1)
		return "local-http-message", nil
	}
	cfg := config.MustLoad()
	cfg.App.Env = "test"
	cfg.Auth.TokenSecret = "local-http-targeted-fixture"
	cfg.Demo.AdminPasswordLogin = true
	server := httptest.NewServer(router.New(router.Dependencies{Config: cfg, Logger: logger.New("test"), Service: learningapp.NewService(s)}))
	defer server.Close()
	token := ""
	call := func(method, path string, body any) json.RawMessage {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+"/api"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope struct {
			Code    int
			Message string
			Data    json.RawMessage
		}
		if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || envelope.Code != 0 {
			t.Fatalf("%s failed: %d %s", path, res.StatusCode, envelope.Message)
		}
		return envelope.Data
	}
	var auth struct{ Token string }
	json.Unmarshal(call("POST", "/auth/admin-password-login", map[string]string{"phone": "13800000003", "password": demoLoginPassword}), &auth)
	token = auth.Token
	var c learning.OfficialCampaign
	json.Unmarshal(call("POST", "/official-account/campaigns", targetedOfficialRequest("http")), &c)
	call("POST", "/official-account/campaigns", targetedOfficialRequest("http"))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		detail, _ := s.OfficialCampaign(c.ID)
		if len(detail.Recipients) == 1 && detail.Recipients[0].MessageID == "local-http-message" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sends.Load() != 1 {
		t.Fatal("HTTP retry duplicated delivery")
	}
	if err := s.HandleOfficialDeliveryReceipt("o1", "local-http-message", "success"); err != nil {
		t.Fatal(err)
	}
	var detail learning.OfficialCampaignDetail
	json.Unmarshal(call("GET", "/official-account/campaigns/"+c.ID, nil), &detail)
	if detail.Campaign.TargetCount != 1 || len(detail.Recipients) != 1 || detail.Recipients[0].Status != "已送达" {
		t.Fatal("HTTP detail did not retain single target and delivery receipt")
	}
}

func TestOfficialTargetedBusinessGuardianTrialScope(t *testing.T) {
	s := businessFixture(t)
	b := s.bindingForBusinessKind(learning.NoticeScheduleConfirmed)
	b.StudentIDs = []string{"s1"}
	b.GuardianIDs = []string{"g1"}
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err != nil {
		t.Fatal(err)
	}
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("guardian-trial", "series")}
	})
	if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].GuardianID != "g1" || s.businessNoticeTasks[0].StudentID != "s1" {
		t.Fatal("business trial scope broadened to other parent or student")
	}
}

func TestOfficialCampaignAuditOperatorStoresNameAndPreservesLog(t *testing.T) {
	for _, draft := range []bool{false, true} {
		s := targetedOfficialFixture()
		ua := strings.Repeat("Chrome browser user agent ", 20)
		operator := middleware.AuditOperatorLabel("测试教务", "test-admin", "127.0.0.1", ua)
		req := targetedOfficialRequest("audit-operator")
		req.Draft = draft
		campaign, err := s.createOfficialCampaignUnlocked(operator, req)
		if err != nil {
			t.Fatal(err)
		}
		if campaign.CreatedBy != "测试教务" {
			t.Fatal("campaign creator must contain name rather than encoded audit payload")
		}
		log := s.logs[0]
		if log.Operator != "测试教务" || log.OperatorID != "test-admin" || log.IP != "127.0.0.1" || log.UserAgent != strings.TrimSpace(ua) {
			t.Fatal("operator audit metadata was lost")
		}
		if !draft {
			again, err := s.createOfficialCampaignUnlocked(middleware.AuditOperatorLabel("测试教务", "test-admin", "127.0.0.2", "different browser"), req)
			if err != nil || again.ID != campaign.ID || len(s.officialCampaigns) != 1 {
				t.Fatal("browser or IP change duplicated request")
			}
		}
	}
}

func TestOfficialTargetedExceptionReasonRequiresApprovedExactValue(t *testing.T) {
	s := targetedOfficialFixture()
	s.officialTemplates[0].Fields = parseOfficialTemplateFields("内容:{{thing1.DATA}}\n异常原因:{{const2.DATA}}")
	req := targetedOfficialRequest("exception-config")
	req.Values["const2"] = "测试"
	if _, err := s.createOfficialCampaignUnlocked("ops", req); err == nil || !strings.Contains(err.Error(), "微信固定选项") {
		t.Fatalf("missing approval was accepted: %v", err)
	}
	if len(s.officialCampaigns) != 0 || len(s.officialCampaignRecipients) != 0 {
		t.Fatal("invalid reason created a send task")
	}
	req.Draft = true
	if _, err := s.createOfficialCampaignUnlocked("ops", req); err != nil {
		t.Fatal("blocked saving a draft", err)
	}
	s.settings[businessNoticeSettingsKey] = mustJSON([]learning.BusinessNoticeBinding{{Kind: learning.NoticeReviewException, TemplateID: "fixture", ApprovedReasons: []string{"已审核选项"}}})
	req.Draft = false
	for _, reason := range []string{"测试", "模板示例", "已审核选项 "} {
		req.Values["const2"] = reason
		if _, err := s.createOfficialCampaignUnlocked("ops", req); err == nil {
			t.Fatal("unapproved exact value accepted")
		}
	}
	req.Values["const2"] = "已审核选项"
	if _, err := s.createOfficialCampaignUnlocked("ops", req); err != nil {
		t.Fatal("approved value rejected", err)
	}
	if len(s.officialCampaignRecipients) != 1 {
		t.Fatal("approved single recipient send was broadened")
	}
}

func TestOfficialTargetedFullFailureStatusAndLegacyRead(t *testing.T) {
	s := targetedOfficialFixture()
	s.officialCampaigns = []learning.OfficialCampaign{{ID: "all", TargetCount: 1, FailureCount: 1, Status: "部分失败", SentAt: "historical"}, {ID: "partial", TargetCount: 2, SuccessCount: 1, FailureCount: 1, Status: "部分失败"}}
	s.officialCampaignRecipients = []learning.OfficialCampaignRecipient{{ID: "failed", CampaignID: "all", Status: "发送失败", FailureReason: "微信明确拒绝", Retryable: false}}
	rows := s.OfficialCampaigns()
	for _, c := range rows {
		if c.ID == "all" && (c.Status != "发送失败" || c.SentAt != "historical") {
			t.Fatal("legacy output not corrected")
		}
		if c.ID == "partial" && c.Status != "部分失败" {
			t.Fatal("partial failure was lost")
		}
	}
	detail, err := s.OfficialCampaign("all")
	if err != nil || detail.Campaign.Status != "发送失败" || detail.Recipients[0].FailureReason != "微信明确拒绝" {
		t.Fatal("detail lost failure evidence")
	}
	if s.officialCampaigns[0].Status != "部分失败" || s.officialCampaigns[0].SentAt != "historical" {
		t.Fatal("read rewrote history")
	}
	s.updateOfficialCampaignDelivery("all")
	if s.officialCampaigns[0].Status != "发送失败" {
		t.Fatal("new full failure is still partial")
	}
}
