package router_test

import (
	"net/http"
	"testing"
)

func TestOfficialTargetedRequiresOperationsRole(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	path := "/api/official-account/recipient?phone=invalid"
	app.doJSON(t, http.MethodGet, path, "", nil, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodGet, path, app.loginStudent(t), nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodGet, path, app.loginAdmin(t, "13800000004"), nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodGet, path, app.loginAdmin(t, "13800000003"), nil, http.StatusBadRequest, nil)
	app.doJSON(t, http.MethodPost, "/api/official-account/campaigns/preview", app.loginAdmin(t, "13800000003"), map[string]any{"recipientMode": "specified", "guardianIds": []string{}}, http.StatusBadRequest, nil)
}
