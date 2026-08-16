package learningapp

import "starline/learning-api/internal/domain/learning"

// AuthRepository is the narrow dependency needed by authentication flows.
type AuthRepository interface {
	LoginWithWechatCode(learning.WechatLoginRequest) (learning.Principal, error)
	LoginWithAdminPassword(phone, password string) (learning.Principal, error)
	LoginWithDemoStudentPassword(phone, password string) (learning.Principal, error)
	ChangePassword(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error
	ResetPassword(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error)
	RecordSecurityEvent(operator, action, target, detail string) error
	PrincipalByUserID(userID string) (learning.Principal, error)
	StudentAccounts(learning.Principal) ([]learning.StudentAccount, error)
	SwitchStudentAccount(learning.Principal, string) (learning.Principal, error)
}

// StaffRepository keeps personnel administration independent from content and scheduling.
type StaffRepository interface {
	AdminStaff() []learning.AdminStaff
	CreateAdminStaff(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error)
	UpdateAdminStaff(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error)
	Teachers(principal learning.Principal) []learning.Teacher
	CreateTeacher(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error)
	UpdateTeacher(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error)
}

func (s *Service) LoginWithWechatCode(req learning.WechatLoginRequest) (learning.Principal, error) {
	return s.auth.LoginWithWechatCode(req)
}
func (s *Service) LoginWithAdminPassword(phone, password string) (learning.Principal, error) {
	return s.auth.LoginWithAdminPassword(phone, password)
}
func (s *Service) LoginWithDemoStudentPassword(phone, password string) (learning.Principal, error) {
	return s.auth.LoginWithDemoStudentPassword(phone, password)
}
func (s *Service) ChangePassword(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error {
	return s.auth.ChangePassword(operator, principal, req)
}
func (s *Service) ResetPassword(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error) {
	return s.auth.ResetPassword(operator, principal, userID)
}
func (s *Service) RecordSecurityEvent(operator, action, target, detail string) error {
	return s.auth.RecordSecurityEvent(operator, action, target, detail)
}
func (s *Service) PrincipalByUserID(userID string) (learning.Principal, error) {
	return s.auth.PrincipalByUserID(userID)
}
func (s *Service) StudentAccounts(principal learning.Principal) ([]learning.StudentAccount, error) {
	return s.auth.StudentAccounts(principal)
}
func (s *Service) SwitchStudentAccount(principal learning.Principal, studentID string) (learning.Principal, error) {
	return s.auth.SwitchStudentAccount(principal, studentID)
}
func (s *Service) AdminStaff() []learning.AdminStaff { return s.staff.AdminStaff() }
func (s *Service) CreateAdminStaff(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	return s.staff.CreateAdminStaff(operator, req)
}
func (s *Service) UpdateAdminStaff(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	return s.staff.UpdateAdminStaff(operator, principal, id, req)
}
func (s *Service) Teachers(principal learning.Principal) []learning.Teacher {
	return s.staff.Teachers(principal)
}
func (s *Service) CreateTeacher(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	return s.staff.CreateTeacher(operator, principal, req)
}
func (s *Service) UpdateTeacher(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	return s.staff.UpdateTeacher(operator, principal, id, req)
}
