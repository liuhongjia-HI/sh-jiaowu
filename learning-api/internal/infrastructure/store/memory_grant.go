package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) grantPreviewUnlocked(studentID, packageID string) (learning.GrantPreview, error) {
	student, pkg, err := s.validateGrantTarget(studentID, packageID)
	if err != nil {
		return learning.GrantPreview{}, err
	}
	openCourses, openMaterials, openHomework := s.openContentForPackage(pkg)
	alreadyOpened, existingStartsAt, existingUntil := s.grantState(student.ID, pkg.ID)
	defaultStartsAt, defaultEndsAt := s.defaultGrantPeriod()
	return learning.GrantPreview{
		StudentID: student.ID, PackageID: pkg.ID, StudentName: student.Name, PackageName: pkg.Name,
		AlreadyOpened: alreadyOpened, ExistingStartsAt: existingStartsAt, ExistingUntil: existingUntil,
		LearningSpaces: s.learningSpaceNamesForPackage(pkg.ID), ContentTypes: s.contentTypeLabelsForPackage(pkg.ID),
		OpenCourses: openCourses, OpenMaterials: openMaterials, OpenHomework: openHomework,
		BlockedContent: s.blockedContentForPackage(pkg), EffectiveDefault: defaultStartsAt + " 至 " + defaultEndsAt,
		StartsAtDefault: defaultStartsAt, EndsAtDefault: defaultEndsAt,
	}, nil
}

func (s *MemoryStore) createGrantUnlocked(operator string, req learning.GrantCreateRequest) (learning.GrantPreview, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.GrantPreview, error) {
			return work.createGrantUnlocked(operator, req)
		})
	}
	return s.createGrantForPackageUnlocked(operator, req, "开通套餐", "调整套餐有效期")
}

func (s *MemoryStore) createGrantForPackageUnlocked(operator string, req learning.GrantCreateRequest, openAction, updateAction string) (learning.GrantPreview, error) {
	preview, err := s.grantPreviewUnlocked(req.StudentID, req.PackageID)
	if err != nil {
		return learning.GrantPreview{}, err
	}
	startsAt, endsAt, err := s.normalizeGrantPeriod(req.StartsAt, req.EndsAt)
	if err != nil {
		return learning.GrantPreview{}, err
	}
	grant := packageGrant{
		ID:             "grant-" + time.Now().Format("20060102150405"),
		StudentID:      req.StudentID,
		PackageID:      req.PackageID,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		OpenedAt:       time.Now().Format("2006-01-02 15:04:05"),
		Status:         "active",
		EffectiveUntil: endsAt,
	}
	if index, ok := s.findGrantIndex(req.StudentID, req.PackageID); ok {
		grant.ID = s.grants[index].ID
		grant.OpenedAt = s.grants[index].OpenedAt
		if grant.OpenedAt == "" {
			grant.OpenedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		s.grants[index] = grant
		s.replaceSpaceAccessForGrant(grant)
	} else {
		s.grants = append(s.grants, grant)
		s.syncSpaceAccessForGrant(grant)
	}
	s.addStudentOpenedPackage(req.StudentID, preview.PackageName)
	preview.ExistingStartsAt = startsAt
	preview.ExistingUntil = endsAt
	if preview.AlreadyOpened {
		s.prependLog(operator, updateAction, preview.StudentName+" / "+preview.PackageName)
	} else {
		s.prependLog(operator, openAction, preview.StudentName+" / "+preview.PackageName)
	}
	preview.AlreadyOpened = true
	return preview, nil
}

func (s *MemoryStore) createDirectGrantUnlocked(operator string, req learning.DirectGrantCreateRequest) (learning.DirectGrantResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.DirectGrantResult, error) {
			return work.createDirectGrantUnlocked(operator, req)
		})
	}
	student, ok := s.findStudent(req.StudentID)
	if !ok {
		return learning.DirectGrantResult{}, errors.New("student not found")
	}
	if strings.TrimSpace(student.AccountStatus) == "停用" {
		return learning.DirectGrantResult{}, errors.New("该学生账号已停用，请先恢复账号")
	}
	spaceIDs := uniqueTrimmed(req.LearningSpaceIDs)
	if len(spaceIDs) == 0 {
		return learning.DirectGrantResult{}, errors.New("请至少选择一个课程范围")
	}
	contentTypes := uniqueTrimmed(req.ContentTypeCodes)
	if len(contentTypes) == 0 {
		return learning.DirectGrantResult{}, errors.New("请至少选择一种学习内容")
	}
	for _, code := range contentTypes {
		if !validContentType(code) {
			return learning.DirectGrantResult{}, errors.New("内容类型不正确：" + code)
		}
	}

	spaces := make([]learningSpace, 0, len(spaceIDs))
	for _, id := range spaceIDs {
		space, exists := s.findLearningSpace(id)
		if !exists || space.Status != learning.StatusEnabled {
			return learning.DirectGrantResult{}, errors.New("课程范围不可用：" + id)
		}
		if space.Grade != student.Grade {
			return learning.DirectGrantResult{}, errors.New("不能给" + student.Grade + "学生开通" + space.Grade + "课程")
		}
		spaces = append(spaces, space)
	}

	result := learning.DirectGrantResult{
		StudentID: student.ID, StudentName: student.Name, ContentTypes: contentTypeLabels(contentTypes),
		LearningSpaces: s.learningSpaceNames(spaceIDs), OpenCourses: []string{}, OpenMaterials: []string{}, OpenHomework: []string{},
	}
	for _, space := range spaces {
		packageID, err := s.ensureDirectGrantPackage(student, space, contentTypes)
		if err != nil {
			return learning.DirectGrantResult{}, err
		}
		preview, err := s.createGrantForPackageUnlocked(operator, learning.GrantCreateRequest{
			StudentID: student.ID,
			PackageID: packageID,
			StartsAt:  req.StartsAt,
			EndsAt:    req.EndsAt,
		}, "开通学习内容", "调整学习内容有效期")
		if err != nil {
			return learning.DirectGrantResult{}, err
		}
		result.OpenCourses = appendUnique(result.OpenCourses, preview.OpenCourses...)
		result.OpenMaterials = appendUnique(result.OpenMaterials, preview.OpenMaterials...)
		result.OpenHomework = appendUnique(result.OpenHomework, preview.OpenHomework...)
	}
	return result, nil
}

func (s *MemoryStore) ensureDirectGrantPackage(student learning.Student, space learningSpace, contentTypes []string) (string, error) {
	packageID := directGrantPackageID(student.ID, space.ID)
	if existing, ok := s.findPackage(packageID); ok {
		mergedTypes := appendUnique(s.contentTypesForPackage(packageID), contentTypes...)
		s.replacePackageRelations(packageID, []string{space.ID}, mergedTypes)
		s.refreshSpaceAccessForPackage(packageID)
		return existing.ID, nil
	}
	pkg, err := s.packageFromRequest(packageID, learning.PackageUpsertRequest{
		Name:             space.Name + " · 直接开通",
		AcademicYear:     s.configuredAcademicYear(),
		Grade:            space.Grade,
		Semester:         space.Semester,
		Subject:          space.Subject,
		Level:            space.Level,
		PhaseScope:       space.Phase,
		PackageType:      packageTypeLabel(contentTypes),
		Summary:          "由学生管理直接开通的学习内容。",
		LearningSpaceIDs: []string{space.ID},
		ContentTypeCodes: contentTypes,
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		return "", err
	}
	s.packages = append(s.packages, pkg)
	s.replacePackageRelations(pkg.ID, []string{space.ID}, contentTypes)
	return pkg.ID, nil
}

func directGrantPackageID(studentID, learningSpaceID string) string {
	sum := sha256.Sum256([]byte(studentID + "\x00" + learningSpaceID))
	return "direct-" + hex.EncodeToString(sum[:20])
}

func isDirectGrantPackage(packageID string) bool {
	return strings.HasPrefix(packageID, "direct-")
}

func uniqueTrimmed(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = appendUnique(result, value)
		}
	}
	return result
}

func contentTypeLabels(values []string) []string {
	labels := make([]string, 0, len(values))
	for _, value := range values {
		labels = appendUnique(labels, contentTypeLabel(value))
	}
	return labels
}

func (s *MemoryStore) validateGrantTarget(studentID, packageID string) (learning.Student, learning.Package, error) {
	student, ok := s.findStudent(studentID)
	if !ok {
		return learning.Student{}, learning.Package{}, errors.New("student not found")
	}
	pkg, ok := s.findPackage(packageID)
	if !ok {
		return learning.Student{}, learning.Package{}, errors.New("package not found")
	}
	if pkg.Status != learning.StatusEnabled {
		return learning.Student{}, learning.Package{}, errors.New("该套餐当前未启用，不能开通")
	}
	if strings.TrimSpace(student.AccountStatus) == "停用" {
		return learning.Student{}, learning.Package{}, errors.New("该学生账号已停用，请先恢复账号")
	}
	if student.Grade != pkg.Grade {
		return learning.Student{}, learning.Package{}, errors.New("该套餐适用于" + pkg.Grade + "，不能给" + student.Grade + "学生开通")
	}
	return student, pkg, nil
}

func (s *MemoryStore) studentPermissionsUnlocked() []learning.StudentPermissionSummary {
	out := make([]learning.StudentPermissionSummary, 0, len(s.students))
	for _, student := range s.students {
		out = append(out, s.permissionForStudent(student))
	}
	return out
}

func (s *MemoryStore) packagePermissionsUnlocked() []learning.PackagePermissionSummary {
	out := make([]learning.PackagePermissionSummary, 0, len(s.packages))
	for _, pkg := range s.packages {
		students := make([]string, 0)
		for _, grant := range s.grants {
			if grant.PackageID != pkg.ID || !grantActive(grant) {
				continue
			}
			student, ok := s.findStudent(grant.StudentID)
			if ok {
				students = appendUnique(students, student.Name)
			}
		}
		courses, materials, homework := s.openContentForPackage(pkg)
		out = append(out, learning.PackagePermissionSummary{
			PackageID: pkg.ID, PackageName: pkg.Name, Status: pkg.Status, OpenedStudents: len(students),
			Students: students, LearningSpaces: s.learningSpaceNamesForPackage(pkg.ID), ContentTypes: s.contentTypeLabelsForPackage(pkg.ID),
			OpenCourses: courses, OpenMaterials: materials, OpenHomework: homework,
		})
	}
	return out
}

// defaultGrantPeriod 返回新开通套餐的默认有效期。
// 优先跟随系统设置里的校历学年：开通日若已在学年内就从当天算起，否则从学年开始日算起，
// 统一到学年结束日到期——这样不管几月开通，同一学年的套餐一起到期，
// 和升年级、续费节奏对得上，而不是各自“从开通日起一年”。
// 校历没配或配错时退回原来的“今天 → 今天+1 年”，不阻塞开通。
// 单个学生的有效期仍可在开通时显式传入覆盖，这里只提供默认值。
func (s *MemoryStore) defaultGrantPeriod() (string, string) {
	now := time.Now()
	today := now.Format("2006-01-02")
	if _, end, ok := s.academicCalendarRange(); ok && today <= end {
		// 开通即生效：起始日始终是当天，不用学年开始日。
		// 学年还没开学时若从开学日起算，八月付费的学生要等到九月才能看到内容。
		// 校历约束的是“到期日”——同一学年开通的套餐统一在学年末到期，
		// 和升年级、续费节奏对齐。
		return today, end
	}
	// 校历没配、配错，或本学年结束日已经过去（管理端忘了更新校历）时的兜底，
	// 避免新开通的套餐一上来就是过期的。
	return today, now.AddDate(1, 0, 0).Format("2006-01-02")
}

// academicCalendarRange 汇总当前学年在校历里配置的全部学期，
// 返回覆盖这些学期的起止范围（最早的开学日 到 最晚的学期结束日）。
// 校历按学年、按学期存（见 academicCalendarTerm），不是一个学年只有一对笼统日期，
// 这里只是把“当前学年”对应的若干条学期记录合并成一个区间给开通逻辑用。
func (s *MemoryStore) academicCalendarRange() (string, string, bool) {
	terms := s.academicCalendarTerms()
	if len(terms) == 0 {
		return "", "", false
	}
	currentYear := s.configuredAcademicYear()
	start, end := "", ""
	for _, term := range terms {
		if term.AcademicYear != currentYear {
			continue
		}
		if _, err := time.Parse("2006-01-02", term.StartDate); err != nil {
			continue
		}
		if _, err := time.Parse("2006-01-02", term.EndDate); err != nil {
			continue
		}
		if start == "" || term.StartDate < start {
			start = term.StartDate
		}
		if end == "" || term.EndDate > end {
			end = term.EndDate
		}
	}
	if start == "" || end == "" || end < start {
		return "", "", false
	}
	return start, end, true
}

// academicCalendarTerms 解析系统设置里的校历列表，解析失败时返回空列表
// （调用方会按“校历未配置”处理，退回兜底逻辑，不会因为一条脏数据阻塞开通）。
func (s *MemoryStore) academicCalendarTerms() []academicCalendarTerm {
	raw := strings.TrimSpace(s.settings["academicCalendar"])
	if raw == "" {
		return nil
	}
	var terms []academicCalendarTerm
	if err := json.Unmarshal([]byte(raw), &terms); err != nil {
		return nil
	}
	return terms
}

// findCalendarTermForDate 在系统设置的校历里找覆盖给定日期的学期条目，找不到
// 时返回 false。resolveScheduleTerm（排课学年判定）和 configuredAcademicYear
// （系统当前学年）共用这一份匹配逻辑，确保“现在是哪个学年”和“某天属于哪个
// 学年”是同一套口径，不会出现两套并行的判定规则。
func (s *MemoryStore) findCalendarTermForDate(date string) (academicCalendarTerm, bool) {
	target := strings.TrimSpace(date)
	if _, err := time.Parse("2006-01-02", target); err != nil {
		return academicCalendarTerm{}, false
	}
	for _, term := range s.academicCalendarTerms() {
		if _, err := time.Parse("2006-01-02", term.StartDate); err != nil {
			continue
		}
		if _, err := time.Parse("2006-01-02", term.EndDate); err != nil {
			continue
		}
		if term.StartDate <= target && target <= term.EndDate {
			return term, true
		}
	}
	return academicCalendarTerm{}, false
}

// resolveScheduleTerm 按开课日期落校历，判定这个班归属哪个学年、哪个学期。
// 只在建班那一刻判定一次，判定结果随排课记录固定下来——校历日后调整、
// 学年切换，都不会改变已排课程的学年归属（否则历史排课的统计会跟着漂移）。
// fallbackSemester 通常来自课程所属学习空间的学期（S1/S2），在开课日落不进
// 任何校历学期时兜底使用，例如尚未配置该学年校历，或者是校历没有覆盖的假期班。
func (s *MemoryStore) resolveScheduleTerm(startDate, fallbackSemester string) (academicYear, semester string) {
	if term, ok := s.findCalendarTermForDate(startDate); ok {
		return term.AcademicYear, term.Semester
	}
	start, err := time.Parse("2006-01-02", strings.TrimSpace(startDate))
	if err != nil {
		// 没有可用的开课日期（理论上建班已校验过），退回当前时间点判定，
		// 保证这个函数总有确定的返回值。
		return currentAcademicYear(), fallbackSemester
	}
	// 校历没覆盖到（未配置该学年校历，或是假期班）：学期退回课程自带的 S1/S2
	// 标签，学年按开课日本身的 7 月 1 日规则推算——而不是按“现在”推算，避免
	// 补排过去或未来日期的课时学年算错。不阻塞建班，但调用方应当把这种情况
	// 回显给教务，提示去补校历。
	return academicYearForDate(start), fallbackSemester
}

func (s *MemoryStore) normalizeGrantPeriod(startsAt, endsAt string) (string, string, error) {
	defaultStartsAt, defaultEndsAt := s.defaultGrantPeriod()
	if startsAt == "" {
		startsAt = defaultStartsAt
	}
	if endsAt == "" {
		endsAt = defaultEndsAt
	}
	start, normalizedStart, err := normalizeGrantTimestamp(startsAt, false)
	if err != nil {
		return "", "", errors.New("请选择正确的开通开始时间")
	}
	end, normalizedEnd, err := normalizeGrantTimestamp(endsAt, true)
	if err != nil {
		return "", "", errors.New("请选择正确的开通结束时间")
	}
	if end.Before(start) {
		return "", "", errors.New("开通结束时间不能早于开始时间")
	}
	return normalizedStart, normalizedEnd, nil
}

func normalizeGrantTimestamp(value string, endOfDay bool) (time.Time, string, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.ParseInLocation("2006-01-02T15:04", value, time.Local); err == nil {
		return parsed, parsed.Format("2006-01-02 15:04:05"), nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed, parsed.Format("2006-01-02 15:04:05"), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, "", err
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Second)
	}
	return parsed, value, nil
}

func (s *MemoryStore) contentPermissionsUnlocked() []learning.ContentPermissionSummary {
	out := make([]learning.ContentPermissionSummary, 0, len(s.courses)+len(s.materials)+len(s.homework))
	for _, course := range s.courses {
		packages, students := s.audienceForContent(course.LearningSpaceID, "course")
		out = append(out, learning.ContentPermissionSummary{
			ContentID: course.ID, ContentTitle: course.Name, ContentType: "课程", Course: course.Name,
			LearningSpace: s.learningSpaceName(course.LearningSpaceID), Status: string(course.Status),
			OpenedPackages: packages, OpenedStudents: students,
		})
	}
	for _, material := range s.materials {
		packages, students := s.audienceForContent(material.LearningSpaceID, "handout")
		out = append(out, learning.ContentPermissionSummary{
			ContentID: material.ID, ContentTitle: material.Title, ContentType: material.Type, Course: material.Course,
			LearningSpace: s.learningSpaceName(material.LearningSpaceID), OwnerTeacherName: material.OwnerTeacherName,
			Status: string(material.Status), OpenedPackages: packages, OpenedStudents: students,
		})
	}
	for _, item := range s.homework {
		packages, students := s.audienceForContent(item.LearningSpaceID, "question")
		out = append(out, learning.ContentPermissionSummary{
			ContentID: item.ID, ContentTitle: item.Title, ContentType: "小挑战", Course: item.Course,
			LearningSpace: s.learningSpaceName(item.LearningSpaceID), OwnerTeacherName: item.OwnerTeacherName,
			Status: item.Status, OpenedPackages: packages, OpenedStudents: students,
		})
	}
	return out
}
