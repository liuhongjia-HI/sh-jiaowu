package learning

type Status string

const (
	StatusEnabled  Status = "启用"
	StatusDraft    Status = "草稿"
	StatusDisabled Status = "停用"
)

type User struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Phone              string   `json:"phone"`
	OpenID             string   `json:"openId"`
	UnionID            string   `json:"unionId"`
	PasswordHash       string   `json:"-"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	TokenVersion       int      `json:"tokenVersion,omitempty"`
	AccountStatus      string   `json:"accountStatus"`
	Remark             string   `json:"remark,omitempty"`
	Roles              []Role   `json:"roles"`
	StudentID          string   `json:"studentId,omitempty"`
	CampusID           string   `json:"campusId,omitempty"`
	CampusScopes       []string `json:"campusScopes,omitempty"`
	LearningSpaceIDs   []string `json:"learningSpaceIds,omitempty"`
	CanUploadHandout   bool     `json:"canUploadHandout,omitempty"`
	CanUploadQuestion  bool     `json:"canUploadQuestion,omitempty"`
	CanReview          bool     `json:"canReview,omitempty"`
}

type Principal struct {
	UserID             string   `json:"userId"`
	Name               string   `json:"name"`
	Phone              string   `json:"phone,omitempty"`
	AuthMethod         string   `json:"authMethod,omitempty"`
	StudentID          string   `json:"studentId,omitempty"`
	CampusID           string   `json:"campusId,omitempty"`
	Roles              []Role   `json:"roles"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	TokenVersion       int      `json:"tokenVersion,omitempty"`
	CampusScopes       []string `json:"campusScopes,omitempty"`
	LearningSpaceIDs   []string `json:"learningSpaceIds,omitempty"`
	CanUploadHandout   bool     `json:"canUploadHandout,omitempty"`
	CanUploadQuestion  bool     `json:"canUploadQuestion,omitempty"`
	CanReview          bool     `json:"canReview,omitempty"`
}

type AuthResult struct {
	Token      string    `json:"token"`
	User       Principal `json:"user"`
	AuthMethod string    `json:"authMethod"`
}
type WechatLoginRequest struct {
	Code        string `json:"code"`
	Phone       string `json:"phone"`
	PhoneCode   string `json:"phoneCode"`
	StudentName string `json:"studentName"`
	SchoolName  string `json:"schoolName"`
	Grade       string `json:"grade"`
}

type Role string

const (
	RoleStudent     Role = "student"
	RoleTeacher     Role = "teacher"
	RoleOpsStaff    Role = "ops_staff"
	RoleCampusAdmin Role = "campus_admin"
	RoleSuperAdmin  Role = "super_admin"
)
