package store

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) ConnectDatabase(dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}
	s.db = db
	if err := s.ensurePersistenceSchema(); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	bootstrap, err := s.needsDatabaseBootstrap()
	if err != nil {
		db.Close()
		s.db = nil
		return err
	}
	if bootstrap {
		if err := s.persistAll(); err != nil {
			db.Close()
			s.db = nil
			return err
		}
	}
	if err := s.loadAllFromDatabase(); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	return nil
}

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
		`CREATE TABLE IF NOT EXISTS student_submission_results (
			id VARCHAR(64) PRIMARY KEY,
			homework_id VARCHAR(64) NOT NULL,
			student_id VARCHAR(64) NOT NULL,
			task_title VARCHAR(128) NOT NULL DEFAULT '',
			score INT NOT NULL DEFAULT 0,
			teacher_comment TEXT NOT NULL,
			reward VARCHAR(128) NOT NULL DEFAULT '',
			status VARCHAR(32) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL
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
			status VARCHAR(32) NOT NULL DEFAULT ''
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
		{"students", "learning_status", "VARCHAR(32) NOT NULL DEFAULT '未开始'"},
		{"students", "streak_days", "INT NOT NULL DEFAULT 0"},
		{"students", "average_score", "INT NOT NULL DEFAULT 0"},
		{"students", "badge_count", "INT NOT NULL DEFAULT 0"},
		{"students", "bind_status", "VARCHAR(32) NOT NULL DEFAULT '待绑定'"},
		{"students", "last_study_at", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"students", "effective_until", "VARCHAR(32) NOT NULL DEFAULT ''"},
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
		{"homework_tasks", "question_num", "INT NOT NULL DEFAULT 0"},
		{"homework_tasks", "submitted_num", "INT NOT NULL DEFAULT 0"},
		{"homework_tasks", "total_num", "INT NOT NULL DEFAULT 0"},
		{"homework_tasks", "file_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"homework_tasks", "file_name", "VARCHAR(255) NOT NULL DEFAULT ''"},
		{"homework_tasks", "file_size", "BIGINT NOT NULL DEFAULT 0"},
		{"homework_tasks", "file_type", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "preview_status", "VARCHAR(32) NOT NULL DEFAULT ''"},
		{"homework_tasks", "preview_url", "TEXT NOT NULL"},
		{"homework_tasks", "download_url", "TEXT NOT NULL"},
		{"student_package_grants", "external_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"student_learning_space_access", "external_grant_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"notices", "external_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"operation_logs", "external_id", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"operation_logs", "ip", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"operation_logs", "user_agent", "TEXT NOT NULL"},
		{"operation_logs", "detail", "TEXT NOT NULL"},
	}
	for _, column := range columns {
		if err := s.ensureColumn(column.table, column.name, column.def); err != nil {
			return err
		}
	}
	return nil
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

func (s *MemoryStore) needsDatabaseBootstrap() (bool, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE id = 'user-super'").Scan(&count); err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM study_packages").Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *MemoryStore) persistAll() error {
	if s.db == nil {
		return errors.New("mysql connection is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := s.persistAllTx(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *MemoryStore) persistAllTx(tx *sql.Tx) error {
	deletes := []string{
		"DELETE FROM parent_notices",
		"DELETE FROM renewal_reminders",
		"DELETE FROM lesson_consumptions",
		"DELETE FROM commercial_invoices",
		"DELETE FROM commercial_contracts",
		"DELETE FROM commercial_refunds",
		"DELETE FROM commercial_payments",
		"DELETE FROM commercial_orders",
		"DELETE FROM student_favorites",
		"DELETE FROM student_submission_results",
		"DELETE FROM pending_reviews",
		"DELETE FROM starline_file_assets",
		"DELETE FROM schedule_class_students",
		"DELETE FROM schedule_classes",
		"DELETE FROM availability_slots",
		"DELETE FROM operation_logs",
		"DELETE FROM system_settings",
		"DELETE FROM notices",
		"DELETE FROM student_learning_space_access",
		"DELETE FROM student_package_grants",
		"DELETE FROM package_content_types",
		"DELETE FROM package_spaces",
		"DELETE FROM materials",
		"DELETE FROM homework_tasks",
		"DELETE FROM courses",
		"DELETE FROM study_packages",
		"DELETE FROM teacher_learning_space_access",
		"DELETE FROM admin_campus_scopes",
		"DELETE FROM user_roles",
		"DELETE FROM users",
		"DELETE FROM students",
		"DELETE FROM learning_spaces",
	}
	for _, statement := range deletes {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	if err := s.persistStaticRowsTx(tx); err != nil {
		return err
	}
	for _, space := range s.learningSpaces {
		if _, err := tx.Exec(
			`INSERT INTO learning_spaces (id, academic_year, grade, subject, semester, phase, name, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			space.ID, space.AcademicYear, space.Grade, space.Subject, space.Semester, space.Phase, space.Name, space.Status,
		); err != nil {
			return err
		}
	}
	for _, student := range s.students {
		if _, err := tx.Exec(
			`INSERT INTO students (id, name, grade, phone, account_status, remark, learning_status, streak_days, average_score, badge_count, bind_status, last_study_at, effective_until)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			student.ID, student.Name, student.Grade, student.Phone, student.AccountStatus, student.Remark, student.LearningStatus,
			student.StreakDays, student.AverageScore, student.BadgeCount, student.BindStatus, student.LastStudyAt, student.EffectiveUntil,
		); err != nil {
			return err
		}
	}
	for _, user := range s.users {
		if _, err := tx.Exec(
			`INSERT INTO users (id, name, phone, open_id, union_id, account_status, remark, student_id, campus_id, password_hash, must_change_password, token_version)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			user.ID, user.Name, user.Phone, user.OpenID, user.UnionID, user.AccountStatus, user.Remark, user.StudentID, user.CampusID, user.PasswordHash, boolInt(user.MustChangePassword), user.TokenVersion,
		); err != nil {
			return err
		}
		for _, role := range user.Roles {
			if _, err := tx.Exec(`INSERT INTO user_roles (user_id, role_code) VALUES (?, ?)`, user.ID, role); err != nil {
				return err
			}
		}
		for _, campusID := range user.CampusScopes {
			if _, err := tx.Exec(`INSERT INTO admin_campus_scopes (user_id, campus_id) VALUES (?, ?)`, user.ID, campusID); err != nil {
				return err
			}
		}
		for _, spaceID := range user.LearningSpaceIDs {
			if _, err := tx.Exec(
				`INSERT INTO teacher_learning_space_access (teacher_id, learning_space_id, can_view, can_upload_handout, can_upload_question, can_review, can_manage_content, status)
				 VALUES (?, ?, 1, ?, ?, ?, 1, 'active')`,
				user.ID, spaceID, boolInt(user.CanUploadHandout), boolInt(user.CanUploadQuestion), boolInt(user.CanReview),
			); err != nil {
				return err
			}
		}
	}
	for _, pkg := range s.packages {
		if _, err := tx.Exec(
			`INSERT INTO study_packages (id, name, academic_year, grade, semester, subject, phase_scope, package_type, summary, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pkg.ID, pkg.Name, pkg.AcademicYear, pkg.Grade, pkg.Semester, pkg.Subject, pkg.PhaseScope, pkg.PackageType, pkg.Summary, pkg.Status,
		); err != nil {
			return err
		}
	}
	for _, relation := range s.packageSpaces {
		if _, err := tx.Exec(`INSERT INTO package_spaces (package_id, learning_space_id) VALUES (?, ?)`, relation.PackageID, relation.LearningSpaceID); err != nil {
			return err
		}
	}
	for _, item := range s.contentTypes {
		if _, err := tx.Exec(`INSERT INTO package_content_types (package_id, content_type) VALUES (?, ?)`, item.PackageID, item.ContentType); err != nil {
			return err
		}
	}
	for _, course := range s.courses {
		if _, err := tx.Exec(
			`INSERT INTO courses (id, learning_space_id, name, subject, grade, status, chapter_count) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			course.ID, course.LearningSpaceID, course.Name, course.Subject, course.Grade, course.Status, course.ChapterCount,
		); err != nil {
			return err
		}
	}
	for _, material := range s.materials {
		if _, err := tx.Exec(
			`INSERT INTO materials (id, learning_space_id, course_id, title, chapter_name, material_type, owner_teacher_id, owner_teacher_name, publish_status, status, view_count, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			material.ID, material.LearningSpaceID, material.CourseID, material.Title, material.Chapter, material.Type, material.OwnerTeacherID,
			material.OwnerTeacherName, material.PublishStatus, material.Status, material.ViewCount, material.FileID, material.FileName,
			material.FileSize, material.FileType, material.PreviewStatus, material.PreviewURL, material.DownloadURL,
		); err != nil {
			return err
		}
	}
	for _, item := range s.homework {
		if _, err := tx.Exec(
			`INSERT INTO homework_tasks (id, learning_space_id, course_id, title, deadline, owner_teacher_id, owner_teacher_name, publish_status, status, package_name, question_num, submitted_num, total_num, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.LearningSpaceID, item.CourseID, item.Title, nullableDate(item.Deadline), item.OwnerTeacherID, item.OwnerTeacherName,
			item.PublishStatus, item.Status, item.PackageName, item.QuestionNum, item.SubmittedNum, item.TotalNum, item.FileID, item.FileName,
			item.FileSize, item.FileType, item.PreviewStatus, item.PreviewURL, item.DownloadURL,
		); err != nil {
			return err
		}
	}
	grantIDs := map[string]int{}
	for index, grant := range s.grants {
		dbID := index + 1
		grantIDs[grant.ID] = dbID
		if _, err := tx.Exec(
			`INSERT INTO student_package_grants (id, external_id, student_id, package_id, starts_at, ends_at, status, operator_id, operator_name)
			 VALUES (?, ?, ?, ?, ?, ?, ?, '', '')`,
			dbID, grant.ID, grant.StudentID, grant.PackageID, nullableDate(grant.StartsAt), nullableDate(grantEndsAt(grant)), grant.Status,
		); err != nil {
			return err
		}
	}
	for _, access := range s.spaceAccess {
		dbGrantID := grantIDs[access.PackageGrantID]
		if dbGrantID == 0 {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO student_learning_space_access (student_id, learning_space_id, package_grant_id, external_grant_id, starts_at, ends_at, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			access.StudentID, access.LearningSpaceID, dbGrantID, access.PackageGrantID, nullableDate(access.StartsAt), nullableDate(access.EndsAt), access.Status,
		); err != nil {
			return err
		}
	}
	for _, asset := range s.fileAssets {
		if _, err := tx.Exec(
			`INSERT INTO starline_file_assets (id, file_name, file_size, file_type, content_type, original_path, preview_path, preview_status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			asset.ID, asset.FileName, asset.FileSize, asset.FileType, asset.ContentType, asset.OriginalPath, asset.PreviewPath, asset.PreviewStatus,
		); err != nil {
			return err
		}
	}
	for _, slot := range s.availability {
		if _, err := tx.Exec(
			`INSERT INTO availability_slots (id, owner_type, owner_id, owner_name, day_of_week, start_time, end_time, start_date, end_date, remark)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			slot.ID, slot.OwnerType, slot.OwnerID, slot.OwnerName, slot.DayOfWeek, slot.StartTime, slot.EndTime, nullableDate(slot.StartDate), nullableDate(slot.EndDate), slot.Remark,
		); err != nil {
			return err
		}
	}
	for _, item := range s.scheduleClasses {
		if _, err := tx.Exec(
			`INSERT INTO schedule_classes (id, name, course_id, course_name, teacher_id, teacher_name, class_type, capacity, duration_minutes, day_of_week, start_time, end_time, start_date, end_date, status, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.Name, item.CourseID, item.CourseName, item.TeacherID, item.TeacherName, item.ClassType, item.Capacity, item.DurationMinutes,
			item.DayOfWeek, item.StartTime, item.EndTime, nullableDate(item.StartDate), nullableDate(item.EndDate), item.Status, nullableDateTime(item.CreatedAt),
		); err != nil {
			return err
		}
		for _, student := range item.Students {
			if _, err := tx.Exec(
				`INSERT INTO schedule_class_students (schedule_class_id, student_id, student_name) VALUES (?, ?, ?)`,
				item.ID, student.ID, student.Name,
			); err != nil {
				return err
			}
		}
	}
	for _, item := range s.commercialOrders {
		if _, err := tx.Exec(
			`INSERT INTO commercial_orders (id, order_no, student_id, student_name, package_id, package_name, amount_cent, paid_amount_cent, refunded_amount_cent, lesson_total, lesson_consumed, status, contract_status, invoice_status, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.OrderNo, item.StudentID, item.StudentName, item.PackageID, item.PackageName, item.AmountCent, item.PaidAmountCent, item.RefundedAmountCent, item.LessonTotal, item.LessonConsumed, item.Status, item.ContractStatus, item.InvoiceStatus, nullableDateTime(item.CreatedAt),
		); err != nil {
			return err
		}
	}
	for _, item := range s.payments {
		if _, err := tx.Exec(`INSERT INTO commercial_payments (id, order_id, amount_cent, method, transaction_no, paid_at, status) VALUES (?, ?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.AmountCent, item.Method, item.TransactionNo, nullableDateTime(item.PaidAt), item.Status); err != nil {
			return err
		}
	}
	for _, item := range s.refunds {
		if _, err := tx.Exec(`INSERT INTO commercial_refunds (id, order_id, amount_cent, reason, refunded_at, status) VALUES (?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.AmountCent, item.Reason, nullableDateTime(item.RefundedAt), item.Status); err != nil {
			return err
		}
	}
	for _, item := range s.contracts {
		if _, err := tx.Exec(`INSERT INTO commercial_contracts (id, order_id, title, signer, signed_at, status) VALUES (?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.Title, item.Signer, nullableDateTime(item.SignedAt), item.Status); err != nil {
			return err
		}
	}
	for _, item := range s.invoices {
		if _, err := tx.Exec(`INSERT INTO commercial_invoices (id, order_id, title, tax_no, amount_cent, invoice_no, issued_at, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.Title, item.TaxNo, item.AmountCent, item.InvoiceNo, nullableDateTime(item.IssuedAt), item.Status); err != nil {
			return err
		}
	}
	for _, item := range s.lessonConsumptions {
		if _, err := tx.Exec(`INSERT INTO lesson_consumptions (id, order_id, student_id, schedule_class_id, lesson_count, consumed_at, remark) VALUES (?, ?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.StudentID, item.ScheduleClassID, item.LessonCount, nullableDateTime(item.ConsumedAt), item.Remark); err != nil {
			return err
		}
	}
	for _, item := range s.renewalReminders {
		if _, err := tx.Exec(`INSERT INTO renewal_reminders (id, order_id, student_id, reason, due_at, status) VALUES (?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.StudentID, item.Reason, item.DueAt, item.Status); err != nil {
			return err
		}
	}
	for _, item := range s.parentNotices {
		if _, err := tx.Exec(`INSERT INTO parent_notices (id, order_id, student_id, title, content, sent_at, status) VALUES (?, ?, ?, ?, ?, ?, ?)`, item.ID, item.OrderID, item.StudentID, item.Title, item.Content, nullableDateTime(item.SentAt), item.Status); err != nil {
			return err
		}
	}
	for _, review := range s.reviews {
		if _, err := tx.Exec(
			`INSERT INTO pending_reviews (id, student_id, homework_id, submission_id, student_name, package_name, homework_title, system_score, teacher_comment, reward, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			review.ID, review.StudentID, review.HomeworkID, review.SubmissionID, review.StudentName, review.PackageName, review.Homework,
			review.SystemScore, review.TeacherComment, review.Reward, review.Status,
		); err != nil {
			return err
		}
	}
	for _, notice := range s.notices {
		if _, err := tx.Exec(
			`INSERT INTO notices (external_id, notice_type, title, target, content, status) VALUES (?, ?, ?, ?, ?, ?)`,
			notice.ID, notice.Type, notice.Title, notice.Target, notice.Summary, notice.Status,
		); err != nil {
			return err
		}
	}
	for index := len(s.logs) - 1; index >= 0; index-- {
		log := s.logs[index]
		if _, err := tx.Exec(
			`INSERT INTO operation_logs (external_id, operator_id, operator_name, action, target, ip, user_agent, detail, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			log.ID, log.OperatorID, log.Operator, log.Action, log.Target, log.IP, log.UserAgent, log.Detail, nullableDateTime(log.Time),
		); err != nil {
			return err
		}
	}
	for key, value := range s.settings {
		if _, err := tx.Exec(`INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?)`, key, value); err != nil {
			return err
		}
	}
	for _, submission := range s.submissions {
		if _, err := tx.Exec(
			`INSERT INTO student_submission_results (id, homework_id, student_id, task_title, score, teacher_comment, reward, status, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			submission.ID, submission.HomeworkID, submission.StudentID, submission.TaskTitle, submission.Score, submission.TeacherComment,
			submission.Reward, submission.Status, nullableDateTime(submission.CreatedAt),
		); err != nil {
			return err
		}
	}
	for _, favorite := range s.favorites {
		if _, err := tx.Exec(
			`INSERT INTO student_favorites (id, student_id, target_type, target_id, title, course, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			favorite.ID, favorite.StudentID, favorite.TargetType, favorite.TargetID, favorite.Title, favorite.Course, nullableDateTime(favorite.CreatedAt),
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryStore) persistStaticRowsTx(tx *sql.Tx) error {
	for _, role := range []struct {
		code string
		name string
	}{
		{"student", "学生"}, {"teacher", "老师"}, {"ops_staff", "运营教务"}, {"campus_admin", "校区管理员"}, {"super_admin", "超级管理员"},
	} {
		if _, err := tx.Exec(`INSERT IGNORE INTO roles (code, name) VALUES (?, ?)`, role.code, role.name); err != nil {
			return err
		}
	}
	for _, subject := range demoSubjects {
		if _, err := tx.Exec(`INSERT IGNORE INTO subjects (id, name, status) VALUES (?, ?, '启用')`, subjectSlug(subject), subject); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryStore) loadAllFromDatabase() error {
	loaders := []func() error{
		s.loadLearningSpacesFromDB,
		s.loadStudentsFromDB,
		s.loadUsersFromDB,
		s.loadPackagesFromDB,
		s.loadCoursesFromDB,
		s.loadMaterialsFromDB,
		s.loadHomeworkFromDB,
		s.loadGrantsFromDB,
		s.loadFileAssetsFromDB,
		s.loadReviewsFromDB,
		s.loadNoticesFromDB,
		s.loadLogsFromDB,
		s.loadSettingsFromDB,
		s.loadSubmissionsFromDB,
		s.loadFavoritesFromDB,
		s.loadCommercialFromDB,
	}
	for _, loader := range loaders {
		if err := loader(); err != nil {
			return err
		}
	}
	slots, err := s.loadAvailabilitySlots()
	if err != nil {
		return err
	}
	s.availability = slots
	classes, err := s.loadScheduleClasses()
	if err != nil {
		return err
	}
	s.scheduleClasses = classes
	return nil
}

func (s *MemoryStore) loadLearningSpacesFromDB() error {
	rows, err := s.db.Query(`SELECT id, academic_year, grade, subject, semester, phase, name, status FROM learning_spaces ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learningSpace{}
	for rows.Next() {
		var item learningSpace
		if err := rows.Scan(&item.ID, &item.AcademicYear, &item.Grade, &item.Subject, &item.Semester, &item.Phase, &item.Name, &item.Status); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.learningSpaces = out
	return rows.Err()
}

func (s *MemoryStore) loadStudentsFromDB() error {
	rows, err := s.db.Query(`SELECT id, name, grade, phone, account_status, remark, learning_status, streak_days, average_score, badge_count, bind_status, last_study_at, effective_until FROM students ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Student{}
	for rows.Next() {
		var item learning.Student
		if err := rows.Scan(&item.ID, &item.Name, &item.Grade, &item.Phone, &item.AccountStatus, &item.Remark, &item.LearningStatus, &item.StreakDays, &item.AverageScore, &item.BadgeCount, &item.BindStatus, &item.LastStudyAt, &item.EffectiveUntil); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.students = out
	return rows.Err()
}

func (s *MemoryStore) loadUsersFromDB() error {
	rows, err := s.db.Query(`SELECT id, name, phone, open_id, union_id, password_hash, must_change_password, token_version, account_status, remark, student_id, campus_id FROM users ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.User{}
	for rows.Next() {
		var item learning.User
		var mustChange int
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.OpenID, &item.UnionID, &item.PasswordHash, &mustChange, &item.TokenVersion, &item.AccountStatus, &item.Remark, &item.StudentID, &item.CampusID); err != nil {
			return err
		}
		item.MustChangePassword = mustChange == 1
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	roles, err := s.loadUserRoles()
	if err != nil {
		return err
	}
	campuses, err := s.loadUserCampusScopes()
	if err != nil {
		return err
	}
	scopes, err := s.loadTeacherScopes()
	if err != nil {
		return err
	}
	for index := range out {
		out[index].Roles = roles[out[index].ID]
		out[index].CampusScopes = campuses[out[index].ID]
		if scope, ok := scopes[out[index].ID]; ok {
			out[index].LearningSpaceIDs = scope.spaces
			out[index].CanUploadHandout = scope.canUploadHandout
			out[index].CanUploadQuestion = scope.canUploadQuestion
			out[index].CanReview = scope.canReview
		}
	}
	s.users = out
	return nil
}

func (s *MemoryStore) loadUserRoles() (map[string][]learning.Role, error) {
	rows, err := s.db.Query(`SELECT user_id, role_code FROM user_roles ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]learning.Role{}
	for rows.Next() {
		var userID string
		var role learning.Role
		if err := rows.Scan(&userID, &role); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], role)
	}
	return out, rows.Err()
}

func (s *MemoryStore) loadUserCampusScopes() (map[string][]string, error) {
	rows, err := s.db.Query(`SELECT user_id, campus_id FROM admin_campus_scopes ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var userID, campusID string
		if err := rows.Scan(&userID, &campusID); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], campusID)
	}
	return out, rows.Err()
}

type teacherDBScope struct {
	spaces            []string
	canUploadHandout  bool
	canUploadQuestion bool
	canReview         bool
}

func (s *MemoryStore) loadTeacherScopes() (map[string]teacherDBScope, error) {
	rows, err := s.db.Query(`SELECT teacher_id, learning_space_id, can_upload_handout, can_upload_question, can_review FROM teacher_learning_space_access WHERE status = 'active' ORDER BY teacher_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]teacherDBScope{}
	for rows.Next() {
		var userID, spaceID string
		var handout, question, review int
		if err := rows.Scan(&userID, &spaceID, &handout, &question, &review); err != nil {
			return nil, err
		}
		scope := out[userID]
		scope.spaces = appendUnique(scope.spaces, spaceID)
		scope.canUploadHandout = scope.canUploadHandout || handout == 1
		scope.canUploadQuestion = scope.canUploadQuestion || question == 1
		scope.canReview = scope.canReview || review == 1
		out[userID] = scope
	}
	return out, rows.Err()
}

func (s *MemoryStore) loadPackagesFromDB() error {
	rows, err := s.db.Query(`SELECT id, name, academic_year, grade, semester, subject, phase_scope, package_type, summary, status FROM study_packages ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	packages := []learning.Package{}
	for rows.Next() {
		var item learning.Package
		if err := rows.Scan(&item.ID, &item.Name, &item.AcademicYear, &item.Grade, &item.Semester, &item.Subject, &item.PhaseScope, &item.PackageType, &item.Summary, &item.Status); err != nil {
			return err
		}
		packages = append(packages, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	spaces, err := s.loadPackageSpaces()
	if err != nil {
		return err
	}
	types, err := s.loadPackageContentTypes()
	if err != nil {
		return err
	}
	s.packages = packages
	s.packageSpaces = spaces
	s.contentTypes = types
	return nil
}

func (s *MemoryStore) loadPackageSpaces() ([]packageSpace, error) {
	rows, err := s.db.Query(`SELECT package_id, learning_space_id FROM package_spaces ORDER BY package_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []packageSpace{}
	for rows.Next() {
		var item packageSpace
		if err := rows.Scan(&item.PackageID, &item.LearningSpaceID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *MemoryStore) loadPackageContentTypes() ([]packageContentType, error) {
	rows, err := s.db.Query(`SELECT package_id, content_type FROM package_content_types ORDER BY package_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []packageContentType{}
	for rows.Next() {
		var item packageContentType
		if err := rows.Scan(&item.PackageID, &item.ContentType); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *MemoryStore) loadCoursesFromDB() error {
	rows, err := s.db.Query(`SELECT id, learning_space_id, name, subject, grade, status, chapter_count FROM courses ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Course{}
	for rows.Next() {
		var item learning.Course
		if err := rows.Scan(&item.ID, &item.LearningSpaceID, &item.Name, &item.Subject, &item.Grade, &item.Status, &item.ChapterCount); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.courses = out
	return rows.Err()
}

func (s *MemoryStore) loadMaterialsFromDB() error {
	rows, err := s.db.Query(`SELECT id, learning_space_id, course_id, title, chapter_name, material_type, owner_teacher_id, owner_teacher_name, publish_status, status, view_count, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url FROM materials ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Material{}
	for rows.Next() {
		var item learning.Material
		if err := rows.Scan(&item.ID, &item.LearningSpaceID, &item.CourseID, &item.Title, &item.Chapter, &item.Type, &item.OwnerTeacherID, &item.OwnerTeacherName, &item.PublishStatus, &item.Status, &item.ViewCount, &item.FileID, &item.FileName, &item.FileSize, &item.FileType, &item.PreviewStatus, &item.PreviewURL, &item.DownloadURL); err != nil {
			return err
		}
		item.Course = s.courseName(item.CourseID)
		out = append(out, item)
	}
	s.materials = out
	return rows.Err()
}

func (s *MemoryStore) loadHomeworkFromDB() error {
	rows, err := s.db.Query(`SELECT id, learning_space_id, course_id, title, deadline, owner_teacher_id, owner_teacher_name, publish_status, status, package_name, question_num, submitted_num, total_num, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url FROM homework_tasks ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Homework{}
	for rows.Next() {
		var item learning.Homework
		var deadline sql.NullTime
		if err := rows.Scan(&item.ID, &item.LearningSpaceID, &item.CourseID, &item.Title, &deadline, &item.OwnerTeacherID, &item.OwnerTeacherName, &item.PublishStatus, &item.Status, &item.PackageName, &item.QuestionNum, &item.SubmittedNum, &item.TotalNum, &item.FileID, &item.FileName, &item.FileSize, &item.FileType, &item.PreviewStatus, &item.PreviewURL, &item.DownloadURL); err != nil {
			return err
		}
		item.Deadline = dateString(deadline)
		item.Course = s.courseName(item.CourseID)
		if item.QuestionNum > 0 {
			item.Questions = demoQuestions(s.courseSubject(item.CourseID))
		}
		out = append(out, item)
	}
	s.homework = out
	return rows.Err()
}

func (s *MemoryStore) loadGrantsFromDB() error {
	rows, err := s.db.Query(`SELECT id, external_id, student_id, package_id, starts_at, ends_at, status FROM student_package_grants ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	grants := []packageGrant{}
	grantIDs := map[int]string{}
	for rows.Next() {
		var dbID int
		var grant packageGrant
		var startsAt, endsAt sql.NullTime
		if err := rows.Scan(&dbID, &grant.ID, &grant.StudentID, &grant.PackageID, &startsAt, &endsAt, &grant.Status); err != nil {
			return err
		}
		if grant.ID == "" {
			grant.ID = "grant-" + strconv.Itoa(dbID)
		}
		grant.StartsAt = dateString(startsAt)
		grant.EndsAt = dateString(endsAt)
		grant.EffectiveUntil = grant.EndsAt
		grantIDs[dbID] = grant.ID
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	accessRows, err := s.db.Query(`SELECT student_id, learning_space_id, package_grant_id, external_grant_id, starts_at, ends_at, status FROM student_learning_space_access ORDER BY id`)
	if err != nil {
		return err
	}
	defer accessRows.Close()
	access := []learningSpaceAccess{}
	for accessRows.Next() {
		var item learningSpaceAccess
		var dbGrantID int
		var startsAt, endsAt sql.NullTime
		if err := accessRows.Scan(&item.StudentID, &item.LearningSpaceID, &dbGrantID, &item.PackageGrantID, &startsAt, &endsAt, &item.Status); err != nil {
			return err
		}
		if item.PackageGrantID == "" {
			item.PackageGrantID = grantIDs[dbGrantID]
		}
		item.StartsAt = dateString(startsAt)
		item.EndsAt = dateString(endsAt)
		access = append(access, item)
	}
	s.grants = grants
	s.spaceAccess = access
	return accessRows.Err()
}

func (s *MemoryStore) loadFileAssetsFromDB() error {
	rows, err := s.db.Query(`SELECT id, file_name, file_size, file_type, content_type, original_path, preview_path, preview_status FROM starline_file_assets ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]learning.FileAsset{}
	for rows.Next() {
		var item learning.FileAsset
		if err := rows.Scan(&item.ID, &item.FileName, &item.FileSize, &item.FileType, &item.ContentType, &item.OriginalPath, &item.PreviewPath, &item.PreviewStatus); err != nil {
			return err
		}
		out[item.ID] = item
	}
	s.fileAssets = out
	return rows.Err()
}

func (s *MemoryStore) loadReviewsFromDB() error {
	rows, err := s.db.Query(`SELECT id, student_id, homework_id, submission_id, student_name, package_name, homework_title, system_score, teacher_comment, reward, status FROM pending_reviews ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Review{}
	for rows.Next() {
		var item learning.Review
		if err := rows.Scan(&item.ID, &item.StudentID, &item.HomeworkID, &item.SubmissionID, &item.StudentName, &item.PackageName, &item.Homework, &item.SystemScore, &item.TeacherComment, &item.Reward, &item.Status); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.reviews = out
	return rows.Err()
}

func (s *MemoryStore) loadNoticesFromDB() error {
	rows, err := s.db.Query(`SELECT id, external_id, notice_type, title, target, content, status FROM notices ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Notice{}
	for rows.Next() {
		var dbID int
		var item learning.Notice
		if err := rows.Scan(&dbID, &item.ID, &item.Type, &item.Title, &item.Target, &item.Summary, &item.Status); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = "notice-" + strconv.Itoa(dbID)
		}
		out = append(out, item)
	}
	s.notices = out
	return rows.Err()
}

func (s *MemoryStore) loadLogsFromDB() error {
	rows, err := s.db.Query(`SELECT id, external_id, operator_id, operator_name, action, target, ip, user_agent, detail, created_at FROM operation_logs ORDER BY id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.OperationLog{}
	for rows.Next() {
		var dbID int
		var item learning.OperationLog
		var createdAt sql.NullTime
		if err := rows.Scan(&dbID, &item.ID, &item.OperatorID, &item.Operator, &item.Action, &item.Target, &item.IP, &item.UserAgent, &item.Detail, &createdAt); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = "log-" + strconv.Itoa(dbID)
		}
		item.Time = dateTimeString(createdAt)
		out = append(out, item)
	}
	s.logs = out
	return rows.Err()
}

func (s *MemoryStore) loadSettingsFromDB() error {
	rows, err := s.db.Query(`SELECT setting_key, setting_value FROM system_settings`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return err
		}
		out[key] = value
	}
	s.settings = out
	return rows.Err()
}

func (s *MemoryStore) loadSubmissionsFromDB() error {
	rows, err := s.db.Query(`SELECT id, homework_id, student_id, task_title, score, teacher_comment, reward, status, created_at FROM student_submission_results ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]learning.Submission{}
	for rows.Next() {
		var item learning.Submission
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.HomeworkID, &item.StudentID, &item.TaskTitle, &item.Score, &item.TeacherComment, &item.Reward, &item.Status, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = dateTimeString(createdAt)
		out[item.ID] = item
	}
	s.submissions = out
	return rows.Err()
}

func (s *MemoryStore) loadFavoritesFromDB() error {
	rows, err := s.db.Query(`SELECT id, student_id, target_type, target_id, title, course, created_at FROM student_favorites ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]learning.Favorite{}
	for rows.Next() {
		var item learning.Favorite
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.StudentID, &item.TargetType, &item.TargetID, &item.Title, &item.Course, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = dateTimeString(createdAt)
		out[item.ID] = item
	}
	s.favorites = out
	return rows.Err()
}

func (s *MemoryStore) loadCommercialFromDB() error {
	if err := s.loadCommercialOrdersFromDB(); err != nil {
		return err
	}
	if err := s.loadCommercialPaymentsFromDB(); err != nil {
		return err
	}
	if err := s.loadCommercialRefundsFromDB(); err != nil {
		return err
	}
	if err := s.loadCommercialContractsFromDB(); err != nil {
		return err
	}
	if err := s.loadCommercialInvoicesFromDB(); err != nil {
		return err
	}
	if err := s.loadLessonConsumptionsFromDB(); err != nil {
		return err
	}
	if err := s.loadRenewalRemindersFromDB(); err != nil {
		return err
	}
	return s.loadParentNoticesFromDB()
}

func (s *MemoryStore) loadCommercialOrdersFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_no, student_id, student_name, package_id, package_name, amount_cent, paid_amount_cent, refunded_amount_cent, lesson_total, lesson_consumed, status, contract_status, invoice_status, created_at FROM commercial_orders ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.CommercialOrder{}
	for rows.Next() {
		var item learning.CommercialOrder
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.StudentID, &item.StudentName, &item.PackageID, &item.PackageName, &item.AmountCent, &item.PaidAmountCent, &item.RefundedAmountCent, &item.LessonTotal, &item.LessonConsumed, &item.Status, &item.ContractStatus, &item.InvoiceStatus, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = dateTimeString(createdAt)
		out = append(out, item)
	}
	s.commercialOrders = out
	return rows.Err()
}

func (s *MemoryStore) loadCommercialPaymentsFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, amount_cent, method, transaction_no, paid_at, status FROM commercial_payments ORDER BY paid_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.PaymentRecord{}
	for rows.Next() {
		var item learning.PaymentRecord
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.AmountCent, &item.Method, &item.TransactionNo, &at, &item.Status); err != nil {
			return err
		}
		item.PaidAt = dateTimeString(at)
		out = append(out, item)
	}
	s.payments = out
	return rows.Err()
}

func (s *MemoryStore) loadCommercialRefundsFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, amount_cent, reason, refunded_at, status FROM commercial_refunds ORDER BY refunded_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.RefundRecord{}
	for rows.Next() {
		var item learning.RefundRecord
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.AmountCent, &item.Reason, &at, &item.Status); err != nil {
			return err
		}
		item.RefundedAt = dateTimeString(at)
		out = append(out, item)
	}
	s.refunds = out
	return rows.Err()
}

func (s *MemoryStore) loadCommercialContractsFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, title, signer, signed_at, status FROM commercial_contracts ORDER BY signed_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.ContractRecord{}
	for rows.Next() {
		var item learning.ContractRecord
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.Title, &item.Signer, &at, &item.Status); err != nil {
			return err
		}
		item.SignedAt = dateTimeString(at)
		out = append(out, item)
	}
	s.contracts = out
	return rows.Err()
}

func (s *MemoryStore) loadCommercialInvoicesFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, title, tax_no, amount_cent, invoice_no, issued_at, status FROM commercial_invoices ORDER BY issued_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.InvoiceRecord{}
	for rows.Next() {
		var item learning.InvoiceRecord
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.Title, &item.TaxNo, &item.AmountCent, &item.InvoiceNo, &at, &item.Status); err != nil {
			return err
		}
		item.IssuedAt = dateTimeString(at)
		out = append(out, item)
	}
	s.invoices = out
	return rows.Err()
}

func (s *MemoryStore) loadLessonConsumptionsFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, student_id, schedule_class_id, lesson_count, consumed_at, remark FROM lesson_consumptions ORDER BY consumed_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.LessonConsumption{}
	for rows.Next() {
		var item learning.LessonConsumption
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.StudentID, &item.ScheduleClassID, &item.LessonCount, &at, &item.Remark); err != nil {
			return err
		}
		item.ConsumedAt = dateTimeString(at)
		out = append(out, item)
	}
	s.lessonConsumptions = out
	return rows.Err()
}

func (s *MemoryStore) loadRenewalRemindersFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, student_id, reason, due_at, status FROM renewal_reminders ORDER BY id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.RenewalReminder{}
	for rows.Next() {
		var item learning.RenewalReminder
		if err := rows.Scan(&item.ID, &item.OrderID, &item.StudentID, &item.Reason, &item.DueAt, &item.Status); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.renewalReminders = out
	return rows.Err()
}

func (s *MemoryStore) loadParentNoticesFromDB() error {
	rows, err := s.db.Query(`SELECT id, order_id, student_id, title, content, sent_at, status FROM parent_notices ORDER BY sent_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.ParentNotice{}
	for rows.Next() {
		var item learning.ParentNotice
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.StudentID, &item.Title, &item.Content, &at, &item.Status); err != nil {
			return err
		}
		item.SentAt = dateTimeString(at)
		out = append(out, item)
	}
	s.parentNotices = out
	return rows.Err()
}

func (s *MemoryStore) courseName(courseID string) string {
	for _, course := range s.courses {
		if course.ID == courseID {
			return course.Name
		}
	}
	return ""
}

func (s *MemoryStore) courseSubject(courseID string) string {
	for _, course := range s.courses {
		if course.ID == courseID {
			return course.Subject
		}
	}
	return ""
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableDateTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02 15:04:05", value); err != nil {
		return nil
	}
	return value
}
