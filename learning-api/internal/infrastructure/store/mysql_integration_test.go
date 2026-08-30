package store

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestMySQLSchemaActivatesLegacyPendingSelfServiceStudents(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	store := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := store.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.db.Close()

	nonce := time.Now().Format("20060102150405.000000000")
	selfServiceID := "legacy-self-service-" + nonce
	guardianID := "legacy-pending-guardian-" + nonce
	guardianStudentID := "legacy-pending-child-" + nonce
	for _, student := range []struct {
		id     string
		source string
	}{
		{id: selfServiceID, source: "小程序"},
		{id: guardianStudentID, source: ""},
	} {
		if _, err := store.db.Exec(`INSERT INTO students (id, name, grade, phone, account_status, registration_source)
			VALUES (?, '迁移测试学生', '五年级', ?, '待审核', ?)`, student.id, "178"+nonce[15:24], student.source); err != nil {
			t.Fatalf("insert pending student %s: %v", student.id, err)
		}
		if _, err := store.db.Exec(`INSERT INTO users (id, name, phone, account_status, student_id, password_hash)
			VALUES (?, '迁移测试学生', ?, '待审核', ?, '')`, "user-"+student.id, "178"+nonce[15:24], student.id); err != nil {
			t.Fatalf("insert pending user %s: %v", student.id, err)
		}
	}
	if _, err := store.db.Exec(`INSERT INTO guardians (id, phone) VALUES (?, ?)`, guardianID, "178"+nonce[15:24]); err != nil {
		t.Fatalf("insert guardian: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO guardian_students (guardian_id, student_id, status) VALUES (?, ?, '待审核')`, guardianID, guardianStudentID); err != nil {
		t.Fatalf("insert pending guardian relation: %v", err)
	}

	if err := store.ensurePersistenceSchema(); err != nil {
		t.Fatalf("run legacy self-service migration: %v", err)
	}
	for _, studentID := range []string{selfServiceID, guardianStudentID} {
		var studentStatus, userStatus string
		if err := store.db.QueryRow(`SELECT account_status FROM students WHERE id = ?`, studentID).Scan(&studentStatus); err != nil {
			t.Fatalf("read migrated student %s: %v", studentID, err)
		}
		if err := store.db.QueryRow(`SELECT account_status FROM users WHERE student_id = ?`, studentID).Scan(&userStatus); err != nil {
			t.Fatalf("read migrated user %s: %v", studentID, err)
		}
		if studentStatus != "正常" || userStatus != "正常" {
			t.Fatalf("legacy student %s not activated: student=%q user=%q", studentID, studentStatus, userStatus)
		}
	}
	var relationStatus string
	if err := store.db.QueryRow(`SELECT status FROM guardian_students WHERE guardian_id = ? AND student_id = ?`, guardianID, guardianStudentID).Scan(&relationStatus); err != nil {
		t.Fatalf("read migrated guardian relation: %v", err)
	}
	if relationStatus != "在读" {
		t.Fatalf("legacy guardian relation status = %q, want 在读", relationStatus)
	}
}

func TestMySQLNoticeExternalIDSchema(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	store := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := store.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.db.Close()

	var columnType, isNullable string
	var defaultValue sql.NullString
	if err := store.db.QueryRow(`SELECT column_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'notices' AND column_name = 'external_id'`).Scan(&columnType, &isNullable, &defaultValue); err != nil {
		t.Fatalf("read notices.external_id definition: %v", err)
	}
	if columnDefinitionNeedsMigration(columnType, isNullable, defaultValue) {
		t.Fatalf("notices.external_id was not migrated: type=%q nullable=%q default=%#v", columnType, isNullable, defaultValue)
	}

	var indexColumns int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'notices' AND index_name = 'uk_notice_external_id' AND non_unique = 0`).Scan(&indexColumns); err != nil {
		t.Fatalf("read notices external-ID index: %v", err)
	}
	if indexColumns != 1 {
		t.Fatalf("uk_notice_external_id column count = %d, want 1", indexColumns)
	}
}

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
