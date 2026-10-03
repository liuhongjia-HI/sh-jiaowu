package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"starline/learning-api/internal/domain/learning"
)

type teacherMaterialRead struct{ ID, UserID, MaterialID, Version string }

func teacherMaterialVersion(m learning.Material) string {
	// Preview conversion and view counts do not change a published file's version.
	data, _ := json.Marshal([]string{m.FileID, m.Title, m.CourseID, m.LearningSpaceID, m.LessonID, m.TagCode, m.UpdatedAt, string(m.Status), m.PublishStatus})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *MemoryStore) recordTeacherMaterialRead(userID string, m learning.Material) {
	s.recordResourceRead(userID, m.ID, teacherMaterialVersion(m))
}
func (s *MemoryStore) recordResourceRead(userID, resourceID, version string) {
	sum := sha256.Sum256([]byte(userID + "\x00" + resourceID))
	read := teacherMaterialRead{hex.EncodeToString(sum[:]), userID, resourceID, version}
	for i, old := range s.teacherMaterialReads {
		if old.ID == read.ID {
			s.teacherMaterialReads[i] = read
			return
		}
	}
	s.teacherMaterialReads = append(s.teacherMaterialReads, read)
}
func (s *MemoryStore) loadTeacherMaterialReadsFromDB() error {
	rows, err := s.db.Query(`SELECT id,user_id,material_id,material_version FROM teacher_material_reads ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	reads := []teacherMaterialRead{}
	for rows.Next() {
		var read teacherMaterialRead
		if err := rows.Scan(&read.ID, &read.UserID, &read.MaterialID, &read.Version); err != nil {
			return err
		}
		reads = append(reads, read)
	}
	s.teacherMaterialReads = reads
	return rows.Err()
}
