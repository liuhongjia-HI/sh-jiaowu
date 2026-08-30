package store

import (
	"errors"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

const trialDurationDays = 7

func trialNow() time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Now()
	}
	return time.Now().In(location)
}

func trialToday() string { return trialNow().Format("2006-01-02") }

func (s *MemoryStore) startStudentTrialUnlocked(principal learning.Principal, packageID string) (learning.StudentTrialStartResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.StudentTrialStartResult, error) {
			return work.startStudentTrialUnlocked(principal, packageID)
		})
	}
	if principal.StudentID == "" {
		return learning.StudentTrialStartResult{}, errors.New("请先完成学生账号绑定")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return learning.StudentTrialStartResult{}, errors.New("student not found")
	}
	if record, ok := s.findTrialRecord(student.ID, s.configuredAcademicYear()); ok {
		trial := s.studentTrialForRecord(record)
		if trial.State == "active" || trial.State == "expiring" {
			return learning.StudentTrialStartResult{Trial: trial, FirstCourseID: s.firstCourseIDForPackage(record.PackageID)}, nil
		}
		return learning.StudentTrialStartResult{}, errors.New("本学年体验已结束，不能再次领取")
	}
	if s.hasActiveFormalGrant(student.ID) {
		return learning.StudentTrialStartResult{}, errors.New("已开通学习套餐，无需领取体验")
	}
	pkg, err := s.trialPackageForStudent(student, packageID)
	if err != nil {
		return learning.StudentTrialStartResult{}, err
	}
	now := trialNow()
	record := studentTrialRecord{
		ID:           "trial-" + now.Format("20060102150405.000000000"),
		StudentID:    student.ID,
		AcademicYear: s.configuredAcademicYear(),
		PackageID:    pkg.ID,
		StartsAt:     now.Format("2006-01-02"),
		EndsAt:       now.AddDate(0, 0, trialDurationDays-1).Format("2006-01-02"),
		Status:       "active",
	}
	if _, err := s.createGrantForPackageUnlocked("学生体验", learning.GrantCreateRequest{
		StudentID: student.ID,
		PackageID: pkg.ID,
		StartsAt:  record.StartsAt,
		EndsAt:    record.EndsAt,
	}, "开启学生体验", "调整学生体验"); err != nil {
		return learning.StudentTrialStartResult{}, err
	}
	s.trials = append(s.trials, record)
	s.prependLog("学生体验", "领取免费体验", student.Name+" / "+pkg.Name)
	return learning.StudentTrialStartResult{Trial: s.studentTrialForRecord(record), FirstCourseID: s.firstCourseIDForPackage(pkg.ID)}, nil
}

// startDefaultStudentTrialUnlocked 在首次建档时自动领取体验。运营通过套餐的
// TrialEnabled 配置决定哪些内容可体验；同一年级若配置了多个体验套餐，按套餐 ID
// 稳定选择第一个，避免请求顺序影响用户看到的体验内容。
func (s *MemoryStore) startDefaultStudentTrialUnlocked(principal learning.Principal) error {
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return errors.New("student not found")
	}
	options := s.trialOptionsForStudent(student, s.configuredAcademicYear())
	if len(options) == 0 {
		return nil
	}
	sort.Slice(options, func(i, j int) bool { return options[i].PackageID < options[j].PackageID })
	_, err := s.startStudentTrialUnlocked(principal, options[0].PackageID)
	return err
}

func (s *MemoryStore) hasActiveFormalGrant(studentID string) bool {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.Status != "revoked" && grantActive(grant) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) studentTrialUnlocked(student learning.Student) learning.StudentTrial {
	academicYear := s.configuredAcademicYear()
	if record, ok := s.findTrialRecord(student.ID, academicYear); ok {
		return s.studentTrialForRecord(record)
	}
	options := s.trialOptionsForStudent(student, academicYear)
	if len(options) == 0 {
		return learning.StudentTrial{State: "unavailable"}
	}
	return learning.StudentTrial{State: "eligible", RemainingDays: trialDurationDays, Options: options}
}

func (s *MemoryStore) findTrialRecord(studentID, academicYear string) (studentTrialRecord, bool) {
	for _, record := range s.trials {
		if record.StudentID == studentID && record.AcademicYear == academicYear {
			return record, true
		}
	}
	return studentTrialRecord{}, false
}

func (s *MemoryStore) studentTrialForRecord(record studentTrialRecord) learning.StudentTrial {
	pkg, _ := s.findPackage(record.PackageID)
	trial := learning.StudentTrial{ID: record.ID, PackageID: record.PackageID, PackageName: pkg.Name, Subject: pkg.Subject, StartedAt: record.StartsAt, EndsAt: record.EndsAt}
	if record.Status == "converted" {
		trial.State = "converted"
		return trial
	}
	today := trialToday()
	if record.EndsAt < today {
		trial.State = "expired"
		return trial
	}
	trial.State = "active"
	start, startErr := time.ParseInLocation("2006-01-02", today, trialNow().Location())
	end, endErr := time.ParseInLocation("2006-01-02", record.EndsAt, trialNow().Location())
	if startErr == nil && endErr == nil {
		trial.RemainingDays = int(end.Sub(start).Hours()/24) + 1
	}
	if trial.RemainingDays <= 2 {
		trial.State = "expiring"
	}
	return trial
}

func (s *MemoryStore) trialOptionsForStudent(student learning.Student, academicYear string) []learning.StudentTrialOption {
	options := make([]learning.StudentTrialOption, 0)
	for _, pkg := range s.packages {
		if !pkg.TrialEnabled || pkg.Status != learning.StatusEnabled || pkg.AcademicYear != academicYear || pkg.Grade != student.Grade {
			continue
		}
		if _, err := s.trialPackageForStudent(student, pkg.ID); err != nil {
			continue
		}
		options = append(options, learning.StudentTrialOption{PackageID: pkg.ID, PackageName: pkg.Name, Subject: pkg.Subject})
	}
	return options
}

func (s *MemoryStore) trialPackageForStudent(student learning.Student, packageID string) (learning.Package, error) {
	pkg, ok := s.findPackage(strings.TrimSpace(packageID))
	if !ok {
		return learning.Package{}, errors.New("体验套餐不存在")
	}
	if !pkg.TrialEnabled {
		return learning.Package{}, errors.New("该套餐暂不提供体验")
	}
	if pkg.AcademicYear != s.configuredAcademicYear() {
		return learning.Package{}, errors.New("该套餐不属于当前学年")
	}
	if _, _, err := s.validateGrantTarget(student.ID, pkg.ID); err != nil {
		return learning.Package{}, err
	}
	if !containsString(s.contentTypesForPackage(pkg.ID), "course") || !containsString(s.contentTypesForPackage(pkg.ID), "question") {
		return learning.Package{}, errors.New("体验套餐需包含课程和练习")
	}
	courses, _, homework := s.openContentForPackage(pkg)
	if len(courses) == 0 || len(homework) == 0 {
		return learning.Package{}, errors.New("体验套餐内容尚未准备完整")
	}
	return pkg, nil
}

func (s *MemoryStore) validateTrialPackageConfig(pkg learning.Package, learningSpaceIDs, contentTypeCodes []string) error {
	if !pkg.TrialEnabled {
		return nil
	}
	if pkg.Status != learning.StatusEnabled {
		return errors.New("体验套餐必须处于启用状态")
	}
	if !containsString(contentTypeCodes, "course") || !containsString(contentTypeCodes, "question") {
		return errors.New("体验套餐需同时包含课程和练习")
	}
	hasCourse, hasHomework := false, false
	for _, course := range s.courses {
		if course.Status == learning.StatusEnabled && containsString(learningSpaceIDs, course.LearningSpaceID) {
			hasCourse = true
			break
		}
	}
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && containsString(learningSpaceIDs, item.LearningSpaceID) {
			hasHomework = true
			break
		}
	}
	if !hasCourse || !hasHomework {
		return errors.New("体验套餐内容尚未准备完整，请先发布课程和练习")
	}
	return nil
}

func (s *MemoryStore) firstCourseIDForPackage(packageID string) string {
	pkg, ok := s.findPackage(packageID)
	if !ok {
		return ""
	}
	for _, course := range s.courses {
		if course.Status == learning.StatusEnabled && s.packageOpensContent(pkg, course.LearningSpaceID, "course") {
			return course.ID
		}
	}
	return ""
}

// trialFirstChapterForGrant 只在授权仍完全等同于体验记录时返回首章。
// 后台后续把同一套餐正式开通后会更新授权有效期，此时不再把正式权限误限为体验权限。
func (s *MemoryStore) trialFirstChapterForGrant(grant packageGrant, courseID string) (string, bool) {
	record, ok := s.findTrialRecord(grant.StudentID, s.configuredAcademicYear())
	if !ok || record.Status != "active" || record.PackageID != grant.PackageID || record.StartsAt != grant.StartsAt || record.EndsAt != grantEndsAt(grant) {
		return "", false
	}
	for _, course := range s.courses {
		if course.ID != courseID {
			continue
		}
		for _, chapter := range course.Chapters {
			if chapter = strings.TrimSpace(chapter); chapter != "" {
				return chapter, true
			}
		}
		return "", false
	}
	return "", false
}
