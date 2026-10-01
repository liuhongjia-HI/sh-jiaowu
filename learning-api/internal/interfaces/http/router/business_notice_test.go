package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestAutomaticNoticeAPIRequiresOperationsAndValidConfiguration(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	const path = "/api/official-account/automatic-notices"
	app.doJSON(t, http.MethodGet, path, "", nil, http.StatusUnauthorized, nil)
	student := app.loginStudent(t)
	teacher := app.loginAdmin(t, "13800000004")
	for _, token := range []string{student, teacher} {
		app.doJSON(t, http.MethodGet, path, token, nil, http.StatusForbidden, nil)
		app.doJSON(t, http.MethodPut, path+"/schedule_confirmed", token, map[string]any{"enabled": true, "templateId": "wrong"}, http.StatusForbidden, nil)
		app.doJSON(t, http.MethodPost, "/api/official-account/automatic-notice-tasks/missing/retry", token, nil, http.StatusForbidden, nil)
	}
	ops := app.loginAdmin(t, "13800000003")
	var bindings []learning.BusinessNoticeBinding
	app.doJSON(t, http.MethodGet, path, ops, nil, http.StatusOK, &bindings)
	if len(bindings) != 8 {
		t.Fatalf("unexpected bindings %d", len(bindings))
	}
	for _, binding := range bindings {
		if binding.Enabled {
			t.Fatal("default-on automatic notice")
		}
	}
	app.doJSON(t, http.MethodPut, path+"/schedule_confirmed", ops, map[string]any{"enabled": true, "templateId": "wrong"}, http.StatusBadRequest, nil)
	app.doJSON(t, http.MethodGet, "/api/student/business-notices/missing", student, nil, http.StatusForbidden, nil)
}

func TestStudentSubmissionRequestIDThroughAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)
	req := learning.SubmissionRequest{RequestID: "api-retry", HomeworkID: "hw-g05-english-s1-q1", Answers: []learning.SubmissionAnswer{{QuestionID: "q1", Choice: "A"}, {QuestionID: "q2", Text: "今天学会了抓中心句。"}}}
	var first, repeat struct {
		SubmissionID string `json:"submissionId"`
	}
	app.doJSON(t, http.MethodPost, "/api/student/submissions", token, req, http.StatusOK, &first)
	app.doJSON(t, http.MethodPost, "/api/student/submissions", token, req, http.StatusOK, &repeat)
	if first.SubmissionID == "" || first.SubmissionID != repeat.SubmissionID {
		t.Fatal("request ID did not survive HTTP binding")
	}
	req.Answers[0].Choice = "B"
	app.doJSON(t, http.MethodPost, "/api/student/submissions", token, req, http.StatusBadRequest, nil)
	var result learning.Submission
	app.doJSON(t, http.MethodGet, "/api/student/submissions/"+first.SubmissionID, token, nil, http.StatusOK, &result)
	if result.Answers[0].Choice != "A" {
		t.Fatal("conflicting retry changed saved answer")
	}
}
