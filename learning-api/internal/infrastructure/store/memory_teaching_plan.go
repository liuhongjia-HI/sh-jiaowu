package store

import (
	"errors"
	"fmt"
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
	plan.ReadVersion = teachingPlanReadVersion(plan)
	for _, course := range s.courses {
		if course.ID == plan.CourseID {
			if path, err := curriculumPathForLesson(course, plan.LessonID); err == nil {
				plan.Chapter = planChapterLabel(path)
			}
			break
		}
	}
	if asset, ok := s.fileAssets[plan.FileID]; ok {
		plan.PreviewStatus = asset.PreviewStatus
		plan.PreviewError = ""
		if asset.PreviewStatus == "转换失败" {
			plan.PreviewError = teachingPlanPreviewError(asset.PreviewError)
		}
	}
	plan.PreviewURL = "/api/teaching-plans/" + plan.ID + "/preview"
	if p.CanDownloadTeacherMaterial() {
		plan.DownloadURL = "/api/teaching-plans/" + plan.ID + "/download"
	} else {
		plan.DownloadURL = ""
	}
	return plan
}

// Conversion diagnostics may contain local paths. Return only actionable,
// controlled messages to readers; retain full diagnostics on the file/job.
func teachingPlanPreviewError(message string) string {
	message = strings.TrimSpace(message)
	switch message {
	case "原文件不存在，请重新上传", "现有预览文件不存在，请重新上传",
		"预览文件已丢失，请重新生成预览":
		return message
	case "服务器未安装 LibreOffice，无法转换 Word/PPT":
		return "Word/PPT 预览服务暂不可用，请联系管理员"
	case "LibreOffice 转换超时，请检查文件大小或内容":
		return "预览生成超时，请检查文件大小或内容"
	case "LibreOffice 转换失败，请检查 Word/PPT 是否损坏或已加密":
		return "预览生成失败，请检查 Word/PPT 是否损坏或已加密"
	case "LibreOffice 未生成 PDF，请检查 Word/PPT 是否损坏或已加密":
		return "未生成预览，请检查 Word/PPT 是否损坏或已加密"
	}
	var pages, limit int
	if n, _ := fmt.Sscanf(message, "课件共%d页，超过%d页上限", &pages, &limit); n == 2 && pages > 0 && limit > 0 && message == fmt.Sprintf("课件共%d页，超过%d页上限", pages, limit) {
		return message
	}
	return "预览生成失败，请重新生成；若仍失败，请更换文件或联系管理员"
}

func (s *MemoryStore) TeachingPlans(p learning.Principal) learning.TeachingPlanList {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := learning.TeachingPlanList{Plans: []learning.TeachingPlan{}, UploadScopes: []learning.TeachingPlanScope{}, Directories: []learning.Course{}, UnreadPlanIDs: []string{}}
	readVersions := map[string]string{}
	for _, read := range s.teacherMaterialReads {
		if read.UserID == p.UserID {
			readVersions[read.MaterialID] = read.Version
		}
	}
	for _, plan := range s.teachingPlans {
		if s.canViewPlan(p, plan.Grade, plan.Subject) {
			out.Plans = append(out.Plans, s.decoratePlan(p, plan))
			if readVersions[teachingPlanReadKey(plan.ID)] != teachingPlanReadVersion(plan) {
				out.UnreadPlanIDs = append(out.UnreadPlanIDs, plan.ID)
			}
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
	for _, course := range s.courses {
		if s.canViewPlan(p, course.Grade, course.Subject) {
			if space, ok := s.findLearningSpace(course.LearningSpaceID); ok {
				familyName := ""
				for _, family := range s.courseFamilies {
					if course.FamilyID != "" && family.ID == course.FamilyID {
						familyName = family.Name
						break
					}
				}
				out.Courses = append(out.Courses, learning.TeachingPlanCourse{ID: course.ID, FamilyID: course.FamilyID, FamilyName: familyName, Name: course.Name, Grade: course.Grade, Subject: course.Subject, Semester: space.Semester, Phase: space.Phase, Level: space.Level})
			}
		}
		if course.Status == learning.StatusEnabled && s.canUploadPlan(p, course.Grade, course.Subject) && (isPlanAdmin(p) || containsString(p.LearningSpaceIDs, course.LearningSpaceID)) {
			out.Directories = append(out.Directories, course)
		}
	}
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
		if len(req.BatchID) > 64 || req.BatchID != strings.TrimSpace(req.BatchID) {
			return learning.TeachingPlan{}, errors.New("上传批次编号无效")
		}
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
		if err := work.assignPlanChapter(p, &plan, req.CourseID, req.LessonID); err != nil {
			return learning.TeachingPlan{}, err
		}
		work.fileAssets[req.File.ID] = req.File
		work.enqueuePreviewJobUnlocked(req.File.ID)
		work.teachingPlans = append([]learning.TeachingPlan{plan}, work.teachingPlans...)
		if err := work.registerTeachingPlanNoticeBatch(p, req.BatchID, plan); err != nil {
			return learning.TeachingPlan{}, err
		}
		work.prependLog(operator, "上传教案", plan.Title)
		return work.decoratePlan(p, plan), nil
	})
}

// Empty references preserve pre-existing unclassified plans. References always
// resolve through the actual course curriculum, including shared families.
func (s *MemoryStore) assignPlanChapter(p learning.Principal, plan *learning.TeachingPlan, courseID, lessonID string) error {
	courseID, lessonID = strings.TrimSpace(courseID), strings.TrimSpace(lessonID)
	if courseID == "" && lessonID == "" {
		plan.CourseID, plan.LessonID, plan.Chapter, plan.Semester, plan.Phase = "", "", "", "", ""
		return nil
	}
	for _, course := range s.courses {
		if course.ID != courseID {
			continue
		}
		if course.Status != learning.StatusEnabled || course.Grade != plan.Grade || !subjectsMatch(course.Subject, plan.Subject) {
			return errors.New("请选择同年级学科的有效课程目录")
		}
		space, ok := s.findLearningSpace(course.LearningSpaceID)
		if !ok || space.Status != learning.StatusEnabled || !isPlanAdmin(p) && !containsString(p.LearningSpaceIDs, space.ID) {
			return errors.New("没有权限关联该教学范围的目录")
		}
		if lessonID == "" {
			plan.CourseID, plan.LessonID, plan.Chapter, plan.Semester, plan.Phase = courseID, "", "", space.Semester, space.Phase
			return nil
		}
		path, err := curriculumPathForLesson(course, lessonID)
		if err != nil {
			return err
		}
		plan.CourseID, plan.LessonID, plan.Chapter, plan.Semester, plan.Phase = courseID, lessonID, planChapterLabel(path), space.Semester, space.Phase
		return nil
	}
	return errors.New("请选择有效课程和章节")
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

func planChapterLabel(path learning.CurriculumPath) string {
	parts := []string{}
	for _, text := range []string{path.Unit, path.Chapter, path.Lesson} {
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " / ")
}

func (s *MemoryStore) UpdateTeachingPlanChapter(operator string, p learning.Principal, id string, req learning.TeachingPlanChapterRequest) (learning.TeachingPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.TeachingPlan, error) {
		for i, plan := range work.teachingPlans {
			if plan.ID != strings.TrimSpace(id) {
				continue
			}
			if !work.canUploadPlan(p, plan.Grade, plan.Subject) {
				return learning.TeachingPlan{}, errors.New("没有权限调整该教案章节")
			}
			if err := work.assignPlanChapter(p, &plan, req.CourseID, req.LessonID); err != nil {
				return learning.TeachingPlan{}, err
			}
			work.teachingPlans[i] = plan
			work.prependLog(operator, "调整教案章节", plan.Title)
			return work.decoratePlan(p, plan), nil
		}
		return learning.TeachingPlan{}, errors.New("教案不存在")
	})
}
