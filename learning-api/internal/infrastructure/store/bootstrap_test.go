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
}

func TestNewMemoryStoreCanSkipAllBootstrapData(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false, SkipBaseData: true})

	if len(store.users) != 0 || len(store.students) != 0 || len(store.packages) != 0 {
		t.Fatalf("expected no demo data, got users=%d students=%d packages=%d", len(store.users), len(store.students), len(store.packages))
	}
	if len(store.settings) != 0 {
		t.Fatalf("expected no base dictionaries when explicitly skipped, got %#v", store.settings)
	}
}
