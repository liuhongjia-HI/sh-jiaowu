package learningapp

import (
	"sort"
	"strings"

	"starline/learning-api/internal/domain/learning"
)

func (s *Service) Courses(p learning.Principal) []learning.Course { return s.content.Courses(p) }
func (s *Service) CreateCourse(o string, p learning.Principal, r learning.CourseUpsertRequest) (learning.Course, error) {
	return s.content.CreateCourse(o, p, r)
}
func (s *Service) CopyCourse(o string, p learning.Principal, id string, r learning.CourseCopyRequest) (learning.CourseCopyResult, error) {
	return s.content.CopyCourse(o, p, id, r)
}
func (s *Service) UpdateCourse(o string, p learning.Principal, id string, r learning.CourseUpsertRequest) (learning.Course, error) {
	return s.content.UpdateCourse(o, p, id, r)
}
func (s *Service) DeleteCourse(o string, p learning.Principal, id string) error {
	return s.content.DeleteCourse(o, p, id)
}
func (s *Service) Questions(p learning.Principal, q learning.QuestionBankQuery) []learning.QuestionBankItem {
	return s.content.Questions(p, q)
}
func (s *Service) CreateQuestion(o string, p learning.Principal, r learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	return s.content.CreateQuestion(o, p, r)
}
func (s *Service) UpdateQuestion(o string, p learning.Principal, id string, r learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	return s.content.UpdateQuestion(o, p, id, r)
}
func (s *Service) Materials(p learning.Principal, q learning.MaterialQuery) []learning.Material {
	return s.content.Materials(p, q)
}

func (s *Service) MaterialOverview(p learning.Principal, q learning.MaterialOverviewQuery) learning.MaterialOverview {
	q.Semester = strings.TrimSpace(q.Semester)
	q.Phase = strings.TrimSpace(q.Phase)
	q.Level = strings.TrimSpace(q.Level)
	q.TagCode = strings.TrimSpace(q.TagCode)
	spaces := make(map[string]learning.LearningSpace)
	for _, space := range s.grant.LearningSpaces() {
		spaces[space.ID] = space
	}
	courses := s.content.Courses(p)
	materials := s.content.Materials(p, learning.MaterialQuery{TagCode: q.TagCode})
	materialsByLesson := make(map[string][]learning.Material)
	unclassified := 0
	for _, item := range materials {
		if strings.TrimSpace(item.FileID) == "" || strings.TrimSpace(item.CourseID) == "" || strings.TrimSpace(item.LessonID) == "" {
			unclassified++
			continue
		}
		key := item.CourseID + "\x00" + item.LessonID
		materialsByLesson[key] = append(materialsByLesson[key], item)
	}

	type cellAccumulator struct {
		cell   learning.MaterialOverviewCell
		levels map[string]int
	}
	cells := make(map[string]*cellAccumulator)
	gradeSet, subjectSet := map[string]bool{}, map[string]bool{}
	semesterSet, phaseSet, levelSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	lessons := make([]learning.MaterialOverviewLesson, 0)
	overview := learning.MaterialOverview{}
	for _, course := range courses {
		space, ok := spaces[course.LearningSpaceID]
		if !ok || course.Status == learning.StatusDisabled {
			continue
		}
		semesterSet[space.Semester], phaseSet[space.Phase], levelSet[space.Level] = true, true, true
		if q.Semester != "" && space.Semester != q.Semester || q.Phase != "" && space.Phase != q.Phase || q.Level != "" && space.Level != q.Level {
			continue
		}
		children := make(map[string]bool)
		for _, node := range course.Curriculum {
			if node.ParentID != "" {
				children[node.ParentID] = true
			}
		}
		leafNodes := make([]learning.CurriculumNode, 0)
		for _, node := range course.Curriculum {
			if !children[node.ID] {
				leafNodes = append(leafNodes, node)
			}
		}
		if len(leafNodes) == 0 {
			continue
		}
		gradeSet[space.Grade], subjectSet[space.Subject] = true, true
		cellKey := space.Grade + "\x00" + space.Subject
		acc := cells[cellKey]
		if acc == nil {
			acc = &cellAccumulator{cell: learning.MaterialOverviewCell{Grade: space.Grade, Subject: space.Subject, LevelCounts: []learning.MaterialOverviewLevelCount{}}, levels: make(map[string]int)}
			cells[cellKey] = acc
		}
		for _, node := range leafNodes {
			key := course.ID + "\x00" + node.ID
			files := materialsByLesson[key]
			if files == nil {
				files = []learning.Material{}
			}
			path, _ := materialOverviewCurriculumPath(course.Curriculum, node.ID)
			lessons = append(lessons, learning.MaterialOverviewLesson{CourseID: course.ID, CourseName: course.Name, LearningSpaceID: course.LearningSpaceID, Grade: space.Grade, Subject: space.Subject, Semester: space.Semester, Phase: space.Phase, Level: space.Level, LessonID: node.ID, Curriculum: path, Materials: files})
			if len(files) == 0 {
				acc.cell.MissingLessonCount++
				overview.Summary.MissingLessonCount++
				continue
			}
			acc.cell.CoveredLessonCount++
			acc.cell.FileCount += len(files)
			acc.levels[space.Level] += len(files)
			overview.Summary.CoveredLessonCount++
			overview.Summary.FileCount += len(files)
		}
	}
	for _, acc := range cells {
		for level, count := range acc.levels {
			acc.cell.LevelCounts = append(acc.cell.LevelCounts, learning.MaterialOverviewLevelCount{Level: level, FileCount: count})
		}
		sort.Slice(acc.cell.LevelCounts, func(i, j int) bool { return acc.cell.LevelCounts[i].Level < acc.cell.LevelCounts[j].Level })
		overview.Cells = append(overview.Cells, acc.cell)
	}
	overview.Summary.UnclassifiedCount = unclassified
	overview.Grades = sortedMaterialOverviewKeys(gradeSet)
	overview.Subjects = sortedMaterialOverviewKeys(subjectSet)
	overview.Semesters = sortedMaterialOverviewKeys(semesterSet)
	overview.Phases = sortedMaterialOverviewKeys(phaseSet)
	overview.Levels = sortedMaterialOverviewKeys(levelSet)
	sort.Slice(overview.Cells, func(i, j int) bool {
		return overview.Cells[i].Grade+"\x00"+overview.Cells[i].Subject < overview.Cells[j].Grade+"\x00"+overview.Cells[j].Subject
	})
	sort.Slice(lessons, func(i, j int) bool {
		left, right := lessons[i], lessons[j]
		return left.Grade+"\x00"+left.Subject+"\x00"+left.Level+"\x00"+left.CourseName+"\x00"+left.LessonID < right.Grade+"\x00"+right.Subject+"\x00"+right.Level+"\x00"+right.CourseName+"\x00"+right.LessonID
	})
	overview.Lessons = lessons
	return overview
}

func sortedMaterialOverviewKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func materialOverviewCurriculumPath(nodes []learning.CurriculumNode, leafID string) (learning.CurriculumPath, bool) {
	byID := make(map[string]learning.CurriculumNode, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
	}
	path := learning.CurriculumPath{}
	seen := map[string]bool{}
	for node, ok := byID[leafID]; ok; node, ok = byID[node.ParentID] {
		if seen[node.ID] {
			return learning.CurriculumPath{}, false
		}
		seen[node.ID] = true
		switch node.Type {
		case learning.CurriculumUnit:
			path.Unit = node.Name
		case learning.CurriculumChapter:
			path.Chapter = node.Name
		case learning.CurriculumLesson:
			path.Lesson = node.Name
		}
		if node.ParentID == "" {
			break
		}
	}
	return path, true
}
func (s *Service) CreateMaterial(o string, p learning.Principal, r learning.MaterialUploadRequest) (learning.Material, error) {
	return s.content.CreateMaterial(o, p, r)
}
func (s *Service) PreviewMaterialSync(p learning.Principal, r learning.MaterialSyncRequest) (learning.MaterialSyncPreview, error) {
	return s.content.PreviewMaterialSync(p, r)
}
func (s *Service) SyncMaterials(o string, p learning.Principal, r learning.MaterialSyncRequest) (learning.MaterialSyncResult, error) {
	return s.content.SyncMaterials(o, p, r)
}
func (s *Service) UpdateMaterial(o string, p learning.Principal, id string, r learning.MaterialUpdateRequest) (learning.Material, error) {
	return s.content.UpdateMaterial(o, p, id, r)
}
func (s *Service) ReorderMaterials(o string, p learning.Principal, r learning.MaterialReorderRequest) error {
	return s.content.ReorderMaterials(o, p, r)
}
func (s *Service) ReorderHomework(o string, p learning.Principal, r learning.HomeworkReorderRequest) error {
	return s.content.ReorderHomework(o, p, r)
}
func (s *Service) DeleteMaterial(o string, p learning.Principal, id string) error {
	return s.content.DeleteMaterial(o, p, id)
}
func (s *Service) Homework(p learning.Principal) []learning.Homework { return s.content.Homework(p) }
func (s *Service) HomeworkSubmissions(p learning.Principal, id string) (learning.HomeworkSubmissionSummary, error) {
	return s.content.HomeworkSubmissions(p, id)
}
func (s *Service) CreateHomework(o string, p learning.Principal, r learning.HomeworkUploadRequest) (learning.Homework, error) {
	return s.content.CreateHomework(o, p, r)
}
func (s *Service) UpdateHomework(o string, p learning.Principal, id string, r learning.HomeworkUpdateRequest) (learning.Homework, error) {
	return s.content.UpdateHomework(o, p, id, r)
}
func (s *Service) DeleteHomework(o string, p learning.Principal, id string) error {
	return s.content.DeleteHomework(o, p, id)
}
func (s *Service) ContentFile(p learning.Principal, id string) (learning.FileAsset, error) {
	return s.content.ContentFile(p, id)
}
func (s *Service) RecoverPreviewJobs() error { return s.content.RecoverPreviewJobs() }
func (s *Service) ClaimPreviewJob() (learning.PreviewJob, bool, error) {
	return s.content.ClaimPreviewJob()
}
func (s *Service) PreviewJobFile(id string) (learning.FileAsset, error) {
	return s.content.PreviewJobFile(id)
}
func (s *Service) CompletePreviewJob(id string, result learning.PreviewResult) error {
	return s.content.CompletePreviewJob(id, result)
}
func (s *Service) FailPreviewJob(id, message string) error {
	return s.content.FailPreviewJob(id, message)
}
func (s *Service) MarkPreviewFileMissing(fileID, message string) error {
	return s.content.MarkPreviewFileMissing(fileID, message)
}
func (s *Service) RetryPreviewJob(operator string, principal learning.Principal, fileID string) error {
	return s.content.RetryPreviewJob(operator, principal, fileID)
}
func (s *Service) Reviews(p learning.Principal) []learning.Review { return s.content.Reviews(p) }
func (s *Service) AssignReview(o string, p learning.Principal, id string, r learning.ReviewAssignRequest) (learning.Review, error) {
	return s.content.AssignReview(o, p, id, r)
}
func (s *Service) CompleteReview(o string, p learning.Principal, id string, r learning.ReviewCompleteRequest) (learning.Submission, error) {
	return s.content.CompleteReview(o, p, id, r)
}

func (s *Service) TeacherLibrary(p learning.Principal) (learning.TeacherLibrary, error) {
	return s.content.TeacherLibrary(p)
}
func (s *Service) RecordTeacherMaterialView(p learning.Principal, id string) error {
	return s.content.RecordTeacherMaterialView(p, id)
}
