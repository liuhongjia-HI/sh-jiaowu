package store

import (
	"errors"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

// 走真实的登录入口而不是直接摆数据，因为 StudentAccounts/SwitchStudentAccount
// 现在读的是 guardian_students 关系表，而这张表只在登录成功那一刻由
// ensureGuardianLink 建立——绕开登录直接调用会让这条测试跟真实用户路径脱节。
func TestParentPhoneCanSwitchBetweenStudentAccounts(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	firstUser, ok := store.findUserByStudentID("stu-001")
	if !ok {
		t.Fatal("expected demo student's login account")
	}
	firstStudent, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected demo student")
	}
	second, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "小明妹妹", Phone: firstUser.Phone, Grade: store.decorateStudent(firstStudent).Grade, AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("expected same guardian phone to be allowed: %v", err)
	}

	_, err = store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "parent-switch-test", Phone: firstUser.Phone, StudentName: firstStudent.Name, SchoolName: "星河小学", Grade: "五年级",
	})
	var selectionErr *learning.StudentSelectionRequiredError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("expected shared phone to require a selection, got %v", err)
	}

	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "parent-switch-test-2", Phone: firstUser.Phone, StudentName: firstStudent.Name, SchoolName: "星河小学", Grade: "五年级",
		SelectedStudentID: "stu-001",
	})
	if err != nil {
		t.Fatalf("expected selecting the first child to log in: %v", err)
	}
	if principal.GuardianID == "" {
		t.Fatalf("expected login to establish a guardian identity, got %#v", principal)
	}

	accounts, err := store.StudentAccounts(principal)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("expected sibling accounts, accounts=%#v err=%v", accounts, err)
	}

	next, err := store.SwitchStudentAccount(principal, second.ID)
	if err != nil || next.StudentID != second.ID || next.Name != second.Name {
		t.Fatalf("expected switched principal, principal=%#v err=%v", next, err)
	}
	if next.GuardianID != principal.GuardianID {
		t.Fatalf("expected switching to keep the same guardian identity, before=%q after=%q", principal.GuardianID, next.GuardianID)
	}

	// 不能切到跟自己毫无关系的学生——即使那个学生账号本身是正常状态。
	if _, err := store.SwitchStudentAccount(principal, "stu-002"); err == nil {
		t.Fatal("expected switching to an unrelated student to be rejected")
	}
}

// 关系被后台解除之后，family 手上那张旧 token 不能继续读那个孩子的数据——
// 这个校验现在挂在 GuardianStudentActive 上，中间件每个请求都会重新调用它。
func TestSwitchedGuardianCannotAccessAfterRelationRevoked(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")
	firstUser, _ := store.findUserByStudentID("stu-001")
	firstStudent, _ := store.findStudent("stu-001")
	second, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "小明妹妹", Phone: firstUser.Phone, Grade: store.decorateStudent(firstStudent).Grade, AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("create sibling: %v", err)
	}
	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "revoke-test", Phone: firstUser.Phone, StudentName: firstStudent.Name, SchoolName: "星河小学", Grade: "五年级",
		SelectedStudentID: "stu-001",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !store.GuardianStudentActive(principal.GuardianID, second.ID) {
		t.Fatal("expected guardian to be linked to the newly created sibling too")
	}

	// 手动解除关系，模拟后台把这个孩子从家长名下移除（转校/家长信息订正）。
	for i := range store.guardianStudents {
		if store.guardianStudents[i].GuardianID == principal.GuardianID && store.guardianStudents[i].StudentID == "stu-001" {
			store.guardianStudents[i].Status = learning.GuardianStudentInactive
		}
	}
	if store.GuardianStudentActive(principal.GuardianID, "stu-001") {
		t.Fatal("expected relation to be inactive after revocation")
	}
	// 另一个孩子的关系没动，应该还在——校验的是"逐条关系"而不是整个家长被拉黑。
	if !store.GuardianStudentActive(principal.GuardianID, second.ID) {
		t.Fatal("expected the untouched sibling relation to remain active")
	}
}
