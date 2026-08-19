package store

import (
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
		Status:         "active",
		EffectiveUntil: endsAt,
	}
	if index, ok := s.findGrantIndex(req.StudentID, req.PackageID); ok {
		grant.ID = s.grants[index].ID
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
		s.prependLog(operator, "调整套餐有效期", preview.StudentName+" / "+preview.PackageName)
	} else {
		s.prependLog(operator, "开通套餐", preview.StudentName+" / "+preview.PackageName)
	}
	preview.AlreadyOpened = true
	return preview, nil
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

// academicCalendarRange 读取系统设置里的校历学年起止，两个值都合法才生效。
func (s *MemoryStore) academicCalendarRange() (string, string, bool) {
	start := strings.TrimSpace(s.settings["academicYearStart"])
	end := strings.TrimSpace(s.settings["academicYearEnd"])
	if start == "" || end == "" {
		return "", "", false
	}
	if _, err := time.Parse("2006-01-02", start); err != nil {
		return "", "", false
	}
	if _, err := time.Parse("2006-01-02", end); err != nil {
		return "", "", false
	}
	if end < start {
		return "", "", false
	}
	return start, end, true
}

func (s *MemoryStore) normalizeGrantPeriod(startsAt, endsAt string) (string, string, error) {
	defaultStartsAt, defaultEndsAt := s.defaultGrantPeriod()
	if startsAt == "" {
		startsAt = defaultStartsAt
	}
	if endsAt == "" {
		endsAt = defaultEndsAt
	}
	start, err := time.Parse("2006-01-02", startsAt)
	if err != nil {
		return "", "", errors.New("请选择正确的开通开始日期")
	}
	end, err := time.Parse("2006-01-02", endsAt)
	if err != nil {
		return "", "", errors.New("请选择正确的开通结束日期")
	}
	if end.Before(start) {
		return "", "", errors.New("开通结束日期不能早于开始日期")
	}
	return startsAt, endsAt, nil
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
