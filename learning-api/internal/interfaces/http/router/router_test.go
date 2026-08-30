package router_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	return newTestAppWithStorageRoot(t, "")
}

func apiTestCurriculum(prefix string) []learning.CurriculumNode {
	return []learning.CurriculumNode{
		{ID: prefix + "-unit-1", Type: learning.CurriculumUnit, Name: "Unit 1", SortOrder: 1},
		{ID: prefix + "-chapter-1", ParentID: prefix + "-unit-1", Type: learning.CurriculumChapter, Name: "Chapter 1", SortOrder: 1},
		{ID: prefix + "-lesson-1", ParentID: prefix + "-chapter-1", Type: learning.CurriculumLesson, Name: "Lesson 1", SortOrder: 1},
	}
}

func newTestAppWithStorageRoot(t *testing.T, storageRoot string) *testApp {
	t.Helper()
	repo := store.NewMemoryStore()
	cfg := config.MustLoad()
	cfg.App.Env = "test"
	cfg.Auth.TokenSecret = "router-test-secret"
	cfg.Demo.AdminPasswordLogin = true
	cfg.Demo.StudentPasswordLogin = true
	if storageRoot != "" {
		cfg.FileStorage.Root = storageRoot
	}
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

func TestStudentAvatarUploadAndPublicReadThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	defer os.RemoveAll(filepath.Join("uploads"))

	// 1x1 PNG，验证服务端按真实图片内容校验，而不是只看文件扩展名。
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}
	var updated learning.Student
	doMultipart(t, app, http.MethodPost, "/api/student/profile/avatar", app.loginStudent(t), nil, "file", "avatar.png", pngData, http.StatusOK, &updated)
	if !strings.HasPrefix(updated.AvatarURL, "/api/student/avatars/avatar-") || !strings.HasSuffix(updated.AvatarURL, ".png") {
		t.Fatalf("expected persisted avatar URL, got %#v", updated)
	}

	request, err := http.NewRequest(http.MethodGet, app.server.URL+updated.AvatarURL, nil)
	if err != nil {
		t.Fatalf("new avatar request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("read public avatar: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("unexpected avatar response status=%d content-type=%q body=%q", response.StatusCode, response.Header.Get("Content-Type"), raw)
	}
}

func TestStudentAvatarUsesConfiguredPersistentStorageRoot(t *testing.T) {
	storageRoot := filepath.Join(t.TempDir(), "persistent-uploads")
	app := newTestAppWithStorageRoot(t, storageRoot)
	defer app.close()

	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}
	var updated learning.Student
	doMultipart(t, app, http.MethodPost, "/api/student/profile/avatar", app.loginStudent(t), nil, "file", "avatar.png", pngData, http.StatusOK, &updated)

	if _, err := os.Stat(filepath.Join(storageRoot, "avatars", filepath.Base(updated.AvatarURL))); err != nil {
		t.Fatalf("expected avatar to be stored in configured persistent root: %v", err)
	}
	response, err := http.Get(app.server.URL + updated.AvatarURL)
	if err != nil {
		t.Fatalf("read public avatar: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(response.Body)
		t.Fatalf("expected public avatar from configured root, status=%d body=%q", response.StatusCode, raw)
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

func TestMaterialReorderEndpointChangesStudentCourseDisplayOrder(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	teacher, err := app.store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("teacher principal: %v", err)
	}
	courseID := "course-g05-english-s1-q1"
	for _, title := range []string{"排序测试资料一", "排序测试资料二"} {
		if _, err := app.store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
			Title: title, CourseID: courseID, LessonID: courseID + "-lesson-1",
			File: learning.FileAsset{ID: "file-" + title, FileName: title + ".pdf", FileType: "PDF"},
		}); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
	}

	materials := app.store.Materials(teacher, learning.MaterialQuery{})
	orderedIDs := make([]string, 0)
	for _, material := range materials {
		if material.CourseID == courseID {
			orderedIDs = append(orderedIDs, material.ID)
		}
	}
	if len(orderedIDs) < 3 {
		t.Fatalf("expected seeded and created materials, got %#v", orderedIDs)
	}
	orderedIDs[0], orderedIDs[1] = orderedIDs[1], orderedIDs[0]

	app.doJSON(t, http.MethodPost, "/api/materials/reorder", app.loginAdmin(t, "13800000004"), learning.MaterialReorderRequest{CourseID: courseID, MaterialIDs: orderedIDs}, http.StatusOK, nil)

	student, err := app.store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("student principal: %v", err)
	}
	detail, err := app.store.StudentCourseDetail(student, courseID)
	if err != nil {
		t.Fatalf("student course detail: %v", err)
	}
	if len(detail.Materials) != len(orderedIDs) {
		t.Fatalf("unexpected student materials: %#v", detail.Materials)
	}
	for index, id := range orderedIDs {
		if detail.Materials[index].ID != id {
			t.Fatalf("student material order = %#v, want %#v", detail.Materials, orderedIDs)
		}
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

func TestCreateDirectGrantThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000002")

	var opened learning.DirectGrantResult
	app.doJSON(t, http.MethodPost, "/api/grants/direct", token, learning.DirectGrantCreateRequest{
		StudentID:        "stu-001",
		LearningSpaceIDs: []string{"space-g05-math-s1-q1"},
		ContentTypeCodes: []string{"course", "handout"},
	}, http.StatusOK, &opened)
	if opened.StudentID != "stu-001" || len(opened.OpenCourses) == 0 || len(opened.OpenMaterials) == 0 || len(opened.OpenHomework) != 0 {
		t.Fatalf("unexpected direct grant response: %#v", opened)
	}

	var packages []learning.Package
	app.doJSON(t, http.MethodGet, "/api/packages", token, nil, http.StatusOK, &packages)
	for _, pkg := range packages {
		if strings.HasPrefix(pkg.ID, "direct-") {
			t.Fatalf("direct grant internals must not be exposed as course plans: %#v", pkg)
		}
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
		LessonID:        "course-g05-english-s1-q1-lesson-1",
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
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
	}, http.StatusOK, &class)
	if class.ID == "" || len(class.Students) != 1 {
		t.Fatalf("unexpected created class: %#v", class)
	}
	// 标题按客户在 Outlook 里的约定：教师 年级 科目 学生（对照 Clara G5 Eng Zoe&Arthur）。
	if class.Name != "英语老师 G5 Eng 小明" || class.CourseName != "五年级英文S1Q1课程" || class.TeacherName != "英语老师" || class.Status != "已确认" {
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
		LessonID: "course-g05-english-s1-q1-lesson-1",
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
	if material.FileID == "" && material.DownloadURL != "" {
		t.Fatalf("material without a file should not expose download url, got %#v", material.DownloadURL)
	}
}

func TestStudentMaterialSecurePreviewRequiresAccess(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)

	app.doJSON(t, http.MethodGet, "/api/student/materials/mat-g05-math-s1-q1/preview", token, nil, http.StatusBadRequest, nil)
}

func TestStudentMaterialDownloadNeverFallsBackToOriginalFile(t *testing.T) {
	t.Setenv("PATH", "")
	app := newTestApp(t)
	defer app.close()

	source := filepath.Join(t.TempDir(), "student-material.pdf")
	original := []byte("%PDF-1.4 original-only-secret")
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	teacher, err := app.store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("teacher principal: %v", err)
	}
	material, err := app.store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title: "学生下载水印测试", CourseID: "course-g05-english-s1-q1", LessonID: "course-g05-english-s1-q1-lesson-1",
		File: learning.FileAsset{
			ID: "file-student-watermark-download", FileName: "lesson.pdf", FileSize: int64(len(original)), FileType: "PDF",
			ContentType: "application/pdf", OriginalPath: source, PreviewPath: source, PreviewStatus: "可预览",
		},
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, app.server.URL+"/api/student/materials/"+material.ID+"/download", nil)
	if err != nil {
		t.Fatalf("new download request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+app.loginStudent(t))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read download response: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected secure download failure without watermark service, got status=%d body=%s", resp.StatusCode, body)
	}
	if bytes.Contains(body, original) {
		t.Fatalf("student response must never contain the original file, got %q", body)
	}
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

// 多子女的完整闭环，走真实 HTTP：手机号命中两个学生 -> 需要选择 -> 选中一个
// 登录 -> 切换器列出两个孩子 -> 切到另一个 -> 新 token 在受保护接口上确实读到
// 了另一个孩子的数据。中间件把 GuardianID 从 token 里正确地拉出来这件事，只有
// 真的过一遍 HTTP + gin 路由 + AuthRequired 才能验证到；store 层的单测测的是
// 逻辑本身，测不到这几层真的接起来了没有。
func TestMultiChildLoginSwitchThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	adminToken := app.loginAdmin(t, "13800000001")
	var sibling learning.Student
	app.doJSON(t, http.MethodPost, "/api/students", adminToken, map[string]any{
		"name": "小明妹妹", "phone": "18500009069", "grade": "五年级", "accountStatus": "正常",
	}, http.StatusOK, &sibling)

	selection := app.doJSON(t, http.MethodPost, "/api/auth/wechat-login", "", map[string]any{
		"code": "multi-child-router-test", "phone": "18500009069", "studentName": "小明", "schoolName": "星河小学", "grade": "五年级",
	}, http.StatusOK, nil)
	var needsSelection struct {
		NeedsSelection bool                      `json:"needsSelection"`
		Candidates     []learning.StudentAccount `json:"candidates"`
	}
	if err := json.Unmarshal(selection.Data, &needsSelection); err != nil {
		t.Fatalf("decode selection payload: %v", err)
	}
	if !needsSelection.NeedsSelection || len(needsSelection.Candidates) != 2 {
		t.Fatalf("expected a two-candidate selection prompt, got %#v", needsSelection)
	}

	var auth authResponse
	app.doJSON(t, http.MethodPost, "/api/auth/wechat-login", "", map[string]any{
		"code": "multi-child-router-test-2", "phone": "18500009069", "studentName": "小明", "schoolName": "星河小学", "grade": "五年级",
		"selectedStudentId": "stu-001",
	}, http.StatusOK, &auth)
	if auth.User.GuardianID == "" || auth.User.StudentID != "stu-001" {
		t.Fatalf("expected a guardian-linked login for stu-001, got %#v", auth.User)
	}

	var accounts []learning.StudentAccount
	app.doJSON(t, http.MethodGet, "/api/student/accounts", auth.Token, nil, http.StatusOK, &accounts)
	if len(accounts) != 2 {
		t.Fatalf("expected two switchable accounts, got %#v", accounts)
	}

	var switched authResponse
	app.doJSON(t, http.MethodPost, "/api/student/accounts/"+sibling.ID+"/switch", auth.Token, nil, http.StatusOK, &switched)
	if switched.User.StudentID != sibling.ID || switched.User.GuardianID != auth.User.GuardianID {
		t.Fatalf("expected switch to move to the sibling under the same guardian, got %#v", switched.User)
	}

	var home learning.StudentHome
	app.doJSON(t, http.MethodGet, "/api/student/home", switched.Token, nil, http.StatusOK, &home)
	if home.Student.ID != sibling.ID {
		t.Fatalf("expected the switched token to read the sibling's own data, got student %#v", home.Student)
	}

	// 反过来也要成立：切完之后，老 token（还停在 stu-001 上）应该继续正常工作，
	// 而不是被这次切换顶掉——多子女场景下家长可能两个孩子的页面都开着。
	var stillFirst learning.StudentHome
	app.doJSON(t, http.MethodGet, "/api/student/home", auth.Token, nil, http.StatusOK, &stillFirst)
	if stillFirst.Student.ID != "stu-001" {
		t.Fatalf("expected the original token to keep reading stu-001, got %#v", stillFirst.Student)
	}
}

func TestParentAdditionalStudentCanSwitchImmediately(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	var parent authResponse
	app.doJSON(t, http.MethodPost, "/api/auth/wechat-login", "", map[string]any{
		"code": "add-child-api", "phone": "18500009069", "studentName": "小明", "schoolName": "星河小学", "grade": "五年级",
	}, http.StatusOK, &parent)
	var initialAccounts []learning.StudentAccount
	app.doJSON(t, http.MethodGet, "/api/student/accounts", parent.Token, nil, http.StatusOK, &initialAccounts)
	if len(initialAccounts) != 1 || !initialAccounts[0].Active || !initialAccounts[0].CanSwitch {
		t.Fatalf("expected a single current student to remain visible, got %#v", initialAccounts)
	}

	var added learning.StudentAccount
	app.doJSON(t, http.MethodPost, "/api/student/accounts", parent.Token, map[string]any{
		"name": "小明妹妹", "grade": "五年级", "schoolName": "星河小学",
	}, http.StatusOK, &added)
	if added.Status != "正常" || !added.CanSwitch {
		t.Fatalf("expected an immediately switchable account response, got %#v", added)
	}
	var switched authResponse
	app.doJSON(t, http.MethodPost, "/api/student/accounts/"+added.StudentID+"/switch", parent.Token, nil, http.StatusOK, &switched)
	if switched.User.StudentID != added.StudentID {
		t.Fatalf("expected immediate switching to the added student, got %#v", switched)
	}

	var accounts []learning.StudentAccount
	app.doJSON(t, http.MethodGet, "/api/student/accounts", parent.Token, nil, http.StatusOK, &accounts)
	if len(accounts) != 2 {
		t.Fatalf("expected the current and added students, got %#v", accounts)
	}
	available := false
	for _, account := range accounts {
		if account.StudentID == added.StudentID {
			available = account.Status == "正常" && account.CanSwitch
		}
	}
	if !available {
		t.Fatalf("expected the added student to remain switchable, got %#v", accounts)
	}
}

func TestParentCanAddMultipleStudentsWithoutPendingApplicationLimit(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	parentToken := app.login(t, "/api/auth/wechat-login", map[string]any{
		"code": "add-child-limit", "phone": "18500009069", "studentName": "小明", "schoolName": "星河小学", "grade": "五年级",
	})
	for _, name := range []string{"学生甲", "学生乙", "学生丙"} {
		app.doJSON(t, http.MethodPost, "/api/student/accounts", parentToken, map[string]any{
			"name": name, "grade": "五年级", "schoolName": "星河小学",
		}, http.StatusOK, nil)
	}
	app.doJSON(t, http.MethodPost, "/api/student/accounts", parentToken, map[string]any{
		"name": "学生丁", "grade": "五年级", "schoolName": "星河小学",
	}, http.StatusOK, nil)
}

// 一孩多家长：爸爸已经绑过了，妈妈自己的手机号跟任何已有档案都不一样——
// 走绑定码这条路而不是手机号匹配。走真实 HTTP：后台生成码 -> 妈妈凭码登录
// -> 她的切换器只看到这一个孩子、且是她自己独立的 guardian 身份，不影响
// 爸爸已有的登录状态。
func TestBindCodeSecondGuardianClaimThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	fatherToken := app.login(t, "/api/auth/wechat-login", map[string]any{
		"code": "father-login", "phone": "18500009069", "studentName": "小明", "schoolName": "星河小学", "grade": "五年级",
	})
	var fatherMe learning.Principal
	app.doJSON(t, http.MethodGet, "/api/auth/me", fatherToken, nil, http.StatusOK, &fatherMe)

	adminToken := app.loginAdmin(t, "13800000001")
	var withCode learning.Student
	app.doJSON(t, http.MethodPost, "/api/students/stu-001/bind-code", adminToken, nil, http.StatusOK, &withCode)
	if withCode.BindCode == "" {
		t.Fatalf("expected a generated bind code, got %#v", withCode)
	}

	var motherAuth authResponse
	app.doJSON(t, http.MethodPost, "/api/auth/wechat-login", "", map[string]any{
		"code": "mother-claim", "phone": "13911119999", "bindCode": withCode.BindCode,
	}, http.StatusOK, &motherAuth)
	if motherAuth.User.StudentID != "stu-001" || motherAuth.User.GuardianID == "" || motherAuth.User.GuardianID == fatherMe.GuardianID {
		t.Fatalf("expected the mother to get her own guardian identity for stu-001, mother=%#v father guardian=%q", motherAuth.User, fatherMe.GuardianID)
	}

	var motherAccounts []learning.StudentAccount
	app.doJSON(t, http.MethodGet, "/api/student/accounts", motherAuth.Token, nil, http.StatusOK, &motherAccounts)
	if len(motherAccounts) != 1 || motherAccounts[0].StudentID != "stu-001" {
		t.Fatalf("expected the mother's switcher to show exactly stu-001, got %#v", motherAccounts)
	}

	// 爸爸的登录状态没被打扰。
	var fatherHome learning.StudentHome
	app.doJSON(t, http.MethodGet, "/api/student/home", fatherToken, nil, http.StatusOK, &fatherHome)
	if fatherHome.Student.ID != "stu-001" {
		t.Fatalf("expected father's token to keep working, got %#v", fatherHome.Student)
	}
}

// 绑定码猜测要能被限流拦住——攻击者每次换一个新的 wx.login code 提交，
// 不能靠"code 变了"绕开限流，必须按 IP 单独限一道。
func TestBindCodeGuessingIsRateLimited(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	// 前 5 次都是"绑定码不存在"的正常失败；第 5 次失败会把这个 IP 锁住，
	// 所以第 6 次应该直接被挡在校验绑定码之前，拿到锁定消息而不是查库结果。
	for i := 0; i < 5; i++ {
		resp := app.doJSON(t, http.MethodPost, "/api/auth/wechat-login", "", map[string]any{
			"code": fmt.Sprintf("guess-attempt-%d", i), "phone": "13911119999", "bindCode": "WRONGCODE",
		}, http.StatusUnauthorized, nil)
		if strings.Contains(resp.Message, "失败次数过多") {
			t.Fatalf("expected attempt %d to fail on an invalid code, not a lockout, got message=%q", i, resp.Message)
		}
	}
	final := app.doJSON(t, http.MethodPost, "/api/auth/wechat-login", "", map[string]any{
		"code": "guess-attempt-final", "phone": "13911119999", "bindCode": "WRONGCODE",
	}, http.StatusUnauthorized, nil)
	if !strings.Contains(final.Message, "失败次数过多") {
		t.Fatalf("expected the account to be locked after repeated failures, got message=%q", final.Message)
	}
}

// 排课权限下放走到 HTTP 这一层是否真的分开了：
// 老师能建课但建出来的是待审核，且审核接口对老师关闭。
func TestTeacherSchedulingPermissionsThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	teacherToken := app.loginAdmin(t, "13800000004")
	opsToken := app.loginAdmin(t, "13800000003")

	var created learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", teacherToken, learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
	}, http.StatusOK, &created)
	if created.AuditStatus != learning.AuditPending {
		t.Fatalf("老师建课应落待审核，实际 %q", created.AuditStatus)
	}

	// 学生这时候不该看到这节课。
	studentToken := app.loginStudent(t)
	var studentSchedule []learning.ScheduleClass
	app.doJSON(t, http.MethodGet, "/api/student/schedule", studentToken, nil, http.StatusOK, &studentSchedule)
	for _, item := range studentSchedule {
		if item.ID == created.ID {
			t.Fatal("待审核的课不能出现在学生课表接口里")
		}
	}

	// 审核接口对老师关闭。
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+created.ID+"/approve", teacherToken, nil, http.StatusForbidden, nil)

	var approved learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+created.ID+"/approve", opsToken, nil, http.StatusOK, &approved)
	if approved.AuditStatus != learning.AuditApproved {
		t.Fatalf("管理员通过后应为已通过，实际 %q", approved.AuditStatus)
	}

	app.doJSON(t, http.MethodGet, "/api/student/schedule", studentToken, nil, http.StatusOK, &studentSchedule)
	found := false
	for _, item := range studentSchedule {
		if item.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("审核通过后学生应该能看到这节课")
	}
}
