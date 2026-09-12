package store

import (
	"os"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func TestMySQLOpeningPreservesFutureGrantsAfterReload(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	s := NewMemoryStoreWithOptions(Options{BootstrapAdminPhone: "17900000001", BootstrapAdminPassword: "OpeningTest123!"})
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	admin := learning.Principal{UserID: "user-super", Roles: []learning.Role{learning.RoleSuperAdmin}}
	student, err := s.CreateStudent("测试", admin, learning.StudentUpsertRequest{Name: "历史开通重载测试", Phone: "179" + time.Now().Format("15040500"), Grade: "四年级"})
	if err != nil {
		t.Fatal(err)
	}
	const id = "space-g04-math-s1-q1"
	_, err = s.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{StudentID: student.ID, LearningSpaceIDs: []string{id}, ContentTypeCodes: []string{"question"}, StartsAt: time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05"), EndsAt: time.Now().Add(72 * time.Hour).Format("2006-01-02 15:04:05")})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.directGrantPeriod(student.ID, directGrantPackageID(student.ID, id))
	var original learning.SubjectMetadata
	for _, meta := range s.Subjects() {
		if meta.ID == "math" {
			original = meta
		}
	}
	update := learning.SubjectMetadataUpdateRequest{ShortLabel: original.ShortLabel, Color: original.Color, SortOrder: original.SortOrder, Status: "停用"}
	if _, err = s.UpdateSubjectMetadata("测试", "math", update); err != nil {
		t.Fatal(err)
	}
	defer func() { update.Status = original.Status; s.UpdateSubjectMetadata("测试", "math", update) }()
	if _, err = s.ReplaceDirectGrant("测试", learning.DirectGrantReplaceRequest{StudentID: student.ID, Selections: []learning.DirectGrantSelection{{LearningSpaceID: "space-g04-english-s1-q1", ContentTypeCodes: []string{"course"}}}}); err != nil {
		t.Fatal(err)
	}
	reloaded := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err = reloaded.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer reloaded.db.Close()
	after, exists := reloaded.directGrantPeriod(student.ID, before.PackageID)
	if !exists || before != after {
		t.Fatalf("future grant not preserved after restart: %#v -> %#v", before, after)
	}
	if _, reason := reloaded.openingSubject(id, "四年级"); reason == "" {
		t.Fatal("restart re-enabled disabled subject")
	}
	if _, err = reloaded.ReplaceDirectGrant("测试", learning.DirectGrantReplaceRequest{StudentID: student.ID, RevokeDirectLearningSpaceIDs: []string{id}}); err != nil {
		t.Fatal(err)
	}
	if err = reloaded.loadAllFromDatabase(); err != nil {
		t.Fatal(err)
	}
	revoked, _ := reloaded.directGrantPeriod(student.ID, before.PackageID)
	if revoked.Status != "revoked" {
		t.Fatal("explicit revoke not persisted")
	}
}
