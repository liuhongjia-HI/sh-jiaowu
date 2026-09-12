package learning

type Package struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	AcademicYear     string   `json:"academicYear"`
	Grade            string   `json:"grade"`
	Semester         string   `json:"semester"`
	Subject          string   `json:"subject"`
	Level            string   `json:"level"`
	PhaseScope       string   `json:"phaseScope"`
	PackageType      string   `json:"packageType"`
	Summary          string   `json:"summary"`
	LearningSpaceIDs []string `json:"learningSpaceIds,omitempty"`
	LearningSpaces   []string `json:"learningSpaces,omitempty"`
	ContentTypeCodes []string `json:"contentTypeCodes,omitempty"`
	ContentTypes     []string `json:"contentTypes,omitempty"`
	TrialEnabled     bool     `json:"trialEnabled"`
	OpenStudentNum   int      `json:"openStudentNum"`
	Status           Status   `json:"status"`
}

type PackageUpsertRequest struct {
	Name             string   `json:"name"`
	AcademicYear     string   `json:"academicYear"`
	Grade            string   `json:"grade"`
	Semester         string   `json:"semester"`
	Subject          string   `json:"subject"`
	Level            string   `json:"level"`
	PhaseScope       string   `json:"phaseScope"`
	PackageType      string   `json:"packageType"`
	Summary          string   `json:"summary"`
	LearningSpaceIDs []string `json:"learningSpaceIds"`
	ContentTypeCodes []string `json:"contentTypeCodes"`
	TrialEnabled     bool     `json:"trialEnabled"`
	Status           Status   `json:"status"`
}

// PackageCopyRequest 复制课程方案。空字段由服务端按最省事的规则补齐：
// 学年默认滚到当前学年（源方案如果已经是未来学年则保持），名称自动去重。
type PackageCopyRequest struct {
	Name         string `json:"name"`
	AcademicYear string `json:"academicYear"`
	Status       Status `json:"status"`
}

// StudentTrial 描述学生当前学年的体验资格与使用状态。体验记录负责资格和
// 转正归因，套餐授权继续负责具体内容的访问控制。
type StudentTrial struct {
	ID            string               `json:"id,omitempty"`
	State         string               `json:"state"`
	PackageID     string               `json:"packageId,omitempty"`
	PackageName   string               `json:"packageName,omitempty"`
	Subject       string               `json:"subject,omitempty"`
	StartedAt     string               `json:"startedAt,omitempty"`
	EndsAt        string               `json:"endsAt,omitempty"`
	RemainingDays int                  `json:"remainingDays"`
	Options       []StudentTrialOption `json:"options,omitempty"`
}

type StudentTrialOption struct {
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
	Subject     string `json:"subject"`
}

type StudentTrialStartResult struct {
	Trial         StudentTrial `json:"trial"`
	FirstCourseID string       `json:"firstCourseId,omitempty"`
}

// StudentSubjectRecommendation 是本年级未开通学科的公开摘要。
// 仅返回统计和公开教学信息，不授予课程或资料访问权限。
type StudentSubjectRecommendation struct {
	QuestionCount        int      `json:"questionCount"`
	HomeworkCount        int      `json:"homeworkCount"`
	TeacherName          string   `json:"teacherName"`
	TeacherIntro         string   `json:"teacherIntro"`
	PackageID            string   `json:"packageId,omitempty"`
	PackageName          string   `json:"packageName,omitempty"`
	AcademicYear         string   `json:"academicYear,omitempty"`
	Grade                string   `json:"grade"`
	Semester             string   `json:"semester,omitempty"`
	Subject              string   `json:"subject"`
	Level                string   `json:"level,omitempty"`
	Summary              string   `json:"summary,omitempty"`
	LearningSpaces       []string `json:"learningSpaces,omitempty"`
	CourseCount          int      `json:"courseCount"`
	MaterialCount        int      `json:"materialCount"`
	ContentSamples       []string `json:"contentSamples,omitempty"`
	RecommendationReason string   `json:"recommendationReason"`
	SameLearningSpace    bool     `json:"sameLearningSpace,omitempty"`
}

// StudentPackageRecommendation 保留 Go 调用兼容，接口内容已改为学科摘要。
type StudentPackageRecommendation = StudentSubjectRecommendation

type StudentGrant struct {
	StudentID        string   `json:"studentId"`
	PackageID        string   `json:"packageId"`
	PackageName      string   `json:"packageName"`
	StartsAt         string   `json:"startsAt"`
	EffectiveUntil   string   `json:"effectiveUntil"`
	PermissionState  string   `json:"permissionState"`
	IsDirect         bool     `json:"isDirect"`
	LearningSpaceIDs []string `json:"learningSpaceIds"`
	LearningSpaces   []string `json:"learningSpaces"`
	ContentTypes     []string `json:"contentTypes"`
	OpenCourses      []string `json:"openCourses"`
	OpenMaterials    []string `json:"openMaterials"`
	OpenHomework     []string `json:"openHomework"`
}

// StudentOpeningItem 是课程开通矩阵中可展开查看的一项具体内容。
type StudentOpeningItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// StudentOpeningPackageGrant identifies a package that contributes an opened
// cell. A package remains shared, but its grant can be revoked for one student.
type StudentOpeningPackageGrant struct {
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
}

// StudentOpeningCell 表示某个课程范围内一种内容的实际开通状态。
// PackageOpened 和 DirectOpened 分别保留来源，避免运营人员误以为可以关闭课程方案内容。
type StudentOpeningCell struct {
	ContentTypeCode string                       `json:"contentTypeCode"`
	Opened          bool                         `json:"opened"`
	PackageOpened   bool                         `json:"packageOpened"`
	DirectOpened    bool                         `json:"directOpened"`
	PackageNames    []string                     `json:"packageNames"`
	PackageGrants   []StudentOpeningPackageGrant `json:"packageGrants"`
	Items           []StudentOpeningItem         `json:"items"`
}

// StudentOpeningScope 是学生课程开通页的一行课程范围。
type StudentOpeningScope struct {
	LearningSpaceID string               `json:"learningSpaceId"`
	Name            string               `json:"name"`
	Subject         string               `json:"subject"`
	Content         []StudentOpeningCell `json:"content"`
}

type GrantCreateRequest struct {
	StudentID string `json:"studentId"`
	PackageID string `json:"packageId"`
	StartsAt  string `json:"startsAt"`
	EndsAt    string `json:"endsAt"`
}

// DirectGrantCreateRequest 用于学生详情中的直接开通：管理员选择课程范围和
// 内容类型即可，不需要先理解或挑选课程方案。
type DirectGrantCreateRequest struct {
	StudentID        string   `json:"studentId"`
	LearningSpaceIDs []string `json:"learningSpaceIds"`
	ContentTypeCodes []string `json:"contentTypeCodes"`
	StartsAt         string   `json:"startsAt"`
	EndsAt           string   `json:"endsAt"`
}

// DirectGrantSelection describes the exact content manually opened for one learning space.
// It is kept separate from package grants so correcting one student's selection never changes a shared package.
type DirectGrantSelection struct {
	LearningSpaceID  string   `json:"learningSpaceId"`
	ContentTypeCodes []string `json:"contentTypeCodes"`
}

// DirectGrantReplaceRequest replaces one student's complete set of direct grants.
// An empty selection list intentionally cancels all of that student's direct grants.
type DirectGrantReplaceRequest struct {
	StudentID  string                 `json:"studentId"`
	Selections []DirectGrantSelection `json:"selections"`
	StartsAt   string                 `json:"startsAt"`
	EndsAt     string                 `json:"endsAt"`
}

// DirectGrantPeriodDefault is the editable default shown before a student is
// directly granted learning content. The server picks the current semester's
// midterm, or the final date after midterm has passed.
type DirectGrantPeriodDefault struct {
	StartsAt string `json:"startsAt"`
	EndsAt   string `json:"endsAt"`
}

type DirectGrantResult struct {
	StudentID      string   `json:"studentId"`
	StudentName    string   `json:"studentName"`
	LearningSpaces []string `json:"learningSpaces"`
	ContentTypes   []string `json:"contentTypes"`
	OpenCourses    []string `json:"openCourses"`
	OpenMaterials  []string `json:"openMaterials"`
	OpenHomework   []string `json:"openHomework"`
}

type GrantRevokeResult struct {
	StudentID      string   `json:"studentId"`
	PackageID      string   `json:"packageId"`
	PackageName    string   `json:"packageName"`
	LearningSpaces []string `json:"learningSpaces"`
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
