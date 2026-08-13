package store

import (
	"errors"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func (s *MemoryStore) grantPreviewUnlocked(studentID, packageID string) (learning.GrantPreview, error) {
	student, pkg, err := s.validateGrantTarget(studentID, packageID)
	if err != nil {
		return learning.GrantPreview{}, err
	}
	openCourses, openMaterials, openHomework := s.openContentForPackage(pkg)
	alreadyOpened, existingUntil := s.activeGrantState(student.ID, pkg.ID)
	return learning.GrantPreview{
		StudentID: student.ID, PackageID: pkg.ID, StudentName: student.Name, PackageName: pkg.Name,
		AlreadyOpened: alreadyOpened, ExistingUntil: existingUntil,
		LearningSpaces: s.learningSpaceNamesForPackage(pkg.ID), ContentTypes: s.contentTypeLabelsForPackage(pkg.ID),
		OpenCourses: openCourses, OpenMaterials: openMaterials, OpenHomework: openHomework,
		BlockedContent: s.blockedContentForPackage(pkg), EffectiveDefault: "今天起 365 天",
	}, nil
}

func (s *MemoryStore) createGrantUnlocked(operator, studentID, packageID string) (learning.GrantPreview, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.GrantPreview, error) {
			return work.createGrantUnlocked(operator, studentID, packageID)
		})
	}
	preview, err := s.grantPreviewUnlocked(studentID, packageID)
	if err != nil {
		return learning.GrantPreview{}, err
	}
	if !s.hasGrant(studentID, packageID) {
		startsAt := time.Now().Format("2006-01-02")
		endsAt := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
		grant := packageGrant{
			ID:             "grant-" + time.Now().Format("20060102150405"),
			StudentID:      studentID,
			PackageID:      packageID,
			StartsAt:       startsAt,
			EndsAt:         endsAt,
			Status:         "active",
			EffectiveUntil: endsAt,
		}
		s.grants = append(s.grants, grant)
		s.syncSpaceAccessForGrant(grant)
		s.addStudentOpenedPackage(studentID, preview.PackageName)
	}
	s.prependLog(operator, "开通套餐", preview.StudentName+" / "+preview.PackageName)
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
