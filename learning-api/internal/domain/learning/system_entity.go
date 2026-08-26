package learning

type SettingUpdateRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SubjectMetadata 是可复用的学科展示元数据。学科名称本身已被课程、学习空间等
// 业务数据引用，因此这里仅允许维护显示属性，不允许在系统设置中改名。
type SubjectMetadata struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ShortLabel string `json:"shortLabel"`
	Color      string `json:"color"`
	SortOrder  int    `json:"sortOrder"`
	Status     string `json:"status"`
}

type SubjectMetadataUpdateRequest struct {
	ShortLabel string `json:"shortLabel"`
	Color      string `json:"color"`
	SortOrder  int    `json:"sortOrder"`
	Status     string `json:"status"`
}

type DashboardOverview struct {
	OpenedStudents   int `json:"openedStudents"`
	PackageCount     int `json:"packageCount"`
	PendingReviews   int `json:"pendingReviews"`
	MaterialViews    int `json:"materialViews"`
	ExpiringStudents int `json:"expiringStudents"`
	UnpublishedFiles int `json:"unpublishedFiles"`
}

type ReadinessItem struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

type SystemReadiness struct {
	ReadyCount int             `json:"readyCount"`
	TotalCount int             `json:"totalCount"`
	Items      []ReadinessItem `json:"items"`
}
