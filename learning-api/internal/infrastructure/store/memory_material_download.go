package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func archiveSegment(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune("/\\:*?\"<>|", r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(value))
	value = strings.Trim(value, ". ")
	if value == "" {
		return "未命名"
	}
	if len([]rune(value)) > 100 {
		value = string([]rune(value)[:100])
	}
	return value
}

func archiveFilename(value string) string {
	ext := path.Ext(value)
	if len([]rune(ext)) > 20 {
		ext = ""
	}
	if ext == "" {
		return archiveSegment(value)
	}
	return archiveSegment(strings.TrimSuffix(value, ext)) + "." + archiveSegment(strings.TrimPrefix(ext, "."))
}

func (s *MemoryStore) selectMaterialDownload(p learning.Principal, scope learning.MaterialDownloadScope) (learning.MaterialDownloadSelection, []learning.MaterialDownloadItem, error) {
	quote := learning.MaterialDownloadSelection{Courses: []string{}}
	student := hasRole(p.Roles, learning.RoleStudent) && p.StudentID != ""
	if !student && (!(p.IsTeacherOnly() || isPlanAdmin(p)) || !p.CanDownloadTeacherMaterial()) {
		return quote, nil, errors.New("当前账号没有批量下载权限")
	}
	if student {
		if len(scope.CourseIDs) != 1 {
			return quote, nil, errors.New("请选择一个课程打包")
		}
		course, ok := s.findCourse(scope.CourseIDs[0])
		if !ok {
			return quote, nil, errors.New("课程不存在")
		}
		scope.Subject = course.Subject
		if record, ok := s.findStudent(p.StudentID); ok {
			quote.StudentName = record.Name
		}
	}
	if strings.TrimSpace(scope.Subject) == "" {
		return quote, nil, errors.New("请选择学科")
	}
	courses := map[string]learning.Course{}
	for _, course := range s.courses {
		space, ok := s.findLearningSpace(course.LearningSpaceID)
		if !ok || course.Status != learning.StatusEnabled || (!student && !s.canReadTeacherCourse(p, course)) || !subjectsMatch(course.Subject, scope.Subject) || scope.Grade != "" && course.Grade != scope.Grade || scope.Semester != "" && space.Semester != scope.Semester || scope.Phase != "" && space.Phase != scope.Phase || len(scope.CourseIDs) > 0 && !containsString(scope.CourseIDs, course.ID) {
			continue
		}
		courses[course.ID] = course
	}
	for _, id := range scope.CourseIDs {
		if _, ok := courses[id]; !ok {
			return quote, nil, errors.New("所选课程不存在或已无权下载")
		}
	}
	items := []learning.MaterialDownloadItem{}
	seenCourses := map[string]bool{}
	seenLessons := map[string]bool{}
	for _, material := range orderMaterialsByCourse(s.materials) {
		course, ok := courses[material.CourseID]
		if !ok || course.LearningSpaceID != material.LearningSpaceID || !materialVisibleToStudents(material) || len(scope.LessonIDs) > 0 && !containsString(scope.LessonIDs, material.LessonID) || len(scope.MaterialIDs) > 0 && !containsString(scope.MaterialIDs, material.ID) {
			continue
		}
		if student && !s.studentBatchMaterialAllowed(p, material) {
			continue
		}
		asset, ok := s.fileAssets[material.FileID]
		if !ok || asset.OriginalPath == "" {
			return quote, nil, errors.New("选中范围有讲义缺少原文件，请先补齐文件")
		}
		chapter, err := curriculumPathForLesson(course, material.LessonID)
		if err != nil {
			return quote, nil, errors.New("选中范围有讲义章节失效，请先调整章节")
		}
		folder := []string{archiveSegment(course.Name)}
		for _, part := range []string{chapter.Unit, chapter.Chapter, chapter.Lesson} {
			if part != "" {
				folder = append(folder, archiveSegment(part))
			}
		}
		// Material ID distinguishes equal file names, including equal names in
		// different courses. Paths never use raw user-controlled separators.
		folder = append(folder, archiveSegment(material.ID)+"_"+archiveFilename(asset.FileName))
		items = append(items, learning.MaterialDownloadItem{MaterialID: material.ID, FileID: material.FileID, Name: path.Join(folder...), Size: asset.FileSize})
		if student {
			quote.Materials = append(quote.Materials, learning.MaterialDownloadChoice{ID: material.ID, Title: material.Title, FileName: asset.FileName, Size: asset.FileSize, Unit: chapter.Unit, Chapter: chapter.Chapter, Lesson: chapter.Lesson})
		}
		quote.Count++
		quote.Size += asset.FileSize
		seenLessons[material.LessonID] = true
		if !seenCourses[course.ID] {
			quote.Courses = append(quote.Courses, course.Name)
			seenCourses[course.ID] = true
		}
	}
	for _, id := range scope.MaterialIDs {
		found := false
		for _, item := range items {
			if item.MaterialID == id {
				found = true
			}
		}
		if !found {
			return quote, nil, errors.New("所选资料已无权下载，请刷新清单")
		}
	}
	for _, id := range scope.LessonIDs {
		if !seenLessons[id] {
			return quote, nil, errors.New("所选章节不存在或没有可下载讲义")
		}
	}
	if quote.Count == 0 {
		return quote, nil, errors.New("当前范围没有可下载的已发布讲义")
	}
	if quote.Count > 2000 || quote.Size > 2*1024*1024*1024 {
		return quote, nil, errors.New("资料超过单次打包限制，请按课程或阶段分批")
	}
	return quote, items, nil
}

func (s *MemoryStore) MaterialDownloadSelection(p learning.Principal, scope learning.MaterialDownloadScope) (learning.MaterialDownloadSelection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, _, err := s.selectMaterialDownload(p, scope)
	return q, err
}

func (s *MemoryStore) CreateMaterialDownload(p learning.Principal, scope learning.MaterialDownloadScope) (learning.MaterialDownloadJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.MaterialDownloadJob, error) {
		quote, items, err := work.selectMaterialDownload(p, scope)
		if err != nil {
			return learning.MaterialDownloadJob{}, err
		}
		active := 0
		for _, job := range work.materialDownloads {
			if job.OwnerID == p.UserID && (job.Status == "准备中" || job.Status == "打包中") {
				active++
			}
		}
		if active >= 3 {
			return learning.MaterialDownloadJob{}, errors.New("已有下载任务正在准备，请完成后再创建")
		}
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return learning.MaterialDownloadJob{}, err
		}
		job := learning.MaterialDownloadJob{ID: "download-" + hex.EncodeToString(nonce[:]), OwnerID: p.UserID, Scope: scope, Status: "准备中", Count: quote.Count, Size: quote.Size, CreatedAt: time.Now().UTC().Format(time.RFC3339), Items: items}
		if hasRole(p.Roles, learning.RoleStudent) {
			job.StudentID, job.StudentName, job.GuardianID = p.StudentID, quote.StudentName, p.GuardianID
			job.Materials = quote.Materials
			job.CourseName = quote.Courses[0]
			course, _ := work.findCourse(scope.CourseIDs[0])
			job.Scope.Subject = course.Subject
		}
		job = cloneDownloadJob(job)
		work.materialDownloads = append([]learning.MaterialDownloadJob{job}, work.materialDownloads...)
		return cloneDownloadJob(job), nil
	})
}

func cloneDownloadJob(job learning.MaterialDownloadJob) learning.MaterialDownloadJob {
	job.Items = append([]learning.MaterialDownloadItem(nil), job.Items...)
	job.Materials = append([]learning.MaterialDownloadChoice(nil), job.Materials...)
	job.Scope.CourseIDs = cloneStrings(job.Scope.CourseIDs)
	job.Scope.LessonIDs = cloneStrings(job.Scope.LessonIDs)
	job.Scope.MaterialIDs = cloneStrings(job.Scope.MaterialIDs)
	return job
}

func publicDownloadJob(job learning.MaterialDownloadJob) learning.MaterialDownloadJob {
	job = cloneDownloadJob(job)
	if expiredDownload(job) {
		job.Status = "已过期"
	}
	return job
}
func expiredDownload(job learning.MaterialDownloadJob) bool {
	end, err := time.Parse(time.RFC3339, job.ExpiresAt)
	return err == nil && !time.Now().Before(end)
}

func (s *MemoryStore) MaterialDownloads(p learning.Principal) []learning.MaterialDownloadJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []learning.MaterialDownloadJob{}
	for _, job := range s.materialDownloads {
		if job.OwnerID == p.UserID && (job.StudentID == "" || job.StudentID == p.StudentID && job.GuardianID == p.GuardianID) {
			out = append(out, publicDownloadJob(job))
		}
	}
	return out
}

func (s *MemoryStore) materialDownloadFiles(p learning.Principal, job learning.MaterialDownloadJob) ([]learning.MaterialDownloadFile, error) {
	student := job.StudentID != ""
	if job.OwnerID != p.UserID || student && (job.StudentID != p.StudentID || job.GuardianID != p.GuardianID || !hasRole(p.Roles, learning.RoleStudent) || job.GuardianID != "" && !s.guardianStudentActive(job.GuardianID, job.StudentID)) || !student && (!p.CanDownloadTeacherMaterial() || !(p.IsTeacherOnly() || isPlanAdmin(p))) {
		return nil, errors.New("没有权限领取该下载包")
	}
	files := []learning.MaterialDownloadFile{}
	for _, item := range job.Items {
		var material learning.Material
		found := false
		for _, candidate := range s.materials {
			if candidate.ID == item.MaterialID {
				material = candidate
				found = true
				break
			}
		}
		course, ok := s.findCourse(material.CourseID)
		if !found || !ok || course.Status != learning.StatusEnabled || (!student && !s.canReadTeacherCourse(p, course)) || student && !s.studentBatchMaterialAllowed(p, material) || !materialVisibleToStudents(material) || material.FileID != item.FileID || material.LearningSpaceID != course.LearningSpaceID {
			return nil, errors.New("资料或访问权限已变化，请重新生成下载包")
		}
		asset, ok := s.fileAssets[item.FileID]
		if !ok {
			return nil, errors.New("原文件缺失，请重新生成下载包")
		}
		files = append(files, learning.MaterialDownloadFile{Name: item.Name, Path: asset.OriginalPath, Size: item.Size})
	}
	if len(files) != job.Count || len(files) == 0 {
		return nil, errors.New("下载包清单不完整，请重新生成")
	}
	return files, nil
}

func (s *MemoryStore) MaterialDownloadArchive(p learning.Principal, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.materialDownloads {
		if job.ID != id {
			continue
		}
		if _, err := s.materialDownloadFiles(p, job); err != nil {
			return "", err
		}
		if job.Status != "可下载" || expiredDownload(job) {
			return "", errors.New("下载包尚未就绪或已过期，请重新生成")
		}
		return job.ArchivePath, nil
	}
	return "", errors.New("下载任务不存在")
}

func (s *MemoryStore) RetryMaterialDownload(p learning.Principal, id string) (learning.MaterialDownloadScope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.materialDownloads {
		if job.ID == id && job.OwnerID == p.UserID && (job.StudentID == "" || job.StudentID == p.StudentID && job.GuardianID == p.GuardianID) {
			if job.Status != "失败" && job.Status != "已过期" && !expiredDownload(job) {
				return learning.MaterialDownloadScope{}, errors.New("只有失败或过期任务可以重新生成")
			}
			return cloneDownloadJob(job).Scope, nil
		}
	}
	return learning.MaterialDownloadScope{}, errors.New("下载任务不存在")
}

func (s *MemoryStore) RecoverMaterialDownloads() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		for i := range work.materialDownloads {
			if work.materialDownloads[i].Status == "打包中" {
				work.materialDownloads[i].Status = "准备中"
			}
		}
		return nil
	})
}

func (s *MemoryStore) ClaimMaterialDownload() (learning.MaterialDownloadJob, []learning.MaterialDownloadFile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type claimed struct {
		job   learning.MaterialDownloadJob
		files []learning.MaterialDownloadFile
		found bool
	}
	value, err := persistentMutation(s, func(work *MemoryStore) (claimed, error) {
		for i := len(work.materialDownloads) - 1; i >= 0; i-- {
			job := work.materialDownloads[i]
			if job.Status != "准备中" {
				continue
			}
			p, err := work.materialDownloadPrincipal(job)
			var files []learning.MaterialDownloadFile
			if err == nil {
				files, err = work.materialDownloadFiles(p, job)
			}
			if err != nil {
				work.materialDownloads[i].Status = "失败"
				work.materialDownloads[i].Error = "账号、资料或权限已变化，请重新生成"
				continue
			}
			work.materialDownloads[i].Status = "打包中"
			job.Status = "打包中"
			return claimed{cloneDownloadJob(job), files, true}, nil
		}
		return claimed{}, nil
	})
	return value.job, value.files, value.found, err
}

func (s *MemoryStore) FinishMaterialDownload(id, archivePath, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		for i, job := range work.materialDownloads {
			if job.ID != id || job.Status != "打包中" {
				continue
			}
			if reason != "" {
				work.materialDownloads[i].Status = "失败"
				work.materialDownloads[i].Error = reason
				return nil
			}
			p, err := work.materialDownloadPrincipal(job)
			if err == nil {
				_, err = work.materialDownloadFiles(p, job)
			}
			if err != nil {
				work.materialDownloads[i].Status = "失败"
				work.materialDownloads[i].Error = "资料或权限已变化，请重新生成"
				return err
			}
			work.materialDownloads[i].Status = "可下载"
			work.materialDownloads[i].ArchivePath = archivePath
			work.materialDownloads[i].ExpiresAt = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
			return nil
		}
		return fmt.Errorf("下载任务状态已变化")
	})
}

func (s *MemoryStore) ExpiredMaterialArchives() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []string{}
	for _, job := range s.materialDownloads {
		if (expiredDownload(job) || job.Status == "失败") && job.ArchivePath != "" {
			out = append(out, job.ArchivePath)
		}
	}
	return out, nil
}

// Clear the stored path only after deletion succeeds, allowing failed cleanup to retry.
func (s *MemoryStore) AcknowledgeMaterialArchiveRemoval(archivePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		for i, job := range work.materialDownloads {
			if job.ArchivePath == archivePath && (expiredDownload(job) || job.Status == "失败") {
				if expiredDownload(job) {
					work.materialDownloads[i].Status = "已过期"
				}
				work.materialDownloads[i].ArchivePath = ""
			}
		}
		return nil
	})
}

// A missing archive becomes retryable; only the owner can invalidate it.
func (s *MemoryStore) InvalidateMaterialDownload(p learning.Principal, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutationError(s, func(work *MemoryStore) error {
		for i, job := range work.materialDownloads {
			if job.ID == id && job.OwnerID == p.UserID && job.Status == "可下载" {
				work.materialDownloads[i].Status = "失败"
				work.materialDownloads[i].Error = "下载包文件不可用，请重新生成"
				return nil
			}
		}
		return errors.New("下载任务状态已变化")
	})
}

func (s *MemoryStore) studentBatchMaterialAllowed(p learning.Principal, material learning.Material) bool {
	resolved, err := s.studentMaterialUnlocked(p, material.ID)
	return err == nil && resolved.DownloadURL != "" && resolved.FileID == material.FileID
}
func (s *MemoryStore) materialDownloadPrincipal(job learning.MaterialDownloadJob) (learning.Principal, error) {
	p, err := s.principalByUserIDUnlocked(job.OwnerID)
	if err == nil && job.StudentID != "" {
		if p.StudentID != job.StudentID || job.GuardianID != "" && !s.guardianStudentActive(job.GuardianID, job.StudentID) {
			return p, errors.New("学生关系已变化")
		}
		p.GuardianID = job.GuardianID
	}
	return p, err
}
