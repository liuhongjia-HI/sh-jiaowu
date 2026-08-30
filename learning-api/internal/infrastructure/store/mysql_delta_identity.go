package store

func identityRows(s *MemoryStore) []persistenceRow {
	rows := make([]persistenceRow, 0, len(s.students)+len(s.users)*3+len(s.learningSpaces)+len(s.guardians)+len(s.guardianStudents)+len(s.tutoringAssignments))
	for _, item := range s.tutoringAssignments {
		rows = append(rows, simpleRow("student_tutoring_assignments", "id", item.ID,
			`INSERT INTO student_tutoring_assignments (id, student_id, teacher_id, teacher_name, campus_id, academic_year, grade_snapshot, subject_id, subject_name, level_code, assignment_role, status, source_type, source_id, starts_at, ends_at, ended_reason, assigned_by, ended_by, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE teacher_id=VALUES(teacher_id), teacher_name=VALUES(teacher_name), campus_id=VALUES(campus_id), academic_year=VALUES(academic_year), grade_snapshot=VALUES(grade_snapshot), subject_id=VALUES(subject_id), subject_name=VALUES(subject_name), level_code=VALUES(level_code), assignment_role=VALUES(assignment_role), status=VALUES(status), source_type=VALUES(source_type), source_id=VALUES(source_id), starts_at=VALUES(starts_at), ends_at=VALUES(ends_at), ended_reason=VALUES(ended_reason), assigned_by=VALUES(assigned_by), ended_by=VALUES(ended_by), version=VALUES(version), updated_at=VALUES(updated_at)`,
			item.ID, item.StudentID, item.TeacherID, item.TeacherName, item.CampusID, item.AcademicYear, item.GradeSnapshot, item.SubjectID, item.SubjectName, item.LevelCode, item.Role, item.Status, item.SourceType, item.SourceID, nullableDate(item.StartsAt), nullableDate(item.EndsAt), item.EndedReason, item.AssignedBy, item.EndedBy, item.Version, nullableDateTime(item.CreatedAt), nullableDateTime(item.UpdatedAt)))
	}
	for _, guardian := range s.guardians {
		rows = append(rows, simpleRow("guardians", "id", guardian.ID,
			`INSERT INTO guardians (id, phone, open_id, union_id, name, nickname, last_student_id, account_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE phone=VALUES(phone), open_id=VALUES(open_id), union_id=VALUES(union_id), name=VALUES(name), nickname=VALUES(nickname), last_student_id=VALUES(last_student_id), account_status=VALUES(account_status)`,
			guardian.ID, guardian.Phone, guardian.OpenID, guardian.UnionID, guardian.Name, guardian.Nickname, guardian.LastStudentID, guardian.AccountStatus))
	}
	for _, relation := range s.guardianStudents {
		rows = append(rows, relationRow("guardian_students", []string{"guardian_id", "student_id"}, []any{relation.GuardianID, relation.StudentID},
			`INSERT INTO guardian_students (guardian_id, student_id, relation, is_primary, status) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE relation=VALUES(relation), is_primary=VALUES(is_primary), status=VALUES(status)`,
			relation.GuardianID, relation.StudentID, relation.Relation, boolInt(relation.IsPrimary), relation.Status))
	}
	for _, space := range s.learningSpaces {
		rows = append(rows, simpleRow("learning_spaces", "id", space.ID,
			`INSERT INTO learning_spaces (id, academic_year, grade, subject, semester, phase, level, name, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE academic_year=VALUES(academic_year), grade=VALUES(grade), subject=VALUES(subject), semester=VALUES(semester), phase=VALUES(phase), level=VALUES(level), name=VALUES(name), status=VALUES(status)`,
			space.ID, space.AcademicYear, space.Grade, space.Subject, space.Semester, space.Phase, space.Level, space.Name, space.Status))
	}
	for _, student := range s.students {
		rows = append(rows, simpleRow("students", "id", student.ID,
			`INSERT INTO students (id, name, nickname, avatar_url, grade, phone, school_name, guardian_name, official_account_open_id, account_status, registration_source, remark, learning_status, streak_days, average_score, badge_count, bind_status, created_at, last_study_at, effective_until, enrollment_academic_year, enrollment_grade, bind_code, bind_code_expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name=VALUES(name), nickname=VALUES(nickname), avatar_url=VALUES(avatar_url), grade=VALUES(grade), phone=VALUES(phone), school_name=VALUES(school_name), guardian_name=VALUES(guardian_name), official_account_open_id=VALUES(official_account_open_id), account_status=VALUES(account_status), registration_source=VALUES(registration_source), remark=VALUES(remark), learning_status=VALUES(learning_status), streak_days=VALUES(streak_days), average_score=VALUES(average_score), badge_count=VALUES(badge_count), bind_status=VALUES(bind_status), last_study_at=VALUES(last_study_at), effective_until=VALUES(effective_until), enrollment_academic_year=VALUES(enrollment_academic_year), enrollment_grade=VALUES(enrollment_grade), bind_code=VALUES(bind_code), bind_code_expires_at=VALUES(bind_code_expires_at)`,
			student.ID, student.Name, student.Nickname, student.AvatarURL, student.Grade, student.Phone, student.SchoolName, student.GuardianName, student.OfficialAccountOpenID, student.AccountStatus, student.RegistrationSource, student.Remark, student.LearningStatus, student.StreakDays, student.AverageScore, student.BadgeCount, student.BindStatus, nullableDateTime(student.CreatedAt), student.LastStudyAt, student.EffectiveUntil, student.EnrollmentAcademicYear, student.EnrollmentGrade, student.BindCode, student.BindCodeExpiresAt))
	}
	for _, user := range s.users {
		rows = append(rows, simpleRow("users", "id", user.ID,
			`INSERT INTO users (id, name, phone, open_id, union_id, account_status, remark, student_id, campus_id, password_hash, must_change_password, token_version) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name=VALUES(name), phone=VALUES(phone), open_id=VALUES(open_id), union_id=VALUES(union_id), account_status=VALUES(account_status), remark=VALUES(remark), student_id=VALUES(student_id), campus_id=VALUES(campus_id), password_hash=VALUES(password_hash), must_change_password=VALUES(must_change_password), token_version=VALUES(token_version)`,
			user.ID, user.Name, user.Phone, user.OpenID, user.UnionID, user.AccountStatus, user.Remark, user.StudentID, user.CampusID, user.PasswordHash, boolInt(user.MustChangePassword), user.TokenVersion))
		for _, role := range user.Roles {
			rows = append(rows, relationRow("user_roles", []string{"user_id", "role_code"}, []any{user.ID, role},
				`INSERT INTO user_roles (user_id, role_code) VALUES (?, ?) ON DUPLICATE KEY UPDATE role_code=VALUES(role_code)`, user.ID, role))
		}
		for _, campusID := range user.CampusScopes {
			rows = append(rows, relationRow("admin_campus_scopes", []string{"user_id", "campus_id"}, []any{user.ID, campusID},
				`INSERT INTO admin_campus_scopes (user_id, campus_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE campus_id=VALUES(campus_id)`, user.ID, campusID))
		}
		for _, spaceID := range user.LearningSpaceIDs {
			rows = append(rows, relationRow("teacher_learning_space_access", []string{"teacher_id", "learning_space_id"}, []any{user.ID, spaceID},
				`INSERT INTO teacher_learning_space_access (teacher_id, learning_space_id, can_view, can_upload_handout, can_upload_question, can_review, can_manage_content, status) VALUES (?, ?, 1, ?, ?, ?, 1, 'active') ON DUPLICATE KEY UPDATE can_view=1, can_upload_handout=VALUES(can_upload_handout), can_upload_question=VALUES(can_upload_question), can_review=VALUES(can_review), can_manage_content=1, status='active'`,
				user.ID, spaceID, boolInt(user.CanUploadHandout), boolInt(user.CanUploadQuestion), boolInt(user.CanReview)))
		}
	}
	return rows
}
