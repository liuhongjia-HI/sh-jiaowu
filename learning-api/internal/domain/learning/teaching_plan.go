package learning

// TeachingPlan is a teacher-only resource. It is intentionally separate from
// Material, whose existing student publication rules must remain unchanged.
type TeachingPlan struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Grade         string `json:"grade"`
	Subject       string `json:"subject"`
	CourseID      string `json:"courseId,omitempty"`
	LessonID      string `json:"lessonId,omitempty"`
	Chapter       string `json:"chapter,omitempty"`
	Semester      string `json:"semester,omitempty"`
	Phase         string `json:"phase,omitempty"`
	FileID        string `json:"-"`
	FileName      string `json:"fileName"`
	FileSize      int64  `json:"fileSize"`
	FileType      string `json:"fileType"`
	PreviewStatus string `json:"previewStatus"`
	PreviewError  string `json:"previewError,omitempty"`
	PreviewURL    string `json:"previewUrl"`
	DownloadURL   string `json:"downloadUrl,omitempty"`
	UploaderID    string `json:"uploaderId"`
	UploaderName  string `json:"uploaderName"`
	CreatedAt     string `json:"createdAt"`
	ReadVersion   string `json:"readVersion"`
}

type TeachingPlanScope struct {
	Grade   string `json:"grade"`
	Subject string `json:"subject"`
}

type TeachingPlanList struct {
	Plans         []TeachingPlan      `json:"plans"`
	UploadScopes  []TeachingPlanScope `json:"uploadScopes"`
	CanUpload     bool                `json:"canUpload"`
	Directories   []Course            `json:"directories"`
	UnreadPlanIDs []string            `json:"unreadPlanIds"`
}

type TeachingPlanReadRequest struct {
	Version string `json:"version"`
}

type TeachingPlanUploadRequest struct {
	BatchID  string
	Title    string
	Grade    string
	Subject  string
	CourseID string
	LessonID string
	File     FileAsset
}

type PendingTeachingPlanNoticeBatch struct {
	BatchID       string `json:"batchId"`
	ResourceCount int    `json:"resourceCount"`
}

type TeachingPlanChapterRequest struct {
	CourseID string `json:"courseId"`
	LessonID string `json:"lessonId"`
}

type TeachingPlanAudienceRequest struct {
	PlanIDs []string `json:"planIds"`
}

type TeachingPlanNoticeRecipient struct {
	UserID    string   `json:"userId"`
	Name      string   `json:"name"`
	PlanIDs   []string `json:"planIds"`
	Reachable bool     `json:"reachable"`
	Reason    string   `json:"reason,omitempty"`
}
