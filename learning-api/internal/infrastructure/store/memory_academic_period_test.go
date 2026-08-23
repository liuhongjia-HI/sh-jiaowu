package store

import (
	"encoding/json"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func currentYearCalendar(endDate string) []academicCalendarTerm {
	now := time.Now()
	startYear := now.Year()
	if now.Month() < time.July {
		startYear--
	}
	return []academicCalendarTerm{
		{AcademicYear: currentAcademicYear(), Semester: "S1 第一学期", StartDate: time.Date(startYear, time.September, 1, 0, 0, 0, 0, time.Local).Format("2006-01-02"), EndDate: time.Date(startYear+1, time.January, 15, 0, 0, 0, 0, time.Local).Format("2006-01-02")},
		{AcademicYear: currentAcademicYear(), Semester: "S2 第二学期", StartDate: time.Date(startYear+1, time.February, 1, 0, 0, 0, 0, time.Local).Format("2006-01-02"), EndDate: endDate},
	}
}

func TestGrantDefaultsFollowCurrentAcademicCalendar(t *testing.T) {
	store := NewMemoryStore()
	endDate := time.Now().AddDate(0, 6, 0).Format("2006-01-02")
	calendar, err := json.Marshal(currentYearCalendar(endDate))
	if err != nil {
		t.Fatalf("failed to encode calendar terms: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicCalendar", Value: string(calendar)}); err != nil {
		t.Fatalf("expected calendar update: %v", err)
	}
	start, end := store.defaultGrantPeriod()
	if start != time.Now().Format("2006-01-02") || end != endDate {
		t.Fatalf("expected current calendar period, got %s - %s", start, end)
	}
}

func TestConfiguredAcademicYearAlwaysFollowsDate(t *testing.T) {
	store := NewMemoryStore()
	if got := store.configuredAcademicYear(); got != currentAcademicYear() {
		t.Fatalf("expected date-derived academic year %q, got %q", currentAcademicYear(), got)
	}
	if _, err := store.UpdateSetting("超级管理员", learning.SettingUpdateRequest{Key: "academicYear", Value: "2099.2100学年"}); err == nil {
		t.Fatal("expected retired academicYear setting to reject updates")
	}
}

// TestConfiguredAcademicYearPrefersCalendarOverDateRule 校历是学年判定的唯一权威口径：
// 今天落在校历配置的哪个学期区间里，就是哪个学年——即便这和「7 月 1 日切学年」的
// 兜底规则算出来的结果不一样。这个口径要和 resolveScheduleTerm（排课学年判定）
// 保持一致，否则同一天，排课和学生年级/套餐默认学年会各自算出不同的学年。
func TestConfiguredAcademicYearPrefersCalendarOverDateRule(t *testing.T) {
	store := NewMemoryStore()
	today := time.Now().Format("2006-01-02")
	naiveYear := currentAcademicYear()
	overrideYear := naiveYear + "-校历订正"
	calendar, err := json.Marshal([]academicCalendarTerm{
		{AcademicYear: overrideYear, Semester: "S1 第一学期", StartDate: today, EndDate: today},
	})
	if err != nil {
		t.Fatalf("failed to encode calendar terms: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicCalendar", Value: string(calendar)}); err != nil {
		t.Fatalf("expected calendar update: %v", err)
	}
	if got := store.configuredAcademicYear(); got != overrideYear {
		t.Fatalf("expected calendar-covered date to override the 7/1 fallback, got %q want %q", got, overrideYear)
	}

	// 校历里没有覆盖今天时，退回 7 月 1 日规则，不阻塞任何业务操作。
	emptyCalendar, err := json.Marshal([]academicCalendarTerm{
		{AcademicYear: overrideYear, Semester: "S1 第一学期", StartDate: "2000-01-01", EndDate: "2000-01-02"},
	})
	if err != nil {
		t.Fatalf("failed to encode calendar terms: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "academicCalendar", Value: string(emptyCalendar)}); err != nil {
		t.Fatalf("expected calendar update: %v", err)
	}
	if got := store.configuredAcademicYear(); got != naiveYear {
		t.Fatalf("expected fallback to 7/1 rule when calendar does not cover today, got %q want %q", got, naiveYear)
	}
}

func TestNewStudentDoesNotReceiveExamPeriodDeadline(t *testing.T) {
	store := NewMemoryStore()
	admin, _ := store.PrincipalByUserID("user-super")
	created, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{Name: "时间测试学生", Phone: "13900008888", Grade: "五年级", AccountStatus: "正常"})
	if err != nil {
		t.Fatalf("expected student creation: %v", err)
	}
	if created.EffectiveUntil != "" {
		t.Fatalf("expected no deadline before a package is opened, got %s", created.EffectiveUntil)
	}
}

func TestEnsureDefaultSettingsRemovesRetiredKeys(t *testing.T) {
	store := NewMemoryStore()
	store.settings["academicYear"] = "2026.2027学年"
	store.settings["academicPeriods"] = `[{"name":"期中","startDate":"2026-11-01","endDate":"2026-11-15"}]`

	store.ensureDefaultSettings()

	for _, key := range retiredSettingKeys {
		if _, exists := store.settings[key]; exists {
			t.Fatalf("expected retired setting %q to be removed", key)
		}
	}
	if store.settings["academicCalendar"] == "" {
		t.Fatal("expected academicCalendar to remain as the date configuration")
	}
}
