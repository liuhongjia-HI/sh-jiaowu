package store

import "database/sql"

import "reflect"

func packageRows(s *MemoryStore) []persistenceRow {
	rows := make([]persistenceRow, 0, len(s.packages)+len(s.packageSpaces)+len(s.contentTypes))
	for _, pkg := range s.packages {
		rows = append(rows, simpleRow("study_packages", "id", pkg.ID,
			`INSERT INTO study_packages (id, name, academic_year, grade, semester, subject, phase_scope, package_type, summary, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE name=VALUES(name), academic_year=VALUES(academic_year), grade=VALUES(grade), semester=VALUES(semester), subject=VALUES(subject), phase_scope=VALUES(phase_scope), package_type=VALUES(package_type), summary=VALUES(summary), status=VALUES(status)`,
			pkg.ID, pkg.Name, pkg.AcademicYear, pkg.Grade, pkg.Semester, pkg.Subject, pkg.PhaseScope, pkg.PackageType, pkg.Summary, pkg.Status))
	}
	for _, relation := range s.packageSpaces {
		rows = append(rows, relationRow("package_spaces", []string{"package_id", "learning_space_id"}, []any{relation.PackageID, relation.LearningSpaceID},
			`INSERT INTO package_spaces (package_id, learning_space_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE learning_space_id=VALUES(learning_space_id)`, relation.PackageID, relation.LearningSpaceID))
	}
	for _, relation := range s.contentTypes {
		rows = append(rows, relationRow("package_content_types", []string{"package_id", "content_type"}, []any{relation.PackageID, relation.ContentType},
			`INSERT INTO package_content_types (package_id, content_type) VALUES (?, ?) ON DUPLICATE KEY UPDATE content_type=VALUES(content_type)`, relation.PackageID, relation.ContentType))
	}
	return rows
}

func syncGrantPersistence(tx *sql.Tx, before, after *MemoryStore) error {
	if reflect.DeepEqual(before.grants, after.grants) && reflect.DeepEqual(before.spaceAccess, after.spaceAccess) {
		return nil
	}
	beforeGrants := make(map[string]packageGrant, len(before.grants))
	afterGrants := make(map[string]packageGrant, len(after.grants))
	for _, grant := range before.grants {
		beforeGrants[grant.ID] = grant
	}
	for _, grant := range after.grants {
		afterGrants[grant.ID] = grant
	}
	for id, grant := range beforeGrants {
		if _, ok := afterGrants[id]; ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM student_learning_space_access WHERE external_grant_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM student_package_grants WHERE student_id = ? AND package_id = ?`, grant.StudentID, grant.PackageID); err != nil {
			return err
		}
	}
	grantDBIDs := make(map[string]int64, len(after.grants))
	for _, grant := range after.grants {
		old, existed := beforeGrants[grant.ID]
		if !existed || old != grant {
			if _, err := tx.Exec(
				`INSERT INTO student_package_grants (external_id, student_id, package_id, starts_at, ends_at, status, operator_id, operator_name) VALUES (?, ?, ?, ?, ?, ?, '', '') ON DUPLICATE KEY UPDATE external_id=VALUES(external_id), starts_at=VALUES(starts_at), ends_at=VALUES(ends_at), status=VALUES(status)`,
				grant.ID, grant.StudentID, grant.PackageID, nullableDate(grant.StartsAt), nullableDate(grantEndsAt(grant)), grant.Status,
			); err != nil {
				return err
			}
		}
		var dbID int64
		if err := tx.QueryRow(`SELECT id FROM student_package_grants WHERE student_id = ? AND package_id = ?`, grant.StudentID, grant.PackageID).Scan(&dbID); err != nil {
			return err
		}
		grantDBIDs[grant.ID] = dbID
	}
	beforeAccess := make(map[string]learningSpaceAccess, len(before.spaceAccess))
	afterAccess := make(map[string]learningSpaceAccess, len(after.spaceAccess))
	for _, access := range before.spaceAccess {
		beforeAccess[rowKey("access", access.StudentID, access.LearningSpaceID, access.PackageGrantID)] = access
	}
	for _, access := range after.spaceAccess {
		afterAccess[rowKey("access", access.StudentID, access.LearningSpaceID, access.PackageGrantID)] = access
	}
	for key, access := range beforeAccess {
		if _, ok := afterAccess[key]; ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM student_learning_space_access WHERE student_id = ? AND learning_space_id = ? AND external_grant_id = ?`, access.StudentID, access.LearningSpaceID, access.PackageGrantID); err != nil {
			return err
		}
	}
	for key, access := range afterAccess {
		old, existed := beforeAccess[key]
		if existed && old == access {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO student_learning_space_access (student_id, learning_space_id, package_grant_id, external_grant_id, starts_at, ends_at, status) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE external_grant_id=VALUES(external_grant_id), starts_at=VALUES(starts_at), ends_at=VALUES(ends_at), status=VALUES(status)`,
			access.StudentID, access.LearningSpaceID, grantDBIDs[access.PackageGrantID], access.PackageGrantID, nullableDate(access.StartsAt), nullableDate(access.EndsAt), access.Status,
		); err != nil {
			return err
		}
	}
	return nil
}
