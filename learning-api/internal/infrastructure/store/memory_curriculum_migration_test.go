package store

import (
	"database/sql"
	"os"
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
)

func migrationFixture(t *testing.T) (*MemoryStore, learning.Principal, learning.Course, learning.CourseUpsertRequest) {
	t.Helper()
	s := NewMemoryStore()
	p, course, plan := chapterPlanFixture(t, s)
	source, target := course.Curriculum[0].ID, course.Curriculum[1].ID
	s.materials = append(s.materials, learning.Material{ID: "migration-material", Title: "生态系统讲义", CourseID: course.ID, LearningSpaceID: course.LearningSpaceID, LessonID: source, Status: learning.StatusDraft, CreatedAt: "2026-10-27 09:00:00"})
	s.homework = append(s.homework, learning.Homework{ID: "migration-homework", Title: "生态系统练习", CourseID: course.ID, Course: course.Name, LearningSpaceID: course.LearningSpaceID, LessonID: source, Status: string(learning.StatusDisabled), PublishStatus: string(learning.StatusDisabled), QuestionIDs: []string{"question-preserved"}, DeadlineAt: "2026-12-01 18:00:00"})
	s.submissions["migration-submission"] = learning.Submission{ID: "migration-submission", HomeworkID: "migration-homework", StudentID: "student-original", CreatedAt: "2026-10-27 09:00:00", Score: 88, FinalScore: 90, TeacherComment: "分析完整", Answers: []learning.SubmissionAnswer{{QuestionID: "question-preserved"}}}
	s.reviews = append(s.reviews, learning.Review{ID: "migration-review", HomeworkID: "migration-homework", SubmissionID: "migration-submission", SystemScore: 88, TeacherComment: "分析完整"})
	req := learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: course.LearningSpaceID, Status: course.Status, Curriculum: course.Curriculum[1:], ReferenceMoves: []learning.CurriculumReferenceMove{
		{ID: plan.ID, Kind: "教案", CourseID: course.ID, FromLessonID: source, ToLessonID: target},
		{ID: "migration-material", Kind: "讲义", CourseID: course.ID, FromLessonID: source, ToLessonID: target},
		{ID: "migration-homework", Kind: "练习", CourseID: course.ID, FromLessonID: source, ToLessonID: target},
	}}
	return s, p, course, req
}

func TestCurriculumMigrationPreservesSubmissionsAndReviews(t *testing.T) {
	s, p, course, req := migrationFixture(t)
	submissions, reviews := cloneMap(s.submissions), append([]learning.Review(nil), s.reviews...)
	homework := cloneHomework(s.homework[len(s.homework)-1])
	refs, err := s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{IncludeAll: true})
	if err != nil || len(refs) != 3 {
		t.Fatalf("all references: %#v %v", refs, err)
	}
	for _, ref := range refs {
		if ref.Path == "" || ref.Status == "" {
			t.Fatalf("missing reference detail: %#v", ref)
		}
		if ref.Kind == "练习" && (ref.Status != "停用" || ref.SubmissionCount != 1) {
			t.Fatalf("hidden homework missing: %#v", ref)
		}
	}
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.submissions, submissions) || !reflect.DeepEqual(s.reviews, reviews) {
		t.Fatal("student answers or reviews changed")
	}
	updated := s.homework[len(s.homework)-1]
	homework.LessonID = req.Curriculum[0].ID
	homework.Curriculum = updated.Curriculum
	if !reflect.DeepEqual(homework, updated) {
		t.Fatalf("homework recreated or state changed: %#v", updated)
	}
	refs, err = s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{NodeIDs: []string{course.Curriculum[0].ID}})
	if err != nil || len(refs) != 0 {
		t.Fatalf("old directory still referenced: %#v %v", refs, err)
	}
	if !strings.Contains(s.logs[0].Detail, "资料迁移") {
		t.Fatal("missing migration audit")
	}
}

func TestCurriculumMigrationRejectsInvalidAndStaleMovesAtomically(t *testing.T) {
	for _, name := range []string{"missing-target", "removed-target", "nonleaf-target", "foreign-course", "stale-binding", "duplicate", "late-reference", "no-question-permission", "scope-change"} {
		t.Run(name, func(t *testing.T) {
			s, p, course, req := migrationFixture(t)
			switch name {
			case "missing-target":
				req.ReferenceMoves[2].ToLessonID = "missing"
			case "removed-target":
				req.ReferenceMoves[2].ToLessonID = course.Curriculum[0].ID
			case "nonleaf-target":
				req.Curriculum = append(req.Curriculum, learning.CurriculumNode{ID: "new-child", ParentID: req.Curriculum[0].ID, Type: learning.CurriculumChapter, Name: "子课节", SortOrder: 1})
			case "foreign-course":
				req.ReferenceMoves[2].CourseID = "other-course"
			case "stale-binding":
				s.homework[len(s.homework)-1].LessonID = req.Curriculum[0].ID
			case "duplicate":
				req.ReferenceMoves = append(req.ReferenceMoves, req.ReferenceMoves[2])
			case "late-reference":
				s.materials = append(s.materials, learning.Material{ID: "late-reference", Title: "刚上传的讲义", CourseID: course.ID, LessonID: course.Curriculum[0].ID})
			case "no-question-permission":
				p.Roles = []learning.Role{learning.RoleTeacher}
				p.LearningSpaceIDs = []string{course.LearningSpaceID}
				p.CanUploadHandout = true
				p.CanUploadQuestion = false
			case "scope-change":
				req.LearningSpaceID = "space-g05-english-s2-q1"
			}
			beforeMaterials, beforeHomework, beforePlans, beforeCourses, beforeLogs := mustJSON(s.materials), mustJSON(s.homework), mustJSON(s.teachingPlans), mustJSON(s.courses), mustJSON(s.logs)
			if _, err := s.UpdateCourse("管理员", p, course.ID, req); err == nil {
				t.Fatal("invalid move accepted")
			}
			if beforeMaterials != mustJSON(s.materials) || beforeHomework != mustJSON(s.homework) || beforePlans != mustJSON(s.teachingPlans) || beforeCourses != mustJSON(s.courses) || beforeLogs != mustJSON(s.logs) {
				t.Fatal("partial migration published after failed save")
			}
		})
	}
}

func TestCurriculumMigrationDatabaseFailureRollsBack(t *testing.T) {
	s, p, course, req := migrationFixture(t)
	db, err := sql.Open(mutationTestDriverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mutationDriverState.reset(true)
	s.db = db
	before := mustJSON(s.homework)
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err == nil || !strings.Contains(err.Error(), "injected database write failure") {
		t.Fatalf("expected DB failure: %v", err)
	}
	if mustJSON(s.homework) != before {
		t.Fatal("migration leaked after database rollback")
	}
	if mutationDriverState.rollbacks != 1 || mutationDriverState.commits != 0 {
		t.Fatal("transaction did not roll back")
	}
}

func TestCurriculumMigrationRepairsInvalidBinding(t *testing.T) {
	s, p, course, req := migrationFixture(t)
	s.homework[len(s.homework)-1].LessonID = "historical-missing-node"
	req.ReferenceMoves[2].FromLessonID = "historical-missing-node"
	refs, err := s.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{IncludeAll: true})
	if err != nil {
		t.Fatal(err)
	}
	if !refs[len(refs)-1].InvalidBinding || !strings.Contains(refs[len(refs)-1].Path, "已失效") {
		t.Fatalf("missing invalid association: %#v", refs)
	}
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err != nil {
		t.Fatal(err)
	}
}

func TestCurriculumMigrationFamilyPreservesClassTypeOwnership(t *testing.T) {
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	nodes := []learning.CurriculumNode{{ID: "family-from", Type: learning.CurriculumUnit, Name: "生态系统", SortOrder: 1}, {ID: "family-to", Type: learning.CurriculumUnit, Name: "环境变化", SortOrder: 2}}
	family, err := s.CreateCourseFamily("管理员", p, learning.CourseFamilyCreateRequest{Name: "科学课程", LearningSpaceIDs: []string{"space-g05-english-s1-q1-splus", "space-g05-english-s1-q1-h"}, Curriculum: nodes})
	if err != nil {
		t.Fatal(err)
	}
	moves := []learning.CurriculumReferenceMove{}
	for i, course := range family.Courses {
		id := []string{"family-hw-splus", "family-hw-h"}[i]
		s.homework = append(s.homework, learning.Homework{ID: id, CourseID: course.ID, Course: course.Name, LearningSpaceID: course.LearningSpaceID, LessonID: "family-from", Status: "草稿"})
		moves = append(moves, learning.CurriculumReferenceMove{ID: id, Kind: "练习", CourseID: course.ID, FromLessonID: "family-from", ToLessonID: "family-to"})
	}
	req := learning.CourseFamilyUpdateRequest{Name: family.Name, Curriculum: nodes[1:], ReferenceMoves: moves[:1]}
	before := mustJSON(s.homework)
	if _, err = s.UpdateCourseFamily("管理员", p, family.ID, req); err == nil {
		t.Fatal("other class type reference ignored")
	}
	if before != mustJSON(s.homework) {
		t.Fatal("family partially moved")
	}
	req.ReferenceMoves = moves
	if _, err = s.UpdateCourseFamily("管理员", p, family.ID, req); err != nil {
		t.Fatal(err)
	}
	for _, move := range moves {
		for _, item := range s.homework {
			if item.ID == move.ID && (item.CourseID != move.CourseID || item.LessonID != "family-to" || item.Status != "草稿") {
				t.Fatalf("cross-class type migration: %#v", item)
			}
		}
	}
}

func TestCurriculumMigrationMySQLRestartPreservesHistory(t *testing.T) {
	dsn := os.Getenv("STARLINE_CURRICULUM_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated curriculum MySQL database required")
	}
	s, p, course, req := migrationFixture(t)
	for i := range s.grants {
		if s.grants[i].OpenedAt == "" {
			s.grants[i].OpenedAt = "2026-10-27 09:00:00"
		}
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	beforeSubmissions, beforeReviews := mustJSON(s.submissions), mustJSON(s.reviews)
	if _, err := s.UpdateCourse("管理员", p, course.ID, req); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	restored := NewMemoryStore()
	if err := restored.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer restored.db.Close()
	if beforeSubmissions != mustJSON(restored.submissions) || beforeReviews != mustJSON(restored.reviews) {
		t.Fatal("persisted student answers or reviews changed")
	}
	saved, ok := restored.findCourse(course.ID)
	if !ok || len(saved.Curriculum) != 1 || saved.Curriculum[0].ID != req.Curriculum[0].ID {
		t.Fatalf("directory not persisted: %#v", saved)
	}
	refs, err := restored.CurriculumReferences(p, course.ID, learning.CurriculumReferencesRequest{IncludeAll: true})
	if err != nil || len(refs) != 3 {
		t.Fatalf("restored references: %#v %v", refs, err)
	}
	for _, ref := range refs {
		if ref.LessonID != req.Curriculum[0].ID || ref.InvalidBinding {
			t.Fatalf("migration not persisted: %#v", ref)
		}
	}
	for _, ref := range refs {
		if ref.Kind == "练习" && (ref.SubmissionCount != 1 || ref.Status != "停用") {
			t.Fatalf("homework history/status lost: %#v", ref)
		}
	}
}
