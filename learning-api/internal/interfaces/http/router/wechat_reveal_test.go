package router_test

import (
	"net/http"
	"testing"
)

func TestWechatRevealRequiresSettingsRole(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	req := map[string]string{"field": "callbackToken"}
	app.doJSON(t, http.MethodPost, "/api/wechat/settings/reveal", "", req, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodPost, "/api/wechat/settings/reveal", app.loginStudent(t), req, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/wechat/settings/reveal", app.loginAdmin(t, "13800000003"), req, http.StatusForbidden, nil)
	for _, phone := range []string{"13800000001", "13800000002"} {
		// Allowed roles reach field validation instead of the permission denial.
		app.doJSON(t, http.MethodPost, "/api/wechat/settings/reveal", app.loginAdmin(t, phone), map[string]string{"field": "invalid"}, http.StatusBadRequest, nil)
	}
}
