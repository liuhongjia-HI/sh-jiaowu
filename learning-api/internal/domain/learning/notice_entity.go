package learning

type Notice struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Target          string `json:"target"`
	Summary         string `json:"summary"`
	Channel         string `json:"channel,omitempty"`
	RecipientOpenID string `json:"recipientOpenId,omitempty"`
	Status          string `json:"status"`
	FailureReason   string `json:"failureReason,omitempty"`
	RelatedType     string `json:"relatedType,omitempty"`
	RelatedID       string `json:"relatedId,omitempty"`
	RetryCount      int    `json:"retryCount,omitempty"`
}

type NoticeCreateRequest struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	Target          string `json:"target"`
	Summary         string `json:"summary"`
	Channel         string `json:"channel"`
	RecipientOpenID string `json:"recipientOpenId"`
	RelatedType     string `json:"relatedType"`
	RelatedID       string `json:"relatedId"`
}

type OperationLog struct {
	ID         string `json:"id"`
	Operator   string `json:"operator"`
	OperatorID string `json:"operatorId,omitempty"`
	IP         string `json:"ip,omitempty"`
	UserAgent  string `json:"userAgent,omitempty"`
	Action     string `json:"action"`
	Target     string `json:"target"`
	Detail     string `json:"detail,omitempty"`
	Time       string `json:"time"`
}
