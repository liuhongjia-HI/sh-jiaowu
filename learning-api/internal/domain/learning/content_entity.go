package learning

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

type Material struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	CourseID         string `json:"courseId,omitempty"`
	Course           string `json:"course"`
	LearningSpaceID  string `json:"learningSpaceId,omitempty"`
	Grade            string `json:"grade,omitempty"`
	Semester         string `json:"semester,omitempty"`
	Subject          string `json:"subject,omitempty"`
	Chapter          string `json:"chapter"`
	TagCode          string `json:"tagCode,omitempty"`
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
	PreviewError     string `json:"previewError,omitempty"`
	PreviewURL       string `json:"previewUrl,omitempty"`
	DownloadURL      string `json:"downloadUrl,omitempty"`
	WatermarkText    string `json:"watermarkText,omitempty"`
	SecurityNotice   string `json:"securityNotice,omitempty"`
	CreatedAt        string `json:"createdAt,omitempty"`
	SortOrder        int    `json:"sortOrder"`
	Status           Status `json:"status"`
}

type MaterialQuery struct {
	Keyword      string `form:"keyword"`
	Subject      string `form:"subject"`
	UploaderID   string `form:"uploaderId"`
	UploadedFrom string `form:"uploadedFrom"`
	UploadedTo   string `form:"uploadedTo"`
	TagCode      string `form:"tagCode"`
}

type MaterialUpdateRequest struct {
	Title           string `json:"title"`
	CourseID        string `json:"courseId"`
	LearningSpaceID string `json:"learningSpaceId"`
	Chapter         string `json:"chapter"`
	TagCode         string `json:"tagCode"`
	Status          Status `json:"status"`
}

type MaterialReorderRequest struct {
	CourseID    string   `json:"courseId"`
	MaterialIDs []string `json:"materialIds"`
}

type Homework struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Chapter          string     `json:"chapter,omitempty"`
	TagCode          string     `json:"tagCode,omitempty"`
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
	DeadlineAt       string     `json:"deadlineAt,omitempty"`
	AssessmentType   string     `json:"assessmentType,omitempty"`
	IsOverdue        bool       `json:"isOverdue,omitempty"`
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
	PreviewError     string     `json:"previewError,omitempty"`
	PreviewURL       string     `json:"previewUrl,omitempty"`
	DownloadURL      string     `json:"downloadUrl,omitempty"`
	WatermarkText    string     `json:"watermarkText,omitempty"`
	SecurityNotice   string     `json:"securityNotice,omitempty"`
	SortOrder        int        `json:"sortOrder"`
	Status           string     `json:"status"`
}

type HomeworkUpdateRequest struct {
	Title           string   `json:"title"`
	CourseID        string   `json:"courseId"`
	LearningSpaceID string   `json:"learningSpaceId"`
	Chapter         string   `json:"chapter"`
	TagCode         string   `json:"tagCode"`
	Deadline        string   `json:"deadline"`
	DeadlineAt      string   `json:"deadlineAt"`
	AssessmentType  string   `json:"assessmentType"`
	Status          string   `json:"status"`
	QuestionIDs     []string `json:"questionIds"`
}

type SecurityEventRequest struct {
	EventType  string `json:"eventType"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	PagePath   string `json:"pagePath"`
	Detail     string `json:"detail"`
}

// Question 是小挑战中的单道题目。Answer 仅用于服务端自动判分，不下发给学生端。
type Question struct {
	ID      string   `json:"id"`
	Title   string   `json:"title,omitempty"`
	Type    string   `json:"type"` // single | multiple | judge | fill | text
	Stem    string   `json:"stem"`
	Options []string `json:"options,omitempty"`
	Answer  string   `json:"-"`
	Answers []string `json:"-"`
	Score   int      `json:"score,omitempty"`
}

// QuestionBankItem 是可跨学年复用的题库题目，按年级、学期、学科归档。
type QuestionBankItem struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Grade            string   `json:"grade"`
	Semester         string   `json:"semester"`
	Subject          string   `json:"subject"`
	Type             string   `json:"type"` // single | multiple | judge | fill | text
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

type QuestionBankQuery struct {
	Grade    string `form:"grade"`
	Semester string `form:"semester"`
	Subject  string `form:"subject"`
	Keyword  string `form:"keyword"`
}

type QuestionBankUpsertRequest struct {
	Title    string   `json:"title"`
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
	TagCode    string `json:"tagCode,omitempty"`
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

type HomeworkSubmissionStudent struct {
	StudentID    string `json:"studentId"`
	StudentName  string `json:"studentName"`
	Phone        string `json:"phone"`
	SubmittedAt  string `json:"submittedAt,omitempty"`
	ReviewStatus string `json:"reviewStatus"`
	SubmissionID string `json:"submissionId,omitempty"`
}

type HomeworkSubmissionSummary struct {
	HomeworkID    string                      `json:"homeworkId"`
	HomeworkTitle string                      `json:"homeworkTitle"`
	TotalNum      int                         `json:"totalNum"`
	SubmittedNum  int                         `json:"submittedNum"`
	Students      []HomeworkSubmissionStudent `json:"students"`
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

type FileAsset struct {
	ID               string
	FileName         string
	FileSize         int64
	FileType         string
	ContentType      string
	OriginalPath     string
	PreviewPath      string
	PreviewPageDir   string
	PreviewPageCount int
	PreviewStatus    string
	PreviewError     string
	// WatermarkText 仅在学生端返回，用于小程序展示专属水印提示。
	WatermarkText string
	// WatermarkStampText 是可被服务端稳定渲染的 ASCII 追溯文本，绝不包含完整手机号或学生 ID。
	WatermarkStampText string
}

type PreviewJob struct {
	ID           string
	FileID       string
	Status       string
	AttemptCount int
	ErrorMessage string
	CreatedAt    string
	StartedAt    string
	FinishedAt   string
}

type PreviewResult struct {
	PreviewPath      string
	PreviewPageDir   string
	PreviewPageCount int
	PreviewWarning   string
}

type MaterialUploadRequest struct {
	Title           string
	LearningSpaceID string
	CourseID        string
	Chapter         string
	TagCode         string
	File            FileAsset
}

type HomeworkUploadRequest struct {
	Title           string    `json:"title"`
	LearningSpaceID string    `json:"learningSpaceId"`
	CourseID        string    `json:"courseId"`
	Chapter         string    `json:"chapter"`
	TagCode         string    `json:"tagCode"`
	Deadline        string    `json:"deadline"`
	DeadlineAt      string    `json:"deadlineAt"`
	AssessmentType  string    `json:"assessmentType"`
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
	FinalStatus    string `json:"finalStatus"`
}
