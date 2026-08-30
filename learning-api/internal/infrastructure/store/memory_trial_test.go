package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestNewStudentCanPermanentlyReadTheFirstChapterOfEachGradeSubject(t *testing.T) {
	store := NewMemoryStore()
	store.grants = nil
	store.spaceAccess = nil

	englishCourseID := "course-g05-english-s1-q1"
	mathCourseID := "course-g05-math-s1-q1"
	for index := range store.courses {
		course := &store.courses[index]
		if course.ID == englishCourseID || course.ID == mathCourseID {
			course.Chapters = []string{"第一章", "第二章"}
		}
	}
	store.materials = append(store.materials,
		learning.Material{ID: "preview-english-first-material", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第一章讲义", Chapter: "第一章", Status: learning.StatusEnabled},
		learning.Material{ID: "preview-english-later-material", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第二章讲义", Chapter: "第二章", Status: learning.StatusEnabled},
		learning.Material{ID: "preview-math-first-material", CourseID: mathCourseID, LearningSpaceID: "space-g05-math-s1-q1", Title: "数学第一章讲义", Chapter: "第一章", Status: learning.StatusEnabled},
	)
	store.homework = append(store.homework,
		learning.Homework{ID: "preview-english-first-homework", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第一章习题", Chapter: "第一章", Status: string(learning.StatusEnabled)},
		learning.Homework{ID: "preview-english-later-homework", CourseID: englishCourseID, LearningSpaceID: "space-g05-english-s1-q1", Title: "英文第二章习题", Chapter: "第二章", Status: string(learning.StatusEnabled)},
		learning.Homework{ID: "preview-math-first-homework", CourseID: mathCourseID, LearningSpaceID: "space-g05-math-s1-q1", Title: "数学第一章习题", Chapter: "第一章", Status: string(learning.StatusEnabled)},
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
	if !containsCourseID(study.Courses, englishCourseID) || !containsCourseID(study.Courses, mathCourseID) {
		t.Fatalf("expected first course of each subject to be available, got %#v", study.Courses)
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
		t.Fatalf("formal package must override preview limit: %v", err)
	}
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
