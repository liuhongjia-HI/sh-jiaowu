package store

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestDirectorySyncMySQLFailureRollbackAndRetry(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated directory retry database required")
	}
	if !strings.Contains(dsn, "/starline_directory_retry_test?") {
		t.Fatal("requires dedicated starline_directory_retry_test database")
	}
	s, p, source, target := directorySyncFixture(t)
	target2, err := s.CreateCourse("管理员", p, learning.CourseUpsertRequest{Name: "第二目标课程", LearningSpaceID: target.LearningSpaceID, Curriculum: []learning.CurriculumNode{{ID: "retry-second-original", Type: learning.CurriculumUnit, Name: "第二目标独有课节", SortOrder: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	material, err := s.CreateMaterial("管理员", p, learning.MaterialUploadRequest{Title: "目标原有讲义", CourseID: target.ID, LearningSpaceID: target.LearningSpaceID, LessonID: target.Curriculum[0].ID, File: learning.FileAsset{ID: "directory-retry-file", FileName: "原有.pdf", OriginalPath: "/controlled/directory-retry.pdf", FileSize: 3}})
	if err != nil {
		t.Fatal(err)
	}
	for i := range s.grants {
		s.grants[i].OpenedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer func() { s.db.Close() }()
	reload := func() {
		t.Helper()
		if err := s.db.Close(); err != nil {
			t.Fatal(err)
		}
		s = NewMemoryStore()
		if err := s.ConnectDatabase(dsn); err != nil {
			t.Fatal(err)
		}
	}
	reload()
	source, _ = s.findCourse(source.ID)
	target, _ = s.findCourse(target.ID)
	target2, _ = s.findCourse(target2.ID)
	beforeLogs := len(s.logs)
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID, target2.ID}}
	preview, err := s.PreviewCourseDirectorySync(p, req)
	if err != nil || len(preview.Targets) != 2 {
		t.Fatalf("preview: %#v %v", preview, err)
	}
	req.Snapshots = map[string]string{}
	for _, row := range preview.Targets {
		if row.Error != "" || len(row.Added) != 1 {
			t.Fatalf("unexpected target: %#v", row)
		}
		req.Snapshots[row.CourseID] = row.Snapshot
	}
	trigger := fmt.Sprintf(`CREATE TRIGGER starline_directory_retry_fail BEFORE INSERT ON course_curriculum_nodes FOR EACH ROW BEGIN IF NEW.course_id = '%s' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced second directory failure'; END IF; END`, strings.ReplaceAll(target2.ID, "'", "''"))
	if _, err := s.db.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncCourseDirectory("管理员", p, req); err == nil {
		t.Fatal("database failure reported sync success")
	}
	assertOriginal := func() {
		t.Helper()
		for _, want := range []learning.Course{source, target, target2} {
			got, ok := s.findCourse(want.ID)
			if !ok || !reflect.DeepEqual(got.Curriculum, want.Curriculum) || !reflect.DeepEqual(got.DirectorySyncMap, want.DirectorySyncMap) {
				t.Fatalf("failed sync changed directory: %#v", got)
			}
		}
		if len(s.logs) != beforeLogs {
			t.Fatal("failed sync persisted success logs")
		}
		for _, current := range s.materials {
			if current.ID == material.ID {
				if current.LessonID != target.Curriculum[0].ID || current.FileID != material.FileID {
					t.Fatal("original material reassigned")
				}
				return
			}
		}
		t.Fatal("original material lost")
	}
	assertOriginal()
	reload()
	assertOriginal()
	if _, err := s.db.Exec(`DROP TRIGGER starline_directory_retry_fail`); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PreviewCourseDirectorySync(p, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range preview.Targets {
		req.Snapshots[row.CourseID] = row.Snapshot
	}
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil || len(result.Targets) != 2 {
		t.Fatalf("retry: %#v %v", result, err)
	}
	for _, row := range result.Targets {
		if row.Status != "已同步" {
			t.Fatalf("retry target failed: %#v", row)
		}
	}
	reload()
	for _, want := range []learning.Course{target, target2} {
		got, _ := s.findCourse(want.ID)
		if len(got.Curriculum) != 2 || !hasDirectoryNodeID(got.Curriculum, want.Curriculum[0].ID) || got.DirectorySyncMap[source.ID+":"+source.Curriculum[0].ID] == "" {
			t.Fatalf("retry result lost identity: %#v", got)
		}
	}
	logs := len(s.logs)
	if _, err := s.SyncCourseDirectory("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.logs) != logs {
		t.Fatal("retry duplicated success logs")
	}
	for _, want := range []learning.Course{target, target2} {
		got, _ := s.findCourse(want.ID)
		if len(got.Curriculum) != 2 {
			t.Fatal("retry duplicated chapters")
		}
	}
}

func directorySyncFixture(t *testing.T) (*MemoryStore, learning.Principal, learning.Course, learning.Course) {
	t.Helper()
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	create := func(name, space, node string) learning.Course {
		course, err := s.CreateCourse("管理员", p, learning.CourseUpsertRequest{Name: name, LearningSpaceID: space, Curriculum: []learning.CurriculumNode{{ID: node, Type: learning.CurriculumUnit, Name: name, SortOrder: 1}}})
		if err != nil {
			t.Fatal(err)
		}
		return course
	}
	return s, p, create("源课节", "space-g05-english-s1-q1", "sync-source-unit"), create("目标原有课节", "space-g05-english-s1-q1-splus", "sync-target-unit")
}

func TestDirectorySyncMySQLRestartPreservesMatchedIdentity(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	s, p, source, target := directorySyncFixture(t)
	target.Curriculum[0].Name = source.Curriculum[0].Name
	if _, err := s.UpdateCourse("管理员", p, target.ID, learning.CourseUpsertRequest{Name: target.Name, LearningSpaceID: target.LearningSpaceID, Curriculum: target.Curriculum}); err != nil {
		t.Fatal(err)
	}
	for i := range s.grants {
		s.grants[i].OpenedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}}
	preview, err := s.PreviewCourseDirectorySync(p, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Snapshots = map[string]string{target.ID: preview.Targets[0].Snapshot}
	if _, err := s.SyncCourseDirectory("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	restart := NewMemoryStore()
	if err := restart.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer restart.db.Close()
	loaded, _ := restart.findCourse(target.ID)
	if loaded.DirectorySyncMap[source.ID+":"+source.Curriculum[0].ID] != target.Curriculum[0].ID {
		t.Fatalf("mapping lost after restart: %#v", loaded.DirectorySyncMap)
	}
	source.Curriculum[0].Name = "重载后改名"
	if _, err := restart.UpdateCourse("管理员", p, source.ID, learning.CourseUpsertRequest{Name: source.Name, LearningSpaceID: source.LearningSpaceID, Curriculum: source.Curriculum}); err != nil {
		t.Fatal(err)
	}
	preview, err = restart.PreviewCourseDirectorySync(p, req)
	if err != nil || len(preview.Targets[0].Added) != 0 || len(preview.Targets[0].Updated) != 1 {
		t.Fatalf("restart rename duplicates identity: %#v %v", preview, err)
	}
}

func TestDirectorySyncPreviewPreservesContentAndRetryDoesNotDuplicate(t *testing.T) {
	s, p, source, target := directorySyncFixture(t)
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID, "missing-target"}}
	beforeNodes, beforeLogs := len(target.Curriculum), len(s.logs)
	preview, err := s.PreviewCourseDirectorySync(p, req)
	if err != nil || len(preview.Targets) != 2 || len(preview.Targets[0].Added) != 1 || preview.Targets[0].Preserved != 1 || preview.Targets[1].Error == "" {
		t.Fatalf("incorrect preview: %#v %v", preview, err)
	}
	current, _ := s.findCourse(target.ID)
	if len(current.Curriculum) != beforeNodes || len(s.logs) != beforeLogs {
		t.Fatal("preview mutated state")
	}
	req.Snapshots = map[string]string{target.ID: preview.Targets[0].Snapshot}
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil || result.Targets[0].Status != "已同步" || result.Targets[1].Status != "失败" {
		t.Fatalf("per-target result: %#v %v", result, err)
	}
	current, _ = s.findCourse(target.ID)
	logs := len(s.logs)
	if len(current.Curriculum) != 2 || current.Curriculum[0].ID != target.Curriculum[0].ID {
		t.Fatal("target original chapter lost")
	}
	if _, err := s.SyncCourseDirectory("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	retried, _ := s.findCourse(target.ID)
	if len(retried.Curriculum) != 2 || len(s.logs) != logs {
		t.Fatal("retry duplicated chapters or logs")
	}
	source.Curriculum[0].Name = "更新课节"
	if _, err := s.UpdateCourse("管理员", p, source.ID, learning.CourseUpsertRequest{Name: source.Name, LearningSpaceID: source.LearningSpaceID, Curriculum: source.Curriculum}); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PreviewCourseDirectorySync(p, req)
	if err != nil || len(preview.Targets[0].Updated) != 1 || len(preview.Targets[0].Added) != 0 {
		t.Fatalf("rename lost identity: %#v %v", preview, err)
	}
	req.Snapshots[target.ID] = preview.Targets[0].Snapshot
	if _, err := s.SyncCourseDirectory("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	renamed, _ := s.findCourse(target.ID)
	if len(renamed.Curriculum) != 2 || renamed.Curriculum[1].Name != "更新课节" {
		t.Fatalf("renamed chapter not synchronized: %#v", renamed)
	}
}

func TestDirectorySyncRejectsStalePreviewAndReadOnlyRange(t *testing.T) {
	s, p, source, target := directorySyncFixture(t)
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}}
	preview, _ := s.PreviewCourseDirectorySync(p, req)
	req.Snapshots = map[string]string{target.ID: preview.Targets[0].Snapshot}
	target.Curriculum[0].Name = "目标新修改"
	if _, err := s.UpdateCourse("管理员", p, target.ID, learning.CourseUpsertRequest{Name: target.Name, LearningSpaceID: target.LearningSpaceID, Curriculum: target.Curriculum}); err != nil {
		t.Fatal(err)
	}
	result, err := s.SyncCourseDirectory("管理员", p, req)
	if err != nil || result.Targets[0].Error == "" {
		t.Fatalf("stale preview accepted: %#v %v", result, err)
	}
	current, _ := s.findCourse(target.ID)
	if len(current.Curriculum) != 1 {
		t.Fatal("stale request wrote nodes")
	}
	teacher, _ := s.PrincipalByUserID("user-teacher")
	teacher.LearningSpaceIDs = []string{source.LearningSpaceID}
	teacher.TeacherLibrary = &learning.TeacherLibraryPolicy{CanManageCourses: true, Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "english"}}}
	preview, err = s.PreviewCourseDirectorySync(teacher, req)
	if err != nil || preview.Targets[0].Error == "" {
		t.Fatalf("extra reading gave directory write: %#v %v", preview, err)
	}
}

func TestDirectorySyncDoesNotTurnBoundLeafIntoParent(t *testing.T) {
	s, p, source, target := directorySyncFixture(t)
	// Same full path maps the source root to the existing target leaf.
	source.Curriculum = []learning.CurriculumNode{{ID: "src-root", Type: learning.CurriculumUnit, Name: target.Curriculum[0].Name, SortOrder: 1}, {ID: "src-child", ParentID: "src-root", Type: learning.CurriculumLesson, Name: "新增下级", SortOrder: 1}}
	s.courses[findCourseIndex(s.courses, source.ID)].Curriculum = source.Curriculum
	s.materials = append(s.materials, learning.Material{ID: "protected-material", CourseID: target.ID, LessonID: target.Curriculum[0].ID})
	preview, err := s.PreviewCourseDirectorySync(p, learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}})
	if err != nil || preview.Targets[0].Error == "" {
		t.Fatalf("orphaned existing material: %#v %v", preview, err)
	}
	current, _ := s.findCourse(target.ID)
	if len(current.Curriculum) != 1 {
		t.Fatal("rejected preview wrote nodes")
	}
}

func TestDirectorySyncRetainsMatchedTargetIdentityOnLaterRename(t *testing.T) {
	s, p, source, target := directorySyncFixture(t)
	target.Curriculum[0].Name = source.Curriculum[0].Name
	if _, err := s.UpdateCourse("管理员", p, target.ID, learning.CourseUpsertRequest{Name: target.Name, LearningSpaceID: target.LearningSpaceID, Curriculum: target.Curriculum}); err != nil {
		t.Fatal(err)
	}
	req := learning.CourseDirectorySyncRequest{SourceCourseID: source.ID, TargetCourseIDs: []string{target.ID}}
	preview, _ := s.PreviewCourseDirectorySync(p, req)
	req.Snapshots = map[string]string{target.ID: preview.Targets[0].Snapshot}
	if _, err := s.SyncCourseDirectory("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	source.Curriculum[0].Name = "已改名"
	if _, err := s.UpdateCourse("管理员", p, source.ID, learning.CourseUpsertRequest{Name: source.Name, LearningSpaceID: source.LearningSpaceID, Curriculum: source.Curriculum}); err != nil {
		t.Fatal(err)
	}
	preview, _ = s.PreviewCourseDirectorySync(p, req)
	if len(preview.Targets[0].Added) != 0 || len(preview.Targets[0].Updated) != 1 {
		t.Fatalf("matched identity lost: %#v", preview)
	}
	req.Snapshots[target.ID] = preview.Targets[0].Snapshot
	if _, err := s.SyncCourseDirectory("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.findCourse(target.ID)
	if len(updated.Curriculum) != 1 || updated.Curriculum[0].ID != target.Curriculum[0].ID || updated.Curriculum[0].Name != "已改名" {
		t.Fatalf("target identity changed: %#v", updated)
	}
}

func hasDirectoryNodeID(nodes []learning.CurriculumNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}
