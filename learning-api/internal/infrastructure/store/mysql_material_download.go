package store

import (
	"encoding/json"
	"sort"
	"starline/learning-api/internal/domain/learning"
)

type storedDownloadJob struct {
	Job         learning.MaterialDownloadJob    `json:"job"`
	OwnerID     string                          `json:"ownerId"`
	ArchivePath string                          `json:"archivePath"`
	Items       []learning.MaterialDownloadItem `json:"items"`
}

func (s *MemoryStore) loadMaterialDownloadsFromDB() error {
	rows, err := s.db.Query(`SELECT payload FROM material_download_jobs ORDER BY id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	jobs := []learning.MaterialDownloadJob{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var stored storedDownloadJob
		if err := json.Unmarshal(raw, &stored); err != nil {
			return err
		}
		job := stored.Job
		job.OwnerID, job.ArchivePath, job.Items = stored.OwnerID, stored.ArchivePath, stored.Items
		jobs = append(jobs, job)
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].CreatedAt > jobs[j].CreatedAt })
	s.materialDownloads = jobs
	return rows.Err()
}
