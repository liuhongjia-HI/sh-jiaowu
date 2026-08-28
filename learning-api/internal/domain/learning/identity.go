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
	UserID     string `json:"userId"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	AuthMethod string `json:"authMethod,omitempty"`
	StudentID  string `json:"studentId,omitempty"`
	// GuardianID 只在学生端登录（家长身份）时有值，标记这个 principal 背后是
	// 哪个家长；StudentID 仍然是当前查看哪个孩子——多子女切换只换 StudentID，
	// 不换 GuardianID。老师/管理员登录不涉及家长身份，这个字段留空。
	GuardianID         string   `json:"guardianId,omitempty"`
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
	// Status 为“正常”或“待审核”。待审核的学生会展示在家长名下，
	// 但不能切换进入，直到管理员在学生管理中审核通过。
	Status    string `json:"status"`
	CanSwitch bool   `json:"canSwitch"`
}

// StudentAccountAddRequest 是家长从小程序申请添加孩子时填写的最小资料。
// 家长手机号、关联关系和审核状态均由服务端从当前登录身份推导，客户端不能指定。
type StudentAccountAddRequest struct {
	Name       string `json:"name"`
	Grade      string `json:"grade"`
	SchoolName string `json:"schoolName"`
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
	// BindCode 是第二个家长（比如已经有一个家长绑过的孩子，妈妈想用自己的
	// 手机号也关联上）用来关联学生的邀请码，后台在学生详情页生成。带了这个
	// 字段就完全走"凭码关联"这条路，不走手机号匹配已有档案那条路——所以
	// 允许她的手机号跟任何已有档案都不一样，也不会因为"查不到"被拒绝。
	BindCode string `json:"bindCode,omitempty"`
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
