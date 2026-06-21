package learningapp

import "starline/learning-api/internal/domain/learning"

type Repository interface {
	LoginWithWechatCode(code, phone, phoneCode string) (learning.Principal, error)
	LoginWithAdminPassword(phone, password string) (learning.Principal, error)
	LoginWithDemoStudentPassword(phone, password string) (learning.Principal, error)
	ChangePassword(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error
	ResetPassword(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error)
	RecordSecurityEvent(operator, action, target, detail string)
	PrincipalByUserID(userID string) (learning.Principal, error)
	AdminStaff() []learning.AdminStaff
	CreateAdminStaff(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error)
	UpdateAdminStaff(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error)
	Teachers(principal learning.Principal) []learning.Teacher
	CreateTeacher(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error)
	UpdateTeacher(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error)
	Dashboard() learning.DashboardOverview
	Packages() []learning.Package
	CreatePackage(operator string, req learning.PackageUpsertRequest) (learning.Package, error)
	UpdatePackage(operator string, id string, req learning.PackageUpsertRequest) (learning.Package, error)
	LearningSpaces() []learning.LearningSpace
	Students(principal learning.Principal, query learning.StudentQuery) []learning.Student
	StudentDetail(principal learning.Principal, id string) (learning.StudentDetail, error)
	CreateStudent(operator string, principal learning.Principal, req learning.StudentUpsertRequest) (learning.Student, error)
	UpdateStudent(operator string, principal learning.Principal, id string, req learning.StudentUpsertRequest) (learning.Student, error)
	RemindStudent(operator string, principal learning.Principal, id string) (learning.StudentRemindResult, error)
	ImportStudents(operator string, principal learning.Principal, rows []learning.StudentUpsertRequest) learning.StudentImportResult
	StudentGrants(principal learning.Principal, id string) ([]learning.StudentGrant, error)
	StudentLearningRecords(principal learning.Principal, id string) ([]learning.StudentLearningRecord, error)
	CommercialSummary(principal learning.Principal) learning.CommercialSummary
	CommercialOrders(principal learning.Principal) []learning.CommercialOrder
	CreateCommercialOrder(operator string, principal learning.Principal, req learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error)
	CreatePayment(operator string, principal learning.Principal, orderID string, req learning.PaymentCreateRequest) (learning.PaymentRecord, error)
	CreateRefund(operator string, principal learning.Principal, orderID string, req learning.RefundCreateRequest) (learning.RefundRecord, error)
	CreateContract(operator string, principal learning.Principal, orderID string, req learning.ContractCreateRequest) (learning.ContractRecord, error)
	CreateInvoice(operator string, principal learning.Principal, orderID string, req learning.InvoiceCreateRequest) (learning.InvoiceRecord, error)
	CreateLessonConsumption(operator string, principal learning.Principal, req learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error)
	CreateRenewalReminder(operator string, principal learning.Principal, req learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error)
	CreateParentNotice(operator string, principal learning.Principal, req learning.ParentNoticeCreateRequest) (learning.ParentNotice, error)
	Courses(principal learning.Principal) []learning.Course
	CreateCourse(operator string, principal learning.Principal, req learning.CourseUpsertRequest) (learning.Course, error)
	UpdateCourse(operator string, principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error)
	Materials(principal learning.Principal) []learning.Material
	CreateMaterial(operator string, principal learning.Principal, req learning.MaterialUploadRequest) (learning.Material, error)
	UpdateMaterial(operator string, principal learning.Principal, id string, req learning.MaterialUpdateRequest) (learning.Material, error)
	Homework(principal learning.Principal) []learning.Homework
	CreateHomework(operator string, principal learning.Principal, req learning.HomeworkUploadRequest) (learning.Homework, error)
	UpdateHomework(operator string, principal learning.Principal, id string, req learning.HomeworkUpdateRequest) (learning.Homework, error)
	ContentFile(principal learning.Principal, fileID string) (learning.FileAsset, error)
	Reviews(principal learning.Principal) []learning.Review
	CompleteReview(operator string, principal learning.Principal, id string, req learning.ReviewCompleteRequest) (learning.Submission, error)
	Notices(principal learning.Principal) []learning.Notice
	CreateNotice(operator string, principal learning.Principal, req learning.NoticeCreateRequest) (learning.Notice, error)
	Logs() []learning.OperationLog
	Settings() map[string]string
	UpdateSetting(operator string, req learning.SettingUpdateRequest) (map[string]string, error)
	StudentPermissions() []learning.StudentPermissionSummary
	PackagePermissions() []learning.PackagePermissionSummary
	ContentPermissions() []learning.ContentPermissionSummary
	GrantPreview(studentID, packageID string) (learning.GrantPreview, error)
	CreateGrant(operator, studentID, packageID string) (learning.GrantPreview, error)
	StudentHome(principal learning.Principal) (learning.StudentHome, error)
	Availability(principal learning.Principal, ownerType, ownerID string) ([]learning.AvailabilitySlot, error)
	AvailabilityOverview(principal learning.Principal) []learning.AvailabilitySlot
	SaveAvailability(operator string, principal learning.Principal, req learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error)
	ScheduleCandidates(principal learning.Principal, req learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error)
	ScheduleClasses(principal learning.Principal) []learning.ScheduleClass
	CreateScheduleClass(operator string, principal learning.Principal, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error)
	UpdateScheduleClass(operator string, principal learning.Principal, id string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error)
	CancelScheduleClass(operator string, principal learning.Principal, id string) (learning.ScheduleClass, error)
	StudentSchedule(principal learning.Principal) ([]learning.ScheduleClass, error)
	StudentCourseDetail(principal learning.Principal, courseID string) (learning.StudentCourseDetail, error)
	StudentMaterial(principal learning.Principal, materialID string) (learning.Material, error)
	StudentHomework(principal learning.Principal, homeworkID string) (learning.Homework, error)
	CreateSubmission(operator string, principal learning.Principal, req learning.SubmissionRequest) (learning.Submission, error)
	StudentSubmission(principal learning.Principal, id string) (learning.Submission, error)
	StudentStudy(principal learning.Principal) (learning.StudentStudyBoard, error)
	StudentTasks(principal learning.Principal) ([]learning.StudentTask, error)
	StudentGrowth(principal learning.Principal) ([]learning.StudentLearningRecord, error)
	StudentBadges(principal learning.Principal) ([]learning.Badge, error)
	StudentFavorites(principal learning.Principal) ([]learning.Favorite, error)
	AddFavorite(operator string, principal learning.Principal, req learning.FavoriteRequest) (learning.Favorite, error)
	RemoveFavorite(operator string, principal learning.Principal, id string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) LoginWithWechatCode(code, phone, phoneCode string) (learning.Principal, error) {
	return s.repo.LoginWithWechatCode(code, phone, phoneCode)
}
func (s *Service) LoginWithAdminPassword(phone, password string) (learning.Principal, error) {
	return s.repo.LoginWithAdminPassword(phone, password)
}
func (s *Service) LoginWithDemoStudentPassword(phone, password string) (learning.Principal, error) {
	return s.repo.LoginWithDemoStudentPassword(phone, password)
}
func (s *Service) ChangePassword(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error {
	return s.repo.ChangePassword(operator, principal, req)
}
func (s *Service) ResetPassword(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error) {
	return s.repo.ResetPassword(operator, principal, userID)
}
func (s *Service) RecordSecurityEvent(operator, action, target, detail string) {
	s.repo.RecordSecurityEvent(operator, action, target, detail)
}
func (s *Service) PrincipalByUserID(userID string) (learning.Principal, error) {
	return s.repo.PrincipalByUserID(userID)
}
func (s *Service) AdminStaff() []learning.AdminStaff {
	return s.repo.AdminStaff()
}
func (s *Service) CreateAdminStaff(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	return s.repo.CreateAdminStaff(operator, req)
}
func (s *Service) UpdateAdminStaff(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	return s.repo.UpdateAdminStaff(operator, principal, id, req)
}
func (s *Service) Teachers(principal learning.Principal) []learning.Teacher {
	return s.repo.Teachers(principal)
}
func (s *Service) CreateTeacher(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	return s.repo.CreateTeacher(operator, principal, req)
}
func (s *Service) UpdateTeacher(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	return s.repo.UpdateTeacher(operator, principal, id, req)
}
func (s *Service) Dashboard() learning.DashboardOverview { return s.repo.Dashboard() }
func (s *Service) Packages() []learning.Package          { return s.repo.Packages() }
func (s *Service) CreatePackage(operator string, req learning.PackageUpsertRequest) (learning.Package, error) {
	return s.repo.CreatePackage(operator, req)
}
func (s *Service) UpdatePackage(operator string, id string, req learning.PackageUpsertRequest) (learning.Package, error) {
	return s.repo.UpdatePackage(operator, id, req)
}
func (s *Service) LearningSpaces() []learning.LearningSpace {
	return s.repo.LearningSpaces()
}
func (s *Service) Students(principal learning.Principal, query learning.StudentQuery) []learning.Student {
	return s.repo.Students(principal, query)
}
func (s *Service) StudentDetail(principal learning.Principal, id string) (learning.StudentDetail, error) {
	return s.repo.StudentDetail(principal, id)
}
func (s *Service) CreateStudent(operator string, principal learning.Principal, req learning.StudentUpsertRequest) (learning.Student, error) {
	return s.repo.CreateStudent(operator, principal, req)
}
func (s *Service) UpdateStudent(operator string, principal learning.Principal, id string, req learning.StudentUpsertRequest) (learning.Student, error) {
	return s.repo.UpdateStudent(operator, principal, id, req)
}
func (s *Service) RemindStudent(operator string, principal learning.Principal, id string) (learning.StudentRemindResult, error) {
	return s.repo.RemindStudent(operator, principal, id)
}
func (s *Service) ImportStudents(operator string, principal learning.Principal, rows []learning.StudentUpsertRequest) learning.StudentImportResult {
	return s.repo.ImportStudents(operator, principal, rows)
}
func (s *Service) StudentGrants(principal learning.Principal, id string) ([]learning.StudentGrant, error) {
	return s.repo.StudentGrants(principal, id)
}
func (s *Service) StudentLearningRecords(principal learning.Principal, id string) ([]learning.StudentLearningRecord, error) {
	return s.repo.StudentLearningRecords(principal, id)
}
func (s *Service) CommercialSummary(principal learning.Principal) learning.CommercialSummary {
	return s.repo.CommercialSummary(principal)
}
func (s *Service) CommercialOrders(principal learning.Principal) []learning.CommercialOrder {
	return s.repo.CommercialOrders(principal)
}
func (s *Service) CreateCommercialOrder(operator string, principal learning.Principal, req learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error) {
	return s.repo.CreateCommercialOrder(operator, principal, req)
}
func (s *Service) CreatePayment(operator string, principal learning.Principal, orderID string, req learning.PaymentCreateRequest) (learning.PaymentRecord, error) {
	return s.repo.CreatePayment(operator, principal, orderID, req)
}
func (s *Service) CreateRefund(operator string, principal learning.Principal, orderID string, req learning.RefundCreateRequest) (learning.RefundRecord, error) {
	return s.repo.CreateRefund(operator, principal, orderID, req)
}
func (s *Service) CreateContract(operator string, principal learning.Principal, orderID string, req learning.ContractCreateRequest) (learning.ContractRecord, error) {
	return s.repo.CreateContract(operator, principal, orderID, req)
}
func (s *Service) CreateInvoice(operator string, principal learning.Principal, orderID string, req learning.InvoiceCreateRequest) (learning.InvoiceRecord, error) {
	return s.repo.CreateInvoice(operator, principal, orderID, req)
}
func (s *Service) CreateLessonConsumption(operator string, principal learning.Principal, req learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error) {
	return s.repo.CreateLessonConsumption(operator, principal, req)
}
func (s *Service) CreateRenewalReminder(operator string, principal learning.Principal, req learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error) {
	return s.repo.CreateRenewalReminder(operator, principal, req)
}
func (s *Service) CreateParentNotice(operator string, principal learning.Principal, req learning.ParentNoticeCreateRequest) (learning.ParentNotice, error) {
	return s.repo.CreateParentNotice(operator, principal, req)
}
func (s *Service) Courses(principal learning.Principal) []learning.Course {
	return s.repo.Courses(principal)
}
func (s *Service) CreateCourse(operator string, principal learning.Principal, req learning.CourseUpsertRequest) (learning.Course, error) {
	return s.repo.CreateCourse(operator, principal, req)
}
func (s *Service) UpdateCourse(operator string, principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error) {
	return s.repo.UpdateCourse(operator, principal, id, req)
}
func (s *Service) Materials(principal learning.Principal) []learning.Material {
	return s.repo.Materials(principal)
}
func (s *Service) CreateMaterial(operator string, principal learning.Principal, req learning.MaterialUploadRequest) (learning.Material, error) {
	return s.repo.CreateMaterial(operator, principal, req)
}
func (s *Service) UpdateMaterial(operator string, principal learning.Principal, id string, req learning.MaterialUpdateRequest) (learning.Material, error) {
	return s.repo.UpdateMaterial(operator, principal, id, req)
}
func (s *Service) Homework(principal learning.Principal) []learning.Homework {
	return s.repo.Homework(principal)
}
func (s *Service) CreateHomework(operator string, principal learning.Principal, req learning.HomeworkUploadRequest) (learning.Homework, error) {
	return s.repo.CreateHomework(operator, principal, req)
}
func (s *Service) UpdateHomework(operator string, principal learning.Principal, id string, req learning.HomeworkUpdateRequest) (learning.Homework, error) {
	return s.repo.UpdateHomework(operator, principal, id, req)
}
func (s *Service) ContentFile(principal learning.Principal, fileID string) (learning.FileAsset, error) {
	return s.repo.ContentFile(principal, fileID)
}
func (s *Service) Reviews(principal learning.Principal) []learning.Review {
	return s.repo.Reviews(principal)
}
func (s *Service) CompleteReview(operator string, principal learning.Principal, id string, req learning.ReviewCompleteRequest) (learning.Submission, error) {
	return s.repo.CompleteReview(operator, principal, id, req)
}
func (s *Service) Notices(principal learning.Principal) []learning.Notice {
	return s.repo.Notices(principal)
}
func (s *Service) CreateNotice(operator string, principal learning.Principal, req learning.NoticeCreateRequest) (learning.Notice, error) {
	return s.repo.CreateNotice(operator, principal, req)
}
func (s *Service) Logs() []learning.OperationLog { return s.repo.Logs() }
func (s *Service) StudentPermissions() []learning.StudentPermissionSummary {
	return s.repo.StudentPermissions()
}
func (s *Service) PackagePermissions() []learning.PackagePermissionSummary {
	return s.repo.PackagePermissions()
}
func (s *Service) ContentPermissions() []learning.ContentPermissionSummary {
	return s.repo.ContentPermissions()
}
func (s *Service) Settings() map[string]string { return s.repo.Settings() }
func (s *Service) UpdateSetting(operator string, req learning.SettingUpdateRequest) (map[string]string, error) {
	return s.repo.UpdateSetting(operator, req)
}
func (s *Service) GrantPreview(studentID, packageID string) (learning.GrantPreview, error) {
	return s.repo.GrantPreview(studentID, packageID)
}
func (s *Service) CreateGrant(operator, studentID, packageID string) (learning.GrantPreview, error) {
	return s.repo.CreateGrant(operator, studentID, packageID)
}
func (s *Service) StudentHome(principal learning.Principal) (learning.StudentHome, error) {
	return s.repo.StudentHome(principal)
}
func (s *Service) Availability(principal learning.Principal, ownerType, ownerID string) ([]learning.AvailabilitySlot, error) {
	return s.repo.Availability(principal, ownerType, ownerID)
}
func (s *Service) AvailabilityOverview(principal learning.Principal) []learning.AvailabilitySlot {
	return s.repo.AvailabilityOverview(principal)
}
func (s *Service) SaveAvailability(operator string, principal learning.Principal, req learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error) {
	return s.repo.SaveAvailability(operator, principal, req)
}
func (s *Service) ScheduleCandidates(principal learning.Principal, req learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error) {
	return s.repo.ScheduleCandidates(principal, req)
}
func (s *Service) ScheduleClasses(principal learning.Principal) []learning.ScheduleClass {
	return s.repo.ScheduleClasses(principal)
}
func (s *Service) CreateScheduleClass(operator string, principal learning.Principal, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	return s.repo.CreateScheduleClass(operator, principal, req)
}
func (s *Service) UpdateScheduleClass(operator string, principal learning.Principal, id string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	return s.repo.UpdateScheduleClass(operator, principal, id, req)
}
func (s *Service) CancelScheduleClass(operator string, principal learning.Principal, id string) (learning.ScheduleClass, error) {
	return s.repo.CancelScheduleClass(operator, principal, id)
}
func (s *Service) StudentSchedule(principal learning.Principal) ([]learning.ScheduleClass, error) {
	return s.repo.StudentSchedule(principal)
}
func (s *Service) StudentCourseDetail(principal learning.Principal, courseID string) (learning.StudentCourseDetail, error) {
	return s.repo.StudentCourseDetail(principal, courseID)
}
func (s *Service) StudentMaterial(principal learning.Principal, materialID string) (learning.Material, error) {
	return s.repo.StudentMaterial(principal, materialID)
}
func (s *Service) StudentHomework(principal learning.Principal, homeworkID string) (learning.Homework, error) {
	return s.repo.StudentHomework(principal, homeworkID)
}
func (s *Service) CreateSubmission(operator string, principal learning.Principal, req learning.SubmissionRequest) (learning.Submission, error) {
	return s.repo.CreateSubmission(operator, principal, req)
}
func (s *Service) StudentSubmission(principal learning.Principal, id string) (learning.Submission, error) {
	return s.repo.StudentSubmission(principal, id)
}
func (s *Service) StudentStudy(principal learning.Principal) (learning.StudentStudyBoard, error) {
	return s.repo.StudentStudy(principal)
}
func (s *Service) StudentTasks(principal learning.Principal) ([]learning.StudentTask, error) {
	return s.repo.StudentTasks(principal)
}
func (s *Service) StudentGrowth(principal learning.Principal) ([]learning.StudentLearningRecord, error) {
	return s.repo.StudentGrowth(principal)
}
func (s *Service) StudentBadges(principal learning.Principal) ([]learning.Badge, error) {
	return s.repo.StudentBadges(principal)
}
func (s *Service) StudentFavorites(principal learning.Principal) ([]learning.Favorite, error) {
	return s.repo.StudentFavorites(principal)
}
func (s *Service) AddFavorite(operator string, principal learning.Principal, req learning.FavoriteRequest) (learning.Favorite, error) {
	return s.repo.AddFavorite(operator, principal, req)
}
func (s *Service) RemoveFavorite(operator string, principal learning.Principal, id string) error {
	return s.repo.RemoveFavorite(operator, principal, id)
}
