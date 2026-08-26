package store

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	s.ensureDefaultSubjectMetadata()
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
	if err := s.reconcileSubjectMetadata(); err != nil {
		return err
	}
	for key, value := range defaultSettings() {
		if _, err := s.db.Exec(`INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_key=VALUES(setting_key)`, key, value); err != nil {
			return err
		}
	}
	// 清掉被取代的旧设置项，否则它们会一直躺在数据库里、一直出现在系统设置列表里，
	// 即使代码早就不读它们了（例如“套餐默认开始/结束日期”被“校历”取代之后）。
	for _, key := range retiredSettingKeys {
		if _, err := s.db.Exec(`DELETE FROM system_settings WHERE setting_key = ?`, key); err != nil {
			return err
		}
	}
	return nil
}

// reconcileSubjectMetadata 先补齐默认行，再将旧版 subjectColors JSON 中已配置
// 的展示值写入对应学科。迁移完成后删除旧设置，后续读写只经过 subjects 表。
func (s *MemoryStore) reconcileSubjectMetadata() error {
	for _, subject := range defaultSubjectMetadata() {
		if _, err := s.db.Exec(`INSERT INTO subjects (id, name, short_label, color, sort_order, status)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			short_label = IF(short_label = '', VALUES(short_label), short_label),
			color = IF(color = '', VALUES(color), color),
			sort_order = IF(sort_order = 0, VALUES(sort_order), sort_order)`,
			subject.ID, subject.Name, subject.ShortLabel, subject.Color, subject.SortOrder, subject.Status); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`UPDATE subjects SET status = '停用' WHERE id IN ('integrated-science', 'history')`); err != nil {
		return err
	}
	var raw string
	err := s.db.QueryRow(`SELECT setting_value FROM system_settings WHERE setting_key = 'subjectColors'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var entries []struct {
		Subject    string `json:"subject"`
		ShortLabel string `json:"shortLabel"`
		Color      string `json:"color"`
		SortOrder  int    `json:"sortOrder"`
	}
	if err := json.Unmarshal([]byte(raw), &entries); err == nil {
		for _, entry := range entries {
			label := strings.TrimSpace(entry.ShortLabel)
			color := strings.ToUpper(strings.TrimSpace(entry.Color))
			if strings.TrimSpace(entry.Subject) == "" || label == "" || !subjectColorPattern.MatchString(color) || entry.SortOrder < 0 {
				continue
			}
			if _, err := s.db.Exec(`UPDATE subjects SET short_label = ?, color = ?, sort_order = ? WHERE name = ?`, label, color, entry.SortOrder, strings.TrimSpace(entry.Subject)); err != nil {
				return err
			}
		}
	}
	_, err = s.db.Exec(`DELETE FROM system_settings WHERE setting_key = 'subjectColors'`)
	return err
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
			`INSERT INTO learning_spaces (id, academic_year, grade, subject, semester, phase, level, name, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE academic_year = VALUES(academic_year), grade = VALUES(grade), subject = VALUES(subject), semester = VALUES(semester), phase = VALUES(phase), level = VALUES(level), name = VALUES(name), status = VALUES(status)`,
			space.ID, space.AcademicYear, space.Grade, space.Subject, space.Semester, space.Phase, space.Level, space.Name, space.Status,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	// 旧课程矩阵中的综合科学、历史和 G4 地理只停用不删除，保住课程、资料、
	// 套餐及审计引用；新建业务只会看到当前矩阵里的启用空间。
	if _, err := tx.Exec(`UPDATE learning_spaces SET status = '停用'
		WHERE id LIKE 'space-g%' AND (subject IN ('综合科学', '历史') OR (grade = '四年级' AND subject = '地理'))`); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.loadLearningSpacesFromDB()
}
