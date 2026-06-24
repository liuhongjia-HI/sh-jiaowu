package router_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestAdminStudentManagementLifecycleThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000002")

	var created learning.Student
	app.doJSON(t, http.MethodPost, "/api/students", token, learning.StudentUpsertRequest{
		Name:   "接口测试学生",
		Phone:  "13900001001",
		Grade:  "五年级",
		Remark: "接口测试新增",
	}, http.StatusOK, &created)
	if created.ID == "" || created.AccountStatus != "正常" || created.BindStatus != "待绑定" {
		t.Fatalf("unexpected created student: %#v", created)
	}

	var filtered []learning.Student
	app.doJSON(t, http.MethodGet, "/api/students?keyword=接口测试学生", token, nil, http.StatusOK, &filtered)
	if len(filtered) != 1 || filtered[0].ID != created.ID {
		t.Fatalf("expected keyword search to find created student, got %#v", filtered)
	}

	var detail learning.StudentDetail
	app.doJSON(t, http.MethodGet, "/api/students/"+created.ID, token, nil, http.StatusOK, &detail)
	if detail.Student.ID != created.ID {
		t.Fatalf("unexpected student detail: %#v", detail)
	}

	var updated learning.Student
	app.doJSON(t, http.MethodPut, "/api/students/"+created.ID, token, learning.StudentUpsertRequest{
		Name:          "接口测试学生-已停用",
		Phone:         "13900001001",
		Grade:         "五年级",
		AccountStatus: "停用",
		Remark:        "接口测试编辑",
	}, http.StatusOK, &updated)
	if updated.AccountStatus != "停用" || updated.Name != "接口测试学生-已停用" {
		t.Fatalf("unexpected updated student: %#v", updated)
	}

	var remind learning.StudentRemindResult
	app.doJSON(t, http.MethodPost, "/api/students/"+created.ID+"/remind", token, map[string]string{}, http.StatusOK, &remind)
	if remind.NoticeID == "" || remind.Message == "" {
		t.Fatalf("unexpected remind result: %#v", remind)
	}

	time.Sleep(1100 * time.Millisecond)
	var imported learning.StudentImportResult
	doMultipart(t, app, http.MethodPost, "/api/students/import", token, map[string]string{}, "file", "students.csv", []byte(strings.Join([]string{
		"name,phone,grade,remark",
		"CSV导入学生,13900001002,五年级,导入成功",
		"缺手机号,,五年级,导入失败",
	}, "\n")), http.StatusOK, &imported)
	if imported.SuccessCount != 1 || imported.FailedCount != 1 || len(imported.Errors) != 1 {
		t.Fatalf("unexpected import result: %#v", imported)
	}

	app.doJSON(t, http.MethodGet, "/api/students/"+created.ID+"/grants", token, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/students/"+created.ID+"/learning-records", token, nil, http.StatusOK, nil)
}

func TestAdminTeachingContentAndFeedbackThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	defer os.RemoveAll(filepath.Join("uploads"))
	token := app.loginAdmin(t, "13800000004")

	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{
		Name:            "接口测试英语课程",
		LearningSpaceID: "space-g05-english-s1-mid",
		ChapterCount:    6,
		Status:          learning.StatusEnabled,
	}, http.StatusOK, &course)
	if course.ID == "" || course.Grade != "五年级" || course.Subject != "英语" {
		t.Fatalf("unexpected course: %#v", course)
	}

	var updatedCourse learning.Course
	app.doJSON(t, http.MethodPut, "/api/courses/"+course.ID, token, learning.CourseUpsertRequest{
		Name:            "接口测试英语课程-已编辑",
		LearningSpaceID: "space-g05-english-s1-mid",
		ChapterCount:    8,
		Status:          learning.StatusEnabled,
	}, http.StatusOK, &updatedCourse)
	if updatedCourse.Name != "接口测试英语课程-已编辑" || updatedCourse.ChapterCount != 8 {
		t.Fatalf("unexpected updated course: %#v", updatedCourse)
	}

	var material learning.Material
	doMultipart(t, app, http.MethodPost, "/api/materials", token, map[string]string{
		"title":           "接口测试讲义",
		"courseId":        course.ID,
		"learningSpaceId": "space-g05-english-s1-mid",
		"chapter":         "第一章",
	}, "file", "material.pdf", []byte("%PDF-1.4 test material"), http.StatusOK, &material)
	if material.ID == "" || material.FileID == "" || material.PreviewStatus != "可预览" {
		t.Fatalf("unexpected material: %#v", material)
	}

	req, err := http.NewRequest(http.MethodGet, app.server.URL+material.PreviewURL, nil)
	if err != nil {
		t.Fatalf("new preview request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preview request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d", resp.StatusCode)
	}

	var updatedMaterial learning.Material
	app.doJSON(t, http.MethodPut, "/api/materials/"+material.ID, token, learning.MaterialUpdateRequest{
		Title:           "接口测试讲义-草稿",
		CourseID:        course.ID,
		LearningSpaceID: "space-g05-english-s1-mid",
		Chapter:         "第二章",
		Status:          learning.StatusDraft,
	}, http.StatusOK, &updatedMaterial)
	if updatedMaterial.PublishStatus != "草稿" || updatedMaterial.Chapter != "第二章" {
		t.Fatalf("unexpected updated material: %#v", updatedMaterial)
	}

	var homework learning.Homework
	doMultipart(t, app, http.MethodPost, "/api/homework", token, map[string]string{
		"title":           "接口测试练习",
		"courseId":        course.ID,
		"learningSpaceId": "space-g05-english-s1-mid",
		"deadline":        "2026-07-31",
	}, "file", "homework.pdf", []byte("%PDF-1.4 test homework"), http.StatusOK, &homework)
	if homework.ID == "" || homework.FileID == "" {
		t.Fatalf("unexpected homework: %#v", homework)
	}

	var updatedHomework learning.Homework
	app.doJSON(t, http.MethodPut, "/api/homework/"+homework.ID, token, learning.HomeworkUpdateRequest{
		Title:           "接口测试练习-停用",
		CourseID:        course.ID,
		LearningSpaceID: "space-g05-english-s1-mid",
		Deadline:        "2026-08-01",
		Status:          string(learning.StatusDisabled),
	}, http.StatusOK, &updatedHomework)
	if updatedHomework.PublishStatus != "停用" || updatedHomework.Deadline != "2026-08-01" {
		t.Fatalf("unexpected updated homework: %#v", updatedHomework)
	}

	var reviews []learning.Review
	app.doJSON(t, http.MethodGet, "/api/reviews/pending", token, nil, http.StatusOK, &reviews)
	if len(reviews) == 0 {
		t.Fatal("expected seeded pending reviews")
	}
	var submission learning.Submission
	app.doJSON(t, http.MethodPost, "/api/reviews/"+reviews[0].ID+"/complete", token, learning.ReviewCompleteRequest{
		Score:          96,
		TeacherComment: "接口测试批改反馈",
		Reward:         "接口测试星章",
	}, http.StatusOK, &submission)
	if submission.Status != "已批改" || submission.Score != 96 {
		t.Fatalf("unexpected review completion: %#v", submission)
	}

	var notice learning.Notice
	app.doJSON(t, http.MethodPost, "/api/notices", token, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "英语接口测试通知",
		Target:  "五年级英语班",
		Summary: "请完成接口测试练习。",
	}, http.StatusOK, &notice)
	if notice.ID == "" || notice.Status != "已发送" {
		t.Fatalf("unexpected notice: %#v", notice)
	}
}

func TestAdminSystemManagementThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	superToken := app.loginAdmin(t, "13800000001")
	campusToken := app.loginAdmin(t, "13800000002")

	var teacher learning.Teacher
	app.doJSON(t, http.MethodPost, "/api/teachers", campusToken, learning.TeacherUpsertRequest{
		Name:              "接口测试教师",
		Phone:             "13900002001",
		LearningSpaceIDs:  []string{"space-g05-english-s1-mid"},
		CanUploadHandout:  true,
		CanUploadQuestion: false,
		CanReview:         true,
		Remark:            "接口测试教师新增",
	}, http.StatusOK, &teacher)
	if teacher.ID == "" || !teacher.CanUploadHandout || teacher.CanUploadQuestion {
		t.Fatalf("unexpected teacher: %#v", teacher)
	}

	var updatedTeacher learning.Teacher
	app.doJSON(t, http.MethodPut, "/api/teachers/"+teacher.ID, campusToken, learning.TeacherUpsertRequest{
		Name:              "接口测试教师-已编辑",
		Phone:             "13900002001",
		LearningSpaceIDs:  []string{"space-g05-english-s1-mid", "space-g05-english-s1-final"},
		CanUploadHandout:  true,
		CanUploadQuestion: true,
		CanReview:         false,
		AccountStatus:     "停用",
		Remark:            "接口测试教师编辑",
	}, http.StatusOK, &updatedTeacher)
	if updatedTeacher.AccountStatus != "停用" || updatedTeacher.CanReview {
		t.Fatalf("unexpected updated teacher: %#v", updatedTeacher)
	}

	var staff learning.AdminStaff
	app.doJSON(t, http.MethodPost, "/api/admin-staff", superToken, learning.AdminStaffUpsertRequest{
		Name:     "接口测试教务",
		Phone:    "13900003001",
		Role:     learning.RoleOpsStaff,
		Remark:   "接口测试管理人员新增",
	}, http.StatusOK, &staff)
	if staff.ID == "" || staff.Role != learning.RoleOpsStaff || staff.AccountStatus != "正常" {
		t.Fatalf("unexpected admin staff: %#v", staff)
	}

	var updatedStaff learning.AdminStaff
	app.doJSON(t, http.MethodPut, "/api/admin-staff/"+staff.ID, superToken, learning.AdminStaffUpsertRequest{
		Name:          "接口测试教务-已编辑",
		Phone:         "13900003001",
		Role:          learning.RoleCampusAdmin,
		CampusID:      "campus-main",
		AccountStatus: "停用",
		Remark:        "接口测试管理人员编辑",
	}, http.StatusOK, &updatedStaff)
	if updatedStaff.Role != learning.RoleCampusAdmin || updatedStaff.CampusID != "campus-main" || updatedStaff.AccountStatus != "停用" {
		t.Fatalf("unexpected updated staff: %#v", updatedStaff)
	}

	var settings map[string]string
	app.doJSON(t, http.MethodPut, "/api/settings", campusToken, learning.SettingUpdateRequest{
		Key:   "downloadPolicy",
		Value: "接口测试允许下载已发布讲义",
	}, http.StatusOK, &settings)
	if settings["downloadPolicy"] != "接口测试允许下载已发布讲义" || settings["academicYear"] == "" {
		t.Fatalf("unexpected settings: %#v", settings)
	}

	app.doJSON(t, http.MethodGet, "/api/admin-staff", campusToken, nil, http.StatusForbidden, nil)
}

func doMultipart(t *testing.T, app *testApp, method, path, token string, fields map[string]string, fileField string, fileName string, fileBody []byte, wantStatus int, out any) apiResponse {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(fileBody); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(method, app.server.URL+path, &body)
	if err != nil {
		t.Fatalf("new multipart request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "router-test-agent")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Operator-ID", "test-operator")
		req.Header.Set("X-Operator-Name", "测试操作人")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("multipart request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read multipart response: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s status = %d want %d body=%s", method, path, resp.StatusCode, wantStatus, string(raw))
	}
	var envelope apiResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode multipart response envelope: %v body=%s", err, string(raw))
	}
	if out != nil && envelope.Data != nil && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			t.Fatalf("decode multipart response data: %v body=%s", err, string(envelope.Data))
		}
	}
	return envelope
}
