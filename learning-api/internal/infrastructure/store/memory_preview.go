package store

import (
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

const maxPreviewAttempts = 3

func (s *MemoryStore) enqueuePreviewJobUnlocked(fileID string) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return
	}
	for _, job := range s.previewJobs {
		if job.FileID == fileID {
			return
		}
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	s.previewJobs = append(s.previewJobs, learning.PreviewJob{
		ID: "preview-job-" + time.Now().Format("20060102150405.000000000"), FileID: fileID, Status: "待处理", CreatedAt: now,
	})
}

func (s *MemoryStore) RecoverPreviewJobs() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		for index := range work.previewJobs {
			if work.previewJobs[index].Status == "处理中" {
				work.previewJobs[index].Status = "待处理"
				work.previewJobs[index].StartedAt = ""
			}
		}
		return nil
	})
}

func (s *MemoryStore) ClaimPreviewJob() (learning.PreviewJob, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type claimResult struct {
		Job learning.PreviewJob
		OK  bool
	}
	result, err := persistentMutation(s, func(work *MemoryStore) (claimResult, error) {
		for index := range work.previewJobs {
			job := &work.previewJobs[index]
			if job.Status != "待处理" || job.AttemptCount >= maxPreviewAttempts {
				continue
			}
			job.Status = "处理中"
			job.AttemptCount++
			job.StartedAt = time.Now().Format("2006-01-02 15:04:05")
			return claimResult{Job: *job, OK: true}, nil
		}
		return claimResult{}, nil
	})
	return result.Job, result.OK, err
}

func (s *MemoryStore) PreviewJobFile(fileID string) (learning.FileAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asset, ok := s.fileAssets[strings.TrimSpace(fileID)]
	if !ok {
		return learning.FileAsset{}, errors.New("预览任务对应的文件不存在")
	}
	return asset, nil
}

func (s *MemoryStore) CompletePreviewJob(jobID string, result learning.PreviewResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		job, asset, err := work.previewJobAssetUnlocked(jobID)
		if err != nil {
			return err
		}
		asset.PreviewPath = result.PreviewPath
		asset.PreviewPageDir = result.PreviewPageDir
		asset.PreviewPageCount = result.PreviewPageCount
		asset.PreviewStatus = "可预览"
		asset.PreviewError = ""
		work.fileAssets[asset.ID] = *asset
		work.syncContentPreviewStatus(asset.ID, "可预览")
		job.Status = "已完成"
		job.ErrorMessage = ""
		job.FinishedAt = time.Now().Format("2006-01-02 15:04:05")
		return nil
	})
}

func (s *MemoryStore) FailPreviewJob(jobID, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		job, asset, err := work.previewJobAssetUnlocked(jobID)
		if err != nil {
			return err
		}
		message = strings.TrimSpace(message)
		if len([]rune(message)) > 500 {
			message = string([]rune(message)[:500])
		}
		job.ErrorMessage = message
		asset.PreviewError = message
		if job.AttemptCount < maxPreviewAttempts {
			job.Status = "待处理"
			asset.PreviewStatus = "待转换"
			work.syncContentPreviewStatus(asset.ID, "待转换")
		} else {
			job.Status = "转换失败"
			job.FinishedAt = time.Now().Format("2006-01-02 15:04:05")
			asset.PreviewStatus = "转换失败"
			work.syncContentPreviewStatus(asset.ID, "转换失败")
		}
		work.fileAssets[asset.ID] = *asset
		return nil
	})
}

func (s *MemoryStore) RetryPreviewJob(operator string, principal learning.Principal, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		fileID = strings.TrimSpace(fileID)
		if _, err := work.contentFileUnlocked(principal, fileID); err != nil {
			return err
		}
		for index := range work.previewJobs {
			job := &work.previewJobs[index]
			if job.FileID != fileID {
				continue
			}
			if job.Status != "转换失败" {
				return errors.New("只有转换失败的课件可以重新生成")
			}
			asset, ok := work.fileAssets[fileID]
			if !ok {
				return errors.New("课件文件不存在，请重新上传")
			}
			job.Status = "待处理"
			job.AttemptCount = 0
			job.ErrorMessage = ""
			job.StartedAt = ""
			job.FinishedAt = ""
			asset.PreviewStatus = "待转换"
			asset.PreviewError = ""
			work.fileAssets[fileID] = asset
			work.syncContentPreviewStatus(fileID, "待转换")
			work.prependLog(operator, "重新生成课件预览", asset.FileName)
			return nil
		}
		return errors.New("课件预览任务不存在，请重新上传")
	})
}

func (s *MemoryStore) previewJobAssetUnlocked(jobID string) (*learning.PreviewJob, *learning.FileAsset, error) {
	jobID = strings.TrimSpace(jobID)
	for index := range s.previewJobs {
		if s.previewJobs[index].ID != jobID {
			continue
		}
		asset, ok := s.fileAssets[s.previewJobs[index].FileID]
		if !ok {
			return nil, nil, errors.New("预览任务对应的文件不存在")
		}
		return &s.previewJobs[index], &asset, nil
	}
	return nil, nil, errors.New("预览任务不存在")
}

func (s *MemoryStore) syncContentPreviewStatus(fileID, status string) {
	for index := range s.materials {
		if s.materials[index].FileID == fileID {
			s.materials[index].PreviewStatus = status
		}
	}
	for index := range s.homework {
		if s.homework[index].FileID == fileID {
			s.homework[index].PreviewStatus = status
		}
	}
}
