package learning

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
	CourseID             string   `json:"courseId"`
	TeacherID            string   `json:"teacherId"`
	CampusID             string   `json:"campusId"`
	RoomName             string   `json:"roomName"`
	ClassType            string   `json:"classType"`
	DurationMinutes      int      `json:"durationMinutes"`
	DayOfWeek            int      `json:"dayOfWeek"`
	StartTime            string   `json:"startTime"`
	EndTime              string   `json:"endTime"`
	StartDate            string   `json:"startDate"`
	EndDate              string   `json:"endDate"`
	StudentIDs           []string `json:"studentIds"`
	ExpectedStudentCount int      `json:"expectedStudentCount"`
	ReservationNote      string   `json:"reservationNote"`
}

type ScheduleClass struct {
	ID                   string             `json:"id"`
	Name                 string             `json:"name"`
	CourseID             string             `json:"courseId"`
	CourseName           string             `json:"courseName"`
	TeacherID            string             `json:"teacherId"`
	TeacherName          string             `json:"teacherName"`
	CampusID             string             `json:"campusId"`
	RoomName             string             `json:"roomName"`
	ClassType            string             `json:"classType"`
	Capacity             int                `json:"capacity"`
	DurationMinutes      int                `json:"durationMinutes"`
	DayOfWeek            int                `json:"dayOfWeek"`
	StartTime            string             `json:"startTime"`
	EndTime              string             `json:"endTime"`
	StartDate            string             `json:"startDate"`
	EndDate              string             `json:"endDate"`
	Students             []CandidateStudent `json:"students"`
	ExpectedStudentCount int                `json:"expectedStudentCount"`
	ReservationNote      string             `json:"reservationNote,omitempty"`
	Status               string             `json:"status"`
	CreatedAt            string             `json:"createdAt"`
}
