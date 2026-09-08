package router_test

import (
	"net/http"
	"testing"
)

// 即使客户端仍持有下载地址，也必须在每次请求时校验当前权限。
func TestStudentMaterialDownloadRejectsMissingPermission(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginStudent(t)
	req, err := http.NewRequest(http.MethodGet, app.server.URL+"/api/student/materials/mat-g05-english-s1-q1/download", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized download status=%d, want 403", resp.StatusCode)
	}
}
