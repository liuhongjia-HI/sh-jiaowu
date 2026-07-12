package store

import "testing"

func TestNewMemoryStoreCanStartWithoutDemoData(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})

	if len(store.users) != 0 {
		t.Fatalf("expected no demo users, got %d", len(store.users))
	}
	if len(store.students) != 0 {
		t.Fatalf("expected no demo students, got %d", len(store.students))
	}
	if len(store.packages) != 0 || len(store.courses) != 0 || len(store.materials) != 0 || len(store.homework) != 0 {
		t.Fatalf("expected no demo learning data, got packages=%d courses=%d materials=%d homework=%d", len(store.packages), len(store.courses), len(store.materials), len(store.homework))
	}
	if store.settings["academicYear"] == "" {
		t.Fatal("expected base settings to remain available")
	}
	if len(store.learningSpaces) == 0 {
		t.Fatal("expected base learning spaces to remain available")
	}
	foundEnglish := false
	for _, space := range store.learningSpaces {
		if space.Grade == "五年级" && space.Subject == "英语" && space.Semester == "S1" {
			foundEnglish = true
			break
		}
	}
	if !foundEnglish {
		t.Fatal("expected base learning spaces to include 五年级英语 S1")
	}
}

func TestNewMemoryStoreCanSkipAllBootstrapData(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false, SkipBaseData: true})

	if len(store.users) != 0 || len(store.students) != 0 || len(store.packages) != 0 || len(store.learningSpaces) != 0 {
		t.Fatalf("expected no bootstrap data, got users=%d students=%d packages=%d spaces=%d", len(store.users), len(store.students), len(store.packages), len(store.learningSpaces))
	}
	if len(store.settings) != 0 {
		t.Fatalf("expected no base dictionaries when explicitly skipped, got %#v", store.settings)
	}
}

func TestNewMemoryStoreCanSeedBootstrapAdminWithoutDemoData(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{
		SeedDemoData:           false,
		BootstrapAdminPhone:    "13800000001",
		BootstrapAdminPassword: "Starline@0621",
	})

	admin, err := store.LoginWithAdminPassword("13800000001", "Starline@0621")
	if err != nil {
		t.Fatalf("expected bootstrap admin login to succeed: %v", err)
	}
	if admin.UserID != "user-super" || !hasRole(admin.Roles, "super_admin") {
		t.Fatalf("unexpected bootstrap admin: %#v", admin)
	}
	if len(store.students) != 0 {
		t.Fatalf("expected no demo students, got %d", len(store.students))
	}
}
