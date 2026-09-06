package store

import (
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestNewStudentCanReadFirstChapterLessonOfEachSubjectWithoutPackage(t *testing.T) {
	store := NewMemoryStore()
	store.grants = nil
	store.spaceAccess = nil

	englishCourseID := "course-g05-english-s1-q1"
	mathCourseID := "course-g05-math-s1-q1"
	for index := range store.courses {
		course := &store.courses[index]
		if course.ID == englishCourseID || course.ID == mathCourseID {
			course.Curriculum = append(course.Curriculum, learning.CurriculumNode{ID: course.ID + "-lesson-2", ParentID: course.ID + "-chapter-1", Type: learning.CurriculumLesson, Name: "拓展课节", SortOrder: 2})
			course.LessonCount = 2
		}
	}
	store.materials = append(store.materials,
		learning.Material{ID: "preview-english-first-material", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第一课节讲义", LessonID: englishCourseID + "-lesson-1", Status: learning.StatusEnabled},
		learning.Material{ID: "preview-english-later-material", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第二课节讲义", LessonID: englishCourseID + "-lesson-2", Status: learning.StatusEnabled},
		learning.Material{ID: "preview-math-first-material", CourseID: mathCourseID, LearningSpaceID: "space-g05-math-s1-q1", Title: "数学第一课节讲义", LessonID: mathCourseID + "-lesson-1", Status: learning.StatusEnabled},
	)
	store.homework = append(store.homework,
		learning.Homework{ID: "preview-english-first-homework", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第一课节习题", LessonID: englishCourseID + "-lesson-1", Status: string(learning.StatusEnabled)},
		learning.Homework{ID: "preview-english-later-homework", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第二课节习题", LessonID: englishCourseID + "-lesson-2", Status: string(learning.StatusEnabled)},
		learning.Homework{ID: "preview-math-first-homework", CourseID: mathCourseID, LearningSpaceID: "space-g05-math-s1-q1", Title: "数学第一课节习题", LessonID: mathCourseID + "-lesson-1", Status: string(learning.StatusEnabled)},
	)

	student, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "permanent-preview-openid", Phone: "13600001234", StudentName: "体验学生", SchoolName: "星河小学", Grade: "五年级",
	})
	if err != nil {
		t.Fatalf("new student login: %v", err)
	}
	for _, grant := range store.grants {
		if grant.StudentID == student.StudentID {
			t.Fatalf("permanent preview must not create a time-limited grant: %#v", grant)
		}
	}

	study, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("load study: %v", err)
	}
	if !containsMaterialID(study.Materials, "preview-english-first-material") || !containsMaterialID(study.Materials, "preview-math-first-material") {
		t.Fatalf("expected first-chapter handouts, got %#v", study.Materials)
	}
	if containsMaterialID(study.Materials, "preview-english-later-material") {
		t.Fatalf("later-chapter handout must stay locked, got %#v", study.Materials)
	}
	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("load home: %v", err)
	}
	if !containsHomeworkID(home.PendingHomework, "preview-english-first-homework") || !containsHomeworkID(home.PendingHomework, "preview-math-first-homework") {
		t.Fatalf("expected first-chapter exercises, got %#v", home.PendingHomework)
	}
	if containsHomeworkID(home.PendingHomework, "preview-english-later-homework") {
		t.Fatalf("later-chapter exercise must stay locked, got %#v", home.PendingHomework)
	}
	if _, err := store.StudentMaterial(student, "preview-english-later-material"); err == nil {
		t.Fatal("later-chapter handout must not be retrievable by ID")
	}
	if _, err := store.StudentHomework(student, "preview-english-later-homework"); err == nil {
		t.Fatal("later-chapter exercise must not be retrievable by ID")
	}

	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{StudentID: student.StudentID, PackageID: "pkg-g05-english-s1-full"}); err != nil {
		t.Fatalf("open formal package: %v", err)
	}
	if _, err := store.StudentMaterial(student, "preview-english-later-material"); err != nil {
		t.Fatalf("formal package must unlock later content: %v", err)
	}
}

func TestStudentStudyListsConfiguredGradeSubjectsAndMarksPreview(t *testing.T) {
	store := NewMemoryStore()
	store.grants = nil
	store.spaceAccess = nil
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}

	study, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("load study: %v", err)
	}
	if len(study.Subjects) != 5 {
		t.Fatalf("five-grade subject catalog = %d, want 5", len(study.Subjects))
	}
	var english, geography, science learning.StudentSubjectCard
	for _, item := range study.Subjects {
		switch item.Subject {
		case "英文":
			english = item
		case "地理":
			geography = item
		case "科学":
			science = item
		}
	}
	if english.ID == "" || english.AccessState != "preview" || english.AccessLabel != "首节可体验" || !english.CanOpen {
		t.Fatalf("english preview card = %#v", english)
	}
	for _, card := range []learning.StudentSubjectCard{geography, science} {
		if card.ID == "" || card.AccessState != "preview" || card.AccessLabel != "首节可体验" || !card.CanOpen || card.MaterialNum == 0 {
			t.Fatalf("subject with first-chapter handout must be previewable: %#v", card)
		}
	}
}

func TestPreviewIgnoresStaleConfiguredCourseID(t *testing.T) {
	store := NewMemoryStore()
	store.settings[gradeSubjectCatalogSetting] = `[{"id":"g5-geography","grade":"五年级","subject":"地理","displayName":"地理","status":"启用","previewCourseId":"deleted-course"}]`
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	study, err := store.StudentStudy(student)
	if err != nil {
		t.Fatal(err)
	}
	if len(study.Subjects) != 1 || study.Subjects[0].AccessState != "preview" || !study.Subjects[0].CanOpen {
		t.Fatalf("stale preview course id must fall back to current course: %#v", study.Subjects)
	}
}

func TestFutureCourseGrantStillAllowsFirstLessonPreview(t *testing.T) {
	store := NewMemoryStore()
	studentID := "stu-001"
	futureStart := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	futureEnd := time.Now().AddDate(0, 1, 1).Format("2006-01-02")
	store.grants = append(store.grants, packageGrant{
		ID: "future-geography-grant", StudentID: studentID,
		PackageID: packageID(4, "地理", 0, "full"), StartsAt: futureStart, EndsAt: futureEnd, Status: "active",
	})
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	study, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("load study: %v", err)
	}
	for _, card := range study.Subjects {
		if card.Subject == "地理" && card.AccessState == "preview" && card.CanOpen {
			return
		}
	}
	t.Fatal("future course grant must not block first-lesson preview")
}

func TestSevenDayTrialIsNoLongerAvailable(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("load student: %v", err)
	}
	_, err = store.StartStudentTrial(principal, "pkg-g05-english-s1-full")
	if err == nil || !strings.Contains(err.Error(), "体验期已取消") {
		t.Fatalf("seven-day trial must be unavailable, got %v", err)
	}
}

func TestTrialPermissionUsesTheFirstLessonBySortOrder(t *testing.T) {
	store := NewMemoryStore()
	studentID := "stu-001"
	packageID := "trial-package"
	courseID := "course-g05-english-s1-q1"
	for index := range store.courses {
		if store.courses[index].ID == courseID {
			store.courses[index].Curriculum = []learning.CurriculumNode{
				{ID: courseID + "-lesson-2", Type: learning.CurriculumLesson, Name: "第二节", SortOrder: 2},
				{ID: courseID + "-lesson-1", Type: learning.CurriculumLesson, Name: "第一节", SortOrder: 1},
			}
		}
	}
	store.trials = append(store.trials, studentTrialRecord{
		StudentID: studentID, AcademicYear: store.configuredAcademicYear(), PackageID: packageID,
		StartsAt: "2026-01-01", EndsAt: "2027-01-01", Status: "active",
	})
	lessonID, limited := store.trialFirstLessonForGrant(packageGrant{
		StudentID: studentID, PackageID: packageID, StartsAt: "2026-01-01", EndsAt: "2027-01-01",
	}, courseID)
	if !limited || lessonID != courseID+"-lesson-1" {
		t.Fatalf("expected trial to use first lesson, got lesson=%q limited=%v", lessonID, limited)
	}
}

func TestPreviewLessonUsesFirstUnitFirstLesson(t *testing.T) {
	store := NewMemoryStore()
	course := learning.Course{ID: "course-preview-order", Curriculum: []learning.CurriculumNode{
		{ID: "unit-later", Type: learning.CurriculumUnit, Name: "第二单元", SortOrder: 2},
		{ID: "lesson-later", ParentID: "unit-later", Type: learning.CurriculumLesson, Name: "第二单元第一节", SortOrder: 1},
		{ID: "unit-first", Type: learning.CurriculumUnit, Name: "第一单元", SortOrder: 1},
		{ID: "chapter-first", ParentID: "unit-first", Type: learning.CurriculumChapter, Name: "第一章", SortOrder: 1},
		{ID: "lesson-first", ParentID: "chapter-first", Type: learning.CurriculumLesson, Name: "第一单元第一节", SortOrder: 9},
	}}
	store.materials = append(store.materials, learning.Material{ID: "preview-first-material", CourseID: course.ID, LessonID: "lesson-first", Status: learning.StatusEnabled})
	store.homework = append(store.homework, learning.Homework{ID: "preview-first-homework", CourseID: course.ID, LessonID: "lesson-first", Status: string(learning.StatusEnabled)})
	lessonID, ok := store.previewLessonForCourse(course)
	if !ok || lessonID != "lesson-first" {
		t.Fatalf("preview lesson = %q, %v; want first unit first lesson", lessonID, ok)
	}
}

func TestPreviewLessonUsesFirstChapterFirstLessonWhenOnlyHandoutExists(t *testing.T) {
	store := NewMemoryStore()
	course := learning.Course{ID: "course-preview-chapter", Curriculum: []learning.CurriculumNode{
		{ID: "unit-first", Type: learning.CurriculumUnit, Name: "第一单元", SortOrder: 1},
		{ID: "chapter-first", ParentID: "unit-first", Type: learning.CurriculumChapter, Name: "第一章", SortOrder: 1},
		{ID: "lesson-first", ParentID: "chapter-first", Type: learning.CurriculumLesson, Name: "第一节", SortOrder: 1},
		{ID: "chapter-later", ParentID: "unit-first", Type: learning.CurriculumChapter, Name: "第二章", SortOrder: 2},
		{ID: "lesson-later", ParentID: "chapter-later", Type: learning.CurriculumLesson, Name: "第一节", SortOrder: 1},
	}}
	store.materials = append(store.materials, learning.Material{ID: "chapter-first-material", CourseID: course.ID, LessonID: "lesson-first", Status: learning.StatusEnabled})
	lessonID, ok := store.previewLessonForCourse(course)
	if !ok || lessonID != "lesson-first" {
		t.Fatalf("preview lesson = %q, %v; want first chapter first lesson with handout only", lessonID, ok)
	}
}

func TestPreviewLessonSupportsLegacyCourseWithoutCurriculumNodes(t *testing.T) {
	store := NewMemoryStore()
	course := learning.Course{ID: "course-legacy-preview"}
	store.materials = append(store.materials, learning.Material{
		ID: "legacy-first-material", CourseID: course.ID, LessonID: "", Chapter: "基础巩固", Status: learning.StatusEnabled,
	})
	lessonID, ok := store.previewLessonForCourse(course)
	if !ok || lessonID != "" {
		t.Fatalf("legacy preview lesson = %q, %v; want empty lesson id accepted", lessonID, ok)
	}
}

func TestPreviewLessonFallsBackToLegacyBlankLessonIDWhenCurriculumExists(t *testing.T) {
	store := NewMemoryStore()
	course := learning.Course{ID: "course-legacy-nodes-preview", Curriculum: []learning.CurriculumNode{
		{ID: "chapter-first", Type: learning.CurriculumChapter, Name: "第一章", SortOrder: 1},
		{ID: "lesson-first", ParentID: "chapter-first", Type: learning.CurriculumLesson, Name: "第一节", SortOrder: 1},
	}}
	store.materials = append(store.materials, learning.Material{
		ID: "legacy-nodes-material", CourseID: course.ID, LessonID: "", Chapter: "基础巩固", Status: learning.StatusEnabled,
	})
	lessonID, ok := store.previewLessonForCourse(course)
	if !ok || lessonID != "" {
		t.Fatalf("preview lesson = %q, %v; want blank lesson id fallback", lessonID, ok)
	}
}

func TestPreviewLessonMatchesContentByLearningSpaceWhenCourseIDIsBlank(t *testing.T) {
	store := NewMemoryStore()
	course := learning.Course{ID: "course-space-preview", LearningSpaceID: "space-space-preview"}
	store.courses = append(store.courses, course)
	store.materials = append(store.materials, learning.Material{
		ID: "space-preview-material", CourseID: "", LearningSpaceID: course.LearningSpaceID, LessonID: "", Status: learning.StatusEnabled,
	})
	lessonID, ok := store.previewLessonForCourse(course)
	if !ok || lessonID != "" {
		t.Fatalf("preview lesson = %q, %v; want learning-space fallback", lessonID, ok)
	}
}

func TestUnopenedDetailShowsFirstLessonAndLocksLaterLessons(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatal(err)
	}
	store.grants = nil
	store.spaceAccess = nil
	for index := range store.courses {
		if store.courses[index].ID == "course-g05-english-s1-q1" {
			store.courses[index].Curriculum = append(store.courses[index].Curriculum, learning.CurriculumNode{ID: "preview-locked-lesson", ParentID: "course-g05-english-s1-q1-chapter-1", Type: learning.CurriculumLesson, Name: "第二节", SortOrder: 2})
		}
	}
	detail, err := store.StudentCourseDetail(student, "course-g05-english-s1-q1")
	if err != nil {
		t.Fatalf("preview detail: %v", err)
	}
	var first, locked learning.Station
	for _, station := range detail.Stations {
		if first.Title == "" && station.MaterialID != "" {
			first = station
		}
		if station.Title == "第二节" {
			locked = station
			break
		}
	}
	if first.Status == "未开通" || first.MaterialID == "" {
		t.Fatalf("first chapter first lesson should be previewable: %#v", first)
	}
	firstCount := 0
	for _, station := range detail.Stations {
		if station.MaterialID == "mat-g05-english-s1-q1" {
			firstCount++
		}
	}
	if firstCount != 1 {
		t.Fatalf("first chapter first lesson should appear once in preview detail, got %d", firstCount)
	}
	if locked.Status != "未开通" || locked.MaterialID != "" || locked.HomeworkID != "" {
		t.Fatalf("locked station must not expose content IDs: %#v", locked)
	}
}

func containsCourseID(values []learning.StudentCourseCard, id string) bool {
	for _, value := range values {
		if value.ID == id {
			return true
		}
	}
	return false
}

func containsMaterialID(values []learning.Material, id string) bool {
	for _, value := range values {
		if value.ID == id {
			return true
		}
	}
	return false
}

func containsHomeworkID(values []learning.Homework, id string) bool {
	for _, value := range values {
		if value.ID == id {
			return true
		}
	}
	return false
}
