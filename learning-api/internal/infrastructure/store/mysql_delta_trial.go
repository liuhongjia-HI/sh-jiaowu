package store

func trialRows(s *MemoryStore) []persistenceRow {
	rows := make([]persistenceRow, 0, len(s.trials))
	for _, trial := range s.trials {
		rows = append(rows, simpleRow("student_trial_records", "external_id", trial.ID,
			`INSERT INTO student_trial_records (external_id, student_id, academic_year, package_id, starts_at, ends_at, status, converted_package_id, converted_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE student_id=VALUES(student_id), academic_year=VALUES(academic_year), package_id=VALUES(package_id), starts_at=VALUES(starts_at), ends_at=VALUES(ends_at), status=VALUES(status), converted_package_id=VALUES(converted_package_id), converted_at=VALUES(converted_at)`,
			trial.ID, trial.StudentID, trial.AcademicYear, trial.PackageID, nullableDate(trial.StartsAt), nullableDate(trial.EndsAt), trial.Status, trial.ConvertedPackageID, nullableDateTime(trial.ConvertedAt)))
	}
	return rows
}
