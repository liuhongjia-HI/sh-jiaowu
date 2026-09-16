package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestStudentNoticeReadAllAPI(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	app.doJSON(t, http.MethodPost, "/api/student/notices/read-all", "", nil, http.StatusUnauthorized, nil)
	token := app.loginStudent(t)
	var notices []learning.Notice
	app.doJSON(t, http.MethodPost, "/api/student/notices/read-all", token, nil, http.StatusOK, &notices)
	app.doJSON(t, http.MethodGet, "/api/student/notices", token, nil, http.StatusOK, &notices)
	for _, notice := range notices {
		if !notice.IsRead {
			t.Fatalf("unread notice after batch: %s", notice.ID)
		}
	}
	app.doJSON(t, http.MethodPost, "/api/student/notices/read-all", token, nil, http.StatusOK, nil)
}
