package store

import (
	"errors"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

// 复现真实用户看到的 bug：演示密码登录、老师/管理员登录、或者阶段2上线前
// 签发的旧 token，principal 里都没有 GuardianID（这些路径压根不经过
// ensureGuardianLink）。这种情况必须安安静静地返回空列表——不能报错，
// 因为小程序"我的"页每次都会调这个接口去判断要不要显示切换器，报错会
// 被 wx.showToast 不看 silent 选项直接弹给用户，绝大多数单孩子家庭会
// 平白多看到一条"当前账号未关联家长身份"的吓人提示（这就是原始 bug）。
func TestStudentAccountsIsQuietForNonGuardianLogins(t *testing.T) {
	store := NewMemoryStore()
	demoPrincipal, err := store.LoginWithDemoStudentPassword("18500009069", "123456")
	if err != nil {
		t.Fatalf("demo student login: %v", err)
	}
	if demoPrincipal.GuardianID != "" {
		t.Fatalf("expected demo password login to have no guardian identity, got %#v", demoPrincipal)
	}
	accounts, err := store.StudentAccounts(demoPrincipal)
	if err != nil {
		t.Fatalf("expected no error for a non-guardian login, got %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected an empty switcher list, got %#v", accounts)
	}
}

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

func TestSilentReloginRestoresDeviceSelectedStudent(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	firstStudent, _ := store.findStudent("stu-001")
	firstUser, _ := store.findUserByStudentID("stu-001")
	second, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "小明妹妹", Phone: firstUser.Phone, Grade: "三年级", AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("create sibling: %v", err)
	}
	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "device-parent-1", Phone: firstUser.Phone, StudentName: firstStudent.Name,
		SchoolName: "星河小学", Grade: "五年级", SelectedStudentID: firstStudent.ID,
	})
	if err != nil {
		t.Fatalf("login parent: %v", err)
	}
	if _, err := store.SwitchStudentAccount(principal, second.ID); err != nil {
		t.Fatalf("switch to sibling: %v", err)
	}
	// 模拟另一台设备切回老大，覆盖家长级别的最近查看值。
	if _, err := store.SwitchStudentAccount(principal, firstStudent.ID); err != nil {
		t.Fatalf("switch other device back: %v", err)
	}
	guardian, ok := store.guardianByID(principal.GuardianID)
	if !ok || guardian.LastStudentID != firstStudent.ID {
		t.Fatalf("expected guardian last student to be first child, got %#v", guardian)
	}
	resumed, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "device-parent-1", SelectedStudentID: second.ID,
	})
	if err != nil {
		t.Fatalf("silent relogin with device selection: %v", err)
	}
	if resumed.StudentID != second.ID || resumed.GuardianID != principal.GuardianID {
		t.Fatalf("expected device-selected sibling, got %#v", resumed)
	}
}

func TestParentCanAddAdditionalStudentAndSwitchWithoutAdminApproval(t *testing.T) {
	store := NewMemoryStore()
	firstStudent, ok := store.findStudent("stu-001")
	if !ok {
		t.Fatal("expected demo student")
	}
	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "add-child-request", Phone: firstStudent.Phone, StudentName: firstStudent.Name, SchoolName: "星河小学", Grade: store.decorateStudent(firstStudent).Grade,
	})
	if err != nil {
		t.Fatalf("login parent: %v", err)
	}

	added, err := store.RequestAdditionalStudent(principal, learning.StudentAccountAddRequest{
		Name: "小明妹妹", Grade: "五年级", SchoolName: "星河小学",
	})
	if err != nil {
		t.Fatalf("add additional student: %v", err)
	}
	if added.Status != "正常" || !added.CanSwitch {
		t.Fatalf("expected a switchable account without approval, got %#v", added)
	}

	accounts, err := store.StudentAccounts(principal)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("expected current and newly added students, accounts=%#v err=%v", accounts, err)
	}
	next, err := store.SwitchStudentAccount(principal, added.StudentID)
	if err != nil || next.StudentID != added.StudentID {
		t.Fatalf("expected newly added student to be switchable, next=%#v err=%v", next, err)
	}
}

func accountByStudentID(accounts []learning.StudentAccount, studentID string) learning.StudentAccount {
	for _, account := range accounts {
		if account.StudentID == studentID {
			return account
		}
	}
	return learning.StudentAccount{}
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
