package store

import (
	"database/sql"
	"errors"
)

// bootstrapPersistAll is startup-only. Request-time mutations use keyed delta
// persistence and must never call this full import path.
func (s *MemoryStore) bootstrapPersistAll() error {
	if s.db == nil {
		return errors.New("mysql connection is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := s.bootstrapPersistAllTx(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *MemoryStore) bootstrapPersistAllTx(tx *sql.Tx) error {
	deletes := []string{
		"DELETE FROM parent_notices",
		"DELETE FROM renewal_reminders",
		"DELETE FROM lesson_consumptions",
		"DELETE FROM commercial_invoices",
		"DELETE FROM commercial_contracts",
		"DELETE FROM commercial_refunds",
		"DELETE FROM commercial_payments",
		"DELETE FROM commercial_orders",
		"DELETE FROM student_subscriptions",
		"DELETE FROM student_favorites",
		"DELETE FROM student_score_records",
		"DELETE FROM student_submission_results",
		"DELETE FROM pending_reviews",
		"DELETE FROM question_bank_items",
		"DELETE FROM preview_jobs",
		"DELETE FROM starline_file_assets",
		"DELETE FROM schedule_class_students",
		"DELETE FROM schedule_classes",
		"DELETE FROM availability_slots",
		"DELETE FROM operation_logs",
		"DELETE FROM system_settings",
		"DELETE FROM subjects",
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
		"DELETE FROM guardian_students",
		"DELETE FROM guardians",
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
	for _, guardian := range s.guardians {
		if _, err := tx.Exec(
			`INSERT INTO guardians (id, phone, open_id, union_id, name, nickname, last_student_id, account_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			guardian.ID, guardian.Phone, guardian.OpenID, guardian.UnionID, guardian.Name, guardian.Nickname, guardian.LastStudentID, guardian.AccountStatus,
		); err != nil {
			return err
		}
	}
	for _, relation := range s.guardianStudents {
		if _, err := tx.Exec(
			`INSERT INTO guardian_students (guardian_id, student_id, relation, is_primary, status) VALUES (?, ?, ?, ?, ?)`,
			relation.GuardianID, relation.StudentID, relation.Relation, boolInt(relation.IsPrimary), relation.Status,
		); err != nil {
			return err
		}
	}
	for _, space := range s.learningSpaces {
		if _, err := tx.Exec(
			`INSERT INTO learning_spaces (id, academic_year, grade, subject, semester, phase, level, name, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			space.ID, space.AcademicYear, space.Grade, space.Subject, space.Semester, space.Phase, space.Level, space.Name, space.Status,
		); err != nil {
			return err
		}
	}
	for _, student := range s.students {
		if _, err := tx.Exec(
			`INSERT INTO students (id, name, nickname, avatar_url, grade, phone, school_name, guardian_name, official_account_open_id, account_status, remark, learning_status, streak_days, average_score, badge_count, bind_status, created_at, last_study_at, effective_until, enrollment_academic_year, enrollment_grade, bind_code, bind_code_expires_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			student.ID, student.Name, student.Nickname, student.AvatarURL, student.Grade, student.Phone, student.SchoolName, student.GuardianName, student.OfficialAccountOpenID, student.AccountStatus, student.Remark, student.LearningStatus,
			student.StreakDays, student.AverageScore, student.BadgeCount, student.BindStatus, nullableDateTime(student.CreatedAt), student.LastStudyAt, student.EffectiveUntil, student.EnrollmentAcademicYear, student.EnrollmentGrade, student.BindCode, student.BindCodeExpiresAt,
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
			`INSERT INTO study_packages (id, name, academic_year, grade, semester, subject, level, phase_scope, package_type, summary, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pkg.ID, pkg.Name, pkg.AcademicYear, pkg.Grade, pkg.Semester, pkg.Subject, pkg.Level, pkg.PhaseScope, pkg.PackageType, pkg.Summary, pkg.Status,
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
			`INSERT INTO materials (id, learning_space_id, course_id, title, chapter_name, material_type, owner_teacher_id, owner_teacher_name, publish_status, status, view_count, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			material.ID, material.LearningSpaceID, material.CourseID, material.Title, material.Chapter, material.Type, material.OwnerTeacherID,
			material.OwnerTeacherName, material.PublishStatus, material.Status, material.ViewCount, material.FileID, material.FileName,
			material.FileSize, material.FileType, material.PreviewStatus, material.PreviewURL, material.DownloadURL, material.SortOrder,
		); err != nil {
			return err
		}
	}
	for _, item := range s.homework {
		if _, err := tx.Exec(
			`INSERT INTO homework_tasks (id, learning_space_id, course_id, title, chapter_name, grade, semester, subject, question_ids_json, deadline, owner_teacher_id, owner_teacher_name, publish_status, status, package_name, question_num, submitted_num, total_num, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.LearningSpaceID, item.CourseID, item.Title, item.Chapter, item.Grade, item.Semester, item.Subject, mustJSON(item.QuestionIDs), nullableDate(item.Deadline), item.OwnerTeacherID, item.OwnerTeacherName,
			item.PublishStatus, item.Status, item.PackageName, item.QuestionNum, item.SubmittedNum, item.TotalNum, item.FileID, item.FileName,
			item.FileSize, item.FileType, item.PreviewStatus, item.PreviewURL, item.DownloadURL,
		); err != nil {
			return err
		}
	}
	for _, item := range s.questionBank {
		if _, err := tx.Exec(
			`INSERT INTO question_bank_items (id, title, grade, semester, subject, question_type, stem, options_json, answer, answers_json, score, status, owner_teacher_id, owner_teacher_name, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.Title, item.Grade, item.Semester, item.Subject, item.Type, item.Stem, mustJSON(item.Options), item.Answer, mustJSON(item.Answers),
			item.Score, item.Status, item.OwnerTeacherID, item.OwnerTeacherName, nullableDateTime(item.CreatedAt), nullableDateTime(item.UpdatedAt),
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
			`INSERT INTO starline_file_assets (id, file_name, file_size, file_type, content_type, original_path, preview_path, preview_page_dir, preview_page_count, preview_status, preview_error)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			asset.ID, asset.FileName, asset.FileSize, asset.FileType, asset.ContentType, asset.OriginalPath, asset.PreviewPath, asset.PreviewPageDir, asset.PreviewPageCount, asset.PreviewStatus, asset.PreviewError,
		); err != nil {
			return err
		}
	}
	for _, job := range s.previewJobs {
		if _, err := tx.Exec(
			`INSERT INTO preview_jobs (id, file_id, status, attempt_count, error_message, created_at, started_at, finished_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			job.ID, job.FileID, job.Status, job.AttemptCount, job.ErrorMessage, nullableDateTime(job.CreatedAt), nullableDateTime(job.StartedAt), nullableDateTime(job.FinishedAt),
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
			`INSERT INTO schedule_classes (id, name, course_id, course_name, teacher_id, teacher_name, campus_id, room_name, class_type, capacity, duration_minutes, day_of_week, start_time, end_time, start_date, end_date, expected_student_count, reservation_note, academic_year, semester, status, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.Name, item.CourseID, item.CourseName, item.TeacherID, item.TeacherName, item.CampusID, item.RoomName, item.ClassType, item.Capacity, item.DurationMinutes,
			item.DayOfWeek, item.StartTime, item.EndTime, nullableDate(item.StartDate), nullableDate(item.EndDate), item.ExpectedStudentCount, item.ReservationNote, item.AcademicYear, item.Semester, item.Status, nullableDateTime(item.CreatedAt),
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
		if _, err := tx.Exec(
			`INSERT INTO parent_notices (id, order_id, student_id, title, content, sent_at, status, notice_id, channel, failure_reason) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.OrderID, item.StudentID, item.Title, item.Content, nullableDateTime(item.SentAt), item.Status, item.NoticeID, item.Channel, item.FailureReason,
		); err != nil {
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
			`INSERT INTO notices (external_id, notice_type, title, target, content, channel, recipient_open_id, status, failure_reason, related_type, related_id, retry_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			notice.ID, notice.Type, notice.Title, notice.Target, notice.Summary, notice.Channel, notice.RecipientOpenID, notice.Status, notice.FailureReason, notice.RelatedType, notice.RelatedID, notice.RetryCount,
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
			`INSERT INTO student_submission_results (id, homework_id, student_id, task_title, score, objective_score, final_score, teacher_comment, reward, status, answers_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			submission.ID, submission.HomeworkID, submission.StudentID, submission.TaskTitle, submission.Score, submission.ObjectiveScore, submission.FinalScore,
			submission.TeacherComment, submission.Reward, submission.Status, mustJSON(submission.Answers), nullableDateTime(submission.CreatedAt),
		); err != nil {
			return err
		}
	}
	for _, item := range s.scoreRecords {
		if _, err := tx.Exec(
			`INSERT INTO student_score_records (id, student_id, subject, exam_type, exam_name, exam_date, score, full_score, average_score, teacher_comment, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.StudentID, item.Subject, firstNonEmpty(item.ExamType, "阶段测评"), item.ExamName, nullableDate(item.ExamDate), item.Score, item.FullScore, item.AverageScore,
			item.TeacherComment, item.CreatedBy, nullableDateTime(item.CreatedAt), nullableDateTime(item.UpdatedAt),
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
	for _, preference := range s.subscriptionPreferences {
		if _, err := tx.Exec(
			`INSERT INTO student_subscriptions (student_id, enabled, template_ids_json, updated_at) VALUES (?, ?, ?, ?)`,
			preference.StudentID, preference.Enabled, mustJSON(preference.TemplateIDs), nullableDateTime(preference.UpdatedAt),
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
	for _, subject := range s.subjects {
		if _, err := tx.Exec(`INSERT INTO subjects (id, name, short_label, color, sort_order, status) VALUES (?, ?, ?, ?, ?, ?)`, subject.ID, subject.Name, subject.ShortLabel, subject.Color, subject.SortOrder, subject.Status); err != nil {
			return err
		}
	}
	return nil
}
