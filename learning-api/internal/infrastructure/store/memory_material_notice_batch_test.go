package store

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestMaterialUploadBatchMySQLReload(t *testing.T) {
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
	p, _ := s.PrincipalByUserID("user-super")
	course, _ := s.findCourse("course-g05-english-s1-q1")
	req := learning.MaterialUploadRequest{BatchID: "persisted-upload-batch", Title: "持久化批量文件", CourseID: course.ID, LessonID: firstLessonID(course), TagCode: "HD", File: learning.FileAsset{ID: "batch-persistence-first", FileName: "first.pdf"}}
	if _, err := s.CreateMaterial("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, n := range s.notices {
		if strings.HasPrefix(n.ID, "notice-upload-") && n.RelatedID == course.ID {
			ids[n.RecipientStudentID] = n.ID
		}
	}
	if len(ids) == 0 {
		t.Fatal("fixture has no batch recipients")
	}
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	s = NewMemoryStore()
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	req.File.ID = "batch-persistence-retry"
	if _, err := s.CreateMaterial("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, n := range s.notices {
		if strings.HasPrefix(n.ID, "notice-upload-") && n.RelatedID == course.ID {
			counts[n.RecipientStudentID]++
			if ids[n.RecipientStudentID] != n.ID {
				t.Fatal("reload lost notification batch identity")
			}
		}
	}
	if len(counts) != len(ids) {
		t.Fatal("reload changed recipient coverage")
	}
	for _, n := range counts {
		if n != 1 {
			t.Fatal("retry after reload duplicated notification")
		}
	}
}

func TestMaterialUploadBatchMergesRecipientsAndFailedFileDoesNotNotify(t *testing.T) {
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	course, _ := s.findCourse("course-g05-english-s1-q1")
	req := learning.MaterialUploadRequest{BatchID: "operation-one", Title: "第一份", CourseID: course.ID, LessonID: firstLessonID(course), TagCode: "HD", File: learning.FileAsset{ID: "batch-file-one", FileName: "first.pdf"}}
	before := len(s.notices)
	if _, err := s.CreateMaterial("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	first := append([]learning.Notice(nil), s.notices[:len(s.notices)-before]...)
	if len(first) == 0 {
		t.Fatal("no eligible recipients in fixture")
	}
	for i := range first {
		if first[i].RecipientStudentID == "" || first[i].RelatedID != course.ID || first[i].Channel != "站内通知" {
			t.Fatalf("incorrect batch recipient: %#v", first[i])
		}
	}
	s.notices[0].IsRead = true
	req.Title, req.File.ID = "第二份", "batch-file-two"
	if _, err := s.CreateMaterial("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	if len(s.notices) != before+len(first) {
		t.Fatal("same batch generated duplicate recipient notices")
	}
	if s.notices[0].IsRead {
		t.Fatal("later successful upload left a previously opened batch notice read")
	}
	state := append([]learning.Notice(nil), s.notices...)
	req.LessonID = "nonexistent-chapter"
	if _, err := s.CreateMaterial("管理员", p, req); err == nil || !reflect.DeepEqual(state, s.notices) {
		t.Fatal("failed file changed batch notices")
	}
	req.LessonID, req.BatchID, req.File.ID = firstLessonID(course), "operation-two", "batch-file-three"
	if _, err := s.CreateMaterial("管理员", p, req); err != nil {
		t.Fatal(err)
	}
	if len(s.notices) != before+2*len(first) {
		t.Fatal("independent operation incorrectly suppressed")
	}
	// A second uploader cannot merge into another operator's batch namespace.
	teacher, _ := s.PrincipalByUserID("user-teacher")
	req.File.ID = "batch-file-teacher"
	if _, err := s.CreateMaterial("老师", teacher, req); err != nil {
		t.Fatal(err)
	}
	if len(s.notices) != before+3*len(first) {
		t.Fatal("different uploader reused another operator's notification batch")
	}
	req.BatchID = strings.Repeat("a", 65)
	beforeMaterials, beforeNotices := len(s.materials), len(s.notices)
	if _, err := s.CreateMaterial("老师", teacher, req); err == nil || len(s.materials) != beforeMaterials || len(s.notices) != beforeNotices {
		t.Fatal("oversized batch identifier accepted or mutated state")
	}
}
