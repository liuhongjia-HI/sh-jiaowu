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

type StudentAccount struct {
	StudentID string `json:"studentId"`
	Name      string `json:"name"`
	Grade     string `json:"grade"`
	Active    bool   `json:"active"`
}
type WechatLoginRequest struct {
	Code        string `json:"code"`
	Phone       string `json:"phone"`
	PhoneCode   string `json:"phoneCode"`
	StudentName string `json:"studentName"`
	SchoolName  string `json:"schoolName"`
	Grade       string `json:"grade"`
	// SelectedStudentID 用于多子女场景的二次提交：手机号命中多个学生档案时，
	// 后端先返回 StudentSelectionRequiredError 里的候选列表，小程序弹选择框，
	// 家长选中后带着这个字段重新提交同一个登录请求完成绑定/登录。
	SelectedStudentID string `json:"selectedStudentId"`
}

// StudentSelectionRequiredError 表示同一个手机号下匹配到多个学生档案（多子女），
// 需要家长明确选择要绑定/登录哪一个，而不是像"账号冲突"那样直接拒绝登录。
// 调用方用 errors.As 识别这个类型来渲染选择界面。
type StudentSelectionRequiredError struct {
	Candidates []StudentAccount
}

func (e *StudentSelectionRequiredError) Error() string {
	return "手机号匹配到多个学生档案，请选择要绑定的学生"
}

type Role string

const (
	RoleStudent     Role = "student"
	RoleTeacher     Role = "teacher"
	RoleOpsStaff    Role = "ops_staff"
	RoleCampusAdmin Role = "campus_admin"
	RoleSuperAdmin  Role = "super_admin"
)
