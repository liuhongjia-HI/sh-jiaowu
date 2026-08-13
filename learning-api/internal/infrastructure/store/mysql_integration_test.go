package store

import (
	"os"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestMySQLMutationSurvivesStoreRestart(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	first := NewMemoryStoreWithOptions(Options{
		SkipBaseData: true, BootstrapAdminName: "集成测试管理员",
		BootstrapAdminPhone: "13900000001", BootstrapAdminPassword: "Integration123!",
	})
	if err := first.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect first store: %v", err)
	}
	admin, err := first.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load bootstrap admin: %v", err)
	}
	created, err := first.CreateStudent("MySQL集成测试", admin, learning.StudentUpsertRequest{
		Name: "重启一致性学生", Phone: "17795550001", Grade: "五年级", SchoolName: "集成测试学校",
	})
	if err != nil {
		t.Fatalf("create persisted student: %v", err)
	}
	if err := first.db.Close(); err != nil {
		t.Fatalf("close first store db: %v", err)
	}

	second := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := second.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect restarted store: %v", err)
	}
	defer second.db.Close()
	reloaded, ok := second.findStudent(created.ID)
	if !ok {
		t.Fatalf("student %s missing after restart", created.ID)
	}
	if reloaded.Name != "重启一致性学生" || reloaded.SchoolName != "集成测试学校" {
		t.Fatalf("reloaded student mismatch: %#v", reloaded)
	}
}
