package learning

type MaterialDownloadScope struct {
	Subject     string   `json:"subject"`
	Grade       string   `json:"grade,omitempty"`
	Semester    string   `json:"semester,omitempty"`
	Phase       string   `json:"phase,omitempty"`
	CourseIDs   []string `json:"courseIds,omitempty"`
	MaterialIDs []string `json:"materialIds,omitempty"`
	LessonIDs   []string `json:"lessonIds,omitempty"`
}

type MaterialDownloadItem struct {
	MaterialID string `json:"materialId"`
	FileID     string `json:"fileId"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
}

type MaterialDownloadSelection struct {
	Count       int                      `json:"count"`
	Size        int64                    `json:"size"`
	Courses     []string                 `json:"courses"`
	Materials   []MaterialDownloadChoice `json:"materials,omitempty"`
	StudentName string                   `json:"studentName,omitempty"`
}

type MaterialDownloadChoice struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	Unit     string `json:"unit"`
	Chapter  string `json:"chapter"`
	Lesson   string `json:"lesson"`
}

type MaterialDownloadJob struct {
	CourseName  string                   `json:"courseName,omitempty"`
	Materials   []MaterialDownloadChoice `json:"materials,omitempty"`
	StudentID   string                   `json:"studentId,omitempty"`
	StudentName string                   `json:"studentName,omitempty"`
	GuardianID  string                   `json:"-"`
	ID          string                   `json:"id"`
	OwnerID     string                   `json:"-"`
	Scope       MaterialDownloadScope    `json:"scope"`
	Status      string                   `json:"status"`
	Count       int                      `json:"count"`
	Size        int64                    `json:"size"`
	Error       string                   `json:"error,omitempty"`
	CreatedAt   string                   `json:"createdAt"`
	ExpiresAt   string                   `json:"expiresAt,omitempty"`
	ArchivePath string                   `json:"-"`
	Items       []MaterialDownloadItem   `json:"-"`
}

type MaterialDownloadFile struct {
	Name string
	Path string
	Size int64
}
