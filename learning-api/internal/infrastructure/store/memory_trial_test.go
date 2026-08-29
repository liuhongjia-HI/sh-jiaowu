package store

import (
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestStartStudentTrialCreatesOneSevenDayGrantAndIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	store.grants = nil
	store.spaceAccess = nil
	packageID := "pkg-g05-english-s1-full"
	for index := range store.packages {
		if store.packages[index].ID == packageID {
			store.packages[index].TrialEnabled = true
		}
	}
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("load student principal: %v", err)
	}

	started, err := store.StartStudentTrial(principal, packageID)
	if err != nil {
		t.Fatalf("start trial: %v", err)
	}
	if started.Trial.State != "active" || started.Trial.RemainingDays != 7 {
		t.Fatalf("expected an active seven-day trial, got %#v", started.Trial)
	}
	if started.FirstCourseID == "" {
		t.Fatalf("expected a first course for the trial, got %#v", started)
	}
	home, err := store.StudentHome(principal)
	if err != nil {
		t.Fatalf("load home after starting trial: %v", err)
	}
	if home.Trial.ID != started.Trial.ID || home.Trial.State != "active" {
		t.Fatalf("home must expose the active trial, got %#v", home.Trial)
	}
	study, err := store.StudentStudy(principal)
	if err != nil {
		t.Fatalf("load study after starting trial: %v", err)
	}
	if study.Trial.ID != started.Trial.ID || study.Trial.State != "active" {
		t.Fatalf("study must expose the active trial, got %#v", study.Trial)
	}
	if len(store.grants) != 1 {
		t.Fatalf("expected one trial grant, got %#v", store.grants)
	}
	if want := time.Now().AddDate(0, 0, 6).Format("2006-01-02"); store.grants[0].EndsAt != want {
		t.Fatalf("expected trial to end on %s, got %#v", want, store.grants[0])
	}

	retried, err := store.StartStudentTrial(principal, packageID)
	if err != nil {
		t.Fatalf("retry start trial: %v", err)
	}
	if retried.Trial.ID != started.Trial.ID || len(store.grants) != 1 {
		t.Fatalf("retry must return the original trial without another grant: started=%#v retried=%#v grants=%#v", started, retried, store.grants)
	}
}

func TestTrialPackageRequiresCourseAndExerciseWhenEnabled(t *testing.T) {
	store := NewMemoryStore()
	pkg, ok := store.findPackage("pkg-g05-english-s1-full")
	if !ok {
		t.Fatal("missing seeded package")
	}
	_, err := store.UpdatePackage("测试操作人", pkg.ID, learning.PackageUpsertRequest{
		Name:             pkg.Name,
		AcademicYear:     pkg.AcademicYear,
		Grade:            pkg.Grade,
		Semester:         pkg.Semester,
		Subject:          pkg.Subject,
		Level:            pkg.Level,
		PhaseScope:       pkg.PhaseScope,
		PackageType:      pkg.PackageType,
		Summary:          pkg.Summary,
		TrialEnabled:     true,
		LearningSpaceIDs: store.learningSpaceIDsForPackage(pkg.ID),
		ContentTypeCodes: []string{"course", "handout"},
		Status:           learning.StatusEnabled,
	})
	if err == nil || !strings.Contains(err.Error(), "课程和练习") {
		t.Fatalf("expected incomplete trial package to be rejected, got %v", err)
	}
}

func TestStudentWithAnActivePackageCannotStartTrial(t *testing.T) {
	store := NewMemoryStore()
	packageID := "pkg-g05-english-s1-full"
	for index := range store.packages {
		if store.packages[index].ID == packageID {
			store.packages[index].TrialEnabled = true
		}
	}
	principal, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("load student principal: %v", err)
	}
	if _, err := store.StartStudentTrial(principal, packageID); err == nil || !strings.Contains(err.Error(), "已开通") {
		t.Fatalf("expected active package to block trial, got %v", err)
	}
}
