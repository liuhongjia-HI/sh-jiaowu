package learning

// Banner 是学生端小程序首页的运营轮播图：一张图片、一个可选跳转目标、
// 一段生效时间段。展示逻辑（是否在有效期内）由后端算好放进 Status，
// 前端只管按 Status 展示，不用自己比日期。
type Banner struct {
	ID        string `json:"id"`
	ImageURL  string `json:"imageUrl"`
	Title     string `json:"title"`
	LinkType  string `json:"linkType"` // none | page | url
	LinkValue string `json:"linkValue"`
	SortOrder int    `json:"sortOrder"`
	// StartsAt/EndsAt 为空表示不限制对应一端：StartsAt 空=立即生效，EndsAt 空=长期有效。
	StartsAt string `json:"startsAt,omitempty"`
	EndsAt   string `json:"endsAt,omitempty"`
	Enabled  bool   `json:"enabled"`
	// Status 是根据 Enabled + 生效时间段算出来的只读展示状态：生效中 / 未开始 / 已结束 / 已停用。
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type BannerUpsertRequest struct {
	ImageURL  string `json:"imageUrl"`
	Title     string `json:"title"`
	LinkType  string `json:"linkType"`
	LinkValue string `json:"linkValue"`
	SortOrder int    `json:"sortOrder"`
	StartsAt  string `json:"startsAt"`
	EndsAt    string `json:"endsAt"`
	Enabled   bool   `json:"enabled"`
}
