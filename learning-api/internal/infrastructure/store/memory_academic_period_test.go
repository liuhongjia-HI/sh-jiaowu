package store

import (
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// 套餐默认有效期跟随系统设置里的校历学年；改校历只影响新开通，不回写存量记录。
func TestGrantDefaultsFollowAcademicCalendarWithoutChangingExistingGrant(t *testing.T) {
	store := NewMemoryStore()
	existingEnd := store.grants[0].EndsAt
	// 用一段完全在未来的学年，避免依赖“今天”落在学年内的哪个位置。
	// 必须先改结束日再改开始日：校验会拿新值和当前存着的另一半比较，
	// 先把开始日推到结束日之后会被判成“结束早于开始”。
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYearEnd", Value: "2100-07-15"}); err != nil {
		t.Fatalf("expected calendar end update: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYearStart", Value: "2099-09-01"}); err != nil {
		t.Fatalf("expected calendar start update: %v", err)
	}
	// 到期日跟随校历学年末；起始日仍是当天——开通即生效，不能等到开学才给内容。
	start, end := store.defaultGrantPeriod()
	if start != time.Now().Format("2006-01-02") {
		t.Fatalf("开通应当天生效，得到起始日 %s", start)
	}
	if end != "2100-07-15" {
		t.Fatalf("到期日应跟随校历学年末，得到 %s", end)
	}
	if store.grants[0].EndsAt != existingEnd {
		t.Fatal("changing the calendar must not rewrite existing student grants")
	}
}

// 学年已经开学的情况下，开通日在学年内就从当天起算，但仍统一到学年结束日到期。
func TestGrantDefaultStartsTodayWhenAcademicYearAlreadyStarted(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYearStart", Value: "2000-09-01"}); err != nil {
		t.Fatalf("expected calendar start update: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYearEnd", Value: "2100-07-15"}); err != nil {
		t.Fatalf("expected calendar end update: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	start, end := store.defaultGrantPeriod()
	if start != today {
		t.Fatalf("expected start to be today (%s) when the year already started, got %s", today, start)
	}
	if end != "2100-07-15" {
		t.Fatalf("expected end to stay on the calendar end date, got %s", end)
	}
}

// 校历结束日已经过去（管理端忘了更新）时不能套用过期学年，否则新开通立刻就是过期的。
func TestGrantDefaultFallsBackWhenCalendarExpired(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYearStart", Value: "2000-09-01"}); err != nil {
		t.Fatalf("expected calendar start update: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYearEnd", Value: "2001-07-15"}); err != nil {
		t.Fatalf("expected calendar end update: %v", err)
	}
	start, end := store.defaultGrantPeriod()
	if end <= start {
		t.Fatalf("fallback period must be valid, got %s - %s", start, end)
	}
	if end < time.Now().Format("2006-01-02") {
		t.Fatalf("fallback must not produce an already-expired grant, got %s", end)
	}
}

// 系统设置里的“当前学年”允许和学习空间上标注的学年不一致：学习空间是
// 跨学年复用的课程目录，不参与学年匹配（见 learningSpaceMatches），
// 这条测试锁的是“不一致时套餐依然能正常绑定”，防止以后又把这两者耦合回去。
func TestConfiguredAcademicYearIndependentFromSeededLearningSpaces(t *testing.T) {
	store := NewMemoryStore()
	if len(store.learningSpaces) == 0 {
		t.Fatal("expected seeded learning spaces")
	}
	if _, err := store.UpdateSetting("超级管理员", learning.SettingUpdateRequest{Key: "academicYear", Value: "2099.2100学年"}); err != nil {
		t.Fatalf("expected academic year update: %v", err)
	}
	if got := store.configuredAcademicYear(); got != "2099.2100学年" {
		t.Fatalf("expected configured academic year to update independently, got %q", got)
	}
	if _, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name: "跟随新学年套餐", Grade: "五年级", Subject: "英文", Semester: "S1",
		PhaseScope: "Q1", PackageType: "题", LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"}, Status: learning.StatusEnabled,
	}); err != nil {
		t.Fatalf("expected package binding to succeed even though the space's year label differs: %v", err)
	}
}

func TestAcademicPeriodProvidesNewStudentDeadline(t *testing.T) {
	store := NewMemoryStore()
	periods := `[{"name":"期中","startDate":"2099-10-01","endDate":"2099-10-15"},{"name":"期末","startDate":"2100-01-01","endDate":"2100-01-15"}]`
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicPeriods", Value: periods}); err != nil {
		t.Fatalf("expected period update: %v", err)
	}
	admin, _ := store.PrincipalByUserID("user-super")
	created, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{Name: "时间测试学生", Phone: "13900008888", Grade: "五年级", AccountStatus: "正常"})
	if err != nil {
		t.Fatalf("expected student creation: %v", err)
	}
	if created.EffectiveUntil != "2099-10-15" {
		t.Fatalf("expected nearest configured exam deadline, got %s", created.EffectiveUntil)
	}
}
