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
	Token      string             `json:"token"`
	User       learning.Principal `json:"user"`
	AuthMethod string             `json:"authMethod"`
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

func TestStudentRecommendations(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	var recommendations []learning.StudentPackageRecommendation
	app.doJSON(t, http.MethodGet, "/api/student/recommendations", app.loginStudent(t), nil, http.StatusOK, &recommendations)
	if len(recommendations) == 0 {
		t.Fatal("expected student recommendations")
	}
	for _, item := range recommendations {
		if item.PackageID == "pkg-g05-english-s1-full" {
			t.Fatalf("opened package should not be returned: %#v", item)
		}
		if item.CourseCount+item.MaterialCount == 0 {
			t.Fatalf("recommendation should contain learning content: %#v", item)
		}
	}
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

func TestSystemReadinessRequiresSystemAdmin(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	teacherToken := app.loginAdmin(t, "13800000004")
	app.doJSON(t, http.MethodGet, "/api/system/readiness", teacherToken, nil, http.StatusForbidden, nil)

	adminToken := app.loginAdmin(t, "13800000002")
	var readiness learning.SystemReadiness
	app.doJSON(t, http.MethodGet, "/api/system/readiness", adminToken, nil, http.StatusOK, &readiness)
	if readiness.TotalCount == 0 || len(readiness.Items) != readiness.TotalCount {
		t.Fatalf("expected readiness payload, got %#v", readiness)
	}
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

	wechatToken := app.login(t, "/api/auth/wechat-login", map[string]string{
		"code": "teacher",
	})
	var wechatMe learning.Principal
	app.doJSON(t, http.MethodGet, "/api/auth/me", wechatToken, nil, http.StatusOK, &wechatMe)
	if wechatMe.AuthMethod != "wechat" || !wechatMe.MustChangePassword {
		t.Fatalf("expected wechat login to keep auth method and account state: %#v", wechatMe)
	}
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", wechatToken, nil, http.StatusOK, nil)

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
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		Summary:          "接口测试专用套餐。",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
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

func TestStudentLearningReviewClosedLoopThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	adminToken := app.loginAdmin(t, "13800000002")
	teacherToken := app.loginAdmin(t, "13800000004")

	var student learning.Student
	app.doJSON(t, http.MethodPost, "/api/students", adminToken, learning.StudentUpsertRequest{
		Name:                  "闭环测试学生",
		Phone:                 "18600009999",
		Grade:                 "五年级",
		SchoolName:            "星线小学",
		OfficialAccountOpenID: "oa-closed-loop",
		AccountStatus:         "正常",
		Remark:                "完整闭环接口测试",
	}, http.StatusOK, &student)
	if student.ID == "" || student.BindStatus != "待绑定" {
		t.Fatalf("unexpected created student: %#v", student)
	}

	var pkg learning.Package
	app.doJSON(t, http.MethodPost, "/api/packages", adminToken, learning.PackageUpsertRequest{
		Name:             "五年级英语闭环测试套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "课程+题+学习资料",
		Summary:          "闭环测试专用套餐。",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"course", "question"},
		Status:           learning.StatusEnabled,
	}, http.StatusOK, &pkg)
	var grant learning.GrantPreview
	app.doJSON(t, http.MethodPost, "/api/grants", adminToken, map[string]string{
		"studentId": student.ID,
		"packageId": pkg.ID,
	}, http.StatusOK, &grant)
	if grant.StudentID != student.ID || grant.PackageID != pkg.ID {
		t.Fatalf("unexpected grant response: %#v", grant)
	}

	var singleQuestion learning.QuestionBankItem
	app.doJSON(t, http.MethodPost, "/api/questions", teacherToken, learning.QuestionBankUpsertRequest{
		Title:    "闭环单选题",
		Grade:    "五年级",
		Semester: "S1",
		Subject:  "英语",
		Type:     "single",
		Stem:     "Which word means 苹果?",
		Options:  []string{"apple", "book", "desk"},
		Answer:   "apple",
		Score:    10,
		Status:   string(learning.StatusEnabled),
	}, http.StatusOK, &singleQuestion)
	var textQuestion learning.QuestionBankItem
	app.doJSON(t, http.MethodPost, "/api/questions", teacherToken, learning.QuestionBankUpsertRequest{
		Title:    "闭环简答题",
		Grade:    "五年级",
		Semester: "S1",
		Subject:  "英语",
		Type:     "text",
		Stem:     "用一句话说说今天学到的阅读方法。",
		Score:    20,
		Status:   string(learning.StatusEnabled),
	}, http.StatusOK, &textQuestion)

	var homework learning.Homework
	app.doJSON(t, http.MethodPost, "/api/homework", teacherToken, learning.HomeworkUploadRequest{
		Title:           "闭环课后小挑战",
		CourseID:        "course-g05-english-s1-q1",
		LearningSpaceID: "space-g05-english-s1-q1",
		Deadline:        "2026-12-31",
		Status:          string(learning.StatusEnabled),
		QuestionIDs:     []string{singleQuestion.ID, textQuestion.ID},
	}, http.StatusOK, &homework)
	if homework.ID == "" || homework.QuestionNum != 2 || homework.OwnerTeacherID == "" {
		t.Fatalf("unexpected created homework: %#v", homework)
	}

	studentToken := app.login(t, "/api/auth/demo-student-login", map[string]string{"phone": student.Phone, "password": "123456"})
	var updatedProfile learning.Student
	app.doJSON(t, http.MethodPut, "/api/student/profile", studentToken, learning.StudentProfileUpdateRequest{
		Nickname:     "星星",
		StudentName:  "闭环测试学生",
		Grade:        "五年级",
		SchoolName:   "星线小学",
		GuardianName: "闭环家长",
	}, http.StatusOK, &updatedProfile)
	if updatedProfile.Nickname != "星星" || updatedProfile.GuardianName != "闭环家长" {
		t.Fatalf("unexpected profile update: %#v", updatedProfile)
	}
	app.store.UseMiniProgramSubscribeTemplates([]string{"vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0", "tpl-review"})

	var home learning.StudentHome
	app.doJSON(t, http.MethodGet, "/api/student/home", studentToken, nil, http.StatusOK, &home)
	if home.Student.ID != student.ID || !homeworkVisible(home.PendingHomework, homework.ID) {
		t.Fatalf("expected opened homework on student home, got %#v", home)
	}
	if _, ok := findStudentTodo(home.TodayTodos, "homework"); !ok {
		t.Fatalf("expected student home to expose homework in today todos, got %#v", home.TodayTodos)
	}
	if _, ok := findStudentTodo(home.TodayTodos, "subscribe"); !ok || home.SubscriptionReminder.ActionText == "" {
		t.Fatalf("expected student home to expose subscribe reminder, got todos=%#v reminder=%#v", home.TodayTodos, home.SubscriptionReminder)
	}
	var reminder learning.SubscriptionReminder
	app.doJSON(t, http.MethodPost, "/api/student/subscription", studentToken, learning.StudentSubscriptionRequest{
		TemplateIDs: []string{"vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0"},
	}, http.StatusOK, &reminder)
	if !reminder.Enabled || reminder.ActionText != "已开启" {
		t.Fatalf("expected subscription reminder to be enabled, got %#v", reminder)
	}
	app.doJSON(t, http.MethodGet, "/api/student/home", studentToken, nil, http.StatusOK, &home)
	if _, ok := findStudentTodo(home.TodayTodos, "subscribe"); ok || !home.SubscriptionReminder.Enabled {
		t.Fatalf("expected subscribe todo removed after enabling reminder, got todos=%#v reminder=%#v", home.TodayTodos, home.SubscriptionReminder)
	}
	var study learning.StudentStudyBoard
	app.doJSON(t, http.MethodGet, "/api/student/study", studentToken, nil, http.StatusOK, &study)
	if len(study.Courses) == 0 {
		t.Fatalf("expected opened course on study board, got %#v", study)
	}
	var homeworkDetail learning.Homework
	app.doJSON(t, http.MethodGet, "/api/student/homework/"+homework.ID, studentToken, nil, http.StatusOK, &homeworkDetail)
	if homeworkDetail.QuestionNum != 2 || len(homeworkDetail.Questions) != 2 {
		t.Fatalf("expected homework questions for student, got %#v", homeworkDetail)
	}

	var submitted struct {
		SubmissionID string `json:"submissionId"`
		Status       string `json:"status"`
		Score        int    `json:"score"`
	}
	app.doJSON(t, http.MethodPost, "/api/student/submissions", studentToken, learning.SubmissionRequest{
		HomeworkID: homework.ID,
		Answers: []learning.SubmissionAnswer{
			{QuestionID: singleQuestion.ID, Choice: "apple"},
			{QuestionID: textQuestion.ID, Text: "先找关键词，再检查答案。"},
		},
	}, http.StatusOK, &submitted)
	if submitted.SubmissionID == "" || submitted.Status != "待批改" {
		t.Fatalf("unexpected submission response: %#v", submitted)
	}

	var reviews []learning.Review
	app.doJSON(t, http.MethodGet, "/api/reviews/pending", teacherToken, nil, http.StatusOK, &reviews)
	review, ok := findReviewBySubmission(reviews, submitted.SubmissionID)
	if !ok || review.Status != "待批改" {
		t.Fatalf("expected pending review for submitted homework, got %#v", reviews)
	}

	var reviewed learning.Submission
	app.doJSON(t, http.MethodPost, "/api/reviews/"+review.ID+"/complete", teacherToken, learning.ReviewCompleteRequest{
		Score:          95,
		TeacherComment: "关键词找得准确，表达清楚。",
		Reward:         "阅读小达人",
		FinalStatus:    "已批改",
	}, http.StatusOK, &reviewed)
	if reviewed.ID != submitted.SubmissionID || reviewed.Status != "已批改" || reviewed.FinalScore != 95 {
		t.Fatalf("unexpected reviewed submission: %#v", reviewed)
	}

	var result learning.Submission
	app.doJSON(t, http.MethodGet, "/api/student/submissions/"+submitted.SubmissionID, studentToken, nil, http.StatusOK, &result)
	if result.Status != "已批改" || result.TeacherComment == "" || result.Reward != "阅读小达人" {
		t.Fatalf("expected reviewed result visible to student, got %#v", result)
	}
	var homeAfterReview learning.StudentHome
	app.doJSON(t, http.MethodGet, "/api/student/home", studentToken, nil, http.StatusOK, &homeAfterReview)
	if homeworkVisible(homeAfterReview.PendingHomework, homework.ID) {
		t.Fatalf("expected reviewed homework to leave student pending list, got %#v", homeAfterReview.PendingHomework)
	}
	feedback, ok := findClassroomFeedback(homeAfterReview.ClassroomFeedback, submitted.SubmissionID)
	if !ok || feedback.Score != 95 || feedback.Focus == "" || feedback.NextStep == "" {
		t.Fatalf("expected reviewed homework to create classroom feedback, got %#v", homeAfterReview.ClassroomFeedback)
	}
	if todo, ok := findStudentTodo(homeAfterReview.TodayTodos, "feedback"); !ok || todo.Path == "" || todo.ActionText != "查看反馈" {
		t.Fatalf("expected student home to expose feedback todo, got %#v", homeAfterReview.TodayTodos)
	}
	var tasksAfterReview []learning.StudentTask
	app.doJSON(t, http.MethodGet, "/api/student/tasks", studentToken, nil, http.StatusOK, &tasksAfterReview)
	task, ok := findStudentTask(tasksAfterReview, homework.ID)
	if !ok || task.StudentStatus != "已完成" || task.Score != 95 || task.SubmissionID != submitted.SubmissionID {
		t.Fatalf("expected reviewed homework to be completed in student tasks, got %#v", tasksAfterReview)
	}
	var summary learning.HomeworkSubmissionSummary
	app.doJSON(t, http.MethodGet, "/api/homework/"+homework.ID+"/submissions", teacherToken, nil, http.StatusOK, &summary)
	if summary.SubmittedNum != 1 || !submissionSummaryContains(summary, student.ID, "已批改") {
		t.Fatalf("expected teacher submission summary to include reviewed student, got %#v", summary)
	}
}

func TestCommercialLifecycleThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000002")

	var pkg learning.Package
	app.doJSON(t, http.MethodPost, "/api/packages", token, learning.PackageUpsertRequest{
		Name:             "五年级英语商业闭环课包",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		Summary:          "商业闭环测试套餐。",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
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
	if afterPay.AlreadyOpened {
		t.Fatalf("payment should not open package grant automatically: %#v", afterPay)
	}

	var opened learning.GrantPreview
	app.doJSON(t, http.MethodPost, "/api/grants", token, map[string]string{
		"studentId": "stu-001",
		"packageId": pkg.ID,
	}, http.StatusOK, &opened)
	if opened.StudentID != "stu-001" || opened.PackageID != pkg.ID {
		t.Fatalf("expected manual grant to open package: %#v", opened)
	}
	var afterManualGrant learning.GrantPreview
	app.doJSON(t, http.MethodGet, "/api/grants/preview?studentId=stu-001&packageId="+pkg.ID, token, nil, http.StatusOK, &afterManualGrant)
	if !afterManualGrant.AlreadyOpened {
		t.Fatalf("expected manual grant to be visible in follow-up preview: %#v", afterManualGrant)
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
	if notice.Status == "" || notice.NoticeID == "" || notice.Channel != "公众号模板消息" {
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
		CourseID:        "course-g05-english-s1-q1",
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
	if class.Name != "英文 1V1 小班" || class.CourseName != "五年级英文S1Q1课程" || class.TeacherName != "英语老师" || class.Status != "已确认" {
		t.Fatalf("unexpected class detail for student schedule: %#v", class)
	}

	studentToken := app.loginStudent(t)
	var studentSchedule []learning.ScheduleClass
	app.doJSON(t, http.MethodGet, "/api/student/schedule", studentToken, nil, http.StatusOK, &studentSchedule)
	found := false
	for _, item := range studentSchedule {
		if item.ID != class.ID {
			continue
		}
		found = true
		if item.CourseName != class.CourseName || item.TeacherName != class.TeacherName || item.DayOfWeek != 3 || item.StartTime != "19:00" || item.EndTime != "20:30" || item.Status != "已确认" {
			t.Fatalf("created class is not visible with expected student schedule fields: %#v", item)
		}
	}
	if !found {
		t.Fatalf("student schedule does not include created class %s: %#v", class.ID, studentSchedule)
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
		Title:    "接口测试学习资料",
		CourseID: "course-g05-english-s1-q1",
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

func TestStudentMaterialDetailIncludesSecurityWatermark(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	var material learning.Material
	app.doJSON(t, http.MethodGet, "/api/student/materials/mat-g05-english-s1-q1", token, nil, http.StatusOK, &material)

	if material.WatermarkText == "" || !strings.Contains(material.WatermarkText, "小明") || !strings.Contains(material.WatermarkText, "9069") {
		t.Fatalf("expected student watermark with name and phone tail, got %#v", material.WatermarkText)
	}
	if material.SecurityNotice == "" {
		t.Fatalf("expected security notice, got %#v", material)
	}
	if material.DownloadURL != "" {
		t.Fatalf("student material should not expose download url, got %#v", material.DownloadURL)
	}
}

func TestStudentMaterialSecurePreviewRequiresAccess(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	app.doJSON(t, http.MethodGet, "/api/student/materials/mat-g05-math-s1-q1/preview", token, nil, http.StatusBadRequest, nil)
}

func TestStudentSecurityEventIsLogged(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	app.doJSON(t, http.MethodPost, "/api/student/security/events", token, learning.SecurityEventRequest{
		EventType:  "screenshot",
		TargetType: "homework",
		TargetID:   "hw-g05-english-s1-q1",
		PagePath:   "pages/answer/index",
		Detail:     "用户触发截屏事件",
	}, http.StatusOK, nil)

	logs := app.store.Logs()
	if len(logs) == 0 || logs[0].Action != "内容防盗版风控" || !strings.Contains(logs[0].Detail, "eventType=screenshot") {
		t.Fatalf("expected security event log, got %#v", logs)
	}
}

func TestStudentHomeworkDetailIncludesSecurityWatermark(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	var homework learning.Homework
	app.doJSON(t, http.MethodGet, "/api/student/homework/hw-g05-english-s1-q1", token, nil, http.StatusOK, &homework)

	if homework.WatermarkText == "" || !strings.Contains(homework.WatermarkText, "小明") {
		t.Fatalf("expected homework watermark, got %#v", homework.WatermarkText)
	}
	if homework.SecurityNotice == "" {
		t.Fatalf("expected homework security notice, got %#v", homework)
	}
	for _, question := range homework.Questions {
		if question.Answer != "" || len(question.Answers) != 0 {
			t.Fatalf("student question should not expose answers, got %#v", question)
		}
	}
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
		HomeworkID: "hw-g05-english-s1-q1",
		Answers: []learning.SubmissionAnswer{
			{QuestionID: "q1", Choice: "A"},
			{QuestionID: "q2", Text: "今天学会了抓中心句。"},
		},
	}, http.StatusOK, &result)
	if result.SubmissionID == "" || result.Status != "待批改" || result.Score == 0 {
		t.Fatalf("unexpected submission result: %#v", result)
	}
	var detail learning.Submission
	app.doJSON(t, http.MethodGet, "/api/student/submissions/"+result.SubmissionID, token, nil, http.StatusOK, &detail)
	if detail.ObjectiveScore == 0 || detail.TeacherComment == "" {
		t.Fatalf("expected pending review result to include objective score and hint: %#v", detail)
	}
}

func homeworkVisible(homework []learning.Homework, homeworkID string) bool {
	for _, item := range homework {
		if item.ID == homeworkID {
			return true
		}
	}
	return false
}

func findReviewBySubmission(reviews []learning.Review, submissionID string) (learning.Review, bool) {
	for _, review := range reviews {
		if review.SubmissionID == submissionID {
			return review, true
		}
	}
	return learning.Review{}, false
}

func findStudentTask(tasks []learning.StudentTask, homeworkID string) (learning.StudentTask, bool) {
	for _, task := range tasks {
		if task.ID == homeworkID {
			return task, true
		}
	}
	return learning.StudentTask{}, false
}

func findStudentTodo(todos []learning.StudentTodo, todoType string) (learning.StudentTodo, bool) {
	for _, todo := range todos {
		if todo.Type == todoType {
			return todo, true
		}
	}
	return learning.StudentTodo{}, false
}

func findClassroomFeedback(feedback []learning.ClassroomFeedback, submissionID string) (learning.ClassroomFeedback, bool) {
	for _, item := range feedback {
		if item.RelatedSubmissionID == submissionID {
			return item, true
		}
	}
	return learning.ClassroomFeedback{}, false
}

func submissionSummaryContains(summary learning.HomeworkSubmissionSummary, studentID string, reviewStatus string) bool {
	for _, item := range summary.Students {
		if item.StudentID == studentID && item.ReviewStatus == reviewStatus {
			return true
		}
	}
	return false
}
