package store

import (
	"encoding/json"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// 套餐默认有效期跟随系统设置里的校历学年；改校历只影响新开通，不回写存量记录。
// setAcademicCalendar 是测试专用的小工具：把“当前学年”和校历学期列表一起配好，
// 因为 academicCalendarRange 只汇总 term.AcademicYear 等于配置的当前学年的条目。
func setAcademicCalendar(t *testing.T, store *MemoryStore, academicYear string, terms []academicCalendarTerm) {
	t.Helper()
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicYear", Value: academicYear}); err != nil {
		t.Fatalf("expected academic year update: %v", err)
	}
	encoded, err := json.Marshal(terms)
	if err != nil {
		t.Fatalf("failed to encode calendar terms: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicCalendar", Value: string(encoded)}); err != nil {
		t.Fatalf("expected calendar update: %v", err)
	}
}

// 到期日应该是当前学年里最晚的学期结束日（校历按学期存，两个学期各自的
// 起止日期需要合并成一个覆盖全学年的区间），存量开通记录不受影响。
func TestGrantDefaultsFollowAcademicCalendarWithoutChangingExistingGrant(t *testing.T) {
	store := NewMemoryStore()
	existingEnd := store.grants[0].EndsAt
	// 用一段完全在未来的学年，避免依赖“今天”落在学年内的哪个位置。
	setAcademicCalendar(t, store, "2099.2100学年", []academicCalendarTerm{
		{AcademicYear: "2099.2100学年", Semester: "S1 第一学期", StartDate: "2099-09-01", EndDate: "2100-01-15"},
		{AcademicYear: "2099.2100学年", Semester: "S2 第二学期", StartDate: "2100-02-01", EndDate: "2100-07-15"},
	})
	// 到期日跟随校历学年末；起始日仍是当天——开通即生效，不能等到开学才给内容。
	start, end := store.defaultGrantPeriod()
	if start != time.Now().Format("2006-01-02") {
		t.Fatalf("开通应当天生效，得到起始日 %s", start)
	}
	if end != "2100-07-15" {
		t.Fatalf("到期日应跟随校历学年最后一个学期的结束日，得到 %s", end)
	}
	if store.grants[0].EndsAt != existingEnd {
		t.Fatal("changing the calendar must not rewrite existing student grants")
	}
}

// 学年已经开学的情况下，开通日在学年内就从当天起算，但仍统一到学年结束日到期。
func TestGrantDefaultStartsTodayWhenAcademicYearAlreadyStarted(t *testing.T) {
	store := NewMemoryStore()
	setAcademicCalendar(t, store, "跨越今天的测试学年", []academicCalendarTerm{
		{AcademicYear: "跨越今天的测试学年", Semester: "S1 第一学期", StartDate: "2000-09-01", EndDate: "2099-01-15"},
		{AcademicYear: "跨越今天的测试学年", Semester: "S2 第二学期", StartDate: "2099-02-01", EndDate: "2100-07-15"},
	})
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
	setAcademicCalendar(t, store, "已过期的测试学年", []academicCalendarTerm{
		{AcademicYear: "已过期的测试学年", Semester: "S1 第一学期", StartDate: "2000-09-01", EndDate: "2001-01-15"},
		{AcademicYear: "已过期的测试学年", Semester: "S2 第二学期", StartDate: "2001-02-01", EndDate: "2001-07-15"},
	})
	start, end := store.defaultGrantPeriod()
	if end <= start {
		t.Fatalf("fallback period must be valid, got %s - %s", start, end)
	}
	if end < time.Now().Format("2006-01-02") {
		t.Fatalf("fallback must not produce an already-expired grant, got %s", end)
	}
}

// 校历允许同时保留往年、当年、下一年的条目：当前学年只挑 AcademicYear 匹配的那几条，
// 不会被无关学年的日期干扰到期日计算。
func TestAcademicCalendarIgnoresOtherAcademicYears(t *testing.T) {
	store := NewMemoryStore()
	setAcademicCalendar(t, store, "2050.2051学年", []academicCalendarTerm{
		{AcademicYear: "2049.2050学年", Semester: "S2 第二学期", StartDate: "2050-02-01", EndDate: "2050-07-15"},
		{AcademicYear: "2050.2051学年", Semester: "S1 第一学期", StartDate: "2050-09-01", EndDate: "2051-01-15"},
		{AcademicYear: "2050.2051学年", Semester: "S2 第二学期", StartDate: "2051-02-01", EndDate: "2051-07-15"},
		{AcademicYear: "2051.2052学年", Semester: "S1 第一学期", StartDate: "2051-09-01", EndDate: "2052-01-15"},
	})
	_, end := store.defaultGrantPeriod()
	if end != "2051-07-15" {
		t.Fatalf("expected end date scoped to the configured academic year only, got %s", end)
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

// 复现管理端截图里的真实 bug：升级前的老部署已经把 grantDefaultStart/
// grantDefaultEnd（以及后来一度存在的 academicYearStart/academicYearEnd）
// 写进了 s.settings。defaultSettings 里已经没有这些键了，但 ensureDefaultSettings
// 原来只补缺的键、从不删多余的键，导致这些废弃项在系统设置列表里永远留着，
// 编辑框还能打开，容易让人误以为它们仍然生效。
func TestEnsureDefaultSettingsRemovesRetiredKeys(t *testing.T) {
	store := NewMemoryStore()
	store.settings["grantDefaultStart"] = "2026-08-16"
	store.settings["grantDefaultEnd"] = "2027-08-16"
	store.settings["academicYearStart"] = "2026-09-01"
	store.settings["academicYearEnd"] = "2027-07-15"

	store.ensureDefaultSettings()

	for _, key := range retiredSettingKeys {
		if _, exists := store.settings[key]; exists {
			t.Fatalf("expected retired setting %q to be removed, still present with value %q", key, store.settings[key])
		}
	}
	if store.settings["academicCalendar"] == "" {
		t.Fatal("expected academicCalendar to be seeded as the replacement setting")
	}
}
