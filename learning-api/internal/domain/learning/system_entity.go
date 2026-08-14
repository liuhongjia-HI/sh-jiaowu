package learning

type SettingUpdateRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
