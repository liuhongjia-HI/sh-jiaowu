package store

import (
	"os"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// 绑定码和"凭码建立的家长关系"都要经得住重启——机构后台生成完码、家长
// 过几天才扫，中间服务重启了一次，码不能跟着丢。
func TestMySQLBindCodeAndClaimSurviveStoreRestart(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	nonce := time.Now().Format("20060102150405.000000000")
	childPhone := "181" + nonce[15:24]
	guardianPhone := "132" + nonce[15:24]

	first := NewMemoryStoreWithOptions(Options{
		SkipBaseData: true, BootstrapAdminName: "集成测试管理员",
		BootstrapAdminPhone: "13900000004", BootstrapAdminPassword: "Integration123!",
	})
	if err := first.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect first store: %v", err)
	}
	admin, err := first.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load bootstrap admin: %v", err)
	}
	child, err := first.CreateStudent("MySQL集成测试", admin, learning.StudentUpsertRequest{
		Name: "绑定码测试学生", Phone: childPhone, Grade: "五年级", SchoolName: "集成测试学校",
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	withCode, err := first.GenerateStudentBindCode("集成测试管理员", admin, child.ID)
	if err != nil {
		t.Fatalf("generate bind code: %v", err)
	}
	principal, err := first.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "mysql-bindcode-claim", Phone: guardianPhone, BindCode: withCode.BindCode,
	})
	if err != nil {
		t.Fatalf("claim by bind code: %v", err)
	}
	if principal.StudentID != child.ID || principal.GuardianID == "" {
		t.Fatalf("unexpected claim principal: %#v", principal)
	}

	if err := first.db.Close(); err != nil {
		t.Fatalf("close first store db: %v", err)
	}

	second := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := second.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect restarted store: %v", err)
	}
	defer second.db.Close()

	reloadedChild, ok := second.findStudent(child.ID)
	if !ok {
		t.Fatalf("child missing after restart")
	}
	if reloadedChild.BindCode != withCode.BindCode || reloadedChild.BindCodeExpiresAt != withCode.BindCodeExpiresAt {
		t.Fatalf("bind code did not survive restart, got %#v", reloadedChild)
	}
	if !second.GuardianStudentActive(principal.GuardianID, child.ID) {
		t.Fatalf("expected the claimed relation to survive restart")
	}
}
