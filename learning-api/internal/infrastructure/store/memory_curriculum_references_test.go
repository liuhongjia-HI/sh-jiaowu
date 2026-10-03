package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestCurriculumReferencesPreviewAndSaveProtection(t *testing.T) {
	s := NewMemoryStore()
	p, course, plan := chapterPlanFixture(t, s)
	node := course.Curriculum[0].ID
	s.materials = append(s.materials, learning.Material{ID: "ref-material", Title: "草稿讲义", CourseID: course.ID, LessonID: node})
	s.homework = append(s.homework, learning.Homework{ID: "ref-homework", Title: "草稿练习", CourseID: course.ID, LessonID: node})
	refs, err := s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{NodeIDs: []string{node, node}})
	if err != nil || len(refs) != 3 {
		t.Fatalf("references: %#v, %v", refs, err)
	}
	if refs[0].ID != plan.ID || refs[0].Kind != "教案" {
		t.Fatalf("missing plan: %#v", refs)
	}
	stored, _ := s.findCourse(course.ID)
	if len(stored.Curriculum) != 2 {
		t.Fatal("preview mutated curriculum")
	}
	student, _ := s.PrincipalByUserID("user-student")
	if _, err := s.CurriculumReferences(student, course.ID, learning.CurriculumReferencesRequest{NodeIDs: []string{node}}); err == nil {
		t.Fatal("student read internal bindings")
	}
	if _, err := s.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{CourseID: course.ID, LessonID: course.Curriculum[1].ID}); err != nil {
		t.Fatal(err)
	}
	request := learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: course.LearningSpaceID, Status: course.Status, Curriculum: course.Curriculum[1:]}
	if _, err := s.UpdateCourse("管理员", p, course.ID, request); err == nil {
		t.Fatal("bound materials were orphaned")
	}
	// A clear preview is not authorization to bypass new bindings at save time.
	refs, err = s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{NodeIDs: []string{course.Curriculum[1].ID}})
	if err != nil || len(refs) != 1 {
		t.Fatalf("reassignment not reflected: %#v %v", refs, err)
	}
	s.materials = s.materials[:len(s.materials)-1]
	s.homework = s.homework[:len(s.homework)-1]
	refs, err = s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{NodeIDs: []string{node}})
	if err != nil || len(refs) != 0 {
		t.Fatalf("empty preview: %#v %v", refs, err)
	}
	s.materials = append(s.materials, learning.Material{ID: "late-binding", Title: "预检后上传", CourseID: course.ID, LessonID: node})
	if _, err := s.UpdateCourse("管理员", p, course.ID, request); err == nil {
		t.Fatal("save trusted stale preview")
	}
}

func TestCurriculumReferencesIncludesOtherFamilyClassTypes(t *testing.T) {
	s := NewMemoryStore()
	p, course, plan := chapterPlanFixture(t, s)
	index := findCourseIndex(s.courses, course.ID)
	s.courses[index].FamilyID = "reference-family"
	other := course
	other.ID = "ref-other-course"
	other.FamilyID = "reference-family"
	other.Name = "另一班型"
	s.courses = append(s.courses, other)
	plan.ID = "other-plan"
	plan.CourseID = other.ID
	s.teachingPlans = append(s.teachingPlans, plan)
	refs, err := s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{NodeIDs: []string{course.Curriculum[0].ID}})
	if err != nil || len(refs) != 2 || refs[1].CourseName != other.Name {
		t.Fatalf("incomplete family impact: %#v %v", refs, err)
	}
}

func TestCurriculumScopeImpactMatchesTeachingPlanConstraints(t *testing.T) {
	s := NewMemoryStore()
	p, course, _ := chapterPlanFixture(t, s)
	s.materials = append(s.materials, learning.Material{ID: "scope-material", Title: "关联讲义", CourseID: course.ID})
	old, _ := s.findLearningSpace(course.LearningSpaceID)
	compatible := old
	compatible.ID = "scope-compatible"
	s.learningSpaces = append(s.learningSpaces, compatible)
	incompatible := old
	incompatible.ID = "scope-other-phase"
	incompatible.Phase = "other-phase"
	s.learningSpaces = append(s.learningSpaces, incompatible)
	for _, tc := range []struct {
		target  string
		blocked bool
	}{{compatible.ID, false}, {incompatible.ID, true}} {
		refs, err := s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{TargetLearningSpaceID: tc.target})
		if err != nil || len(refs) != 2 || refs[0].Blocking != tc.blocked || refs[1].Blocking {
			t.Fatalf("impact: %#v %v", refs, err)
		}
		request := learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: tc.target, Status: course.Status, Curriculum: course.Curriculum}
		_, err = s.UpdateCourse("管理员", p, course.ID, request)
		if (err != nil) != tc.blocked {
			t.Fatalf("preview/save disagreement: %v", err)
		}
	}
	if _, err := s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{TargetLearningSpaceID: "missing"}); err == nil {
		t.Fatal("unknown target accepted")
	}
}
