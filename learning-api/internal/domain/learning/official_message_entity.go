package learning

// WechatSettings 是管理后台可见的单例微信配置。敏感值永远不回传，
// 只通过 *Configured 告知前端是否已经保存。
type WechatSettings struct {
	MiniProgramName                 string `json:"miniProgramName"`
	MiniProgramAppID                string `json:"miniProgramAppId"`
	MiniProgramSecretConfigured     bool   `json:"miniProgramSecretConfigured"`
	OfficialAccountName             string `json:"officialAccountName"`
	OfficialAccountAppID            string `json:"officialAccountAppId"`
	OfficialAccountOriginalID       string `json:"officialAccountOriginalId"`
	OfficialAccountSecretConfigured bool   `json:"officialAccountSecretConfigured"`
	CallbackTokenConfigured         bool   `json:"callbackTokenConfigured"`
	EncodingAESKeyConfigured        bool   `json:"encodingAesKeyConfigured"`
	CallbackURL                     string `json:"callbackUrl"`
}

type WechatSettingsUpdateRequest struct {
	MiniProgramName           string `json:"miniProgramName"`
	MiniProgramAppID          string `json:"miniProgramAppId"`
	MiniProgramAppSecret      string `json:"miniProgramAppSecret"`
	OfficialAccountName       string `json:"officialAccountName"`
	OfficialAccountAppID      string `json:"officialAccountAppId"`
	OfficialAccountAppSecret  string `json:"officialAccountAppSecret"`
	OfficialAccountOriginalID string `json:"officialAccountOriginalId"`
	CallbackToken             string `json:"callbackToken"`
	EncodingAESKey            string `json:"encodingAesKey"`
}

type OfficialTemplateField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Example   string `json:"example,omitempty"`
	MaxLength int    `json:"maxLength,omitempty"`
}

type OfficialTemplate struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title"`
	Content  string                  `json:"content"`
	Example  string                  `json:"example,omitempty"`
	Fields   []OfficialTemplateField `json:"fields"`
	Status   string                  `json:"status"`
	SyncedAt string                  `json:"syncedAt"`
}

type OfficialFollower struct {
	OpenID         string `json:"openId"`
	UnionID        string `json:"unionId,omitempty"`
	Subscribed     bool   `json:"subscribed"`
	SubscribedAt   string `json:"subscribedAt,omitempty"`
	UnsubscribedAt string `json:"unsubscribedAt,omitempty"`
	SyncedAt       string `json:"syncedAt"`
}

type OfficialFollowerSyncResult struct {
	TotalCount      int `json:"totalCount"`
	SubscribedCount int `json:"subscribedCount"`
	MatchedCount    int `json:"matchedCount"`
	UnmatchedCount  int `json:"unmatchedCount"`
}

type OfficialAudiencePreviewRequest struct {
	Grades []string `json:"grades"`
}

type OfficialAudiencePreview struct {
	StudentCount       int      `json:"studentCount"`
	GuardianCount      int      `json:"guardianCount"`
	ReachableCount     int      `json:"reachableCount"`
	UnreachableCount   int      `json:"unreachableCount"`
	UnmatchedCount     int      `json:"unmatchedCount"`
	DuplicateCount     int      `json:"duplicateCount"`
	Grades             []string `json:"grades"`
	UnreachableReasons []string `json:"unreachableReasons,omitempty"`
}

type OfficialCampaignCreateRequest struct {
	TemplateID string            `json:"templateId"`
	Grades     []string          `json:"grades"`
	Values     map[string]string `json:"values"`
	PagePath   string            `json:"pagePath"`
	Draft      bool              `json:"draft"`
}

type OfficialCampaign struct {
	ID            string            `json:"id"`
	TemplateID    string            `json:"templateId"`
	TemplateTitle string            `json:"templateTitle"`
	Grades        []string          `json:"grades"`
	Values        map[string]string `json:"values"`
	PagePath      string            `json:"pagePath"`
	TargetCount   int               `json:"targetCount"`
	SuccessCount  int               `json:"successCount"`
	FailureCount  int               `json:"failureCount"`
	Status        string            `json:"status"`
	CreatedBy     string            `json:"createdBy"`
	CreatedAt     string            `json:"createdAt"`
	SentAt        string            `json:"sentAt,omitempty"`
}

type OfficialCampaignRecipient struct {
	ID            string `json:"id"`
	CampaignID    string `json:"campaignId"`
	GuardianID    string `json:"guardianId"`
	GuardianName  string `json:"guardianName"`
	OpenID        string `json:"-"`
	StudentNames  string `json:"studentNames"`
	Status        string `json:"status"`
	FailureReason string `json:"failureReason,omitempty"`
	RetryCount    int    `json:"retryCount"`
	SentAt        string `json:"sentAt,omitempty"`
}

type OfficialCampaignDetail struct {
	Campaign   OfficialCampaign            `json:"campaign"`
	Recipients []OfficialCampaignRecipient `json:"recipients"`
}
