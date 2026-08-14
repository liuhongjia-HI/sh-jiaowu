package learning

type Package struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	AcademicYear     string   `json:"academicYear"`
	Grade            string   `json:"grade"`
	Semester         string   `json:"semester"`
	Subject          string   `json:"subject"`
	PhaseScope       string   `json:"phaseScope"`
	PackageType      string   `json:"packageType"`
	Summary          string   `json:"summary"`
	LearningSpaceIDs []string `json:"learningSpaceIds,omitempty"`
	LearningSpaces   []string `json:"learningSpaces,omitempty"`
	ContentTypeCodes []string `json:"contentTypeCodes,omitempty"`
	ContentTypes     []string `json:"contentTypes,omitempty"`
	OpenStudentNum   int      `json:"openStudentNum"`
	Status           Status   `json:"status"`
}

type PackageUpsertRequest struct {
	Name             string   `json:"name"`
	AcademicYear     string   `json:"academicYear"`
	Grade            string   `json:"grade"`
	Semester         string   `json:"semester"`
	Subject          string   `json:"subject"`
	PhaseScope       string   `json:"phaseScope"`
	PackageType      string   `json:"packageType"`
	Summary          string   `json:"summary"`
	LearningSpaceIDs []string `json:"learningSpaceIds"`
	ContentTypeCodes []string `json:"contentTypeCodes"`
	Status           Status   `json:"status"`
}

// StudentPackageRecommendation 是学生端可见的未开通套餐摘要。
// 仅返回用于了解套餐的信息，不授予课程或资料访问权限。
type StudentPackageRecommendation struct {
	PackageID            string   `json:"packageId"`
	PackageName          string   `json:"packageName"`
	AcademicYear         string   `json:"academicYear"`
	Grade                string   `json:"grade"`
	Semester             string   `json:"semester"`
	Subject              string   `json:"subject"`
	Summary              string   `json:"summary"`
	LearningSpaces       []string `json:"learningSpaces"`
	CourseCount          int      `json:"courseCount"`
	MaterialCount        int      `json:"materialCount"`
	ContentSamples       []string `json:"contentSamples"`
	RecommendationReason string   `json:"recommendationReason"`
	SameLearningSpace    bool     `json:"sameLearningSpace"`
}

type StudentGrant struct {
	StudentID       string `json:"studentId"`
	PackageID       string `json:"packageId"`
	PackageName     string `json:"packageName"`
	StartsAt        string `json:"startsAt"`
	EffectiveUntil  string `json:"effectiveUntil"`
	PermissionState string `json:"permissionState"`
}

type GrantCreateRequest struct {
	StudentID string `json:"studentId"`
	PackageID string `json:"packageId"`
	StartsAt  string `json:"startsAt"`
	EndsAt    string `json:"endsAt"`
}

type GrantPreview struct {
	StudentID        string   `json:"studentId"`
	PackageID        string   `json:"packageId"`
	StudentName      string   `json:"studentName"`
	PackageName      string   `json:"packageName"`
	AlreadyOpened    bool     `json:"alreadyOpened"`
	ExistingStartsAt string   `json:"existingStartsAt,omitempty"`
	ExistingUntil    string   `json:"existingUntil,omitempty"`
	LearningSpaces   []string `json:"learningSpaces"`
	ContentTypes     []string `json:"contentTypes"`
	OpenCourses      []string `json:"openCourses"`
	OpenMaterials    []string `json:"openMaterials"`
	OpenHomework     []string `json:"openHomework"`
	BlockedContent   []string `json:"blockedContent"`
	EffectiveDefault string   `json:"effectiveDefault"`
	StartsAtDefault  string   `json:"startsAtDefault"`
	EndsAtDefault    string   `json:"endsAtDefault"`
}

type StudentPermissionSummary struct {
	StudentID       string   `json:"studentId"`
	StudentName     string   `json:"studentName"`
	Grade           string   `json:"grade"`
	AccountStatus   string   `json:"accountStatus"`
	OpenedPackages  []string `json:"openedPackages"`
	LearningSpaces  []string `json:"learningSpaces"`
	ContentTypes    []string `json:"contentTypes"`
	OpenCourses     []string `json:"openCourses"`
	OpenMaterials   []string `json:"openMaterials"`
	OpenHomework    []string `json:"openHomework"`
	EffectiveUntil  string   `json:"effectiveUntil"`
	PermissionState string   `json:"permissionState"`
}

type PackagePermissionSummary struct {
	PackageID      string   `json:"packageId"`
	PackageName    string   `json:"packageName"`
	Status         Status   `json:"status"`
	OpenedStudents int      `json:"openedStudents"`
	Students       []string `json:"students"`
	LearningSpaces []string `json:"learningSpaces"`
	ContentTypes   []string `json:"contentTypes"`
	OpenCourses    []string `json:"openCourses"`
	OpenMaterials  []string `json:"openMaterials"`
	OpenHomework   []string `json:"openHomework"`
}

type ContentPermissionSummary struct {
	ContentID        string   `json:"contentId"`
	ContentTitle     string   `json:"contentTitle"`
	ContentType      string   `json:"contentType"`
	Course           string   `json:"course"`
	LearningSpace    string   `json:"learningSpace"`
	OwnerTeacherName string   `json:"ownerTeacherName,omitempty"`
	Status           string   `json:"status"`
	OpenedPackages   []string `json:"openedPackages"`
	OpenedStudents   []string `json:"openedStudents"`
}
