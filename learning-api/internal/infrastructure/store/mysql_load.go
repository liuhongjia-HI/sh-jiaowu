package store

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) loadAllFromDatabase() error {
	loaders := []func() error{
		s.loadLearningSpacesFromDB,
		s.loadStudentsFromDB,
		s.loadUsersFromDB,
		s.loadGuardiansFromDB,
		s.loadGuardianStudentsFromDB,
		s.loadPackagesFromDB,
		s.loadCoursesFromDB,
		s.loadQuestionBankFromDB,
		s.loadMaterialsFromDB,
		s.loadHomeworkFromDB,
		s.loadGrantsFromDB,
		s.loadFileAssetsFromDB,
		s.loadPreviewJobsFromDB,
		s.loadReviewsFromDB,
		s.loadNoticesFromDB,
		s.loadLogsFromDB,
		s.loadSettingsFromDB,
		s.loadSubmissionsFromDB,
		s.loadScoreRecordsFromDB,
		s.loadFavoritesFromDB,
		s.loadSubscriptionPreferencesFromDB,
		s.loadCommercialFromDB,
		s.loadBannersFromDB,
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
	rows, err := s.db.Query(`SELECT id, name, nickname, avatar_url, grade, phone, school_name, guardian_name, official_account_open_id, account_status, remark, learning_status, streak_days, average_score, badge_count, bind_status, last_study_at, effective_until, enrollment_academic_year, enrollment_grade, bind_code, bind_code_expires_at FROM students ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Student{}
	for rows.Next() {
		var item learning.Student
		if err := rows.Scan(&item.ID, &item.Name, &item.Nickname, &item.AvatarURL, &item.Grade, &item.Phone, &item.SchoolName, &item.GuardianName, &item.OfficialAccountOpenID, &item.AccountStatus, &item.Remark, &item.LearningStatus, &item.StreakDays, &item.AverageScore, &item.BadgeCount, &item.BindStatus, &item.LastStudyAt, &item.EffectiveUntil, &item.EnrollmentAcademicYear, &item.EnrollmentGrade, &item.BindCode, &item.BindCodeExpiresAt); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.students = out
	return rows.Err()
}

func (s *MemoryStore) loadGuardiansFromDB() error {
	rows, err := s.db.Query(`SELECT id, phone, open_id, union_id, name, nickname, last_student_id, account_status FROM guardians ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Guardian{}
	for rows.Next() {
		var item learning.Guardian
		if err := rows.Scan(&item.ID, &item.Phone, &item.OpenID, &item.UnionID, &item.Name, &item.Nickname, &item.LastStudentID, &item.AccountStatus); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.guardians = out
	return rows.Err()
}

func (s *MemoryStore) loadGuardianStudentsFromDB() error {
	rows, err := s.db.Query(`SELECT guardian_id, student_id, relation, is_primary, status FROM guardian_students ORDER BY guardian_id, student_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.GuardianStudent{}
	for rows.Next() {
		var item learning.GuardianStudent
		var isPrimary int
		if err := rows.Scan(&item.GuardianID, &item.StudentID, &item.Relation, &isPrimary, &item.Status); err != nil {
			return err
		}
		item.IsPrimary = isPrimary == 1
		out = append(out, item)
	}
	s.guardianStudents = out
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
		item.Status = normalizeMaterialStatus(item.Status)
		item.PublishStatus = publishStatus(item.Status)
		item.Course = s.courseName(item.CourseID)
		out = append(out, item)
	}
	s.materials = out
	return rows.Err()
}

func (s *MemoryStore) loadQuestionBankFromDB() error {
	rows, err := s.db.Query(`SELECT id, title, grade, semester, subject, question_type, stem, options_json, answer, answers_json, score, status, owner_teacher_id, owner_teacher_name, created_at, updated_at FROM question_bank_items ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.QuestionBankItem{}
	for rows.Next() {
		var item learning.QuestionBankItem
		var optionsJSON, answersJSON string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Title, &item.Grade, &item.Semester, &item.Subject, &item.Type, &item.Stem, &optionsJSON, &item.Answer, &answersJSON, &item.Score, &item.Status, &item.OwnerTeacherID, &item.OwnerTeacherName, &createdAt, &updatedAt); err != nil {
			return err
		}
		if strings.TrimSpace(item.Title) == "" {
			item.Title = shortQuestionTitle(item.Stem)
		}
		item.Options = parseStringSliceJSON(optionsJSON)
		item.Answers = parseStringSliceJSON(answersJSON)
		item.CreatedAt = dateTimeString(createdAt)
		item.UpdatedAt = dateTimeString(updatedAt)
		out = append(out, item)
	}
	s.questionBank = out
	return rows.Err()
}

func (s *MemoryStore) loadHomeworkFromDB() error {
	rows, err := s.db.Query(`SELECT id, learning_space_id, course_id, title, grade, semester, subject, question_ids_json, deadline, owner_teacher_id, owner_teacher_name, publish_status, status, package_name, question_num, submitted_num, total_num, file_id, file_name, file_size, file_type, preview_status, preview_url, download_url FROM homework_tasks ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Homework{}
	for rows.Next() {
		var item learning.Homework
		var questionIDsJSON string
		var deadline sql.NullTime
		if err := rows.Scan(&item.ID, &item.LearningSpaceID, &item.CourseID, &item.Title, &item.Grade, &item.Semester, &item.Subject, &questionIDsJSON, &deadline, &item.OwnerTeacherID, &item.OwnerTeacherName, &item.PublishStatus, &item.Status, &item.PackageName, &item.QuestionNum, &item.SubmittedNum, &item.TotalNum, &item.FileID, &item.FileName, &item.FileSize, &item.FileType, &item.PreviewStatus, &item.PreviewURL, &item.DownloadURL); err != nil {
			return err
		}
		item.Deadline = dateString(deadline)
		item.Course = s.courseName(item.CourseID)
		if item.Grade == "" || item.Semester == "" || item.Subject == "" {
			if space, ok := s.findLearningSpace(item.LearningSpaceID); ok {
				item.Grade = space.Grade
				item.Semester = space.Semester
				item.Subject = space.Subject
			}
		}
		item.QuestionIDs = parseStringSliceJSON(questionIDsJSON)
		for _, id := range item.QuestionIDs {
			if bankItem, ok := s.findQuestionBankItem(id); ok {
				item.Questions = append(item.Questions, bankItemQuestion(bankItem))
			}
		}
		if len(item.Questions) == 0 && item.QuestionNum > 0 {
			item.Questions = s.ensureDemoQuestionBank(item.Grade, item.Semester, item.Subject)
			item.QuestionIDs = questionIDs(item.Questions)
		}
		item.QuestionNum = len(item.Questions)
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
	rows, err := s.db.Query(`SELECT id, file_name, file_size, file_type, content_type, original_path, preview_path, preview_page_dir, preview_page_count, preview_status, preview_error FROM starline_file_assets ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]learning.FileAsset{}
	for rows.Next() {
		var item learning.FileAsset
		if err := rows.Scan(&item.ID, &item.FileName, &item.FileSize, &item.FileType, &item.ContentType, &item.OriginalPath, &item.PreviewPath, &item.PreviewPageDir, &item.PreviewPageCount, &item.PreviewStatus, &item.PreviewError); err != nil {
			return err
		}
		out[item.ID] = item
	}
	s.fileAssets = out
	return rows.Err()
}

func (s *MemoryStore) loadPreviewJobsFromDB() error {
	rows, err := s.db.Query(`SELECT id, file_id, status, attempt_count, error_message, created_at, started_at, finished_at FROM preview_jobs ORDER BY created_at, id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.PreviewJob{}
	for rows.Next() {
		var item learning.PreviewJob
		var createdAt time.Time
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.FileID, &item.Status, &item.AttemptCount, &item.ErrorMessage, &createdAt, &startedAt, &finishedAt); err != nil {
			return err
		}
		item.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		item.StartedAt = dateTimeString(startedAt)
		item.FinishedAt = dateTimeString(finishedAt)
		out = append(out, item)
	}
	s.previewJobs = out
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
	rows, err := s.db.Query(`SELECT id, external_id, notice_type, title, target, content, channel, recipient_open_id, status, failure_reason, related_type, related_id, retry_count FROM notices ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Notice{}
	for rows.Next() {
		var dbID int
		var item learning.Notice
		if err := rows.Scan(&dbID, &item.ID, &item.Type, &item.Title, &item.Target, &item.Summary, &item.Channel, &item.RecipientOpenID, &item.Status, &item.FailureReason, &item.RelatedType, &item.RelatedID, &item.RetryCount); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = "notice-" + strconv.Itoa(dbID)
		}
		out = append(out, item)
	}
	s.notices = out
	if err := rows.Err(); err != nil {
		return err
	}
	s.restorePendingNoticeDeliveries()
	return nil
}

func (s *MemoryStore) loadBannersFromDB() error {
	rows, err := s.db.Query(`SELECT id, image_url, title, link_type, link_value, sort_order, starts_at, ends_at, enabled, created_at FROM banners ORDER BY sort_order, id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.Banner{}
	for rows.Next() {
		var item learning.Banner
		if err := rows.Scan(&item.ID, &item.ImageURL, &item.Title, &item.LinkType, &item.LinkValue, &item.SortOrder, &item.StartsAt, &item.EndsAt, &item.Enabled, &item.CreatedAt); err != nil {
			return err
		}
		out = append(out, item)
	}
	s.banners = out
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
	rows, err := s.db.Query(`SELECT id, homework_id, student_id, task_title, score, objective_score, final_score, teacher_comment, reward, status, answers_json, created_at FROM student_submission_results ORDER BY created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]learning.Submission{}
	for rows.Next() {
		var item learning.Submission
		var answersJSON string
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.HomeworkID, &item.StudentID, &item.TaskTitle, &item.Score, &item.ObjectiveScore, &item.FinalScore, &item.TeacherComment, &item.Reward, &item.Status, &answersJSON, &createdAt); err != nil {
			return err
		}
		item.Answers = parseSubmissionAnswersJSON(answersJSON)
		item.CreatedAt = dateTimeString(createdAt)
		out[item.ID] = item
	}
	s.submissions = out
	return rows.Err()
}

func (s *MemoryStore) loadScoreRecordsFromDB() error {
	rows, err := s.db.Query(`SELECT id, student_id, subject, exam_type, exam_name, exam_date, score, full_score, average_score, teacher_comment, created_by, created_at, updated_at FROM student_score_records ORDER BY exam_date DESC, created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.StudentScoreRecord{}
	for rows.Next() {
		var item learning.StudentScoreRecord
		var examDate, createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.StudentID, &item.Subject, &item.ExamType, &item.ExamName, &examDate, &item.Score, &item.FullScore, &item.AverageScore, &item.TeacherComment, &item.CreatedBy, &createdAt, &updatedAt); err != nil {
			return err
		}
		if item.ExamType == "" {
			item.ExamType = "阶段测评"
		}
		item.ExamDate = dateString(examDate)
		item.CreatedAt = dateTimeString(createdAt)
		item.UpdatedAt = dateTimeString(updatedAt)
		out = append(out, item)
	}
	s.scoreRecords = out
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

func (s *MemoryStore) loadSubscriptionPreferencesFromDB() error {
	rows, err := s.db.Query(`SELECT student_id, enabled, template_ids_json, updated_at FROM student_subscriptions`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := map[string]learning.StudentSubscriptionPreference{}
	for rows.Next() {
		var item learning.StudentSubscriptionPreference
		var updatedAt sql.NullTime
		var templateJSON string
		if err := rows.Scan(&item.StudentID, &item.Enabled, &templateJSON, &updatedAt); err != nil {
			return err
		}
		item.TemplateIDs = parseStringSliceJSON(templateJSON)
		item.UpdatedAt = dateTimeString(updatedAt)
		out[item.StudentID] = item
	}
	s.subscriptionPreferences = out
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
	rows, err := s.db.Query(`SELECT id, order_id, student_id, title, content, sent_at, status, notice_id, channel, failure_reason FROM parent_notices ORDER BY sent_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []learning.ParentNotice{}
	for rows.Next() {
		var item learning.ParentNotice
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.StudentID, &item.Title, &item.Content, &at, &item.Status, &item.NoticeID, &item.Channel, &item.FailureReason); err != nil {
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
