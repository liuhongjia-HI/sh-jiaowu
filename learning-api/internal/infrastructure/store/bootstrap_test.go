package store

import (
	"testing"
	"time"
)

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
	if store.settings["academicCalendar"] == "" {
		t.Fatal("expected base settings to remain available")
	}
	if len(store.learningSpaces) == 0 {
		t.Fatal("expected base learning spaces to remain available")
	}
	foundEnglish := false
	for _, space := range store.learningSpaces {
		if space.Grade == "五年级" && space.Subject == "英文" && space.Semester == "S1" {
			foundEnglish = true
			break
		}
	}
	if !foundEnglish {
		t.Fatal("expected base learning spaces to include 五年级英文 S1")
	}
}

func TestBaseLearningSpacesFollowGradeSubjectMatrix(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	want := map[string]map[string][]string{
		"一年级":  {"数学": {"S"}, "英文": {"S"}, "语文": {"S"}, "科学": {"S"}},
		"二年级":  {"数学": {"S"}, "英文": {"S"}, "语文": {"S"}, "科学": {"S"}},
		"三年级":  {"数学": {"S"}, "英文": {"S"}, "语文": {"S"}, "科学": {"S"}},
		"四年级":  {"数学": {"S"}, "英文": {"S"}, "语文": {"S"}, "科学": {"S"}},
		"五年级":  {"数学": {"S", "S+", "H"}, "英文": {"S", "S+", "H"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+"}, "科学": {"S"}},
		"六年级":  {"数学": {"S", "S+", "H"}, "英文": {"S", "S+", "H"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+"}, "科学": {"S", "S+"}},
		"七年级":  {"数学": {"S", "S+", "H", "H+"}, "英文": {"S", "S+", "H", "H+"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+", "H"}, "科学": {"S", "S+", "H"}},
		"八年级":  {"数学": {"S", "S+", "H", "H+"}, "英文": {"S", "S+", "H", "H+"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+", "H"}, "科学": {"S", "S+", "H"}, "物理": {"S"}},
		"九年级":  {"数学": {"S", "S+", "H", "H+"}, "英文": {"S", "S+", "H", "H+"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+", "H"}, "科学": {"S", "S+", "H"}, "物理": {"S", "S+", "H"}, "化学": {"S", "S+"}},
		"十年级":  {"数学": {"S", "S+", "H", "H+"}, "英文": {"S", "S+", "H", "H+"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+", "H"}, "科学": {"S", "S+", "H"}, "物理": {"S", "S+", "H"}, "化学": {"S", "S+", "H"}},
		"十一年级": {"数学": {"S", "S+", "H", "H+"}, "英文": {"S", "S+", "H", "H+"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+", "H"}, "科学": {"S", "S+", "H"}, "物理": {"S", "S+", "H"}, "化学": {"S", "S+", "H"}},
		"十二年级": {"数学": {"S", "S+", "H", "H+"}, "英文": {"S", "S+", "H", "H+"}, "语文": {"S", "S+", "H"}, "地理": {"S", "S+", "H"}, "科学": {"S", "S+", "H"}, "物理": {"S", "S+", "H"}, "化学": {"S", "S+", "H"}},
	}
	if len(store.learningSpaces) != 668 {
		t.Fatalf("expected 668 learning spaces, got %d", len(store.learningSpaces))
	}

	counts := map[string]map[string]map[string]int{}
	for _, space := range store.learningSpaces {
		gradeSubjects, ok := want[space.Grade]
		if !ok {
			t.Fatalf("unexpected seeded grade %q", space.Grade)
		}
		levels, ok := gradeSubjects[space.Subject]
		if !ok {
			t.Fatalf("unexpected subject %q for %s", space.Subject, space.Grade)
		}
		if !containsString(levels, space.Level) {
			t.Fatalf("unexpected level %q for %s %s", space.Level, space.Grade, space.Subject)
		}
		if counts[space.Grade] == nil {
			counts[space.Grade] = map[string]map[string]int{}
		}
		if counts[space.Grade][space.Subject] == nil {
			counts[space.Grade][space.Subject] = map[string]int{}
		}
		counts[space.Grade][space.Subject][space.Level]++
	}

	for grade, subjects := range want {
		for subject, levels := range subjects {
			for _, level := range levels {
				if counts[grade][subject][level] != 4 {
					t.Fatalf("expected %s %s %s to have 4 semester/phase spaces, got %d", grade, subject, level, counts[grade][subject][level])
				}
			}
		}
	}
}

func TestAcademicYearForDateUsesJulyFirstBoundary(t *testing.T) {
	beforeBoundary := time.Date(2026, time.June, 30, 23, 59, 59, 0, time.Local)
	afterBoundary := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.Local)
	if got := academicYearForDate(beforeBoundary); got != "2025.2026学年" {
		t.Fatalf("expected school year before July to be 2025.2026学年, got %s", got)
	}
	if got := academicYearForDate(afterBoundary); got != "2026.2027学年" {
		t.Fatalf("expected school year from July to be 2026.2027学年, got %s", got)
	}
}

func TestBaseLearningSpacesUsesRequestedAcademicYear(t *testing.T) {
	spaces := baseLearningSpaces("2026.2027学年")
	if len(spaces) != 668 {
		t.Fatalf("expected 668 base learning spaces, got %d", len(spaces))
	}
	for _, space := range spaces {
		if space.AcademicYear != "2026.2027学年" {
			t.Fatalf("expected current academic year on %s, got %s", space.ID, space.AcademicYear)
		}
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
