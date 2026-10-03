package learning

const (
	NoticeScheduleConfirmed     = "schedule_confirmed"
	NoticeScheduleChanged       = "schedule_changed"
	NoticeScheduleReminder      = "schedule_reminder"
	NoticeScheduleCancelled     = "schedule_cancelled"
	NoticeHomeworkSubmitted     = "homework_submitted"
	NoticeHomeworkPublished     = "homework_published"
	NoticeReviewCompleted       = "review_completed"
	NoticeReviewException       = "review_exception"
	NoticeMaterialsPublished    = "materials_published"
	NoticeTeachingPlansUploaded = "teaching_plans_uploaded"
)

type BusinessNoticeBinding struct {
	TeacherIDs      []string          `json:"teacherIds,omitempty"`
	WebOrigin       string            `json:"webOrigin,omitempty"`
	FieldMappings   map[string]string `json:"fieldMappings,omitempty"`
	AvailableFields map[string]string `json:"availableFields,omitempty"`
	TriggerReady    bool              `json:"triggerReady"`
	Kind            string            `json:"kind"`
	Title           string            `json:"title"`
	TemplateID      string            `json:"templateId"`
	Enabled         bool              `json:"enabled"`
	EnabledAt       string            `json:"enabledAt,omitempty"`
	Ready           bool              `json:"ready"`
	Reason          string            `json:"reason,omitempty"`
	RequiredFields  map[string]string `json:"requiredFields"`
	ApprovedReasons []string          `json:"approvedReasons,omitempty"`
	StudentIDs      []string          `json:"studentIds,omitempty"`
}

type BusinessLessonChange struct {
	Before *ScheduleClass `json:"before,omitempty"`
	After  ScheduleClass  `json:"after"`
}

type BusinessNoticeEvent struct {
	RecipientUserID        string                 `json:"recipientUserId,omitempty"`
	RecipientName          string                 `json:"recipientName,omitempty"`
	ResourceIDs            []string               `json:"resourceIds,omitempty"`
	UploaderID             string                 `json:"uploaderId,omitempty"`
	CourseID               string                 `json:"courseId,omitempty"`
	BatchCompleted         bool                   `json:"batchCompleted,omitempty"`
	CompletedResourceCount int                    `json:"completedResourceCount,omitempty"`
	StationNoticeID        string                 `json:"stationNoticeId,omitempty"`
	ID                     string                 `json:"id"`
	BatchID                string                 `json:"batchId"`
	Kind                   string                 `json:"kind"`
	StudentID              string                 `json:"studentId"`
	StudentName            string                 `json:"studentName"`
	SeriesID               string                 `json:"seriesId,omitempty"`
	RelatedID              string                 `json:"relatedId,omitempty"`
	Title                  string                 `json:"title"`
	Summary                string                 `json:"summary"`
	Lessons                []BusinessLessonChange `json:"lessons"`
	CreatedAt              string                 `json:"createdAt"`
	ExpiresAt              string                 `json:"expiresAt"`
	Values                 map[string]string      `json:"values,omitempty"`
}

type BusinessNoticeTask struct {
	RecipientUserID string            `json:"recipientUserId,omitempty"`
	RecipientName   string            `json:"recipientName,omitempty"`
	URL             string            `json:"url,omitempty"`
	ID              string            `json:"id"`
	EventID         string            `json:"eventId"`
	Kind            string            `json:"kind"`
	StudentID       string            `json:"studentId"`
	StudentName     string            `json:"studentName"`
	GuardianID      string            `json:"guardianId"`
	GuardianName    string            `json:"guardianName"`
	OpenID          string            `json:"recipientOpenId,omitempty"`
	TemplateID      string            `json:"templateId"`
	Values          map[string]string `json:"values"`
	PagePath        string            `json:"pagePath"`
	ClientMessageID string            `json:"clientMessageId"`
	Status          string            `json:"status"`
	FailureReason   string            `json:"failureReason,omitempty"`
	DueAt           string            `json:"dueAt"`
	ExpiresAt       string            `json:"expiresAt"`
	CreatedAt       string            `json:"createdAt"`
	ClaimedAt       string            `json:"claimedAt,omitempty"`
	FirstAttemptAt  string            `json:"firstAttemptAt,omitempty"`
	AcceptedAt      string            `json:"acceptedAt,omitempty"`
	DeliveredAt     string            `json:"deliveredAt,omitempty"`
	MessageID       string            `json:"messageId,omitempty"`
	Attempts        int               `json:"attempts"`
	Retryable       bool              `json:"retryable"`
}

type OfficialMessageRequest struct {
	URL             string
	TemplateID      string
	OpenID          string
	Values          map[string]string
	PagePath        string
	ClientMessageID string
}

type OfficialDeliveryReceipt struct {
	MessageID  string `json:"messageId"`
	OpenID     string `json:"recipientOpenId,omitempty"`
	Status     string `json:"status"`
	ReceivedAt string `json:"receivedAt"`
}

type BusinessNoticeDetail struct {
	CurrentMaterials []Material          `json:"currentMaterials,omitempty"`
	NoticeID         string              `json:"noticeId,omitempty"`
	Event            BusinessNoticeEvent `json:"event"`
	CurrentLessons   []ScheduleClass     `json:"currentLessons"`
	CanSwitch        bool                `json:"canSwitch"`
}

type MaterialNoticeBatchRequest struct {
	CourseID string `json:"courseId"`
}

type MaterialNoticeBatchResult struct {
	ResourceCount    int  `json:"resourceCount"`
	RecipientCount   int  `json:"recipientCount"`
	AlreadyCompleted bool `json:"alreadyCompleted"`
}

type PendingMaterialNoticeBatch struct {
	BatchID       string `json:"batchId"`
	CourseID      string `json:"courseId"`
	ResourceCount int    `json:"resourceCount"`
}
