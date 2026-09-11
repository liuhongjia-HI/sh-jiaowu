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

func TestStudentStudyReturnsEnglishCardCopy(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	study, err := store.StudentStudy(student)
	if err != nil {
		t.Fatal(err)
	}
	if len(study.Subjects) == 0 {
		t.Fatal("expected subject catalog")
	}
	for _, card := range study.Subjects {
		if card.Grade != "Grade 5" {
			t.Fatalf("subject grade = %q, want Grade 5", card.Grade)
		}
		if card.DisplayName != subjectEnglishName(card.Subject) {
			t.Fatalf("displayName = %q, want English name for %q", card.DisplayName, card.Subject)
		}
		switch card.AccessState {
		case "preview":
			if card.AccessLabel != "Preview" {
				t.Fatalf("preview label = %q", card.AccessLabel)
			}
		case "locked":
			if card.AccessLabel != "Unavailable" {
				t.Fatalf("locked label = %q", card.AccessLabel)
			}
		case "full":
			if card.AccessLabel != "Ready" {
				t.Fatalf("full label = %q", card.AccessLabel)
			}
		case "pending":
			if card.AccessLabel != "Preparing" {
				t.Fatalf("pending label = %q", card.AccessLabel)
			}
		}
	}
	if len(study.Courses) == 0 {
		t.Fatal("expected opened courses")
	}
	for _, course := range study.Courses {
		if course.Grade != "Grade 5" {
			t.Fatalf("course grade = %q, want Grade 5", course.Grade)
		}
		if course.DisplayName == "" || course.DisplayName == course.Name {
			t.Fatalf("course displayName should use catalog English name, got %q / %q", course.DisplayName, course.Name)
		}
	}
}
