package store

import (
	"database/sql"
	"errors"
	"strings"
)

// mysqlIndexedExternalIDLength stays within the utf8mb4 index limit on both
// current and older MySQL deployments while allowing composite, generated IDs
// such as notice-homework-<homework>-<student>-station.
const mysqlIndexedExternalIDLength = 191

const mysqlNoticeExternalIDDefinition = "VARCHAR(191) NOT NULL DEFAULT ''"

func (s *MemoryStore) ensurePersistenceSchema() error {
	if s.db == nil {
		return errors.New("mysql connection is required")
	}
	if err := s.ensureSchedulingTables(); err != nil {
		return err
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS starline_file_assets (
			id VARCHAR(64) PRIMARY KEY,
			file_name VARCHAR(255) NOT NULL DEFAULT '',
			file_size BIGINT NOT NULL DEFAULT 0,
			file_type VARCHAR(32) NOT NULL DEFAULT '',
			content_type VARCHAR(128) NOT NULL DEFAULT '',
			original_path TEXT NOT NULL,
			preview_path TEXT NOT NULL,
			preview_status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS pending_reviews (
			id VARCHAR(64) PRIMARY KEY,
			student_id VARCHAR(64) NOT NULL DEFAULT '',
			homework_id VARCHAR(64) NOT NULL DEFAULT '',
			submission_id VARCHAR(64) NOT NULL DEFAULT '',
			student_name VARCHAR(64) NOT NULL DEFAULT '',
			package_name VARCHAR(128) NOT NULL DEFAULT '',
			homework_title VARCHAR(128) NOT NULL DEFAULT '',
			system_score INT NOT NULL DEFAULT 0,
			teacher_comment TEXT NOT NULL,
			reward VARCHAR(128) NOT NULL DEFAULT '',
			status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS question_bank_items (
			id VARCHAR(64) PRIMARY KEY,
			title VARCHAR(128) NOT NULL DEFAULT '',
			grade VARCHAR(32) NOT NULL DEFAULT '',
			semester VARCHAR(32) NOT NULL DEFAULT '',
			subject VARCHAR(32) NOT NULL DEFAULT '',
			question_type VARCHAR(32) NOT NULL DEFAULT '',
			stem TEXT NOT NULL,
			options_json TEXT NOT NULL,
			answer VARCHAR(255) NOT NULL DEFAULT '',
			answers_json TEXT NOT NULL,
			score INT NOT NULL DEFAULT 10,
			status VARCHAR(32) NOT NULL DEFAULT '启用',
			owner_teacher_id VARCHAR(64) NOT NULL DEFAULT '',
			owner_teacher_name VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS student_submission_results (
			id VARCHAR(64) PRIMARY KEY,
			homework_id VARCHAR(64) NOT NULL,
			student_id VARCHAR(64) NOT NULL,
			task_title VARCHAR(128) NOT NULL DEFAULT '',
			score INT NOT NULL DEFAULT 0,
			objective_score INT NOT NULL DEFAULT 0,
			final_score INT NOT NULL DEFAULT 0,
			teacher_comment TEXT NOT NULL,
			reward VARCHAR(128) NOT NULL DEFAULT '',
			status VARCHAR(32) NOT NULL DEFAULT '',
			answers_json TEXT NOT NULL,
			created_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS student_score_records (
			id VARCHAR(64) PRIMARY KEY,
			student_id VARCHAR(64) NOT NULL,
			subject VARCHAR(32) NOT NULL,
			exam_type VARCHAR(32) NOT NULL DEFAULT '阶段测评',
			exam_name VARCHAR(64) NOT NULL,
			exam_date DATE NOT NULL,
			score INT NOT NULL,
			full_score INT NOT NULL,
			average_score INT NOT NULL DEFAULT 0,
			teacher_comment TEXT NOT NULL,
			created_by VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			KEY idx_student_score_student (student_id, subject, exam_date)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS student_favorites (
			id VARCHAR(64) PRIMARY KEY,
			student_id VARCHAR(64) NOT NULL,
			target_type VARCHAR(32) NOT NULL,
			target_id VARCHAR(64) NOT NULL,
			title VARCHAR(128) NOT NULL DEFAULT '',
			course VARCHAR(128) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			UNIQUE KEY uk_student_favorite_target (student_id, target_type, target_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS student_subscriptions (
			student_id VARCHAR(64) PRIMARY KEY,
			enabled TINYINT(1) NOT NULL DEFAULT 0,
			template_ids_json TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS commercial_orders (
			id VARCHAR(64) PRIMARY KEY,
			order_no VARCHAR(64) NOT NULL DEFAULT '',
			student_id VARCHAR(64) NOT NULL DEFAULT '',
			student_name VARCHAR(64) NOT NULL DEFAULT '',
			package_id VARCHAR(64) NOT NULL DEFAULT '',
			package_name VARCHAR(255) NOT NULL DEFAULT '',
			amount_cent INT NOT NULL DEFAULT 0,
			paid_amount_cent INT NOT NULL DEFAULT 0,
			refunded_amount_cent INT NOT NULL DEFAULT 0,
			lesson_total INT NOT NULL DEFAULT 0,
			lesson_consumed INT NOT NULL DEFAULT 0,
			status VARCHAR(32) NOT NULL DEFAULT '',
			contract_status VARCHAR(32) NOT NULL DEFAULT '',
			invoice_status VARCHAR(32) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS commercial_payments (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			amount_cent INT NOT NULL DEFAULT 0,
			method VARCHAR(64) NOT NULL DEFAULT '',
			transaction_no VARCHAR(128) NOT NULL DEFAULT '',
			paid_at DATETIME NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS commercial_refunds (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			amount_cent INT NOT NULL DEFAULT 0,
			reason TEXT NOT NULL,
			refunded_at DATETIME NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS commercial_contracts (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			title VARCHAR(255) NOT NULL DEFAULT '',
			signer VARCHAR(64) NOT NULL DEFAULT '',
			signed_at DATETIME NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS commercial_invoices (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			title VARCHAR(255) NOT NULL DEFAULT '',
			tax_no VARCHAR(64) NOT NULL DEFAULT '',
			amount_cent INT NOT NULL DEFAULT 0,
			invoice_no VARCHAR(128) NOT NULL DEFAULT '',
			issued_at DATETIME NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS lesson_consumptions (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			student_id VARCHAR(64) NOT NULL DEFAULT '',
			schedule_class_id VARCHAR(64) NOT NULL DEFAULT '',
			lesson_count INT NOT NULL DEFAULT 0,
			consumed_at DATETIME NOT NULL,
			remark TEXT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS renewal_reminders (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			student_id VARCHAR(64) NOT NULL DEFAULT '',
			reason TEXT NOT NULL,
			due_at VARCHAR(32) NOT NULL DEFAULT '',
			status VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS parent_notices (
			id VARCHAR(64) PRIMARY KEY,
			order_id VARCHAR(64) NOT NULL DEFAULT '',
			student_id VARCHAR(64) NOT NULL DEFAULT '',
			title VARCHAR(255) NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			sent_at DATETIME NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT '',
			notice_id VARCHAR(64) NOT NULL DEFAULT '',
			channel VARCHAR(32) NOT NULL DEFAULT '',
			failure_reason TEXT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	columns := []struct {
		table string
		name  string
		def   string
	}{
		{"users", "password_hash", "TEXT NOT NULL"},
		{"users", "must_change_password", "TINYINT(1) NOT NULL DEFAULT 0"},
		{"users", "token_version", "INT NOT NULL DEFAULT 0"},
		{"students", "remark", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"students", "nickname", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"students", "avatar_url", "TEXT NOT NULL"},
		{"students", "school_name", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"students", "guardian_name", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "official_account_open_id", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"students", "learning_status", "VARCHAR(32) NOT NULL DEFAULT '未开始'"},
		{"students", "streak_days", "INT NOT NULL DEFAULT 0"},
		{"students", "average_score", "INT NOT NULL DEFAULT 0"},
		{"students", "badge_count", "INT NOT NULL DEFAULT 0"},
		{"students", "bind_status", "VARCHAR(32) NOT NULL DEFAULT '待绑定'"},
		{"students", "last_study_at", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "effective_until", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "enrollment_academic_year", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "enrollment_grade", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"study_packages", "summary", "TEXT NOT NULL"},
		{"courses", "chapter_count", "INT NOT NULL DEFAULT 0"},
		{"materials", "view_count", "INT NOT NULL DEFAULT 0"},
		{"materials", "file_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"materials", "file_name", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"materials", "file_size", "BIGINT NOT NULL DEFAULT 0"},
		{"materials", "file_type", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"materials", "preview_status", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"materials", "preview_url", "TEXT NOT NULL"},
		{"materials", "download_url", "TEXT NOT NULL"},
		{"homework_tasks", "package_name", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"homework_tasks", "grade", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "semester", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "subject", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "question_num", "INT NOT NULL DEFAULT 0"},
		{"homework_tasks", "question_ids_json", "TEXT NOT NULL"},
		{"homework_tasks", "submitted_num", "INT NOT NULL DEFAULT 0"},
		{"homework_tasks", "total_num", "INT NOT NULL DEFAULT 0"},
		{"homework_tasks", "file_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"homework_tasks", "file_name", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"homework_tasks", "file_size", "BIGINT NOT NULL DEFAULT 0"},
		{"homework_tasks", "file_type", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "preview_status", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "preview_url", "TEXT NOT NULL"},
		{"homework_tasks", "download_url", "TEXT NOT NULL"},
		{"schedule_classes", "campus_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"schedule_classes", "room_name", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"student_package_grants", "external_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"student_learning_space_access", "external_grant_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"notices", "external_id", mysqlNoticeExternalIDDefinition},
		{"notices", "channel", "VARCHAR(32) NOT NULL DEFAULT '站内通知'"},
		{"notices", "recipient_open_id", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"notices", "failure_reason", "TEXT NOT NULL"},
		{"notices", "related_type", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"notices", "related_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"notices", "retry_count", "INT NOT NULL DEFAULT 0"},
		{"parent_notices", "notice_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"parent_notices", "channel", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"parent_notices", "failure_reason", "TEXT NOT NULL"},
		{"operation_logs", "external_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"operation_logs", "ip", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"operation_logs", "user_agent", "TEXT NOT NULL"},
		{"operation_logs", "detail", "TEXT NOT NULL"},
		{"question_bank_items", "title", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"student_submission_results", "objective_score", "INT NOT NULL DEFAULT 0"},
		{"student_submission_results", "final_score", "INT NOT NULL DEFAULT 0"},
		{"student_submission_results", "answers_json", "TEXT NOT NULL"},
		{"student_score_records", "exam_type", "VARCHAR(32) NOT NULL DEFAULT '阶段测评'"},
	}
	for _, column := range columns {
		if err := s.ensureColumn(column.table, column.name, column.def); err != nil {
			return err
		}
	}
	// ensureColumn only adds missing columns. Existing deployments created the
	// notices key as VARCHAR(64), so explicitly normalize it before upserts
	// write generated station-notice IDs and before validating its unique index.
	if err := s.ensureColumnDefinition("notices", "external_id", mysqlNoticeExternalIDDefinition); err != nil {
		return err
	}
	// Stable external keys are required for request-time keyed upserts. Backfill
	// legacy rows using the same IDs the loader historically synthesized.
	backfills := []string{
		`UPDATE student_package_grants SET external_id = CONCAT('grant-', id) WHERE external_id = ''`,
		`UPDATE notices SET external_id = CONCAT('notice-', id) WHERE external_id = ''`,
		`UPDATE operation_logs SET external_id = CONCAT('log-', id) WHERE external_id = ''`,
		`UPDATE student_learning_space_access access_row JOIN student_package_grants grant_row ON grant_row.id = access_row.package_grant_id SET access_row.external_grant_id = grant_row.external_id WHERE access_row.external_grant_id = ''`,
	}
	for _, statement := range backfills {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	for _, index := range []struct{ table, name, columns string }{
		{"student_package_grants", "uk_grant_external_id", "external_id"},
		{"notices", "uk_notice_external_id", "external_id"},
		{"operation_logs", "uk_operation_log_external_id", "external_id"},
	} {
		if err := s.ensureUniqueIndex(index.table, index.name, index.columns); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryStore) ensureUniqueIndex(table, name, columns string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, table, name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.Exec("CREATE UNIQUE INDEX " + name + " ON " + table + " (" + columns + ")")
	return err
}

func (s *MemoryStore) ensureColumn(table, column, definition string) error {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

// ensureColumnDefinition applies an idempotent in-place migration for a
// column whose definition has changed. It intentionally runs before the
// unique-index check: widening an indexed VARCHAR preserves existing values
// and indexes, while allowing the subsequent keyed notice upsert to succeed.
func (s *MemoryStore) ensureColumnDefinition(table, column, definition string) error {
	var columnType, isNullable string
	var defaultValue sql.NullString
	err := s.db.QueryRow(
		`SELECT column_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column,
	).Scan(&columnType, &isNullable, &defaultValue)
	if err != nil {
		return err
	}
	if !columnDefinitionNeedsMigration(columnType, isNullable, defaultValue) {
		return nil
	}
	_, err = s.db.Exec("ALTER TABLE " + table + " MODIFY COLUMN " + column + " " + definition)
	return err
}

func columnDefinitionNeedsMigration(columnType, isNullable string, defaultValue sql.NullString) bool {
	return !strings.EqualFold(columnType, "varchar(191)") ||
		!strings.EqualFold(isNullable, "NO") ||
		!defaultValue.Valid ||
		defaultValue.String != ""
}

func (s *MemoryStore) needsDatabaseBootstrap() (bool, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE id = 'user-super'").Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}
