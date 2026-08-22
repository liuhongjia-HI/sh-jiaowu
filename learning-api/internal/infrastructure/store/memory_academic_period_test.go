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
