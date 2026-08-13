package store

import (
	"database/sql"
	"encoding/json"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

func mustJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(bytes)
}

func parseStringSliceJSON(value string) []string {
	out := []string{}
	if strings.TrimSpace(value) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(value), &out)
	return out
}

func parseSubmissionAnswersJSON(value string) []learning.SubmissionAnswer {
	out := []learning.SubmissionAnswer{}
	if strings.TrimSpace(value) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(value), &out)
	return out
}

func (s *MemoryStore) connectDatabaseUnlocked(dsn string) error {
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
	if err := s.reconcileDefaultSettings(); err != nil {
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
		if err := s.bootstrapPersistAll(); err != nil {
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
	if err := s.reconcileBaseLearningSpaces(); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	return nil
}

func (s *MemoryStore) reconcileDefaultSettings() error {
	for key, value := range defaultSettings() {
		if _, err := s.db.Exec(`INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_key=VALUES(setting_key)`, key, value); err != nil {
			return err
		}
	}
	return nil
}

// reconcileBaseLearningSpaces 补齐当前学年的系统学习空间。
// 只处理固定的 space-g* 系统 ID，不删除现有课程、套餐或客户创建的数据。
func (s *MemoryStore) reconcileBaseLearningSpaces() error {
	if s.db == nil {
		return nil
	}
	academicYear := currentAcademicYear()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, space := range baseLearningSpaces(academicYear) {
		if _, err := tx.Exec(
			`INSERT INTO learning_spaces (id, academic_year, grade, subject, semester, phase, name, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE academic_year = VALUES(academic_year), grade = VALUES(grade), subject = VALUES(subject), semester = VALUES(semester), phase = VALUES(phase), name = VALUES(name), status = VALUES(status)`,
			space.ID, space.AcademicYear, space.Grade, space.Subject, space.Semester, space.Phase, space.Name, space.Status,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE system_settings SET setting_value = ? WHERE setting_key = 'academicYear' AND setting_value = '2025.2026学年'`, academicYear); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.settings["academicYear"] = academicYear
	return s.loadLearningSpacesFromDB()
}
