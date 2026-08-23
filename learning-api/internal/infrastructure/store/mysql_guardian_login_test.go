package store

import (
	"errors"
	"os"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// 登录现在对同一次调用可能触发两次独立的持久化事务（先把手机号命中的几个
// 学生都关联到家长，再走正常的登录/绑定逻辑，见 memory_identity.go 里的注释）。
// 内存态测试测不出"两次事务分别落库"这件事本身有没有问题，必须过一遍真实
// MySQL：登录、重启、确认家长和两条关系都完整地在库里，且这中间没有产生
// 任何重复/半途而废的行。
func TestMySQLMultiChildLoginPersistsGuardianAcrossRestart(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	nonce := time.Now().Format("20060102150405.000000000")
	phone := "180" + nonce[15:24]

	first := NewMemoryStoreWithOptions(Options{
		SkipBaseData: true, BootstrapAdminName: "集成测试管理员",
		BootstrapAdminPhone: "13900000003", BootstrapAdminPassword: "Integration123!",
	})
	if err := first.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect first store: %v", err)
	}
	admin, err := first.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load bootstrap admin: %v", err)
	}
	elder, err := first.CreateStudent("MySQL集成测试", admin, learning.StudentUpsertRequest{
		Name: "登录测试哥哥", Phone: phone, Grade: "五年级", SchoolName: "集成测试学校",
	})
	if err != nil {
		t.Fatalf("create elder: %v", err)
	}
	younger, err := first.CreateStudent("MySQL集成测试", admin, learning.StudentUpsertRequest{
		Name: "登录测试弟弟", Phone: phone, Grade: "三年级", SchoolName: "集成测试学校",
	})
	if err != nil {
		t.Fatalf("create younger: %v", err)
	}

	_, err = first.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "mysql-multi-child", Phone: phone, StudentName: elder.Name, SchoolName: "集成测试学校", Grade: "五年级",
	})
	var selectionErr *learning.StudentSelectionRequiredError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("expected selection required, got %v", err)
	}
	principal, err := first.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "mysql-multi-child-2", Phone: phone, StudentName: elder.Name, SchoolName: "集成测试学校", Grade: "五年级",
		SelectedStudentID: elder.ID,
	})
	if err != nil {
		t.Fatalf("expected login to succeed after selection: %v", err)
	}
	if principal.GuardianID == "" {
		t.Fatalf("expected a guardian identity, got %#v", principal)
	}

	switched, err := first.SwitchStudentAccount(principal, younger.ID)
	if err != nil {
		t.Fatalf("switch to younger sibling: %v", err)
	}
	if switched.StudentID != younger.ID {
		t.Fatalf("expected switched principal to be the younger sibling, got %#v", switched)
	}

	if err := first.db.Close(); err != nil {
		t.Fatalf("close first store db: %v", err)
	}

	second := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := second.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect restarted store: %v", err)
	}
	defer second.db.Close()

	if !containsGuardian(second.guardians, principal.GuardianID) {
		t.Fatalf("expected guardian %q to survive restart", principal.GuardianID)
	}
	if countRelationsFor(second.guardianStudents, principal.GuardianID) != 2 {
		t.Fatalf("expected exactly 2 relations to survive restart, got %d", countRelationsFor(second.guardianStudents, principal.GuardianID))
	}
	var reloadedGuardian learning.Guardian
	for _, guardian := range second.guardians {
		if guardian.ID == principal.GuardianID {
			reloadedGuardian = guardian
			break
		}
	}
	// switch 之后应该把"最近查看"更新成弟弟，而不是停在最初登录选的哥哥上。
	if reloadedGuardian.LastStudentID != younger.ID {
		t.Fatalf("expected LastStudentID to persist the switch, got %#v", reloadedGuardian)
	}
	if !second.GuardianStudentActive(principal.GuardianID, elder.ID) || !second.GuardianStudentActive(principal.GuardianID, younger.ID) {
		t.Fatalf("expected both sibling relations to remain active after restart")
	}
}
