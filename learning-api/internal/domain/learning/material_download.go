package learning

type MaterialDownloadScope struct {
	Subject   string   `json:"subject"`
	Grade     string   `json:"grade,omitempty"`
	Semester  string   `json:"semester,omitempty"`
	Phase     string   `json:"phase,omitempty"`
	CourseIDs []string `json:"courseIds,omitempty"`
	LessonIDs []string `json:"lessonIds,omitempty"`
}

type MaterialDownloadItem struct {
	MaterialID string `json:"materialId"`
	FileID     string `json:"fileId"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
}

type MaterialDownloadSelection struct {
	Count   int      `json:"count"`
	Size    int64    `json:"size"`
	Courses []string `json:"courses"`
}

type MaterialDownloadJob struct {
	ID          string                 `json:"id"`
	OwnerID     string                 `json:"-"`
	Scope       MaterialDownloadScope  `json:"scope"`
	Status      string                 `json:"status"`
	Count       int                    `json:"count"`
	Size        int64                  `json:"size"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   string                 `json:"createdAt"`
	ExpiresAt   string                 `json:"expiresAt,omitempty"`
	ArchivePath string                 `json:"-"`
	Items       []MaterialDownloadItem `json:"-"`
}

type MaterialDownloadFile struct {
	Name string
	Path string
	Size int64
}
