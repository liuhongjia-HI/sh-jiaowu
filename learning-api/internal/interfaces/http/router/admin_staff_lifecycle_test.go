package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestAdminStaffLoginHandoffLifecycle(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	super := app.loginAdmin(t, "13800000001")
	req := learning.AdminStaffUpsertRequest{Name: "账号交接测试", Phone: "13900003901", Role: learning.RoleOpsStaff, AccountStatus: "正常"}
	var staff learning.AdminStaff
	app.doJSON(t, http.MethodPost, "/api/admin-staff", super, req, http.StatusOK, &staff)
	if staff.TemporaryPassword == "" || staff.TemporaryPassword == "123456" || !staff.PasswordEnabled || !staff.MustChangePassword {
		t.Fatal("new staff must receive a generated temporary password and require change")
	}
	var rows []learning.AdminStaff
	app.doJSON(t, http.MethodGet, "/api/admin-staff", super, nil, http.StatusOK, &rows)
	for _, row := range rows {
		if row.TemporaryPassword != "" {
			t.Fatal("list must not expose temporary passwords")
		}
	}
	token := app.login(t, "/api/auth/admin-password-login", map[string]string{"phone": req.Phone, "password": staff.TemporaryPassword})
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", token, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/auth/change-password", token, learning.PasswordChangeRequest{OldPassword: staff.TemporaryPassword, NewPassword: "Staff2026Changed"}, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/auth/me", token, nil, http.StatusUnauthorized, nil)
	token = app.login(t, "/api/auth/admin-password-login", map[string]string{"phone": req.Phone, "password": "Staff2026Changed"})
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", token, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/admin-staff", token, nil, http.StatusForbidden, nil)
	req.AccountStatus = "停用"
	app.doJSON(t, http.MethodPut, "/api/admin-staff/"+staff.ID, super, req, http.StatusOK, nil)
	app.doJSON(t, http.MethodPost, "/api/auth/admin-password-login", "", map[string]string{"phone": req.Phone, "password": "Staff2026Changed"}, http.StatusUnauthorized, nil)
	req.AccountStatus = "正常"
	app.doJSON(t, http.MethodPut, "/api/admin-staff/"+staff.ID, super, req, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/auth/me", token, nil, http.StatusUnauthorized, nil)
	token = app.login(t, "/api/auth/admin-password-login", map[string]string{"phone": req.Phone, "password": "Staff2026Changed"})
	var reset learning.PasswordResetResult
	app.doJSON(t, http.MethodPost, "/api/admin-staff/"+staff.ID+"/reset-password", super, nil, http.StatusOK, &reset)
	app.doJSON(t, http.MethodGet, "/api/auth/me", token, nil, http.StatusUnauthorized, nil)
	app.doJSON(t, http.MethodPost, "/api/auth/admin-password-login", "", map[string]string{"phone": req.Phone, "password": "Staff2026Changed"}, http.StatusUnauthorized, nil)
	token = app.login(t, "/api/auth/admin-password-login", map[string]string{"phone": req.Phone, "password": reset.TemporaryPassword})
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", token, nil, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/auth/change-password", token, learning.PasswordChangeRequest{OldPassword: reset.TemporaryPassword, NewPassword: "Staff2026Reset"}, http.StatusOK, nil)
	token = app.login(t, "/api/auth/admin-password-login", map[string]string{"phone": req.Phone, "password": "Staff2026Reset"})
	app.doJSON(t, http.MethodGet, "/api/dashboard/overview", token, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/admin-staff", super, nil, http.StatusOK, &rows)
	for _, row := range rows {
		if row.ID == staff.ID && (row.MustChangePassword || !row.PasswordEnabled) {
			t.Fatal("list must reflect completed password setup")
		}
	}
	wechat := app.login(t, "/api/auth/wechat-login", map[string]string{"code": "staff-lifecycle-wechat", "phone": req.Phone})
	app.doJSON(t, http.MethodGet, "/api/auth/me", wechat, nil, http.StatusOK, nil)
	app.doJSON(t, http.MethodGet, "/api/admin-staff", super, nil, http.StatusOK, &rows)
	for _, row := range rows {
		if row.ID == staff.ID && row.BindStatus != "已绑定" {
			t.Fatal("list must reflect WeChat binding")
		}
	}
}
