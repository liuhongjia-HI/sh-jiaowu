package learning

type Status string

const (
	StatusEnabled  Status = "启用"
	StatusDraft    Status = "草稿"
	StatusDisabled Status = "停用"
)

type Role string

const (
	RoleStudent     Role = "student"
	RoleTeacher     Role = "teacher"
	RoleOpsStaff    Role = "ops_staff"
	RoleCampusAdmin Role = "campus_admin"
	RoleSuperAdmin  Role = "super_admin"
)

type User struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Phone              string   `json:"phone"`
	OpenID             string   `json:"openId"`
	UnionID            string   `json:"unionId"`
	PasswordHash       string   `json:"-"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	TokenVersion       int      `json:"tokenVersion,omitempty"`
	AccountStatus      string   `json:"accountStatus"`
	Remark             string   `json:"remark,omitempty"`
	Roles              []Role   `json:"roles"`
	StudentID          string   `json:"studentId,omitempty"`
	CampusID           string   `json:"campusId,omitempty"`
	CampusScopes       []string `json:"campusScopes,omitempty"`
	LearningSpaceIDs   []string `json:"learningSpaceIds,omitempty"`
	CanUploadHandout   bool     `json:"canUploadHandout,omitempty"`
	CanUploadQuestion  bool     `json:"canUploadQuestion,omitempty"`
	CanReview          bool     `json:"canReview,omitempty"`
}

type Principal struct {
	UserID             string   `json:"userId"`
	Name               string   `json:"name"`
	Phone              string   `json:"phone,omitempty"`
	StudentID          string   `json:"studentId,omitempty"`
	CampusID           string   `json:"campusId,omitempty"`
	Roles              []Role   `json:"roles"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	TokenVersion       int      `json:"tokenVersion,omitempty"`
	CampusScopes       []string `json:"campusScopes,omitempty"`
	LearningSpaceIDs   []string `json:"learningSpaceIds,omitempty"`
	CanUploadHandout   bool     `json:"canUploadHandout,omitempty"`
	CanUploadQuestion  bool     `json:"canUploadQuestion,omitempty"`
	CanReview          bool     `json:"canReview,omitempty"`
}

type AuthResult struct {
	Token string    `json:"token"`
	User  Principal `json:"user"`
}

type Teacher struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	CampusID          string   `json:"campusId"`
	LearningSpaceIDs  []string `json:"learningSpaceIds"`
	LearningSpaces    []string `json:"learningSpaces"`
	Grades            []string `json:"grades"`
	Subjects          []string `json:"subjects"`
	CanUploadHandout  bool     `json:"canUploadHandout"`
	CanUploadQuestion bool     `json:"canUploadQuestion"`
	CanReview         bool     `json:"canReview"`
	AccountStatus     string   `json:"accountStatus"`
	BindStatus        string   `json:"bindStatus"`
	Remark            string   `json:"remark"`
}

type TeacherUpsertRequest struct {
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	CampusID          string   `json:"campusId"`
	LearningSpaceIDs  []string `json:"learningSpaceIds"`
	CanUploadHandout  bool     `json:"canUploadHandout"`
	CanUploadQuestion bool     `json:"canUploadQuestion"`
	CanReview         bool     `json:"canReview"`
	AccountStatus     string   `json:"accountStatus"`
	Remark            string   `json:"remark"`
}

type LearningSpace struct {
	ID           string `json:"id"`
	AcademicYear string `json:"academicYear"`
	Grade        string `json:"grade"`
	Subject      string `json:"subject"`
	Semester     string `json:"semester"`
	Phase        string `json:"phase"`
	Name         string `json:"name"`
	Status       Status `json:"status"`
}

type AdminStaff struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Role          Role   `json:"role"`
	CampusID      string `json:"campusId,omitempty"`
	AccountStatus string `json:"accountStatus"`
	BindStatus    string `json:"bindStatus"`
	Remark        string `json:"remark"`
}

type AdminStaffUpsertRequest struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Role          Role   `json:"role"`
	CampusID      string `json:"campusId"`
	AccountStatus string `json:"accountStatus"`
	Remark        string `json:"remark"`
}

type PasswordChangeRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type PasswordResetResult struct {
	UserID             string `json:"userId"`
	TemporaryPassword  string `json:"temporaryPassword"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

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

type Student struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Nickname       string   `json:"nickname,omitempty"`
	AvatarURL      string   `json:"avatarUrl,omitempty"`
	Grade          string   `json:"grade"`
	Phone          string   `json:"phone"`
	OpenedPackages []string `json:"openedPackages"`
	LearningStatus string   `json:"learningStatus"`
	AccountStatus  string   `json:"accountStatus"`
	StreakDays     int      `json:"streakDays"`
	AverageScore   int      `json:"averageScore"`
	BadgeCount     int      `json:"badgeCount"`
	Remark         string   `json:"remark,omitempty"`
	BindStatus     string   `json:"bindStatus"`
	LastStudyAt    string   `json:"lastStudyAt,omitempty"`
	EffectiveUntil string   `json:"effectiveUntil,omitempty"`
}

type StudentUpsertRequest struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Grade         string `json:"grade"`
	AccountStatus string `json:"accountStatus"`
	Remark        string `json:"remark"`
}

type StudentProfileUpdateRequest struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

type StudentQuery struct {
	Keyword        string
	Grade          string
	AccountStatus  string
	LearningStatus string
	PackageState   string
}

type StudentGrant struct {
	StudentID       string `json:"studentId"`
	PackageID       string `json:"packageId"`
	PackageName     string `json:"packageName"`
	EffectiveUntil  string `json:"effectiveUntil"`
	PermissionState string `json:"permissionState"`
}

type StudentLearningRecord struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Course      string `json:"course"`
	Status      string `json:"status"`
	Score       int    `json:"score,omitempty"`
	OccurredAt  string `json:"occurredAt"`
	Description string `json:"description"`
}

type StudentDetail struct {
	Student         Student                  `json:"student"`
	Grants          []StudentGrant           `json:"grants"`
	Permissions     StudentPermissionSummary `json:"permissions"`
	LearningRecords []StudentLearningRecord  `json:"learningRecords"`
	Notices         []Notice                 `json:"notices"`
	Logs            []OperationLog           `json:"logs"`
}

type StudentImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type StudentImportResult struct {
	SuccessCount int                     `json:"successCount"`
	FailedCount  int                     `json:"failedCount"`
	Errors       []StudentImportRowError `json:"errors"`
}

type StudentRemindResult struct {
	NoticeID string `json:"noticeId"`
	Message  string `json:"message"`
}

type CommercialOrder struct {
	ID                 string `json:"id"`
	OrderNo            string `json:"orderNo"`
	StudentID          string `json:"studentId"`
	StudentName        string `json:"studentName"`
	PackageID          string `json:"packageId"`
	PackageName        string `json:"packageName"`
	AmountCent         int    `json:"amountCent"`
	PaidAmountCent     int    `json:"paidAmountCent"`
	RefundedAmountCent int    `json:"refundedAmountCent"`
	LessonTotal        int    `json:"lessonTotal"`
	LessonConsumed     int    `json:"lessonConsumed"`
	Status             string `json:"status"`
	ContractStatus     string `json:"contractStatus"`
	InvoiceStatus      string `json:"invoiceStatus"`
	CreatedAt          string `json:"createdAt"`
}

type CommercialOrderCreateRequest struct {
	StudentID   string `json:"studentId"`
	PackageID   string `json:"packageId"`
	AmountCent  int    `json:"amountCent"`
	LessonTotal int    `json:"lessonTotal"`
	Remark      string `json:"remark"`
}

type PaymentRecord struct {
	ID            string `json:"id"`
	OrderID       string `json:"orderId"`
	AmountCent    int    `json:"amountCent"`
	Method        string `json:"method"`
	TransactionNo string `json:"transactionNo"`
	PaidAt        string `json:"paidAt"`
	Status        string `json:"status"`
}

type PaymentCreateRequest struct {
	AmountCent    int    `json:"amountCent"`
	Method        string `json:"method"`
	TransactionNo string `json:"transactionNo"`
}

type RefundRecord struct {
	ID         string `json:"id"`
	OrderID    string `json:"orderId"`
	AmountCent int    `json:"amountCent"`
	Reason     string `json:"reason"`
	RefundedAt string `json:"refundedAt"`
	Status     string `json:"status"`
}

type RefundCreateRequest struct {
	AmountCent int    `json:"amountCent"`
	Reason     string `json:"reason"`
}

type ContractRecord struct {
	ID       string `json:"id"`
	OrderID  string `json:"orderId"`
	Title    string `json:"title"`
	Signer   string `json:"signer"`
	SignedAt string `json:"signedAt"`
	Status   string `json:"status"`
}

type ContractCreateRequest struct {
	Title  string `json:"title"`
	Signer string `json:"signer"`
}

type InvoiceRecord struct {
	ID         string `json:"id"`
	OrderID    string `json:"orderId"`
	Title      string `json:"title"`
	TaxNo      string `json:"taxNo"`
	AmountCent int    `json:"amountCent"`
	InvoiceNo  string `json:"invoiceNo"`
	IssuedAt   string `json:"issuedAt"`
	Status     string `json:"status"`
}

type InvoiceCreateRequest struct {
	Title      string `json:"title"`
	TaxNo      string `json:"taxNo"`
	AmountCent int    `json:"amountCent"`
	InvoiceNo  string `json:"invoiceNo"`
}

type LessonConsumption struct {
	ID              string `json:"id"`
	OrderID         string `json:"orderId"`
	StudentID       string `json:"studentId"`
	ScheduleClassID string `json:"scheduleClassId"`
	LessonCount     int    `json:"lessonCount"`
	ConsumedAt      string `json:"consumedAt"`
	Remark          string `json:"remark"`
}

type LessonConsumptionCreateRequest struct {
	OrderID         string `json:"orderId"`
	ScheduleClassID string `json:"scheduleClassId"`
	LessonCount     int    `json:"lessonCount"`
	Remark          string `json:"remark"`
}

type RenewalReminder struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	StudentID string `json:"studentId"`
	Reason    string `json:"reason"`
	DueAt     string `json:"dueAt"`
	Status    string `json:"status"`
}

type RenewalReminderCreateRequest struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
	DueAt   string `json:"dueAt"`
}

type ParentNotice struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	StudentID string `json:"studentId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	SentAt    string `json:"sentAt"`
	Status    string `json:"status"`
}

type ParentNoticeCreateRequest struct {
	OrderID string `json:"orderId"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type CommercialSummary struct {
	OrderCount        int `json:"orderCount"`
	PaidOrderCount    int `json:"paidOrderCount"`
	RevenueCent       int `json:"revenueCent"`
	RefundCent        int `json:"refundCent"`
	LessonRemainCount int `json:"lessonRemainCount"`
	RenewalTodoCount  int `json:"renewalTodoCount"`
}

type Course struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Subject         string `json:"subject"`
	Grade           string `json:"grade"`
	LearningSpaceID string `json:"learningSpaceId,omitempty"`
	ChapterCount    int    `json:"chapterCount"`
	MaterialNum     int    `json:"materialNum"`
	HomeworkNum     int    `json:"homeworkNum"`
	Status          Status `json:"status"`
}

type CourseUpsertRequest struct {
	Name            string `json:"name"`
	LearningSpaceID string `json:"learningSpaceId"`
	ChapterCount    int    `json:"chapterCount"`
	Status          Status `json:"status"`
}

type SettingUpdateRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Material struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	CourseID         string `json:"courseId,omitempty"`
	Course           string `json:"course"`
	LearningSpaceID  string `json:"learningSpaceId,omitempty"`
	Chapter          string `json:"chapter"`
	Type             string `json:"type"`
	ViewCount        int    `json:"viewCount"`
	OwnerTeacherID   string `json:"ownerTeacherId,omitempty"`
	OwnerTeacherName string `json:"ownerTeacherName,omitempty"`
	PublishStatus    string `json:"publishStatus,omitempty"`
	FileID           string `json:"fileId,omitempty"`
	FileName         string `json:"fileName,omitempty"`
	FileSize         int64  `json:"fileSize,omitempty"`
	FileType         string `json:"fileType,omitempty"`
	PreviewStatus    string `json:"previewStatus,omitempty"`
	PreviewURL       string `json:"previewUrl,omitempty"`
	DownloadURL      string `json:"downloadUrl,omitempty"`
	Status           Status `json:"status"`
}

type MaterialUpdateRequest struct {
	Title           string `json:"title"`
	CourseID        string `json:"courseId"`
	LearningSpaceID string `json:"learningSpaceId"`
	Chapter         string `json:"chapter"`
	Status          Status `json:"status"`
}

type Homework struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	PackageName      string     `json:"packageName"`
	CourseID         string     `json:"courseId,omitempty"`
	Course           string     `json:"course"`
	LearningSpaceID  string     `json:"learningSpaceId,omitempty"`
	Grade            string     `json:"grade,omitempty"`
	Semester         string     `json:"semester,omitempty"`
	Subject          string     `json:"subject,omitempty"`
	QuestionNum      int        `json:"questionNum"`
	QuestionIDs      []string   `json:"questionIds,omitempty"`
	Questions        []Question `json:"questions,omitempty"`
	Deadline         string     `json:"deadline"`
	SubmittedNum     int        `json:"submittedNum"`
	TotalNum         int        `json:"totalNum"`
	OwnerTeacherID   string     `json:"ownerTeacherId,omitempty"`
	OwnerTeacherName string     `json:"ownerTeacherName,omitempty"`
	PublishStatus    string     `json:"publishStatus,omitempty"`
	FileID           string     `json:"fileId,omitempty"`
	FileName         string     `json:"fileName,omitempty"`
	FileSize         int64      `json:"fileSize,omitempty"`
	FileType         string     `json:"fileType,omitempty"`
	PreviewStatus    string     `json:"previewStatus,omitempty"`
	PreviewURL       string     `json:"previewUrl,omitempty"`
	DownloadURL      string     `json:"downloadUrl,omitempty"`
	Status           string     `json:"status"`
}

type HomeworkUpdateRequest struct {
	Title           string   `json:"title"`
	CourseID        string   `json:"courseId"`
	LearningSpaceID string   `json:"learningSpaceId"`
	Deadline        string   `json:"deadline"`
	Status          string   `json:"status"`
	QuestionIDs     []string `json:"questionIds"`
}

// Question 是小挑战中的单道题目。Answer 仅用于服务端自动判分，不下发给学生端。
type Question struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"` // single | multiple | text
	Stem    string   `json:"stem"`
	Options []string `json:"options,omitempty"`
	Answer  string   `json:"-"`
	Answers []string `json:"-"`
	Score   int      `json:"score,omitempty"`
}

// QuestionBankItem 是可跨学年复用的题库题目，按年级、学期、学科归档。
type QuestionBankItem struct {
	ID               string   `json:"id"`
	Grade            string   `json:"grade"`
	Semester         string   `json:"semester"`
	Subject          string   `json:"subject"`
	Type             string   `json:"type"` // single | multiple | text
	Stem             string   `json:"stem"`
	Options          []string `json:"options,omitempty"`
	Answer           string   `json:"answer,omitempty"`
	Answers          []string `json:"answers,omitempty"`
	Score            int      `json:"score"`
	Status           string   `json:"status"`
	OwnerTeacherID   string   `json:"ownerTeacherId,omitempty"`
	OwnerTeacherName string   `json:"ownerTeacherName,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

type QuestionBankUpsertRequest struct {
	Grade    string   `json:"grade"`
	Semester string   `json:"semester"`
	Subject  string   `json:"subject"`
	Type     string   `json:"type"`
	Stem     string   `json:"stem"`
	Options  []string `json:"options"`
	Answer   string   `json:"answer"`
	Answers  []string `json:"answers"`
	Score    int      `json:"score"`
	Status   string   `json:"status"`
}

// Station 是学习详情页的「学习地图」站点。
type Station struct {
	Icon       string `json:"icon"`
	Title      string `json:"title"`
	Desc       string `json:"desc"`
	Status     string `json:"status"`
	MaterialID string `json:"materialId,omitempty"`
	HomeworkID string `json:"homeworkId,omitempty"`
}

// StudentCourseDetail 是学生端「学习详情页」所需的聚合数据。
type StudentCourseDetail struct {
	Course    Course     `json:"course"`
	Materials []Material `json:"materials"`
	Homework  []Homework `json:"homework"`
	Stations  []Station  `json:"stations"`
	Progress  int        `json:"progress"`
}

// Submission 是学生提交的一次小挑战及其批改结果。
type Submission struct {
	ID             string             `json:"id"`
	HomeworkID     string             `json:"homeworkId"`
	StudentID      string             `json:"studentId"`
	TaskTitle      string             `json:"taskTitle"`
	Score          int                `json:"score"`
	ObjectiveScore int                `json:"objectiveScore"`
	FinalScore     int                `json:"finalScore"`
	TeacherComment string             `json:"teacherComment"`
	Reward         string             `json:"reward"`
	Status         string             `json:"status"`
	CreatedAt      string             `json:"createdAt"`
	Answers        []SubmissionAnswer `json:"answers,omitempty"`
}

type SubmissionAnswer struct {
	QuestionID string   `json:"questionId"`
	Choice     string   `json:"choice"`
	Choices    []string `json:"choices"`
	Text       string   `json:"text"`
}

type SubmissionRequest struct {
	HomeworkID string             `json:"homeworkId"`
	Answers    []SubmissionAnswer `json:"answers"`
}

// StudentCourseCard 是学习列表中的课程卡片，带真实学习进度。
type StudentCourseCard struct {
	Course
	Progress int `json:"progress"`
}

// StudentStudyBoard 是学习页的聚合数据：课程卡（带进度）+ 资料。
type StudentStudyBoard struct {
	Courses   []StudentCourseCard `json:"courses"`
	Materials []Material          `json:"materials"`
}

// StudentTask 是任务列表中的一项，studentStatus 由提交记录派生。
type StudentTask struct {
	Homework
	StudentStatus string `json:"studentStatus"` // 待完成 | 已完成
	Score         int    `json:"score,omitempty"`
	SubmissionID  string `json:"submissionId,omitempty"`
}

// Badge 是成长徽章；Obtained 表示当前学生是否已获得。
type Badge struct {
	ID       string `json:"id"`
	Icon     string `json:"icon"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Obtained bool   `json:"obtained"`
}

// Favorite 是学生收藏的一条内容（讲义或小挑战）。
type Favorite struct {
	ID         string `json:"id"`
	StudentID  string `json:"studentId"`
	TargetType string `json:"targetType"` // material | homework
	TargetID   string `json:"targetId"`
	Title      string `json:"title"`
	Course     string `json:"course"`
	CreatedAt  string `json:"createdAt"`
}

type FavoriteRequest struct {
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
}

type FileAsset struct {
	ID            string
	FileName      string
	FileSize      int64
	FileType      string
	ContentType   string
	OriginalPath  string
	PreviewPath   string
	PreviewStatus string
}

type MaterialUploadRequest struct {
	Title           string
	LearningSpaceID string
	CourseID        string
	Chapter         string
	File            FileAsset
}

type HomeworkUploadRequest struct {
	Title           string    `json:"title"`
	LearningSpaceID string    `json:"learningSpaceId"`
	CourseID        string    `json:"courseId"`
	Deadline        string    `json:"deadline"`
	Status          string    `json:"status"`
	QuestionIDs     []string  `json:"questionIds"`
	File            FileAsset `json:"-"`
}

type Review struct {
	ID             string `json:"id"`
	StudentID      string `json:"studentId,omitempty"`
	HomeworkID     string `json:"homeworkId,omitempty"`
	SubmissionID   string `json:"submissionId,omitempty"`
	StudentName    string `json:"studentName"`
	PackageName    string `json:"packageName"`
	Homework       string `json:"homework"`
	SystemScore    int    `json:"systemScore"`
	TeacherComment string `json:"teacherComment,omitempty"`
	Reward         string `json:"reward,omitempty"`
	Status         string `json:"status"`
}

type ReviewCompleteRequest struct {
	Score          int    `json:"score"`
	TeacherComment string `json:"teacherComment"`
	Reward         string `json:"reward"`
}

type Notice struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Target  string `json:"target"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type NoticeCreateRequest struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Target  string `json:"target"`
	Summary string `json:"summary"`
}

type OperationLog struct {
	ID         string `json:"id"`
	Operator   string `json:"operator"`
	OperatorID string `json:"operatorId,omitempty"`
	IP         string `json:"ip,omitempty"`
	UserAgent  string `json:"userAgent,omitempty"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Detail     string `json:"detail,omitempty"`
	Time       string `json:"time"`
}

type GrantPreview struct {
	StudentID        string   `json:"studentId"`
	PackageID        string   `json:"packageId"`
	StudentName      string   `json:"studentName"`
	PackageName      string   `json:"packageName"`
	AlreadyOpened    bool     `json:"alreadyOpened"`
	ExistingUntil    string   `json:"existingUntil,omitempty"`
	LearningSpaces   []string `json:"learningSpaces"`
	ContentTypes     []string `json:"contentTypes"`
	OpenCourses      []string `json:"openCourses"`
	OpenMaterials    []string `json:"openMaterials"`
	OpenHomework     []string `json:"openHomework"`
	BlockedContent   []string `json:"blockedContent"`
	EffectiveDefault string   `json:"effectiveDefault"`
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

type DashboardOverview struct {
	OpenedStudents   int `json:"openedStudents"`
	PackageCount     int `json:"packageCount"`
	PendingReviews   int `json:"pendingReviews"`
	MaterialViews    int `json:"materialViews"`
	ExpiringStudents int `json:"expiringStudents"`
	UnpublishedFiles int `json:"unpublishedFiles"`
}

type StudentHome struct {
	Student          Student    `json:"student"`
	ContinueCourse   Course     `json:"continueCourse"`
	ContinueProgress int        `json:"continueProgress"`
	PendingHomework  []Homework `json:"pendingHomework"`
	Notices          []Notice   `json:"notices"`
	Materials        []Material `json:"materials"`
}

type AvailabilitySlot struct {
	ID        string `json:"id"`
	OwnerType string `json:"ownerType"`
	OwnerID   string `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	DayOfWeek int    `json:"dayOfWeek"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Remark    string `json:"remark,omitempty"`
}

type AvailabilityUpsertRequest struct {
	OwnerType string             `json:"ownerType"`
	OwnerID   string             `json:"ownerId"`
	Slots     []AvailabilitySlot `json:"slots"`
}

type ScheduleCandidateRequest struct {
	Subject         string `json:"subject"`
	Grade           string `json:"grade"`
	CourseID        string `json:"courseId"`
	TeacherID       string `json:"teacherId"`
	ClassType       string `json:"classType"`
	DurationMinutes int    `json:"durationMinutes"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
}

type CandidateStudent struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Grade          string   `json:"grade"`
	OpenedPackages []string `json:"openedPackages"`
}

type ScheduleCandidate struct {
	ID                string             `json:"id"`
	DayOfWeek         int                `json:"dayOfWeek"`
	StartTime         string             `json:"startTime"`
	EndTime           string             `json:"endTime"`
	TeacherID         string             `json:"teacherId"`
	TeacherName       string             `json:"teacherName"`
	CourseID          string             `json:"courseId"`
	CourseName        string             `json:"courseName"`
	Subject           string             `json:"subject"`
	Grade             string             `json:"grade"`
	ClassType         string             `json:"classType"`
	Capacity          int                `json:"capacity"`
	AvailableStudents []CandidateStudent `json:"availableStudents"`
	MissingStudents   []CandidateStudent `json:"missingStudents"`
	StudentCount      int                `json:"studentCount"`
	Score             int                `json:"score"`
	Reason            string             `json:"reason"`
}

type ScheduleClassCreateRequest struct {
	CourseID        string   `json:"courseId"`
	TeacherID       string   `json:"teacherId"`
	ClassType       string   `json:"classType"`
	DurationMinutes int      `json:"durationMinutes"`
	DayOfWeek       int      `json:"dayOfWeek"`
	StartTime       string   `json:"startTime"`
	EndTime         string   `json:"endTime"`
	StartDate       string   `json:"startDate"`
	EndDate         string   `json:"endDate"`
	StudentIDs      []string `json:"studentIds"`
}

type ScheduleClass struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	CourseID        string             `json:"courseId"`
	CourseName      string             `json:"courseName"`
	TeacherID       string             `json:"teacherId"`
	TeacherName     string             `json:"teacherName"`
	ClassType       string             `json:"classType"`
	Capacity        int                `json:"capacity"`
	DurationMinutes int                `json:"durationMinutes"`
	DayOfWeek       int                `json:"dayOfWeek"`
	StartTime       string             `json:"startTime"`
	EndTime         string             `json:"endTime"`
	StartDate       string             `json:"startDate"`
	EndDate         string             `json:"endDate"`
	Students        []CandidateStudent `json:"students"`
	Status          string             `json:"status"`
	CreatedAt       string             `json:"createdAt"`
}
