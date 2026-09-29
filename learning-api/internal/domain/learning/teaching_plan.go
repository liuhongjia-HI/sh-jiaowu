package learning

// TeachingPlan is a teacher-only resource. It is intentionally separate from
// Material, whose existing student publication rules must remain unchanged.
type TeachingPlan struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Grade         string `json:"grade"`
	Subject       string `json:"subject"`
	FileID        string `json:"-"`
	FileName      string `json:"fileName"`
	FileSize      int64  `json:"fileSize"`
	FileType      string `json:"fileType"`
	PreviewStatus string `json:"previewStatus"`
	PreviewURL    string `json:"previewUrl"`
	DownloadURL   string `json:"downloadUrl,omitempty"`
	UploaderID    string `json:"uploaderId"`
	UploaderName  string `json:"uploaderName"`
	CreatedAt     string `json:"createdAt"`
}

type TeachingPlanScope struct {
	Grade   string `json:"grade"`
	Subject string `json:"subject"`
}

type TeachingPlanList struct {
	Plans        []TeachingPlan      `json:"plans"`
	UploadScopes []TeachingPlanScope `json:"uploadScopes"`
	CanUpload    bool                `json:"canUpload"`
}

type TeachingPlanUploadRequest struct {
	Title   string
	Grade   string
	Subject string
	File    FileAsset
}
