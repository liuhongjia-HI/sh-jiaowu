package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestGrantDefaultsAreConfigurableWithoutChangingExistingGrant(t *testing.T) {
	store := NewMemoryStore()
	existingEnd := store.grants[0].EndsAt
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "grantDefaultStart", Value: "2026-09-01"}); err != nil {
		t.Fatalf("expected default start update: %v", err)
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "grantDefaultEnd", Value: "2027-06-30"}); err != nil {
		t.Fatalf("expected default end update: %v", err)
	}
	start, end := store.defaultGrantPeriod()
	if start != "2026-09-01" || end != "2027-06-30" {
		t.Fatalf("unexpected defaults %s - %s", start, end)
	}
	if store.grants[0].EndsAt != existingEnd {
		t.Fatal("changing defaults must not rewrite existing student grants")
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
