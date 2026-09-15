package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

type materialSyncPlan struct {
	preview learning.MaterialSyncPreview
	sources []learning.Material
	targets []learning.Course
}

func (s *MemoryStore) previewMaterialSyncUnlocked(principal learning.Principal, req learning.MaterialSyncRequest) (learning.MaterialSyncPreview, error) {
	plan, err := s.materialSyncPlan(principal, req)
	if err != nil {
		return learning.MaterialSyncPreview{}, err
	}
	return plan.preview, nil
}

func (s *MemoryStore) materialSyncPlan(principal learning.Principal, req learning.MaterialSyncRequest) (materialSyncPlan, error) {
	if !canUploadHandout(principal) {
		return materialSyncPlan{}, errors.New("当前账号没有上传课程讲义权限")
	}
	sourceCourseID := strings.TrimSpace(req.SourceCourseID)
	sourceLessonID := strings.TrimSpace(req.SourceLessonID)
	sourceCourse, ok := s.findCourse(sourceCourseID)
	if !ok || !canSeeCourse(principal, sourceCourse) {
		return materialSyncPlan{}, errors.New("源课程不存在或无权访问")
	}
	if _, err := curriculumPathForLesson(sourceCourse, sourceLessonID); err != nil {
		return materialSyncPlan{}, errors.New("源课节不存在或已发生变化")
	}
	if len(req.MaterialIDs) == 0 {
		return materialSyncPlan{}, errors.New("请选择需要同步的课程讲义")
	}
	materialByID := make(map[string]learning.Material, len(s.materials))
	for _, item := range s.materials {
		materialByID[item.ID] = item
	}
	seenMaterials := map[string]bool{}
	sources := make([]learning.Material, 0, len(req.MaterialIDs))
	seenTags := map[string]bool{}
	for _, rawID := range req.MaterialIDs {
		id := strings.TrimSpace(rawID)
		if id == "" || seenMaterials[id] {
			continue
		}
		seenMaterials[id] = true
		item, exists := materialByID[id]
		if !exists || item.CourseID != sourceCourseID || item.LessonID != sourceLessonID || !canSeeCourse(principal, sourceCourse) {
			return materialSyncPlan{}, errors.New("源课程讲义不存在或不属于所选课节")
		}
		if strings.TrimSpace(item.TagCode) == "" || item.FileID == "" {
			return materialSyncPlan{}, errors.New("源课程讲义缺少标签或文件，不能同步")
		}
		if !materialVisibleToStudents(item) {
			return materialSyncPlan{}, errors.New("源课程讲义尚未发布，不能同步")
		}
		if _, exists := s.fileAssets[item.FileID]; !exists {
			return materialSyncPlan{}, errors.New("源课程讲义文件不存在，不能同步")
		}
		if seenTags[item.TagCode] {
			return materialSyncPlan{}, errors.New("同一批次不能包含重复标签")
		}
		seenTags[item.TagCode] = true
		sources = append(sources, item)
	}
	if len(sources) == 0 {
		return materialSyncPlan{}, errors.New("请选择需要同步的课程讲义")
	}
	if len(req.Targets) == 0 {
		return materialSyncPlan{}, errors.New("请选择同步目标课程")
	}

	seenCourses := map[string]bool{}
	targetCourses := make([]learning.Course, 0, len(req.Targets))
	previews := make([]learning.MaterialSyncTargetPreview, 0, len(req.Targets))
	for _, target := range req.Targets {
		courseID := strings.TrimSpace(target.CourseID)
		lessonID := strings.TrimSpace(target.LessonID)
		if courseID == "" || lessonID == "" {
			return materialSyncPlan{}, errors.New("请选择每个目标课程的课节")
		}
		if courseID == sourceCourseID {
			return materialSyncPlan{}, errors.New("不能将课程讲义同步回源课程")
		}
		if seenCourses[courseID] {
			return materialSyncPlan{}, errors.New("同一目标课程只能选择一次")
		}
		seenCourses[courseID] = true
		course, exists := s.findCourse(courseID)
		if !exists || !canSeeCourse(principal, course) {
			return materialSyncPlan{}, errors.New("目标课程不存在或无权操作")
		}
		if !s.sameMaterialSyncScope(sourceCourse, course) {
			return materialSyncPlan{}, errors.New("目标课程必须与源课程的年级、学科、学期和阶段一致")
		}
		path, err := curriculumPathForLesson(course, lessonID)
		if err != nil {
			return materialSyncPlan{}, fmt.Errorf("目标课程“%s”的课节不存在或已发生变化", course.Name)
		}
		items := make([]learning.MaterialSyncItemPreview, 0, len(sources))
		for _, source := range sources {
			matches := s.materialSlotMatches(course.ID, lessonID, source.TagCode)
			if len(matches) > 1 {
				return materialSyncPlan{}, fmt.Errorf("目标课程“%s”的 %s 标签存在重复资料，请先处理冲突", course.Name, source.TagCode)
			}
			item := learning.MaterialSyncItemPreview{SourceMaterialID: source.ID, Title: source.Title, TagCode: source.TagCode, Action: "create"}
			if len(matches) == 1 {
				item.Action = "replace"
				item.ExistingID = matches[0].ID
				item.ExistingTitle = matches[0].Title
				item.ExistingVersion = fmt.Sprintf("%s|%t|%s|%s", matches[0].FileID, matches[0].AllowDownload, matches[0].Status, matches[0].CreatedAt)
			}
			items = append(items, item)
		}
		targetCourses = append(targetCourses, course)
		previews = append(previews, learning.MaterialSyncTargetPreview{CourseID: course.ID, CourseName: course.Name, LessonID: lessonID, Curriculum: path, Items: items})
	}
	preview := learning.MaterialSyncPreview{Targets: previews}
	preview.Snapshot = materialSyncSnapshot(preview, sources)
	return materialSyncPlan{preview: preview, sources: sources, targets: targetCourses}, nil
}

func (s *MemoryStore) sameMaterialSyncScope(source, target learning.Course) bool {
	if source.Grade != target.Grade || !subjectsMatch(source.Subject, target.Subject) {
		return false
	}
	sourceSpace, sourceOK := s.findLearningSpace(source.LearningSpaceID)
	targetSpace, targetOK := s.findLearningSpace(target.LearningSpaceID)
	return sourceOK && targetOK && sourceSpace.Semester == targetSpace.Semester && sourceSpace.Phase == targetSpace.Phase
}

func (s *MemoryStore) materialSlotMatches(courseID, lessonID, tagCode string) []learning.Material {
	matches := []learning.Material{}
	for _, item := range s.materials {
		if item.CourseID == courseID && item.LessonID == lessonID && item.TagCode == tagCode {
			matches = append(matches, item)
		}
	}
	return matches
}

func materialSyncSnapshot(preview learning.MaterialSyncPreview, sources []learning.Material) string {
	parts := make([]string, 0)
	for _, source := range sources {
		parts = append(parts, fmt.Sprintf("s:%s:%s:%s:%s:%t:%s:%s", source.ID, source.FileID, source.TagCode, source.Title, source.AllowDownload, source.PreviewStatus, source.FileName))
	}
	for _, target := range preview.Targets {
		parts = append(parts, "t:"+target.CourseID+":"+target.LessonID)
		for _, item := range target.Items {
			parts = append(parts, "i:"+item.SourceMaterialID+":"+item.Action+":"+item.ExistingID+":"+item.ExistingTitle+":"+item.ExistingVersion)
		}
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func (s *MemoryStore) syncMaterialsUnlocked(operator string, principal learning.Principal, req learning.MaterialSyncRequest) (learning.MaterialSyncResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.MaterialSyncResult, error) {
			return work.syncMaterialsUnlocked(operator, principal, req)
		})
	}
	work := s.cloneForMutation()
	result, err := work.applyMaterialSyncUnlocked(operator, principal, req)
	if err != nil {
		return learning.MaterialSyncResult{}, err
	}
	s.publishMutation(work)
	return result, nil
}

func (s *MemoryStore) applyMaterialSyncUnlocked(operator string, principal learning.Principal, req learning.MaterialSyncRequest) (learning.MaterialSyncResult, error) {
	plan, err := s.materialSyncPlan(principal, req)
	if err != nil {
		return learning.MaterialSyncResult{}, err
	}
	if strings.TrimSpace(req.Snapshot) == "" {
		return learning.MaterialSyncResult{}, errors.New("请先预检查同步内容")
	}
	if req.Snapshot != plan.preview.Snapshot {
		if result, ok := s.materialSyncAlreadyCompleted(plan); ok {
			result.AlreadySynced = true
			return result, nil
		}
		return learning.MaterialSyncResult{}, errors.New("课程讲义或目标课节已发生变化，请重新预检查")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	stamp := time.Now().Format("20060102150405.000000000")
	result := learning.MaterialSyncResult{Targets: make([]learning.MaterialSyncTargetResult, 0, len(plan.targets))}
	for targetIndex, target := range plan.targets {
		preview := plan.preview.Targets[targetIndex]
		targetResult := learning.MaterialSyncTargetResult{CourseID: target.ID, CourseName: target.Name}
		for sourceIndex, source := range plan.sources {
			action := preview.Items[sourceIndex]
			if action.Action == "replace" {
				for index := range s.materials {
					if s.materials[index].ID != action.ExistingID {
						continue
					}
					targetItem := &s.materials[index]
					targetItem.Title = source.Title
					targetItem.FileID = source.FileID
					targetItem.FileName = source.FileName
					targetItem.FileSize = source.FileSize
					targetItem.FileType = source.FileType
					targetItem.PreviewStatus = source.PreviewStatus
					targetItem.PreviewError = source.PreviewError
					targetItem.PreviewURL = source.PreviewURL
					targetItem.DownloadURL = source.DownloadURL
					targetItem.AllowDownload = source.AllowDownload
					targetItem.OwnerTeacherID = principal.UserID
					targetItem.OwnerTeacherName = principal.Name
					targetItem.PublishStatus = "已发布"
					targetItem.Status = learning.StatusEnabled
					targetResult.Replaced++
					targetResult.MaterialIDs = append(targetResult.MaterialIDs, targetItem.ID)
					break
				}
				continue
			}
			item := source
			item.ID = fmt.Sprintf("material-sync-%s-%d-%d", stamp, targetIndex, sourceIndex)
			item.CourseID = target.ID
			item.Course = target.Name
			item.LearningSpaceID = target.LearningSpaceID
			item.LessonID = preview.LessonID
			item.Curriculum = preview.Curriculum
			item.ViewCount = 0
			item.OwnerTeacherID = principal.UserID
			item.OwnerTeacherName = principal.Name
			item.PublishStatus = "已发布"
			item.Status = learning.StatusEnabled
			item.CreatedAt = now
			item.SortOrder = s.nextMaterialSortOrder(target.ID)
			s.materials = append([]learning.Material{item}, s.materials...)
			targetResult.Created++
			targetResult.MaterialIDs = append(targetResult.MaterialIDs, item.ID)
		}
		s.prependLogDetail(operator, "同步课程讲义", target.Name, fmt.Sprintf("%s/%s → %s/%s；新增 %d，替换 %d", req.SourceCourseID, req.SourceLessonID, target.ID, preview.LessonID, targetResult.Created, targetResult.Replaced))
		s.notifyCourseContentUploaded(target)
		result.Targets = append(result.Targets, targetResult)
	}
	return result, nil
}

func (s *MemoryStore) materialSyncAlreadyCompleted(plan materialSyncPlan) (learning.MaterialSyncResult, bool) {
	result := learning.MaterialSyncResult{Targets: make([]learning.MaterialSyncTargetResult, 0, len(plan.targets))}
	for targetIndex, target := range plan.targets {
		preview := plan.preview.Targets[targetIndex]
		targetResult := learning.MaterialSyncTargetResult{CourseID: target.ID, CourseName: target.Name}
		for _, source := range plan.sources {
			matches := s.materialSlotMatches(target.ID, preview.LessonID, source.TagCode)
			if len(matches) != 1 || matches[0].FileID != source.FileID || matches[0].Title != source.Title || matches[0].AllowDownload != source.AllowDownload {
				return learning.MaterialSyncResult{}, false
			}
			targetResult.MaterialIDs = append(targetResult.MaterialIDs, matches[0].ID)
		}
		result.Targets = append(result.Targets, targetResult)
	}
	return result, true
}
