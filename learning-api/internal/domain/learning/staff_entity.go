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
