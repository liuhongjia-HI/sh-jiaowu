package store

import (
	"database/sql"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func noticeReadTestStore() *MemoryStore {
	s := NewMemoryStore()
	s.notices = []learning.Notice{
		{ID: "mine", RelatedType: "course", RelatedID: "course-001", RecipientStudentID: "stu-001", Status: "已发送"},
		{ID: "other", RelatedType: "course", RelatedID: "course-001", RecipientStudentID: "stu-002", Status: "已发送"},
		{ID: "hidden", RelatedType: "course", RelatedID: "course-001", RecipientStudentID: "stu-001", Status: "发送失败"},
	}
	return s
}

func TestMarkStudentNoticeRead(t *testing.T) {
	s := noticeReadTestStore()
	p, _ := s.PrincipalByUserID("user-student-001")
	for _, id := range []string{"other", "hidden", "missing"} {
		if _, err := s.MarkStudentNoticeRead(p, id); err == nil {
			t.Fatalf("must reject inaccessible notice %q", id)
		}
	}
	for i := 0; i < 2; i++ {
		notice, err := s.MarkStudentNoticeRead(p, "mine")
		if err != nil || !notice.IsRead || notice.Status != "已发送" {
			t.Fatalf("mark read must be idempotent and preserve delivery status: %#v, %v", notice, err)
		}
	}
	home, err := s.StudentHome(p)
	if err != nil || len(home.Notices) != 1 || !home.Notices[0].IsRead {
		t.Fatalf("reloaded inbox must include read state: %#v, %v", home.Notices, err)
	}
	if s.notices[1].IsRead || s.notices[2].IsRead {
		t.Fatal("mark read changed another notice")
	}
	if _, err := s.MarkStudentNoticeRead(learning.Principal{}, "mine"); err == nil {
		t.Fatal("must reject unbound account")
	}
}

func TestMarkStudentNoticeReadPersistsOnlyAfterCommit(t *testing.T) {
	for _, failWrite := range []bool{true, false} {
		mutationDriverState.reset(failWrite)
		db, err := sql.Open(mutationTestDriverName, "")
		if err != nil {
			t.Fatal(err)
		}
		s := noticeReadTestStore()
		s.db = db
		p, _ := s.PrincipalByUserID("user-student-001")
		_, err = s.MarkStudentNoticeRead(p, "mine")
		db.Close()
		if (err != nil) != failWrite || s.notices[0].IsRead == failWrite {
			t.Fatalf("failWrite=%v: err=%v, isRead=%v", failWrite, err, s.notices[0].IsRead)
		}
		mutationDriverState.mu.Lock()
		statements := append([]string(nil), mutationDriverState.statements...)
		commits, rollbacks := mutationDriverState.commits, mutationDriverState.rollbacks
		mutationDriverState.mu.Unlock()
		if len(statements) != 1 || !strings.Contains(statements[0], "is_read=VALUES(is_read)") {
			t.Fatalf("must persist the notice read state: %v", statements)
		}
		if failWrite && rollbacks != 1 || !failWrite && commits != 1 {
			t.Fatalf("unexpected transaction result: commits=%d rollbacks=%d", commits, rollbacks)
		}
	}
}
