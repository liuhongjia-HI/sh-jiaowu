package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

// 复现生产上的真实数据形状（见管理端截图）：家长手机号 18518673993 下有两个
// 学生档案，但小程序里登录着的那个学生账号，档案上存的是**打码**手机号
// 185****3993（早期通过掩码手机号绑定流程建的），而后台新建的兄弟姐妹存的是
// 完整手机号。切换器必须把这两种写法认成同一个家长，否则家长永远只看得到
// 自己当初绑定的那一个孩子。
func TestSilentReloginLinksSiblingsAcrossMaskedAndPlainPhone(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})

	// 已经绑过微信的孩子：档案和登录账号上都是打码手机号。
	store.students = append(store.students, learning.Student{
		ID: "stu-bound", Name: "浪花", EnrollmentAcademicYear: currentAcademicYear(),
		EnrollmentGrade: "五年级", Phone: "185****3993", SchoolName: "乐成学校",
		AccountStatus: "正常", BindStatus: "已绑定",
	})
	store.users = append(store.users, learning.User{
		ID: "user-stu-bound", Name: "浪花", Phone: "185****3993", OpenID: "demo-family-openid",
		AccountStatus: "正常", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-bound",
	})
	// 后台新建的两个孩子：完整手机号，都还没绑微信。
	store.students = append(store.students,
		learning.Student{
			ID: "stu-plain-a", Name: "一个家长两个孩子", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "五年级", Phone: "18518673993", SchoolName: "星河小学", AccountStatus: "正常",
		},
		learning.Student{
			ID: "stu-plain-b", Name: "另外一个孩子", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "二年级", Phone: "18518673993", SchoolName: "星河小学", AccountStatus: "正常",
		},
	)

	// 家长没有重新授权手机号，只是照常打开小程序：静默登录，只带 openID。
	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "family-openid"})
	if err != nil {
		t.Fatalf("expected silent relogin to succeed: %v", err)
	}
	if principal.GuardianID == "" {
		t.Fatal("expected silent relogin to establish a guardian identity")
	}

	accounts, err := store.StudentAccounts(principal)
	if err != nil {
		t.Fatalf("expected account list: %v", err)
	}
	names := make([]string, 0, len(accounts))
	for _, account := range accounts {
		names = append(names, account.Name)
	}
	if len(accounts) != 3 {
		t.Fatalf("expected all 3 children on the same guardian phone, got %d: %v", len(accounts), names)
	}
}

// 存量会话兜底：多子女功能上线之前签发的 token 里没有 GuardianID，而小程序只要
// 本地 token 没过期就不会重新登录，家长自己没有任何办法触发刷新。中间件重建
// principal 时必须能从关系表反查回家长身份，否则这些家长要等 token 自然过期
// 才看得到切换器——这正是生产上"两个孩子但小程序只显示一个"的表现。
func TestLegacyTokenWithoutGuardianIDStillSeesSiblings(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	store.students = append(store.students,
		learning.Student{
			ID: "stu-a", Name: "老大", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "五年级", Phone: "18518673993", AccountStatus: "正常",
		},
		learning.Student{
			ID: "stu-b", Name: "老二", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "二年级", Phone: "18518673993", AccountStatus: "正常",
		},
	)
	store.users = append(store.users, learning.User{
		ID: "user-stu-a", Name: "老大", Phone: "18518673993", OpenID: "demo-legacy",
		AccountStatus: "正常", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-a",
	})
	// 先让关系建立起来（相当于家长某次打开过小程序）。
	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "legacy"}); err != nil {
		t.Fatalf("seed login: %v", err)
	}

	// 模拟中间件用**老 token**（没有 GuardianID）重建 principal 的那条路径。
	rebuilt, err := store.PrincipalByUserID("user-stu-a")
	if err != nil {
		t.Fatalf("rebuild principal: %v", err)
	}
	if rebuilt.GuardianID == "" {
		t.Fatal("expected guardian identity to be recovered for a token that never carried one")
	}
	accounts, err := store.StudentAccounts(rebuilt)
	if err != nil {
		t.Fatalf("account list: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected both siblings for a legacy session, got %#v", accounts)
	}
}

// 最坏的存量情况：token 里没有 GuardianID，库里也一条关系行都没有（多子女
// 功能上线之前就绑好的家长，之后从没重新登录过）。切换器必须仍然能显示，
// 并且点下去真的切得过去——只显示不能用比不显示更糟。
func TestLegacySessionWithNoRelationRowsCanStillListAndSwitch(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	store.students = append(store.students,
		learning.Student{
			ID: "stu-a", Name: "老大", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "五年级", Phone: "185****3993", AccountStatus: "正常",
		},
		learning.Student{
			ID: "stu-b", Name: "老二", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "二年级", Phone: "18518673993", AccountStatus: "正常",
		},
	)
	store.users = append(store.users,
		learning.User{
			ID: "user-stu-a", Name: "老大", Phone: "185****3993", AccountStatus: "正常",
			Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-a",
		},
		learning.User{
			ID: "user-stu-b", Name: "老二", Phone: "18518673993", AccountStatus: "正常",
			Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-b",
		},
	)
	if len(store.guardianStudents) != 0 {
		t.Fatal("fixture must start with no guardian relations at all")
	}

	principal, err := store.PrincipalByUserID("user-stu-a")
	if err != nil {
		t.Fatalf("rebuild principal: %v", err)
	}
	if principal.GuardianID != "" {
		t.Fatalf("fixture should have no guardian to derive, got %q", principal.GuardianID)
	}

	accounts, err := store.StudentAccounts(principal)
	if err != nil {
		t.Fatalf("account list: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected the phone fallback to surface both siblings, got %#v", accounts)
	}

	switched, err := store.SwitchStudentAccount(principal, "stu-b")
	if err != nil {
		t.Fatalf("expected the displayed switcher entry to actually work: %v", err)
	}
	if switched.StudentID != "stu-b" || switched.GuardianID == "" {
		t.Fatalf("expected switch to stu-b with a repaired guardian identity, got %#v", switched)
	}
	// 兜底只需要发生一次：切换之后关系应该已经补进库，回到正常路径。
	if !store.GuardianStudentActive(switched.GuardianID, "stu-b") {
		t.Fatal("expected the switch to persist a real guardian relation")
	}
}

// 安全边界：兜底用的是"同一个家长手机号"，不能因此换到别人家的孩子。
func TestPhoneFallbackDoesNotAllowSwitchingToAnotherFamily(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	store.students = append(store.students,
		learning.Student{ID: "stu-mine", Name: "我家孩子", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "五年级", Phone: "18500000001", AccountStatus: "正常"},
		learning.Student{ID: "stu-other", Name: "别人家孩子", EnrollmentAcademicYear: currentAcademicYear(),
			EnrollmentGrade: "五年级", Phone: "18900009999", AccountStatus: "正常"},
	)
	store.users = append(store.users,
		learning.User{ID: "user-stu-mine", Name: "我家孩子", Phone: "18500000001", AccountStatus: "正常",
			Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-mine"},
		learning.User{ID: "user-stu-other", Name: "别人家孩子", Phone: "18900009999", AccountStatus: "正常",
			Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-other"},
	)
	principal, err := store.PrincipalByUserID("user-stu-mine")
	if err != nil {
		t.Fatalf("rebuild principal: %v", err)
	}
	if _, err := store.SwitchStudentAccount(principal, "stu-other"); err == nil {
		t.Fatal("expected switching to a different family to be refused")
	}
}
