package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/infrastructure/config"
	"starline/learning-api/internal/infrastructure/logger"
	"starline/learning-api/internal/infrastructure/store"
	"starline/learning-api/internal/interfaces/http/router"
)

type testApp struct {
	server *httptest.Server
	store  *store.MemoryStore
}

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type authResponse struct {
	Token string             `json:"token"`
	User  learning.Principal `json:"user"`
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	repo := store.NewMemoryStore()
	cfg := config.MustLoad()
	cfg.App.Env = "test"
	cfg.Auth.TokenSecret = "router-test-secret"
	cfg.Demo.AdminPasswordLogin = true
	cfg.Demo.StudentPasswordLogin = true
	service := learningapp.NewService(repo)
	engine := router.New(router.Dependencies{
		Config:  cfg,
		Logger:  logger.New("test"),
		Service: service,
	})
	return &testApp{server: httptest.NewServer(engine), store: repo}
}

func (a *testApp) close() {
	a.server.Close()
}

func (a *testApp) loginAdmin(t *testing.T, phone string) string {
	t.Helper()
	return a.login(t, "/api/auth/admin-password-login", map[string]string{"phone": phone, "password": "123456"})
}

func (a *testApp) loginStudent(t *testing.T) string {
	t.Helper()
	return a.login(t, "/api/auth/demo-student-login", map[string]string{"phone": "18500009069", "password": "123456"})
}

func (a *testApp) login(t *testing.T, path string, body any) string {
	t.Helper()
	var auth authResponse
	a.doJSON(t, http.MethodPost, path, "", body, http.StatusOK, &auth)
	if auth.Token == "" {
		t.Fatalf("expected login token from %s", path)
	}
	return auth.Token
}

func (a *testApp) doJSON(t *testing.T, method, path, token string, body any, wantStatus int, out any) apiResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, a.server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "router-test-agent")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Operator-ID", "test-operator")
		req.Header.Set("X-Operator-Name", "测试操作人")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s status = %d want %d body=%s", method, path, resp.StatusCode, wantStatus, string(raw))
	}
	var envelope apiResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode response envelope: %v body=%s", err, string(raw))
	}
	if out != nil && envelope.Data != nil && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			t.Fatalf("decode response data: %v body=%s", err, string(envelope.Data))
		}
	}
	return envelope
}

func TestCORSPreflightForAdminLogin(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	req, err := http.NewRequest(http.MethodOptions, app.server.URL+"/api/auth/admin-password-login", nil)
	if err != nil {
		t.Fatalf("new preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://sa.starlineeducation.com.cn")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("preflight status = %d want %d body=%s", resp.StatusCode, http.StatusNoContent, string(raw))
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://sa.starlineeducation.com.cn" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Fatalf("Access-Control-Allow-Headers = %q", got)
	}
}

func TestAdminAuthAndPermissionBoundaries(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	app.doJSON(t, http.MethodGet, "/api/students", "", nil, http.StatusUnauthorized, nil)

	studentToken := app.loginStudent(t)
	app.doJSON(t, http.MethodGet, "/api/students", studentToken, nil, http.StatusForbidden, nil)

	teacherToken := app.loginAdmin(t, "13800000004")
	app.doJSON(t, http.MethodPost, "/api/grants", teacherToken, map[string]string{
		"studentId": "stu-001",
		"packageId": "pkg-g05-english-s1-full",
	}, http.StatusForbidden, nil)
}

func TestLoginFailureLockout(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	for i := 0; i < 5; i++ {
		app.doJSON(t, http.MethodPost, "/api/auth/admin-password-login", "", map[string]string{
			"phone":    "13800000001",
			"password": "wrong-password",
		}, http.StatusUnauthorized, nil)
	}
	envelope := app.doJSON(t, http.MethodPost, "/api/auth/admin-password-login", "", map[string]string{
		"phone":    "13800000001",
		"password": "123456",
	}, http.StatusUnauthorized, nil)
	if envelope.Message != "登录失败次数过多，请稍后再试" {
		t.Fatalf("expected lockout message, got %q", envelope.Message)
	}

	adminToken := app.loginAdmin(t, "13800000002")
	var logs []learning.OperationLog
	app.doJSON(t, http.MethodGet, "/api/logs", adminToken, nil, http.StatusOK, &logs)
	foundFailure := false
	for _, item := range logs {
		if item.Action == "后台登录失败" && item.Target == "13800000001" && item.Detail != "" {
			foundFailure = true
			break
		}
	}
	if !foundFailure {
		t.Fatalf("expected failed login audit log, got %#v", logs)
	}
}

func TestAdminLoginRequiresCaptchaAfterRepeatedFailures(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	for i := 0; i < 3; i++ {
		app.doJSON(t, http.MethodPost, "/api/auth/admin-password-login", "", map[string]string{
			"phone":    "13800000002",
			"password": "wrong-password",
		}, http.StatusUnauthorized, nil)
	}
	envelope := app.doJSON(t, http.MethodPost, "/api/auth/admin-password-login", "", map[string]string{
		"phone":    "13800000002",
		"password": "123456",
	}, http.StatusUnauthorized, nil)
	if envelope.Message != "请输入正确验证码" {
		t.Fatalf("expected captcha challenge, got %q", envelope.Message)
	}

	var captcha struct {
		ID       string `json:"captchaId"`
		Question string `json:"question"`
	}
	app.doJSON(t, http.MethodGet, "/api/auth/captcha", "", nil, http.StatusOK, &captcha)
	answer := answerCaptcha(t, captcha.Question)
	token := app.login(t, "/api/auth/admin-password-login", map[string]string{
		"phone":         "13800000002",
		"password":      "123456",
		"captchaId":     captcha.ID,
		"captchaAnswer": answer,
	})
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", token, nil, http.StatusOK, nil)
}

func answerCaptcha(t *testing.T, question string) string {
	t.Helper()
	parts := strings.Fields(question)
	if len(parts) < 3 {
		t.Fatalf("unexpected captcha question: %q", question)
	}
	a, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("parse captcha left operand: %v", err)
	}
	b, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("parse captcha right operand: %v", err)
	}
	return strconv.Itoa(a + b)
}

func TestResetPasswordForcesChangeAndRotatesTokens(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	adminToken := app.loginAdmin(t, "13800000002")

	var reset learning.PasswordResetResult
	app.doJSON(t, http.MethodPost, "/api/teachers/user-teacher/reset-password", adminToken, nil, http.StatusOK, &reset)
	if reset.TemporaryPassword == "" || !reset.MustChangePassword {
		t.Fatalf("unexpected reset response: %#v", reset)
	}

	tempToken := app.login(t, "/api/auth/admin-password-login", map[string]string{
		"phone":    "13800000004",
		"password": reset.TemporaryPassword,
	})
	var me learning.Principal
	app.doJSON(t, http.MethodGet, "/api/auth/me", tempToken, nil, http.StatusOK, &me)
	if !me.MustChangePassword {
		t.Fatalf("expected temporary password login to require password change: %#v", me)
	}
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", tempToken, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/auth/change-password", tempToken, learning.PasswordChangeRequest{
		OldPassword: reset.TemporaryPassword,
		NewPassword: "Teacher2026",
	}, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", tempToken, nil, http.StatusUnauthorized, nil)

	nextToken := app.login(t, "/api/auth/admin-password-login", map[string]string{
		"phone":    "13800000004",
		"password": "Teacher2026",
	})
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", nextToken, nil, http.StatusOK, nil)
}

func TestLogoutRevokesCurrentToken(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	app.doJSON(t, http.MethodPost, "/api/auth/logout", token, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/student/home", token, nil, http.StatusUnauthorized, nil)
}

func TestRefreshTokenRotatesAndRevokesOldToken(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	var refreshed learning.AuthResult
	app.doJSON(t, http.MethodPost, "/api/auth/refresh", token, nil, http.StatusOK, &refreshed)
	if refreshed.Token == "" || refreshed.Token == token {
		t.Fatalf("expected rotated token, got %#v", refreshed)
	}
	app.doJSON(t, http.MethodGet, "/api/student/home", token, nil, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodGet, "/api/student/home", refreshed.Token, nil, http.StatusOK, nil)
}

func TestGrantPreviewAndCreateGrantThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000002")

	var created learning.Package
	app.doJSON(t, http.MethodPost, "/api/packages", token, learning.PackageUpsertRequest{
		Name:             "五年级英语接口测试题包",
		AcademicYear:     "2026 学年",
		Grade:            "五年级",
		Semester:         "第一学期",
		Subject:          "英语",
		PhaseScope:       "期中前",
		PackageType:      "题",
		Summary:          "接口测试专用套餐。",
		LearningSpaceIDs: []string{"space-g05-english-s1-mid"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	}, http.StatusOK, &created)

	var preview learning.GrantPreview
	app.doJSON(t, http.MethodGet, "/api/grants/preview?studentId=stu-001&packageId="+created.ID, token, nil, http.StatusOK, &preview)
	if preview.AlreadyOpened || len(preview.OpenHomework) == 0 {
		t.Fatalf("unexpected grant preview: %#v", preview)
	}

	var opened learning.GrantPreview
	app.doJSON(t, http.MethodPost, "/api/grants", token, map[string]string{
		"studentId": "stu-001",
		"packageId": created.ID,
	}, http.StatusOK, &opened)
	if opened.StudentID != "stu-001" || opened.PackageID != created.ID {
		t.Fatalf("unexpected created grant response: %#v", opened)
	}
	var after learning.GrantPreview
	app.doJSON(t, http.MethodGet, "/api/grants/preview?studentId=stu-001&packageId="+created.ID, token, nil, http.StatusOK, &after)
	if !after.AlreadyOpened {
		t.Fatalf("expected created grant to be marked opened on follow-up preview: %#v", after)
	}

	var updated learning.Package
	app.doJSON(t, http.MethodPut, "/api/packages/"+created.ID, token, learning.PackageUpsertRequest{
		Name:             created.Name,
		AcademicYear:     created.AcademicYear,
		Grade:            created.Grade,
		Semester:         created.Semester,
		Subject:          created.Subject,
		PhaseScope:       created.PhaseScope,
		PackageType:      created.PackageType,
		Summary:          "接口测试专用套餐，已更新。",
		LearningSpaceIDs: created.LearningSpaceIDs,
		ContentTypeCodes: created.ContentTypeCodes,
		Status:           learning.StatusEnabled,
	}, http.StatusOK, &updated)
	if updated.Summary != "接口测试专用套餐，已更新。" {
		t.Fatalf("expected package summary to be updated, got %#v", updated)
	}

	var logs []learning.OperationLog
	app.doJSON(t, http.MethodGet, "/api/logs", token, nil, http.StatusOK, &logs)
	if len(logs) == 0 || logs[0].OperatorID != "test-operator" || logs[0].UserAgent != "router-test-agent" {
		t.Fatalf("expected structured audit metadata on latest log, got %#v", logs)
	}
	if logs[0].Action != "编辑学习套餐" || !strings.Contains(logs[0].Detail, `"before"`) || !strings.Contains(logs[0].Detail, `"after"`) || !strings.Contains(logs[0].Detail, "已更新") {
		t.Fatalf("expected before/after change detail on latest log, got %#v", logs[0])
	}
}

func TestCommercialLifecycleThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000002")

	var pkg learning.Package
	app.doJSON(t, http.MethodPost, "/api/packages", token, learning.PackageUpsertRequest{
		Name:             "五年级英语商业闭环课包",
		AcademicYear:     "2026 学年",
		Grade:            "五年级",
		Semester:         "第一学期",
		Subject:          "英语",
		PhaseScope:       "期中前",
		PackageType:      "题",
		Summary:          "商业闭环测试套餐。",
		LearningSpaceIDs: []string{"space-g05-english-s1-mid"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	}, http.StatusOK, &pkg)

	var order learning.CommercialOrder
	app.doJSON(t, http.MethodPost, "/api/commercial/orders", token, learning.CommercialOrderCreateRequest{
		StudentID:   "stu-001",
		PackageID:   pkg.ID,
		AmountCent:  128000,
		LessonTotal: 10,
	}, http.StatusOK, &order)
	if order.ID == "" || order.Status != "待支付" {
		t.Fatalf("unexpected order: %#v", order)
	}

	var payment learning.PaymentRecord
	app.doJSON(t, http.MethodPost, "/api/commercial/orders/"+order.ID+"/payments", token, learning.PaymentCreateRequest{
		AmountCent:    128000,
		Method:        "微信支付",
		TransactionNo: "wx-test-001",
	}, http.StatusOK, &payment)
	if payment.Status != "已确认" {
		t.Fatalf("unexpected payment: %#v", payment)
	}
	var afterPay learning.GrantPreview
	app.doJSON(t, http.MethodGet, "/api/grants/preview?studentId=stu-001&packageId="+pkg.ID, token, nil, http.StatusOK, &afterPay)
	if !afterPay.AlreadyOpened {
		t.Fatalf("expected paid order to open package grant: %#v", afterPay)
	}

	var contract learning.ContractRecord
	app.doJSON(t, http.MethodPost, "/api/commercial/orders/"+order.ID+"/contracts", token, learning.ContractCreateRequest{
		Title:  "英语专项服务协议",
		Signer: "小明家长",
	}, http.StatusOK, &contract)
	if contract.Status != "已签署" {
		t.Fatalf("unexpected contract: %#v", contract)
	}

	var invoice learning.InvoiceRecord
	app.doJSON(t, http.MethodPost, "/api/commercial/orders/"+order.ID+"/invoices", token, learning.InvoiceCreateRequest{
		Title:      "小明家长",
		AmountCent: 128000,
		InvoiceNo:  "INV-001",
	}, http.StatusOK, &invoice)
	if invoice.Status != "已开票" {
		t.Fatalf("unexpected invoice: %#v", invoice)
	}

	var consumed learning.LessonConsumption
	app.doJSON(t, http.MethodPost, "/api/commercial/lesson-consumptions", token, learning.LessonConsumptionCreateRequest{
		OrderID:     order.ID,
		LessonCount: 8,
		Remark:      "暑期集训课消",
	}, http.StatusOK, &consumed)
	if consumed.LessonCount != 8 {
		t.Fatalf("unexpected lesson consumption: %#v", consumed)
	}

	var reminder learning.RenewalReminder
	app.doJSON(t, http.MethodPost, "/api/commercial/renewal-reminders", token, learning.RenewalReminderCreateRequest{
		OrderID: order.ID,
		Reason:  "剩余 2 课时，建议续费",
		DueAt:   "2026-07-01",
	}, http.StatusOK, &reminder)
	if reminder.Status != "待跟进" {
		t.Fatalf("unexpected renewal reminder: %#v", reminder)
	}

	var notice learning.ParentNotice
	app.doJSON(t, http.MethodPost, "/api/commercial/parent-notices", token, learning.ParentNoticeCreateRequest{
		OrderID: order.ID,
		Title:   "续费提醒",
		Content: "小明的英语课包快用完了，建议提前安排续费。",
	}, http.StatusOK, &notice)
	if notice.Status != "已发送" {
		t.Fatalf("unexpected parent notice: %#v", notice)
	}

	var summary learning.CommercialSummary
	app.doJSON(t, http.MethodGet, "/api/commercial/summary", token, nil, http.StatusOK, &summary)
	if summary.OrderCount == 0 || summary.RevenueCent != 128000 || summary.LessonRemainCount != 2 || summary.RenewalTodoCount == 0 {
		t.Fatalf("unexpected commercial summary: %#v", summary)
	}

	studentToken := app.loginStudent(t)
	app.doJSON(t, http.MethodGet, "/api/commercial/orders", studentToken, nil, http.StatusForbidden, nil)
}

func TestSchedulingCandidateAndCreateClassThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000002")

	var candidates []learning.ScheduleCandidate
	app.doJSON(t, http.MethodPost, "/api/scheduling/candidates", token, learning.ScheduleCandidateRequest{
		Subject:         "英语",
		Grade:           "五年级",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
	}, http.StatusOK, &candidates)
	if len(candidates) == 0 || candidates[0].StudentCount < 1 {
		t.Fatalf("expected 1V1 candidate with available students, got %#v", candidates)
	}

	var class learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", token, learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-mid",
		TeacherID:       "user-teacher",
		ClassType:       "1V1",
		DurationMinutes: 90,
		DayOfWeek:       3,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
		StudentIDs:      []string{"stu-001"},
	}, http.StatusOK, &class)
	if class.ID == "" || len(class.Students) != 1 {
		t.Fatalf("unexpected created class: %#v", class)
	}
}

func TestFileDownloadRequiresVisibleContent(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	source := filepath.Join(t.TempDir(), "material.pdf")
	if err := os.WriteFile(source, []byte("test pdf"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	teacher, err := app.store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("teacher principal: %v", err)
	}
	material, err := app.store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "接口测试讲义",
		CourseID: "course-g05-english-s1-mid",
		File: learning.FileAsset{
			ID:            "file-router-download",
			FileName:      "material.pdf",
			FileSize:      8,
			FileType:      "PDF",
			ContentType:   "application/pdf",
			OriginalPath:  source,
			PreviewPath:   source,
			PreviewStatus: "可预览",
		},
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	token := app.loginAdmin(t, "13800000004")
	req, err := http.NewRequest(http.MethodGet, app.server.URL+material.DownloadURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("download status = %d body=%s", resp.StatusCode, string(body))
	}

	studentToken := app.loginStudent(t)
	app.doJSON(t, http.MethodGet, material.DownloadURL, studentToken, nil, http.StatusForbidden, nil)
}

func TestStudentSubmissionThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	var result struct {
		SubmissionID string `json:"submissionId"`
		Status       string `json:"status"`
		Score        int    `json:"score"`
	}
	app.doJSON(t, http.MethodPost, "/api/student/submissions", token, learning.SubmissionRequest{
		HomeworkID: "hw-g05-english-s1-mid",
		Answers: []learning.SubmissionAnswer{
			{QuestionID: "q1", Choice: "A"},
			{QuestionID: "q2", Text: "今天学会了抓中心句。"},
		},
	}, http.StatusOK, &result)
	if result.SubmissionID == "" || result.Status != "已批改" || result.Score == 0 {
		t.Fatalf("unexpected submission result: %#v", result)
	}
}
