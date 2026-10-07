package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestReviewExceptionRoutesPermissionAndExplicitState(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	reasons := "/api/reviews/exception-reasons"
	path := "/api/reviews/rev-001/exception"
	req := learning.ReviewExceptionRequest{RequestID: "incident-1", ClassName: "隔离测试班", Reason: "作业缺页"}
	app.doJSON(t, http.MethodGet, reasons, "", nil, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodPost, path, "", req, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodGet, reasons, app.loginStudent(t), nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, path, app.loginStudent(t), req, http.StatusForbidden, nil)
	if _, err := app.store.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeReviewException, Enabled: false, ApprovedReasons: []string{"作业缺页"}}); err != nil {
		t.Fatal(err)
	}
	teacher := app.loginAdmin(t, "13800000004")
	var list []string
	app.doJSON(t, http.MethodGet, reasons, teacher, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0] != req.Reason {
		t.Fatal("approved choices not available to teacher")
	}
	var review learning.Review
	app.doJSON(t, http.MethodPost, path, teacher, req, http.StatusOK, &review)
	if review.Status != "批改异常" || review.ExceptionReason != req.Reason || review.ExceptionEventID == "" {
		t.Fatal("explicit exception not saved")
	}
	app.doJSON(t, http.MethodPost, path, teacher, req, http.StatusOK, &review)
	if len(app.store.BusinessNoticeTasks()) != 0 {
		t.Fatal("disabled business emitted external tasks")
	}
	req.Reason = "未配置原因"
	app.doJSON(t, http.MethodPost, path, teacher, req, http.StatusBadRequest, nil)
	app.doJSON(t, http.MethodPost, "/api/reviews/rev-001/complete", teacher, learning.ReviewCompleteRequest{Score: 90, TeacherComment: "恢复正常批改", FinalStatus: "待复核"}, http.StatusOK, nil)
	var pending []learning.Review
	app.doJSON(t, http.MethodGet, "/api/reviews/pending", teacher, nil, http.StatusOK, &pending)
	for _, item := range pending {
		if item.ID == "rev-001" && (item.Status != "待复核" || item.ExceptionEventID != "" || item.ExceptionReason != "") {
			t.Fatal("completion did not clear exception")
		}
	}
}
