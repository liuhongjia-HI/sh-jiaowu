package learning

type CourseDirectorySyncRequest struct {
	SourceCourseID  string            `json:"sourceCourseId"`
	TargetCourseIDs []string          `json:"targetCourseIds"`
	Snapshots       map[string]string `json:"snapshots,omitempty"`
}

type CourseDirectorySyncTarget struct {
	CourseID   string   `json:"courseId"`
	CourseName string   `json:"courseName"`
	Added      []string `json:"added"`
	Updated    []string `json:"updated"`
	Preserved  int      `json:"preserved"`
	Snapshot   string   `json:"snapshot"`
	Error      string   `json:"error,omitempty"`
	Status     string   `json:"status,omitempty"`
}

type CourseDirectorySyncResult struct {
	Targets []CourseDirectorySyncTarget `json:"targets"`
}
