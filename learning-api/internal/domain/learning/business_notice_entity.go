package learning

const (
	NoticeScheduleConfirmed = "schedule_confirmed"
	NoticeScheduleChanged   = "schedule_changed"
	NoticeScheduleReminder  = "schedule_reminder"
	NoticeScheduleCancelled = "schedule_cancelled"
	NoticeHomeworkSubmitted = "homework_submitted"
	NoticeHomeworkPublished = "homework_published"
	NoticeReviewCompleted   = "review_completed"
	NoticeReviewException   = "review_exception"
)

type BusinessNoticeBinding struct {
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
	ID          string                 `json:"id"`
	BatchID     string                 `json:"batchId"`
	Kind        string                 `json:"kind"`
	StudentID   string                 `json:"studentId"`
	StudentName string                 `json:"studentName"`
	SeriesID    string                 `json:"seriesId,omitempty"`
	RelatedID   string                 `json:"relatedId,omitempty"`
	Title       string                 `json:"title"`
	Summary     string                 `json:"summary"`
	Lessons     []BusinessLessonChange `json:"lessons"`
	CreatedAt   string                 `json:"createdAt"`
	ExpiresAt   string                 `json:"expiresAt"`
	Values      map[string]string      `json:"values,omitempty"`
}

type BusinessNoticeTask struct {
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
	Event          BusinessNoticeEvent `json:"event"`
	CurrentLessons []ScheduleClass     `json:"currentLessons"`
	CanSwitch      bool                `json:"canSwitch"`
}
