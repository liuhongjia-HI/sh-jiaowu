package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"starline/learning-api/internal/domain/learning"
	"strings"
)

func teachingPlanReadKey(id string) string { return "teaching-plan:" + id }
func teachingPlanReadVersion(plan learning.TeachingPlan) string {
	// Rendering retries and directory label changes do not replace the document.
	data, _ := json.Marshal([]string{plan.FileID, plan.Title, plan.Grade, plan.Subject, plan.CourseID, plan.LessonID, plan.Semester, plan.Phase})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *MemoryStore) RecordTeachingPlanView(p learning.Principal, id string, req learning.TeachingPlanReadRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		plan, err := work.planUnlocked(p, strings.TrimSpace(id))
		if err != nil {
			return err
		}
		if req.Version == "" || req.Version != plan.ReadVersion {
			return errors.New("教案已更新，请刷新后重新查看")
		}
		asset, ok := work.fileAssets[plan.FileID]
		if !ok || asset.PreviewStatus != "可预览" {
			return errors.New("教案尚不可预览")
		}
		work.recordResourceRead(p.UserID, teachingPlanReadKey(plan.ID), req.Version)
		return nil
	})
}
