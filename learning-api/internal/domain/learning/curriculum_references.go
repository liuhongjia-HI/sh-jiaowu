package learning

type CurriculumReference struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	CourseID   string `json:"courseId"`
	CourseName string `json:"courseName"`
	LessonID   string `json:"lessonId"`
	Blocking   bool   `json:"blocking,omitempty"`
}

type CurriculumReferencesRequest struct {
	NodeIDs               []string `json:"nodeIds"`
	TargetLearningSpaceID string   `json:"targetLearningSpaceId,omitempty"`
}
