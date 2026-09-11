package store

import "testing"

func TestSubjectEnglishNameUnifiesChineseAndEnglishAliases(t *testing.T) {
	cases := map[string]string{
		"英文":                 "English",
		"英语":                 "English",
		"English":            "English",
		"数学":                 "Mathematics",
		"Math":               "Mathematics",
		"语文":                 "Chinese",
		"科学":                 "Science",
		"地理":                 "Geography",
		"历史":                 "History",
		"History":            "History",
		"物理":                 "Physics",
		"化学":                 "Chemistry",
		"综合科学":               "Integrated Science",
		"Integrated Science": "Integrated Science",
	}
	for input, want := range cases {
		if got := subjectEnglishName(input); got != want {
			t.Fatalf("subjectEnglishName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSubjectsMatchTreatsEnglishNamesAsAliases(t *testing.T) {
	pairs := [][2]string{
		{"英文", "英语"},
		{"英文", "English"},
		{"数学", "Mathematics"},
		{"数学", "Math"},
		{"历史", "History"},
		{"地理", "Geography"},
		{"语文", "Chinese"},
	}
	for _, pair := range pairs {
		if !subjectsMatch(pair[0], pair[1]) {
			t.Fatalf("expected %q to match %q", pair[0], pair[1])
		}
	}
	if subjectsMatch("数学", "英文") {
		t.Fatal("math must not match english")
	}
}

func TestDefaultGradeSubjectCatalogUsesEnglishDisplayNames(t *testing.T) {
	for _, item := range defaultGradeSubjectCatalog() {
		want := subjectEnglishName(item.Subject)
		if item.DisplayName != want {
			t.Fatalf("%s/%s displayName = %q, want %q", item.Grade, item.Subject, item.DisplayName, want)
		}
	}
}

func TestGradeSubjectCatalogTranslatesStoredChineseDisplayNames(t *testing.T) {
	store := NewMemoryStore()
	store.settings[gradeSubjectCatalogSetting] = `[{"id":"g6-history","grade":"六年级","subject":"历史","displayName":"History","status":"启用","sortOrder":3},{"id":"g6-math","grade":"六年级","subject":"数学","displayName":"数学","status":"启用","sortOrder":2},{"id":"g6-english","grade":"六年级","subject":"英文","displayName":"英文","status":"启用","sortOrder":1}]`
	got := map[string]string{}
	for _, item := range store.GradeSubjects() {
		if item.Grade == "六年级" {
			got[item.Subject] = item.DisplayName
		}
	}
	if got["历史"] != "History" || got["数学"] != "Mathematics" || got["英文"] != "English" {
		t.Fatalf("expected unified English display names, got %#v", got)
	}
}

func TestSubjectShortLabelMatchesEnglishAliases(t *testing.T) {
	store := NewMemoryStore()
	if got := store.subjectShortLabel("English"); got != "Eng" {
		t.Fatalf("English should map to Eng, got %q", got)
	}
	if got := store.subjectShortLabel("Mathematics"); got != "Math" {
		t.Fatalf("Mathematics should map to Math, got %q", got)
	}
}
