package store

import (
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) dashboardUnlocked() learning.DashboardOverview {
	views := 0
	for _, material := range s.materials {
		views += material.ViewCount
	}
	openedStudents := map[string]bool{}
	for _, grant := range s.grants {
		if grantActive(grant) {
			openedStudents[grant.StudentID] = true
		}
	}
	pendingReviews := 0
	for _, review := range s.reviews {
		if review.Status != "已批改" {
			pendingReviews++
		}
	}
	unpublishedFiles := 0
	for _, material := range s.materials {
		if material.PublishStatus != "已发布" && material.Status != learning.StatusEnabled {
			unpublishedFiles++
		}
	}
	for _, homework := range s.homework {
		if homework.PublishStatus != "已发布" && homework.Status != string(learning.StatusEnabled) {
			unpublishedFiles++
		}
	}
	return learning.DashboardOverview{
		OpenedStudents:   len(openedStudents),
		PackageCount:     len(s.packages),
		PendingReviews:   pendingReviews,
		MaterialViews:    views,
		ExpiringStudents: s.expiringStudentCount(30),
		UnpublishedFiles: unpublishedFiles,
	}
}

func (s *MemoryStore) expiringStudentCount(days int) int {
	today := time.Now()
	deadline := today.AddDate(0, 0, days)
	students := map[string]bool{}
	for _, grant := range s.grants {
		if !grantActive(grant) {
			continue
		}
		endsAt, err := time.Parse("2006-01-02", grantEndsAt(grant))
		if err != nil {
			continue
		}
		if !endsAt.Before(today) && !endsAt.After(deadline) {
			students[grant.StudentID] = true
		}
	}
	return len(students)
}

func (s *MemoryStore) systemReadinessUnlocked() learning.SystemReadiness {
	items := []learning.ReadinessItem{
		readinessFromConfirmedSetting(
			"miniProgramDomainStatus",
			"小程序域名备案",
			s.settings["miniProgramDomainStatus"],
			"已确认小程序业务域名和服务器域名可用于开发、体验和发布。",
			"在微信公众平台完成域名备案和合法域名配置后，把系统设置改为“已完成”。",
		),
		readinessFromDomain(s.settings["productionApiDomain"]),
		readinessFromConfirmedSetting(
			"officialAccountBindingStatus",
			"微信公众号关联",
			s.settings["officialAccountBindingStatus"],
			"已确认小程序主体和公众号关联关系，可用于后续统一触达。",
			"在微信公众平台完成公众号关联后，把系统设置改为“已完成”。",
		),
		readinessFromConfirmedSetting(
			"templateMessageStatus",
			"模板消息审核",
			s.settings["templateMessageStatus"],
			"已确认课程提醒、练习发布、批改完成等模板可用。",
			"在公众号后台完成模板申请/审核后，把系统设置改为“已完成”。",
		),
		s.officialAccountConfigReadiness(),
		s.studentOfficialAccountReadiness(),
	}
	ready := 0
	for _, item := range items {
		if item.Status == "ready" {
			ready++
		}
	}
	return learning.SystemReadiness{ReadyCount: ready, TotalCount: len(items), Items: items}
}

func (s *MemoryStore) packagesUnlocked() []learning.Package {
	out := make([]learning.Package, 0, len(s.packages))
	for _, pkg := range s.packages {
		if isDirectGrantPackage(pkg.ID) {
			continue
		}
		out = append(out, s.decoratePackage(pkg))
	}
	return out
}

func (s *MemoryStore) createPackageUnlocked(operator string, req learning.PackageUpsertRequest) (learning.Package, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Package, error) { return work.createPackageUnlocked(operator, req) })
	}
	pkg, err := s.packageFromRequest("", req)
	if err != nil {
		return learning.Package{}, err
	}
	if s.packageNameExists("", pkg.Name) {
		return learning.Package{}, errors.New("学习套餐名称已存在")
	}
	pkg.ID = "pkg-custom-" + time.Now().Format("20060102150405.000000000")
	s.packages = append([]learning.Package{pkg}, s.packages...)
	s.replacePackageRelations(pkg.ID, req.LearningSpaceIDs, req.ContentTypeCodes)
	s.prependLog(operator, "创建学习套餐", pkg.Name)
	return s.decoratePackage(pkg), nil
}

func (s *MemoryStore) updatePackageUnlocked(operator string, id string, req learning.PackageUpsertRequest) (learning.Package, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.Package, error) {
			return work.updatePackageUnlocked(operator, id, req)
		})
	}
	id = strings.TrimSpace(id)
	pkg, err := s.packageFromRequest(id, req)
	if err != nil {
		return learning.Package{}, err
	}
	if s.packageNameExists(id, pkg.Name) {
		return learning.Package{}, errors.New("学习套餐名称已存在")
	}
	for index := range s.packages {
		if s.packages[index].ID != id {
			continue
		}
		before := s.decoratePackage(s.packages[index])
		s.packages[index] = pkg
		s.replacePackageRelations(id, req.LearningSpaceIDs, req.ContentTypeCodes)
		s.refreshSpaceAccessForPackage(id)
		after := s.decoratePackage(pkg)
		s.prependLogDetail(operator, "编辑学习套餐", pkg.Name, auditChangeDetail(packageAuditSnapshot(before), packageAuditSnapshot(after)))
		return after, nil
	}
	return learning.Package{}, errors.New("学习套餐不存在")
}

func (s *MemoryStore) learningSpacesUnlocked() []learning.LearningSpace {
	out := make([]learning.LearningSpace, 0, len(s.learningSpaces))
	for _, space := range s.learningSpaces {
		out = append(out, learning.LearningSpace{
			ID:           space.ID,
			AcademicYear: space.AcademicYear,
			Grade:        space.Grade,
			Subject:      space.Subject,
			Semester:     space.Semester,
			Phase:        space.Phase,
			Level:        space.Level,
			Name:         space.Name,
			Status:       space.Status,
		})
	}
	return out
}
