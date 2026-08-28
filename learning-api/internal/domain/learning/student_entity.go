package learning

type Student struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Nickname  string `json:"nickname,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	// Grade 是按入学基准推导出的当前年级，不直接持久化，也不接受前端写入。
	Grade string `json:"grade"`
	// EnrollmentAcademicYear 与 EnrollmentGrade 是年级推导的基准，入学后不再变化。
	EnrollmentAcademicYear string              `json:"enrollmentAcademicYear,omitempty"`
	EnrollmentGrade        string              `json:"enrollmentGrade,omitempty"`
	Graduated              bool                `json:"graduated,omitempty"`
	Phone                  string              `json:"phone"`
	SchoolName             string              `json:"schoolName,omitempty"`
	GuardianName           string              `json:"guardianName,omitempty"`
	OfficialAccountOpenID  string              `json:"officialAccountOpenId,omitempty"`
	OpenedPackages         []string            `json:"openedPackages"`
	OpenedPackageRefs      []StudentPackageRef `json:"openedPackageRefs"`
	LearningStatus         string              `json:"learningStatus"`
	AccountStatus          string              `json:"accountStatus"`
	StreakDays             int                 `json:"streakDays"`
	AverageScore           int                 `json:"averageScore"`
	BadgeCount             int                 `json:"badgeCount"`
	Remark                 string              `json:"remark,omitempty"`
	BindStatus             string              `json:"bindStatus"`
	CreatedAt              string              `json:"createdAt"`
	LastStudyAt            string              `json:"lastStudyAt,omitempty"`
	LastSubmittedAt        string              `json:"lastSubmittedAt,omitempty"`
	LastSubmissionStatus   string              `json:"lastSubmissionStatus,omitempty"`
	EffectiveUntil         string              `json:"effectiveUntil,omitempty"`
	// BindCode/BindCodeExpiresAt 是"关联第二个家长"用的邀请码：机构后台生成，
	// 分享给爸爸/妈妈/其他家长后，对方在小程序里输入即可关联到这个学生，
	// 不需要走"手机号命中已有档案"那条路。到期后需要在后台重新生成。
	BindCode          string `json:"bindCode,omitempty"`
	BindCodeExpiresAt string `json:"bindCodeExpiresAt,omitempty"`
}

type StudentPackageRef struct {
	PackageID   string `json:"packageId"`
	PackageName string `json:"packageName"`
}

type StudentUpsertRequest struct {
	Name                  string `json:"name"`
	Phone                 string `json:"phone"`
	Grade                 string `json:"grade"`
	SchoolName            string `json:"schoolName"`
	GuardianName          string `json:"guardianName"`
	OfficialAccountOpenID string `json:"officialAccountOpenId"`
	AccountStatus         string `json:"accountStatus"`
	Remark                string `json:"remark"`
}

type StudentProfileUpdateRequest struct {
	Nickname     string `json:"nickname"`
	AvatarURL    string `json:"avatarUrl"`
	StudentName  string `json:"studentName"`
	Grade        string `json:"grade"`
	SchoolName   string `json:"schoolName"`
	GuardianName string `json:"guardianName"`
	// PhoneCode 只能是微信 getPhoneNumber 返回的一次性授权码，后端负责换取真实手机号。
	// 不接受前端直接提交明文手机号，避免客户端伪造绑定关系。
	PhoneCode string `json:"phoneCode,omitempty"`
}

type StudentQuery struct {
	Keyword        string
	Grade          string
	AccountStatus  string
	LearningStatus string
	PackageState   string
}

// Guardian 是登录主体：家长用手机号/微信登录，登录之后可能同时看得到多个孩子。
// 学生档案（Student）本身不再假定"一个手机号 = 一个孩子"，谁能看哪个孩子由
// GuardianStudent 关系表决定，而不是靠 students.phone 撞出来。
type Guardian struct {
	ID       string `json:"id"`
	Phone    string `json:"phone"`
	OpenID   string `json:"openId,omitempty"`
	UnionID  string `json:"unionId,omitempty"`
	Name     string `json:"name,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	// LastStudentID 只是新会话默认展示哪个孩子的提示值，不是权威状态——权威值
	// 是 token 里的 activeStudentID，每次切换只更新这里，从不用来做权限判断。
	LastStudentID string `json:"lastStudentId,omitempty"`
	AccountStatus string `json:"accountStatus"`
}

// GuardianRelation 描述家长和学生之间是什么关系，决定权限展示和消息落款怎么称呼。
type GuardianRelation string

const (
	GuardianRelationSelf   GuardianRelation = "本人"
	GuardianRelationFather GuardianRelation = "爸爸"
	GuardianRelationMother GuardianRelation = "妈妈"
	GuardianRelationOther  GuardianRelation = "其他家长"
	// GuardianRelationGuardian 是登录时自动建立关系的默认值——系统不知道
	// 登录的人是爸爸、妈妈还是本人，只知道"是这个孩子的家长"。具体是谁由
	// 家长自己在后续管理里改，不强求登录那一刻就填清楚。
	GuardianRelationGuardian GuardianRelation = "家长"
)

// GuardianStudentStatus 决定这个孩子还出不出现在家长的切换器里。结课/退费不能
// 直接删关系——家长事后还要能查到历史学习记录，只是不再默认展示。
type GuardianStudentStatus string

const (
	GuardianStudentActive   GuardianStudentStatus = "在读"
	GuardianStudentInactive GuardianStudentStatus = "结课"
	// GuardianStudentPending 表示家长已提交添加申请，待管理员审核。
	// 它不会通过 GuardianStudentActive，因此待审核学生不能被切换或访问。
	GuardianStudentPending GuardianStudentStatus = "待审核"
)

// GuardianStudent 是家长与学生的多对多关系：一个家长可能关联多个孩子（多子女），
// 一个孩子也可能关联多个家长（爸爸/妈妈各自的微信）。
type GuardianStudent struct {
	GuardianID string                `json:"guardianId"`
	StudentID  string                `json:"studentId"`
	Relation   GuardianRelation      `json:"relation"`
	IsPrimary  bool                  `json:"isPrimary,omitempty"`
	Status     GuardianStudentStatus `json:"status"`
}

type StudentLearningRecord struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Course      string `json:"course"`
	Status      string `json:"status"`
	Score       int    `json:"score,omitempty"`
	FullScore   int    `json:"fullScore,omitempty"`
	OccurredAt  string `json:"occurredAt"`
	Description string `json:"description"`
}

type StudentScoreRecord struct {
	ID             string `json:"id"`
	StudentID      string `json:"studentId"`
	Subject        string `json:"subject"`
	ExamType       string `json:"examType"`
	ExamName       string `json:"examName"`
	ExamDate       string `json:"examDate"`
	Score          int    `json:"score"`
	FullScore      int    `json:"fullScore"`
	AverageScore   int    `json:"averageScore,omitempty"`
	TeacherComment string `json:"teacherComment,omitempty"`
	CreatedBy      string `json:"createdBy,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type StudentScoreUpsertRequest struct {
	Subject        string `json:"subject"`
	ExamType       string `json:"examType"`
	ExamName       string `json:"examName"`
	ExamDate       string `json:"examDate"`
	Score          int    `json:"score"`
	FullScore      int    `json:"fullScore"`
	AverageScore   int    `json:"averageScore"`
	TeacherComment string `json:"teacherComment"`
}

type StudentScoreSummary struct {
	Subject        string               `json:"subject"`
	Records        []StudentScoreRecord `json:"records"`
	FirstRecord    *StudentScoreRecord  `json:"firstRecord,omitempty"`
	LatestRecord   *StudentScoreRecord  `json:"latestRecord,omitempty"`
	Improvement    int                  `json:"improvement"`
	ImprovementPct int                  `json:"improvementPct"`
	Description    string               `json:"description"`
	ProblemPoint   string               `json:"problemPoint,omitempty"`
	NextStep       string               `json:"nextStep,omitempty"`
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

type StudentCourseCard struct {
	Course
	Progress int `json:"progress"`
}

// StudentStudyBoard 是学习页的聚合数据：课程卡（带进度）+ 资料。
type StudentStudyBoard struct {
	Student   Student             `json:"student"`
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

// StudentTodo 是学生首页的今日待办，聚合课程、练习、反馈和提醒授权。
type StudentTodo struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	ActionText string `json:"actionText"`
	Path       string `json:"path,omitempty"`
	Priority   int    `json:"priority"`
	Status     string `json:"status"`
}

// ClassroomFeedback 是家长可读的课后课堂反馈，由已批改练习沉淀。
type ClassroomFeedback struct {
	ID                  string `json:"id"`
	CourseName          string `json:"courseName"`
	LessonTitle         string `json:"lessonTitle"`
	TeacherName         string `json:"teacherName"`
	Performance         string `json:"performance"`
	Focus               string `json:"focus"`
	NextStep            string `json:"nextStep"`
	Score               int    `json:"score"`
	CreatedAt           string `json:"createdAt"`
	RelatedSubmissionID string `json:"relatedSubmissionId,omitempty"`
}

type SubscriptionReminder struct {
	Enabled            bool     `json:"enabled"`
	TemplateConfigured bool     `json:"templateConfigured"`
	TemplateIDs        []string `json:"templateIds,omitempty"`
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	ActionText         string   `json:"actionText"`
}

type StudentSubscriptionRequest struct {
	TemplateIDs []string `json:"templateIds"`
}

type StudentSubscriptionPreference struct {
	StudentID   string   `json:"studentId"`
	Enabled     bool     `json:"enabled"`
	TemplateIDs []string `json:"templateIds"`
	UpdatedAt   string   `json:"updatedAt"`
}

// Badge 是成长徽章；Obtained 表示当前学生是否已获得。
type Badge struct {
	ID       string `json:"id"`
	Icon     string `json:"icon"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Obtained bool   `json:"obtained"`
}

// Favorite 是学生收藏的一条内容（学习资料或小挑战）。
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

type StudentHome struct {
	Student              Student              `json:"student"`
	ContinueCourse       Course               `json:"continueCourse"`
	ContinueProgress     int                  `json:"continueProgress"`
	PendingHomework      []Homework           `json:"pendingHomework"`
	Notices              []Notice             `json:"notices"`
	Materials            []Material           `json:"materials"`
	TodayTodos           []StudentTodo        `json:"todayTodos"`
	ClassroomFeedback    []ClassroomFeedback  `json:"classroomFeedback"`
	SubscriptionReminder SubscriptionReminder `json:"subscriptionReminder"`
}
