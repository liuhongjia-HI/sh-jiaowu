package learning

// Nil policy preserves existing teachers' exact learning-space scope and capabilities.
type TeacherLibraryScope struct {
	Subject string `json:"subject"`
	Grade   string `json:"grade"` // empty means all grades
}
type TeacherLibraryPolicy struct {
	SpaceIDs          []string              `json:"spaceIds"`
	Scopes            []TeacherLibraryScope `json:"scopes"`
	CanDownload       bool                  `json:"canDownload"`
	CanManageCourses  bool                  `json:"canManageCourses"`
	CanViewDrafts     bool                  `json:"canViewDrafts"`
	RecentMaterialIDs []string              `json:"recentMaterialIds,omitempty"`
}
type TeacherLibrary struct {
	Policy            *TeacherLibraryPolicy `json:"policy,omitempty"`
	Courses           []Course              `json:"courses"`
	Materials         []Material            `json:"materials"`
	Spaces            []LearningSpace       `json:"spaces"`
	RecentMaterialIDs []string              `json:"recentMaterialIds"`
	CanDownload       bool                  `json:"canDownload"`
}

func (p Principal) IsTeacherOnly() bool {
	teacher := false
	for _, r := range p.Roles {
		if r == RoleSuperAdmin || r == RoleCampusAdmin || r == RoleOpsStaff {
			return false
		}
		if r == RoleTeacher {
			teacher = true
		}
	}
	return teacher
}
func (p Principal) CanDownloadTeacherMaterial() bool {
	return !p.IsTeacherOnly() || p.TeacherLibrary == nil || p.TeacherLibrary.CanDownload
}
func (p Principal) CanMaintainCourses() bool {
	return !p.IsTeacherOnly() || p.TeacherLibrary == nil || p.TeacherLibrary.CanManageCourses
}

func (p Principal) IsReadOnlyTeacher() bool {
	return p.IsTeacherOnly() && p.TeacherLibrary != nil && !p.TeacherLibrary.CanManageCourses && !p.CanUploadHandout && !p.CanUploadQuestion && !p.CanReview
}
