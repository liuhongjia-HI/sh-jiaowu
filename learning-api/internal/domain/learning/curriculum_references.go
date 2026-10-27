package learning

type CurriculumReference struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Title           string `json:"title"`
	CourseID        string `json:"courseId"`
	CourseName      string `json:"courseName"`
	LessonID        string `json:"lessonId"`
	Status          string `json:"status"`
	Path            string `json:"path"`
	SubmissionCount int    `json:"submissionCount"`
	InvalidBinding  bool   `json:"invalidBinding,omitempty"`
	Blocking        bool   `json:"blocking,omitempty"`
}

type CurriculumReferencesRequest struct {
	IncludeAll            bool     `json:"includeAll,omitempty"`
	NodeIDs               []string `json:"nodeIds"`
	TargetLearningSpaceID string   `json:"targetLearningSpaceId,omitempty"`
}

// Reference moves are checked against the current binding at save time.
// IDs, publication state, student submissions and reviews are never recreated.
type CurriculumReferenceMove struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	CourseID     string `json:"courseId"`
	FromLessonID string `json:"fromLessonId"`
	ToLessonID   string `json:"toLessonId"`
}
