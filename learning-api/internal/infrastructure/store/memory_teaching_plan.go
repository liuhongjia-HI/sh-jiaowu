package store

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func isPlanAdmin(p learning.Principal) bool {
	return hasRole(p.Roles, learning.RoleOpsStaff) || hasRole(p.Roles, learning.RoleCampusAdmin) || hasRole(p.Roles, learning.RoleSuperAdmin)
}

func (s *MemoryStore) planScopeExists(grade, subject string) bool {
	for _, space := range s.learningSpaces {
		if space.Status == learning.StatusEnabled && space.Grade == grade && subjectsMatch(space.Subject, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) canViewPlan(p learning.Principal, grade, subject string) bool {
	if isPlanAdmin(p) {
		return true
	}
	if !hasRole(p.Roles, learning.RoleTeacher) {
		return false
	}
	ids := append([]string(nil), p.LearningSpaceIDs...)
	if p.TeacherLibrary != nil {
		ids = append(ids, p.TeacherLibrary.SpaceIDs...)
		for _, scope := range p.TeacherLibrary.Scopes {
			if subjectsMatch(scope.Subject, subject) && (scope.Grade == "" || scope.Grade == grade) {
				return true
			}
		}
	}
	for _, space := range s.learningSpaces {
		if containsString(ids, space.ID) && space.Grade == grade && subjectsMatch(space.Subject, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) canUploadPlan(p learning.Principal, grade, subject string) bool {
	if !canUploadHandout(p) || !s.planScopeExists(grade, subject) {
		return false
	}
	if isPlanAdmin(p) {
		return true
	}
	for _, space := range s.learningSpaces {
		if containsString(p.LearningSpaceIDs, space.ID) && space.Status == learning.StatusEnabled && space.Grade == grade && subjectsMatch(space.Subject, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) decoratePlan(p learning.Principal, plan learning.TeachingPlan) learning.TeachingPlan {
	if asset, ok := s.fileAssets[plan.FileID]; ok {
		plan.PreviewStatus = asset.PreviewStatus
	}
	plan.PreviewURL = "/api/teaching-plans/" + plan.ID + "/preview"
	if p.CanDownloadTeacherMaterial() {
		plan.DownloadURL = "/api/teaching-plans/" + plan.ID + "/download"
	} else {
		plan.DownloadURL = ""
	}
	return plan
}

func (s *MemoryStore) TeachingPlans(p learning.Principal) learning.TeachingPlanList {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := learning.TeachingPlanList{Plans: []learning.TeachingPlan{}, UploadScopes: []learning.TeachingPlanScope{}}
	for _, plan := range s.teachingPlans {
		if s.canViewPlan(p, plan.Grade, plan.Subject) {
			out.Plans = append(out.Plans, s.decoratePlan(p, plan))
		}
	}
	seen := map[string]bool{}
	for _, space := range s.learningSpaces {
		if !s.canUploadPlan(p, space.Grade, space.Subject) {
			continue
		}
		key := space.Grade + "\x00" + space.Subject
		if seen[key] {
			continue
		}
		seen[key] = true
		out.UploadScopes = append(out.UploadScopes, learning.TeachingPlanScope{Grade: space.Grade, Subject: space.Subject})
	}
	out.CanUpload = len(out.UploadScopes) > 0
	return out
}

func (s *MemoryStore) planUnlocked(p learning.Principal, id string) (learning.TeachingPlan, error) {
	for _, plan := range s.teachingPlans {
		if plan.ID == id {
			if s.canViewPlan(p, plan.Grade, plan.Subject) {
				return s.decoratePlan(p, plan), nil
			}
			return learning.TeachingPlan{}, errors.New("没有权限查看该教案")
		}
	}
	return learning.TeachingPlan{}, errors.New("教案不存在")
}

func (s *MemoryStore) TeachingPlan(p learning.Principal, id string) (learning.TeachingPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planUnlocked(p, strings.TrimSpace(id))
}

func (s *MemoryStore) TeachingPlanFile(p learning.Principal, id string) (learning.FileAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planUnlocked(p, strings.TrimSpace(id))
	if err != nil {
		return learning.FileAsset{}, err
	}
	asset, ok := s.fileAssets[plan.FileID]
	if !ok {
		return learning.FileAsset{}, errors.New("教案文件不存在")
	}
	return asset, nil
}

func (s *MemoryStore) CreateTeachingPlan(operator string, p learning.Principal, req learning.TeachingPlanUploadRequest) (learning.TeachingPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.TeachingPlan, error) {
		grade, subject := strings.TrimSpace(req.Grade), strings.TrimSpace(req.Subject)
		if !work.canUploadPlan(p, grade, subject) {
			return learning.TeachingPlan{}, errors.New("没有权限上传该年级学科的教案")
		}
		if req.File.ID == "" || req.File.OriginalPath == "" {
			return learning.TeachingPlan{}, errors.New("请选择教案文件")
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = strings.TrimSuffix(req.File.FileName, filepath.Ext(req.File.FileName))
		}
		if len([]rune(title)) > 128 {
			return learning.TeachingPlan{}, errors.New("教案标题不能超过 128 字")
		}
		plan := learning.TeachingPlan{ID: "plan-" + time.Now().Format("20060102150405.000000000"), Title: title, Grade: grade, Subject: subject, FileID: req.File.ID, FileName: req.File.FileName, FileSize: req.File.FileSize, FileType: req.File.FileType, UploaderID: p.UserID, UploaderName: p.Name, CreatedAt: time.Now().Format("2006-01-02 15:04:05")}
		work.fileAssets[req.File.ID] = req.File
		work.enqueuePreviewJobUnlocked(req.File.ID)
		work.teachingPlans = append([]learning.TeachingPlan{plan}, work.teachingPlans...)
		work.prependLog(operator, "上传教案", plan.Title)
		return work.decoratePlan(p, plan), nil
	})
}

func (s *MemoryStore) RetryTeachingPlanPreview(operator string, p learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		plan, err := work.planUnlocked(p, strings.TrimSpace(id))
		if err != nil || !work.canUploadPlan(p, plan.Grade, plan.Subject) {
			return errors.New("没有权限重新生成该教案预览")
		}
		asset, ok := work.fileAssets[plan.FileID]
		if !ok {
			return errors.New("教案文件不存在，请重新上传")
		}
		for index := range work.previewJobs {
			job := &work.previewJobs[index]
			if job.FileID != plan.FileID {
				continue
			}
			if job.Status != "转换失败" {
				return errors.New("只有预览失败的教案可以重试")
			}
			job.Status, job.AttemptCount, job.ErrorMessage = "待处理", 0, ""
			job.StartedAt, job.FinishedAt = "", ""
			asset.PreviewStatus, asset.PreviewError = "待转换", ""
			work.fileAssets[plan.FileID] = asset
			work.prependLog(operator, "重新生成教案预览", plan.Title)
			return nil
		}
		return errors.New("教案预览任务不存在，请重新上传")
	})
}
