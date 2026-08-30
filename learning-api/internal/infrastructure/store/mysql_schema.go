package store

import (
	"database/sql"
	"errors"
	"strings"

	"starline/learning-api/internal/domain/learning"
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
		`CREATE TABLE IF NOT EXISTS subjects (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(64) NOT NULL,
			short_label VARCHAR(20) NOT NULL DEFAULT '',
			color VARCHAR(16) NOT NULL DEFAULT '',
			sort_order INT NOT NULL DEFAULT 0,
			status VARCHAR(32) NOT NULL DEFAULT '启用',
			UNIQUE KEY uk_subject_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS starline_file_assets (
			id VARCHAR(64) PRIMARY KEY,
			file_name VARCHAR(255) NOT NULL DEFAULT '',
			file_size BIGINT NOT NULL DEFAULT 0,
			file_type VARCHAR(32) NOT NULL DEFAULT '',
			content_type VARCHAR(128) NOT NULL DEFAULT '',
			original_path TEXT NOT NULL,
			preview_path TEXT NOT NULL,
			preview_page_dir TEXT NOT NULL,
			preview_page_count INT NOT NULL DEFAULT 0,
			preview_status VARCHAR(32) NOT NULL DEFAULT '',
			preview_error TEXT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS preview_jobs (
			id VARCHAR(64) PRIMARY KEY,
			file_id VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT '待处理',
			attempt_count INT NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			started_at DATETIME NULL,
			finished_at DATETIME NULL,
			KEY idx_preview_jobs_status (status, created_at),
			UNIQUE KEY uk_preview_jobs_file (file_id)
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
		`CREATE TABLE IF NOT EXISTS student_trial_records (
			external_id VARCHAR(64) PRIMARY KEY,
			student_id VARCHAR(64) NOT NULL,
			academic_year VARCHAR(32) NOT NULL,
			package_id VARCHAR(64) NOT NULL,
			starts_at DATE NOT NULL,
			ends_at DATE NOT NULL,
			status VARCHAR(16) NOT NULL,
			converted_package_id VARCHAR(64) NOT NULL DEFAULT '',
			converted_at DATETIME NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_student_trial_year (student_id, academic_year)
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
		// banners 是学生端小程序首页的运营轮播图，纯新表，不像其它表那样有历史数据要迁移，
		// 直接建表即可，不需要额外的 ensureColumnDefinition 补列步骤。
		`CREATE TABLE IF NOT EXISTS banners (
			id VARCHAR(64) PRIMARY KEY,
			image_url TEXT NOT NULL,
			title VARCHAR(128) NOT NULL DEFAULT '',
			link_type VARCHAR(16) NOT NULL DEFAULT 'none',
			link_value VARCHAR(512) NOT NULL DEFAULT '',
			sort_order INT NOT NULL DEFAULT 0,
			starts_at VARCHAR(32) NOT NULL DEFAULT '',
			ends_at VARCHAR(32) NOT NULL DEFAULT '',
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			created_at VARCHAR(32) NOT NULL DEFAULT ''
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		// guardians / guardian_students 是多子女/多家长改造的关系表：登录主体从
		// "学生"换成"家长"，谁能看哪个孩子由这张关系表决定，不再靠 students.phone
		// 撞出来。这一步只建表和打通存取，登录逻辑本身还没切过来（阶段2再改），
		// 现在这两张表暂时不影响任何现有读写路径。
		`CREATE TABLE IF NOT EXISTS guardians (
			id VARCHAR(64) PRIMARY KEY,
			phone VARCHAR(32) NOT NULL DEFAULT '',
			open_id VARCHAR(128) NOT NULL DEFAULT '',
			union_id VARCHAR(128) NOT NULL DEFAULT '',
			name VARCHAR(64) NOT NULL DEFAULT '',
			nickname VARCHAR(64) NOT NULL DEFAULT '',
			last_student_id VARCHAR(64) NOT NULL DEFAULT '',
			account_status VARCHAR(32) NOT NULL DEFAULT '正常',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_guardian_phone (phone),
			KEY idx_guardian_open_id (open_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS guardian_students (
			guardian_id VARCHAR(64) NOT NULL,
			student_id VARCHAR(64) NOT NULL,
			relation VARCHAR(16) NOT NULL DEFAULT '家长',
			is_primary TINYINT(1) NOT NULL DEFAULT 0,
			status VARCHAR(16) NOT NULL DEFAULT '在读',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (guardian_id, student_id),
			KEY idx_guardian_students_student (student_id)
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
		{"students", "registration_source", "VARCHAR(16) NOT NULL DEFAULT ''"},
		{"students", "last_study_at", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "effective_until", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "enrollment_academic_year", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "enrollment_grade", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "bind_code", "VARCHAR(16) NOT NULL DEFAULT ''"},
		{"students", "bind_code_expires_at", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"study_packages", "summary", "TEXT NOT NULL"},
		{"learning_spaces", "level", "VARCHAR(16) NOT NULL DEFAULT 'S'"},
		{"study_packages", "level", "VARCHAR(16) NOT NULL DEFAULT 'S'"},
		{"courses", "chapter_count", "INT NOT NULL DEFAULT 0"},
		{"courses", "chapters_json", "TEXT NULL"},
		{"materials", "view_count", "INT NOT NULL DEFAULT 0"},
		{"materials", "file_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"materials", "file_name", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"materials", "file_size", "BIGINT NOT NULL DEFAULT 0"},
		{"materials", "file_type", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"materials", "preview_status", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"materials", "preview_url", "TEXT NOT NULL"},
		{"materials", "download_url", "TEXT NOT NULL"},
		{"materials", "sort_order", "INT NOT NULL DEFAULT 0"},
		{"materials", "tag_code", "VARCHAR(16) NOT NULL DEFAULT ''"},
		{"homework_tasks", "package_name", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"homework_tasks", "chapter_name", "VARCHAR(128) NOT NULL DEFAULT ''"},
		{"homework_tasks", "tag_code", "VARCHAR(16) NOT NULL DEFAULT ''"},
		{"homework_tasks", "sort_order", "INT NOT NULL DEFAULT 0"},
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
		{"homework_tasks", "deadline_at", "DATETIME NULL"},
		{"homework_tasks", "assessment_type", "VARCHAR(16) NOT NULL DEFAULT 'practice'"},
		{"starline_file_assets", "preview_page_dir", "TEXT NOT NULL"},
		{"starline_file_assets", "preview_page_count", "INT NOT NULL DEFAULT 0"},
		{"starline_file_assets", "preview_error", "TEXT NOT NULL"},
		{"schedule_classes", "campus_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"schedule_classes", "room_name", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"schedule_classes", "academic_year", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"schedule_classes", "semester", "VARCHAR(32) NOT NULL DEFAULT ''"},
		// 课次模型：一行 = 一节课。lesson_date 是这节课的具体日期，
		// series_id 标出同一次重复排课生成的课次，detached 表示已被单独调整过。
		{"schedule_classes", "series_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"schedule_classes", "lesson_date", "DATE NULL"},
		{"schedule_classes", "detached", "TINYINT NOT NULL DEFAULT 0"},
		{"schedule_classes", "override_note", "TEXT NULL"},
		// 审核维度，与 status 的成班维度分开存。存量数据统一按「已通过」补，
		// 见 backfillScheduleAuditStatus——升级前排的课本来就是生效状态，
		// 不能因为加了审核字段就在学生端集体消失。
		{"schedule_classes", "audit_status", "VARCHAR(16) NOT NULL DEFAULT ''"},
		{"schedule_classes", "audit_reason", "TEXT NULL"},
		{"schedule_classes", "audited_by", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"schedule_classes", "audited_at", "DATETIME NULL"},
		{"schedule_classes", "created_by", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"schedule_classes", "created_by_role", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"student_package_grants", "external_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"student_package_grants", "opened_at", "DATETIME NULL"},
		{"study_packages", "trial_enabled", "TINYINT(1) NOT NULL DEFAULT 0"},
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
		{"subjects", "short_label", "VARCHAR(20) NOT NULL DEFAULT ''"},
		{"subjects", "color", "VARCHAR(16) NOT NULL DEFAULT ''"},
		{"subjects", "sort_order", "INT NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		if err := s.ensureColumn(column.table, column.name, column.def); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec("UPDATE students SET registration_source = '小程序' WHERE registration_source = '' AND remark = '小程序自助建档'"); err != nil {
		return err
	}
	// ensureColumn only adds missing columns. Existing deployments created the
	// notices key as VARCHAR(64), so explicitly normalize it before upserts
	// write generated station-notice IDs and before validating its unique index.
	if err := s.ensureColumnDefinition("notices", "external_id", mysqlNoticeExternalIDDefinition); err != nil {
		return err
	}
	if err := s.ensureColumnDefinition("student_package_grants", "starts_at", "DATETIME NOT NULL"); err != nil {
		return err
	}
	if err := s.ensureColumnDefinition("student_package_grants", "ends_at", "DATETIME NOT NULL"); err != nil {
		return err
	}
	if err := s.ensureColumnDefinition("student_learning_space_access", "starts_at", "DATETIME NOT NULL"); err != nil {
		return err
	}
	if err := s.ensureColumnDefinition("student_learning_space_access", "ends_at", "DATETIME NOT NULL"); err != nil {
		return err
	}
	// Stable external keys are required for request-time keyed upserts. Backfill
	// legacy rows using the same IDs the loader historically synthesized.
	backfills := []string{
		`UPDATE homework_tasks SET deadline_at = DATE_ADD(deadline, INTERVAL 86399 SECOND) WHERE deadline_at IS NULL AND deadline IS NOT NULL`,
		`UPDATE homework_tasks SET assessment_type = 'practice' WHERE assessment_type = ''`,
		`UPDATE student_package_grants SET external_id = CONCAT('grant-', id) WHERE external_id = ''`,
		`UPDATE student_package_grants SET opened_at = created_at WHERE opened_at IS NULL`,
		`UPDATE notices SET external_id = CONCAT('notice-', id) WHERE external_id = ''`,
		`UPDATE operation_logs SET external_id = CONCAT('log-', id) WHERE external_id = ''`,
		`UPDATE operation_logs log_row
JOIN (
	SELECT duplicate_rows.id
	FROM operation_logs duplicate_rows
	JOIN (
		SELECT external_id, MIN(id) AS keep_id
		FROM operation_logs
		WHERE external_id <> ''
		GROUP BY external_id
		HAVING COUNT(*) > 1
	) duplicate_groups ON duplicate_groups.external_id = duplicate_rows.external_id AND duplicate_rows.id <> duplicate_groups.keep_id
) duplicate_keys ON duplicate_keys.id = log_row.id
SET log_row.external_id = CONCAT('log-db-', log_row.id)`,
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
	if err := s.ensureLearningSpaceUniqueIndex(); err != nil {
		return err
	}
	if err := s.ensureIndex("schedule_classes", "idx_schedule_term", "academic_year, semester, status"); err != nil {
		return err
	}
	if err := s.backfillScheduleClassTerms(); err != nil {
		return err
	}
	if err := s.expandScheduleClassesIntoLessons(); err != nil {
		return err
	}
	if err := s.backfillScheduleAuditStatus(); err != nil {
		return err
	}
	return nil
}

// ensureLearningSpaceUniqueIndex 将等级纳入空间业务唯一性，并去掉已停用的学年维度。
// level 新列默认回填 S，因此升级不会改变存量空间的业务归属。
func (s *MemoryStore) ensureLearningSpaceUniqueIndex() error {
	const indexName = "uk_learning_space"
	const expectedColumns = "grade,subject,semester,phase,level"
	rows, err := s.db.Query(`SELECT column_name FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'learning_spaces' AND index_name = ?
		ORDER BY seq_in_index`, indexName)
	if err != nil {
		return err
	}
	columns := make([]string, 0)
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			rows.Close()
			return err
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if strings.Join(columns, ",") == expectedColumns {
		return nil
	}
	if len(columns) == 0 {
		_, err = s.db.Exec(`ALTER TABLE learning_spaces ADD UNIQUE KEY uk_learning_space (grade, subject, semester, phase, level)`)
		return err
	}
	// MySQL 8 的单条 ALTER TABLE 是原子 DDL：如果新唯一键因存量重复数据创建失败，
	// 旧唯一键也不会被单独删除，应用启动失败并保留原约束，便于先清理脏数据再重试。
	_, err = s.db.Exec(`ALTER TABLE learning_spaces
		DROP INDEX uk_learning_space,
		ADD UNIQUE KEY uk_learning_space (grade, subject, semester, phase, level)`)
	return err
}

// backfillScheduleAuditStatus 给升级前的排课补上审核状态。
//
// 存量课全部记为「已通过」：它们本来就是生效的安排，学生端也一直看得见。
// 如果留空，scheduleVisibleToStudent 会把它们全部判为不可见——
// 升级当天所有学生的课表会一起清空。
// 只补 audit_status 为空的行，重复执行安全。
func (s *MemoryStore) backfillScheduleAuditStatus() error {
	_, err := s.db.Exec(
		`UPDATE schedule_classes SET audit_status = ? WHERE audit_status = ''`,
		learning.AuditApproved,
	)
	return err
}

// expandScheduleClassesIntoLessons 把升级前的排课记录展开成课次。
//
// 升级前一条记录 = 「每周三 19:30，3/1 到 6/30」的一整串课；升级后一条记录 = 一节课。
// 展开时第一节沿用原记录的 id，这一点很关键：schedule_class_students 和
// lesson_consumptions 都靠这个 id 挂着，换 id 会把学生名单和课时消耗记录甩掉。
//
// 只处理 lesson_date 为空的行，所以重复执行是安全的。
func (s *MemoryStore) expandScheduleClassesIntoLessons() error {
	rows, err := s.db.Query(`SELECT id, day_of_week, start_date, end_date FROM schedule_classes WHERE lesson_date IS NULL`)
	if err != nil {
		return err
	}
	type pending struct {
		id        string
		dayOfWeek int
		startDate string
		endDate   string
	}
	items := make([]pending, 0)
	for rows.Next() {
		var item pending
		var startDate, endDate sql.NullTime
		if err := rows.Scan(&item.id, &item.dayOfWeek, &startDate, &endDate); err != nil {
			rows.Close()
			return err
		}
		item.startDate = dateString(startDate)
		item.endDate = dateString(endDate)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range items {
		dates := weeklyLessonDates(item.dayOfWeek, item.startDate, item.endDate)
		if len(dates) == 0 {
			// 连开始日期都解析不出来的脏数据：标成单节并保留原样，
			// 不要因为一条坏记录让整个迁移卡死。
			if _, err := s.db.Exec(`UPDATE schedule_classes SET lesson_date = start_date WHERE id = ?`, item.id); err != nil {
				return err
			}
			continue
		}
		seriesID := ""
		if len(dates) > 1 {
			seriesID = "series-migrated-" + item.id
		}
		// 第一节就地改写，保住原 id 上挂的学生名单与课时消耗。
		if _, err := s.db.Exec(
			`UPDATE schedule_classes SET lesson_date = ?, start_date = ?, end_date = ?, series_id = ? WHERE id = ?`,
			dates[0], dates[0], dates[0], seriesID, item.id,
		); err != nil {
			return err
		}
		for index, date := range dates[1:] {
			newID := item.id + "-l" + itoa(index+1)
			if _, err := s.db.Exec(`INSERT INTO schedule_classes (
				id, name, course_id, course_name, teacher_id, teacher_name, campus_id, room_name,
				class_type, capacity, duration_minutes, day_of_week, start_time, end_time,
				start_date, end_date, expected_student_count, reservation_note, academic_year,
				semester, status, created_at, series_id, lesson_date, detached, override_note)
				SELECT ?, name, course_id, course_name, teacher_id, teacher_name, campus_id, room_name,
				class_type, capacity, duration_minutes, day_of_week, start_time, end_time,
				?, ?, expected_student_count, reservation_note, academic_year,
				semester, status, created_at, ?, ?, 0, override_note
				FROM schedule_classes WHERE id = ?
				ON DUPLICATE KEY UPDATE lesson_date = VALUES(lesson_date)`,
				newID, date, date, seriesID, date, item.id,
			); err != nil {
				return err
			}
			if _, err := s.db.Exec(
				`INSERT IGNORE INTO schedule_class_students (schedule_class_id, student_id, student_name)
				SELECT ?, student_id, student_name FROM schedule_class_students WHERE schedule_class_id = ?`,
				newID, item.id,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// backfillScheduleClassTerms 给升级前建的历史排课记录一次性补上学年、学期，
// 判定口径和新建排课时（见 resolveScheduleTerm）完全一致：按开课日期落校历，
// 落不进任何学期时兜底用课程所属学习空间的学期 + 开课日期本身的 7 月 1 日规则。
// 只补 academic_year 为空的行，重复执行是安全的、不会覆盖已判定过的记录。
func (s *MemoryStore) backfillScheduleClassTerms() error {
	rows, err := s.db.Query(`SELECT sc.id, sc.start_date, ls.semester
		FROM schedule_classes sc
		LEFT JOIN courses c ON c.id = sc.course_id
		LEFT JOIN learning_spaces ls ON ls.id = c.learning_space_id
		WHERE sc.academic_year = ''`)
	if err != nil {
		return err
	}
	type pending struct {
		id, startDate, fallbackSemester string
	}
	items := make([]pending, 0)
	for rows.Next() {
		var item pending
		var startDate sql.NullTime
		var fallbackSemester sql.NullString
		if err := rows.Scan(&item.id, &startDate, &fallbackSemester); err != nil {
			rows.Close()
			return err
		}
		item.startDate = dateString(startDate)
		item.fallbackSemester = fallbackSemester.String
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, item := range items {
		academicYear, semester := s.resolveScheduleTerm(item.startDate, item.fallbackSemester)
		if _, err := s.db.Exec(`UPDATE schedule_classes SET academic_year = ?, semester = ? WHERE id = ?`, academicYear, semester, item.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryStore) ensureIndex(table, name, columns string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, table, name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.Exec("CREATE INDEX " + name + " ON " + table + " (" + columns + ")")
	return err
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
