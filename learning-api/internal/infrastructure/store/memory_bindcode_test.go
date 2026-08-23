package store

import (
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestGenerateStudentBindCodeProducesFreshHighEntropyCode(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("admin principal: %v", err)
	}
	updated, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("generate bind code: %v", err)
	}
	if len(updated.BindCode) != bindCodeLength {
		t.Fatalf("expected an %d-char bind code, got %q", bindCodeLength, updated.BindCode)
	}
	for _, r := range updated.BindCode {
		if !strings.ContainsRune(bindCodeAlphabet, r) {
			t.Fatalf("bind code %q contains a character outside the allowed alphabet", updated.BindCode)
		}
	}
	wantExpiry := time.Now().AddDate(0, 0, bindCodeValidDays).Format("2006-01-02")
	if updated.BindCodeExpiresAt != wantExpiry {
		t.Fatalf("expected expiry %q, got %q", wantExpiry, updated.BindCodeExpiresAt)
	}
}

// 重新生成让旧码立刻失效——同一时刻只应该有一个有效码在外面流通。
func TestRegeneratingStudentBindCodeInvalidatesTheOldOne(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")
	first, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	second, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if first.BindCode == second.BindCode {
		t.Fatalf("expected regenerating to change the code, both are %q", first.BindCode)
	}
	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "old-code-claim", Phone: "13600001234", BindCode: first.BindCode,
	}); err == nil || !strings.Contains(err.Error(), "绑定码不存在") {
		t.Fatalf("expected the old code to stop working, got %v", err)
	}
}

// 核心场景：第二个家长（自己的手机号跟已有任何档案都不一样）凭码关联到孩子，
// 不经过手机号匹配那条路，也不打扰第一个家长已有的登录状态。
func TestSecondGuardianClaimsChildByBindCode(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")

	firstGuardianPrincipal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "first-guardian-login", Phone: "18500009069", StudentName: "小明", SchoolName: "星河小学", Grade: "五年级",
	})
	if err != nil {
		t.Fatalf("first guardian login: %v", err)
	}

	code, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("generate bind code: %v", err)
	}

	secondGuardianPrincipal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "second-guardian-claim", Phone: "13911112222", BindCode: code.BindCode,
	})
	if err != nil {
		t.Fatalf("second guardian claim: %v", err)
	}
	if secondGuardianPrincipal.StudentID != "stu-001" {
		t.Fatalf("expected claim to resolve to stu-001, got %#v", secondGuardianPrincipal)
	}
	if secondGuardianPrincipal.GuardianID == "" || secondGuardianPrincipal.GuardianID == firstGuardianPrincipal.GuardianID {
		t.Fatalf("expected the second guardian to get their own distinct guardian identity, first=%q second=%q",
			firstGuardianPrincipal.GuardianID, secondGuardianPrincipal.GuardianID)
	}
	if !store.GuardianStudentActive(secondGuardianPrincipal.GuardianID, "stu-001") {
		t.Fatal("expected the second guardian to be actively linked to stu-001")
	}
	// 没打扰到第一个家长。
	if !store.GuardianStudentActive(firstGuardianPrincipal.GuardianID, "stu-001") {
		t.Fatal("expected the first guardian's relation to remain untouched")
	}

	accounts, err := store.StudentAccounts(secondGuardianPrincipal)
	if err != nil || len(accounts) != 1 || accounts[0].StudentID != "stu-001" {
		t.Fatalf("expected the second guardian's switcher to show exactly stu-001, accounts=%#v err=%v", accounts, err)
	}
}

func TestClaimByBindCodeRejectsUnknownOrExpiredCode(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")

	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "unknown-code", Phone: "13911112222", BindCode: "ZZZZZZZZ",
	}); err == nil || !strings.Contains(err.Error(), "绑定码不存在") {
		t.Fatalf("expected unknown bind code to be rejected, got %v", err)
	}

	code, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("generate bind code: %v", err)
	}
	for i := range store.students {
		if store.students[i].ID == "stu-001" {
			store.students[i].BindCodeExpiresAt = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		}
	}
	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "expired-code", Phone: "13911112222", BindCode: code.BindCode,
	}); err == nil || !strings.Contains(err.Error(), "已过期") {
		t.Fatalf("expected expired bind code to be rejected, got %v", err)
	}
}

// 凭码关联不需要填学生姓名/学校/年级这些信息——那是"手机号匹配不到档案"
// 那条路才需要的核验，凭码本身就是已经验证过身份的凭证。
func TestClaimByBindCodeDoesNotRequireProfileFields(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")
	code, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("generate bind code: %v", err)
	}
	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "no-profile-claim", Phone: "13911112222", BindCode: code.BindCode,
	})
	if err != nil {
		t.Fatalf("expected claim without profile fields to succeed, got %v", err)
	}
	if principal.StudentID != "stu-001" {
		t.Fatalf("unexpected principal: %#v", principal)
	}
}

func TestClaimByBindCodeRequiresPhone(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")
	code, err := store.GenerateStudentBindCode("超级管理员", admin, "stu-001")
	if err != nil {
		t.Fatalf("generate bind code: %v", err)
	}
	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "no-phone-claim", BindCode: code.BindCode,
	}); err == nil || !strings.Contains(err.Error(), "请授权手机号") {
		t.Fatalf("expected missing phone to be rejected, got %v", err)
	}
}

// generateStudentBindCodeUnlocked 复用了 visibleStudent 而不是自己另起一套
// 权限判断——用一个明确没有权限访问该学生的账号来锁住这一点：换一个类似
// UpdateStudent 那样自建校验的实现，这条测试会失败。
func TestGenerateStudentBindCodeGoesThroughVisibilityCheck(t *testing.T) {
	store := NewMemoryStore()
	unrelatedTeacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("teacher principal: %v", err)
	}
	_, genErr := store.GenerateStudentBindCode("英语老师", unrelatedTeacher, "stu-001")
	_, visErr := store.StudentDetail(unrelatedTeacher, "stu-001")
	if (genErr == nil) != (visErr == nil) {
		t.Fatalf("expected GenerateStudentBindCode's access decision to match StudentDetail's (same visibleStudent gate), bindCodeErr=%v detailErr=%v", genErr, visErr)
	}
}
