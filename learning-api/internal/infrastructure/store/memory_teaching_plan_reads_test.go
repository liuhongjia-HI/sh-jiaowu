package store

import (
	"os"
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestTeachingPlanReadMySQLReloadAndFailedWrite(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated test database required")
	}
	s := NewMemoryStore()
	for i := range s.grants {
		s.grants[i].OpenedAt = "2026-10-03 09:00:00"
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	p, course, plan := chapterPlanFixture(t, s)
	for _, job := range s.previewJobs {
		if job.FileID == plan.FileID {
			if err := s.CompletePreviewJob(job.ID, learning.PreviewResult{PreviewPath: "/tmp/persistence-only-plan-preview.pdf"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	plan, _ = s.TeachingPlan(p, plan.ID)
	if err := s.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: plan.ReadVersion}); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	restored := NewMemoryStore()
	if err := restored.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	p, _ = restored.PrincipalByUserID(p.UserID)
	if containsString(restored.TeachingPlans(p).UnreadPlanIDs, plan.ID) {
		t.Fatal("read version lost on reload")
	}
	if _, err := restored.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{CourseID: course.ID, LessonID: course.Curriculum[1].ID}); err != nil {
		t.Fatal(err)
	}
	if !containsString(restored.TeachingPlans(p).UnreadPlanIDs, plan.ID) {
		t.Fatal("chapter reassignment did not create new reading version")
	}
	newPlan, _ := restored.TeachingPlan(p, plan.ID)
	before := append([]teacherMaterialRead(nil), restored.teacherMaterialReads...)
	if err := restored.db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := restored.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: newPlan.ReadVersion}); err == nil {
		t.Fatal("closed database accepted reading record")
	}
	if !reflect.DeepEqual(before, restored.teacherMaterialReads) || !containsString(restored.TeachingPlans(p).UnreadPlanIDs, plan.ID) {
		t.Fatal("failed write contaminated reading state")
	}
	reopened := NewMemoryStore()
	if err := reopened.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer reopened.db.Close()
	if !containsString(reopened.TeachingPlans(p).UnreadPlanIDs, plan.ID) {
		t.Fatal("failed write persisted new read version")
	}
	if err := reopened.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: newPlan.ReadVersion}); err != nil {
		t.Fatal(err)
	}
	if containsString(reopened.TeachingPlans(p).UnreadPlanIDs, plan.ID) {
		t.Fatal("retry did not mark current version read")
	}
}

func TestTeachingPlanUnreadVersionsAndReadOnlyPermission(t *testing.T) {
	s := NewMemoryStore()
	_, course, plan := chapterPlanFixture(t, s)
	p, _ := s.PrincipalByUserID("user-teacher")
	p.LearningSpaceIDs = []string{course.LearningSpaceID}
	p.CanUploadHandout = false
	p.TeacherLibrary = &learning.TeacherLibraryPolicy{SpaceIDs: []string{course.LearningSpaceID}, CanManageCourses: false}
	list := s.TeachingPlans(p)
	if len(list.Plans) != 1 || len(list.UnreadPlanIDs) != 1 {
		t.Fatalf("initial unread: %#v", list)
	}
	version := list.Plans[0].ReadVersion
	if err := s.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: version}); err == nil {
		t.Fatal("unavailable preview marked read")
	}
	asset := s.fileAssets[plan.FileID]
	asset.PreviewStatus = "可预览"
	s.fileAssets[plan.FileID] = asset
	// Reading a similarly named material must not consume the plan's unread state.
	s.recordResourceRead(p.UserID, plan.ID, version)
	if len(s.TeachingPlans(p).UnreadPlanIDs) != 1 {
		t.Fatal("material record collided with plan")
	}
	if err := s.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: version}); err != nil {
		t.Fatal(err)
	}
	if len(s.TeachingPlans(p).UnreadPlanIDs) != 0 {
		t.Fatal("read version still unread")
	}
	s.courses[findCourseIndex(s.courses, course.ID)].Curriculum[0].Name = "仅改章节标签"
	if len(s.TeachingPlans(p).UnreadPlanIDs) != 0 {
		t.Fatal("chapter rename replaced document version")
	}
	s.teachingPlans[0].Title = "教案新版本"
	if len(s.TeachingPlans(p).UnreadPlanIDs) != 1 {
		t.Fatal("updated document not unread")
	}
	if err := s.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: version}); err == nil {
		t.Fatal("stale view marked fresh version read")
	}
	fresh := s.TeachingPlans(p).Plans[0].ReadVersion
	if err := s.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: fresh}); err != nil {
		t.Fatal(err)
	}
	p.LearningSpaceIDs = nil
	p.TeacherLibrary.SpaceIDs = nil
	if len(s.TeachingPlans(p).UnreadPlanIDs) != 0 || len(s.TeachingPlans(p).Plans) != 0 {
		t.Fatal("revoked scope leaked plan")
	}
	if err := s.RecordTeachingPlanView(p, plan.ID, learning.TeachingPlanReadRequest{Version: fresh}); err == nil {
		t.Fatal("revoked scope wrote reading record")
	}
}
