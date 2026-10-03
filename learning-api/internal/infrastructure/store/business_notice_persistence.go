package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"starline/learning-api/internal/domain/learning"
)

func businessNoticeRows(s *MemoryStore) []persistenceRow {
	rows := []persistenceRow{}
	for _, event := range s.businessNoticeEvents {
		rows = append(rows, simpleRow("business_notice_events", "id", event.ID, `INSERT INTO business_notice_events (id, student_id, kind, payload) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE payload=VALUES(payload)`, event.ID, event.StudentID, event.Kind, mustJSON(event)))
	}
	for _, task := range s.businessNoticeTasks {
		rows = append(rows, simpleRow("business_notice_tasks", "id", task.ID, `INSERT INTO business_notice_tasks (id, event_id, student_id, recipient_key, status, due_at, payload) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE recipient_key=VALUES(recipient_key), status=VALUES(status), due_at=VALUES(due_at), payload=VALUES(payload)`, task.ID, task.EventID, task.StudentID, businessNoticeHash(task.OpenID, task.GuardianID), task.Status, businessSQLTime(task.DueAt), mustJSON(task)))
	}
	for id, snapshot := range s.businessScheduleSnapshots {
		rows = append(rows, simpleRow("business_schedule_snapshots", "id", id, `INSERT INTO business_schedule_snapshots (id, payload) VALUES (?, ?) ON DUPLICATE KEY UPDATE payload=VALUES(payload)`, id, mustJSON(snapshot)))
	}
	for _, receipt := range s.businessNoticeReceipts {
		rows = append(rows, simpleRow("business_notice_receipts", "id", receipt.MessageID, `INSERT INTO business_notice_receipts (id, payload) VALUES (?, ?) ON DUPLICATE KEY UPDATE payload=VALUES(payload)`, receipt.MessageID, mustJSON(receipt)))
	}
	return rows
}

func (s *MemoryStore) ensureBusinessNoticeSchema() error {
	if err := s.ensureColumn("student_submission_results", "request_id", "VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL"); err != nil {
		return err
	}

	// Request IDs are opaque and case-sensitive, unlike ordinary name columns.
	var collation string
	if err := s.db.QueryRow("SELECT collation_name FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='student_submission_results' AND column_name='request_id'").Scan(&collation); err != nil {
		return err
	}
	if collation != "utf8mb4_bin" {
		if _, err := s.db.Exec("ALTER TABLE student_submission_results MODIFY COLUMN request_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL"); err != nil {
			return err
		}
	}

	if err := s.ensureUniqueIndex("student_submission_results", "uk_submission_request", "student_id, request_id"); err != nil {
		return err
	}
	if err := s.ensureColumn("official_message_recipients", "delivery_json", "TEXT NULL"); err != nil {
		return err
	}
	for _, statement := range businessNoticeSchema {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

var businessNoticeSchema = []string{
	`CREATE TABLE IF NOT EXISTS business_notice_events (id VARCHAR(64) PRIMARY KEY, student_id VARCHAR(64) NOT NULL, kind VARCHAR(40) NOT NULL, payload LONGTEXT NOT NULL, KEY idx_business_notice_student (student_id, kind)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS business_notice_tasks (id VARCHAR(64) PRIMARY KEY, event_id VARCHAR(64) NOT NULL, student_id VARCHAR(64) NOT NULL, recipient_key VARCHAR(64) NOT NULL, status VARCHAR(32) NOT NULL, due_at DATETIME NULL, payload LONGTEXT NOT NULL, UNIQUE KEY uk_business_notice_recipient (event_id, student_id, recipient_key), KEY idx_business_notice_due (status, due_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS business_schedule_snapshots (id VARCHAR(64) PRIMARY KEY, payload LONGTEXT NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	`CREATE TABLE IF NOT EXISTS business_notice_receipts (id VARCHAR(64) PRIMARY KEY, payload LONGTEXT NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
}

func (s *MemoryStore) loadBusinessNoticesFromDB() error {
	load := func(table string, accept func(string, []byte) error) error {
		rows, err := s.db.Query("SELECT id, payload FROM " + table + " ORDER BY id")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var payload []byte
			if err := rows.Scan(&id, &payload); err != nil {
				return err
			}
			if err := accept(id, payload); err != nil {
				return fmt.Errorf("load %s: %w", table, err)
			}
		}
		return rows.Err()
	}
	s.businessNoticeEvents = nil
	s.businessNoticeTasks = nil
	s.businessNoticeReceipts = nil
	s.businessScheduleSnapshots = map[string]learning.ScheduleClass{}
	if err := load("business_notice_events", func(_ string, raw []byte) error {
		var event learning.BusinessNoticeEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return err
		}
		s.businessNoticeEvents = append(s.businessNoticeEvents, event)
		return nil
	}); err != nil {
		return err
	}
	if err := load("business_notice_tasks", func(_ string, raw []byte) error {
		var task learning.BusinessNoticeTask
		if err := json.Unmarshal(raw, &task); err != nil {
			return err
		}
		s.businessNoticeTasks = append(s.businessNoticeTasks, task)
		return nil
	}); err != nil {
		return err
	}
	if err := load("business_schedule_snapshots", func(id string, raw []byte) error {
		var item learning.ScheduleClass
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		s.businessScheduleSnapshots[id] = item
		return nil
	}); err != nil {
		return err
	}
	return load("business_notice_receipts", func(_ string, raw []byte) error {
		var item learning.OfficialDeliveryReceipt
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		s.businessNoticeReceipts = append(s.businessNoticeReceipts, item)
		return nil
	})
}

func persistBusinessNoticeBootstrap(tx *sql.Tx, s *MemoryStore) error {
	for _, row := range businessNoticeRows(s) {
		if _, err := tx.Exec(row.upsertSQL, row.upsertArgs...); err != nil {
			return err
		}
	}
	return nil
}

func cloneBusinessNoticeValue[T any](value T) T {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var copy T
	if err := json.Unmarshal(raw, &copy); err != nil {
		panic(err)
	}
	return copy
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
