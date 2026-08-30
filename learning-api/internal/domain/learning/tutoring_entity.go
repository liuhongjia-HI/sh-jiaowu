package learning

// TutoringAssignment is the durable responsibility relationship between a
// student and a teacher. A schedule class records one lesson; it must not be
// used as the source of truth for an ongoing tutoring responsibility.
type TutoringAssignment struct {
	ID            string `json:"id"`
	StudentID     string `json:"studentId"`
	TeacherID     string `json:"teacherId"`
	TeacherName   string `json:"teacherName"`
	CampusID      string `json:"campusId"`
	AcademicYear  string `json:"academicYear"`
	GradeSnapshot string `json:"gradeSnapshot"`
	SubjectID     string `json:"subjectId"`
	SubjectName   string `json:"subjectName"`
	LevelCode     string `json:"levelCode"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	SourceType    string `json:"sourceType"`
	SourceID      string `json:"sourceId,omitempty"`
	StartsAt      string `json:"startsAt"`
	EndsAt        string `json:"endsAt,omitempty"`
	EndedReason   string `json:"endedReason,omitempty"`
	AssignedBy    string `json:"assignedBy"`
	EndedBy       string `json:"endedBy,omitempty"`
	Version       int    `json:"version"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

const (
	TutoringAssignmentPrimary   = "primary"
	TutoringAssignmentAssistant = "assistant"

	TutoringAssignmentPending = "pending"
	TutoringAssignmentActive  = "active"
	TutoringAssignmentEnded   = "ended"
)

type TutoringAssignmentCreateRequest struct {
	TeacherID  string `json:"teacherId"`
	SubjectID  string `json:"subjectId"`
	LevelCode  string `json:"levelCode"`
	Role       string `json:"role"`
	StartsAt   string `json:"startsAt"`
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
}

type TutoringAssignmentEndRequest struct {
	EndsAt  string `json:"endsAt"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
}

type TutoringAssignmentTransferRequest struct {
	TeacherID string `json:"teacherId"`
	StartsAt  string `json:"startsAt"`
	Reason    string `json:"reason"`
	Version   int    `json:"version"`
}
