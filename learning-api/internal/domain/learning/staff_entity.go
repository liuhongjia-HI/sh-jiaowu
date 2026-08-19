package learning

type Teacher struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	CampusID          string   `json:"campusId"`
	LearningSpaceIDs  []string `json:"learningSpaceIds"`
	LearningSpaces    []string `json:"learningSpaces"`
	Grades            []string `json:"grades"`
	Subjects          []string `json:"subjects"`
	CanUploadHandout  bool     `json:"canUploadHandout"`
	CanUploadQuestion bool     `json:"canUploadQuestion"`
	CanReview         bool     `json:"canReview"`
	AccountStatus     string   `json:"accountStatus"`
	BindStatus        string   `json:"bindStatus"`
	Remark            string   `json:"remark"`
	TemporaryPassword string   `json:"temporaryPassword,omitempty"`
	// ActiveClassCount 是这个老师名下还没结束、也没取消的排课数量。
	// 停用一个还带着课的老师会让教务照着课表联系一个登不进去的人，
	// 前端在停用前用这个数字弹确认，而不是等出了事故才发现。
	ActiveClassCount int `json:"activeClassCount"`
}

type TeacherUpsertRequest struct {
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	CampusID          string   `json:"campusId"`
	LearningSpaceIDs  []string `json:"learningSpaceIds"`
	CanUploadHandout  bool     `json:"canUploadHandout"`
	CanUploadQuestion bool     `json:"canUploadQuestion"`
	CanReview         bool     `json:"canReview"`
	AccountStatus     string   `json:"accountStatus"`
	Remark            string   `json:"remark"`
}

type LearningSpace struct {
	ID           string `json:"id"`
	AcademicYear string `json:"academicYear"`
	Grade        string `json:"grade"`
	Subject      string `json:"subject"`
	Semester     string `json:"semester"`
	Phase        string `json:"phase"`
	Name         string `json:"name"`
	Status       Status `json:"status"`
}

type AdminStaff struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Role          Role   `json:"role"`
	CampusID      string `json:"campusId,omitempty"`
	AccountStatus string `json:"accountStatus"`
	BindStatus    string `json:"bindStatus"`
	Remark        string `json:"remark"`
}

type AdminStaffUpsertRequest struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Role          Role   `json:"role"`
	CampusID      string `json:"campusId"`
	AccountStatus string `json:"accountStatus"`
	Remark        string `json:"remark"`
}

type PasswordChangeRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type PasswordResetResult struct {
	UserID             string `json:"userId"`
	TemporaryPassword  string `json:"temporaryPassword"`
	MustChangePassword bool   `json:"mustChangePassword"`
}
