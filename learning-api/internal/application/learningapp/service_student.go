package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Students(principal learning.Principal, query learning.StudentQuery) []learning.Student {
	return s.student.Students(principal, query)
}
func (s *Service) StudentDetail(principal learning.Principal, id string) (learning.StudentDetail, error) {
	return s.student.StudentDetail(principal, id)
}
func (s *Service) CreateStudent(operator string, principal learning.Principal, req learning.StudentUpsertRequest) (learning.Student, error) {
	return s.student.CreateStudent(operator, principal, req)
}
func (s *Service) UpdateStudent(operator string, principal learning.Principal, id string, req learning.StudentUpsertRequest) (learning.Student, error) {
	return s.student.UpdateStudent(operator, principal, id, req)
}
func (s *Service) RemindStudent(operator string, principal learning.Principal, id string) (learning.StudentRemindResult, error) {
	return s.student.RemindStudent(operator, principal, id)
}
func (s *Service) CleanupTestStudents(operator string, principal learning.Principal) (learning.StudentCleanupResult, error) {
	return s.student.CleanupTestStudents(operator, principal)
}
func (s *Service) BatchDeleteStudents(operator string, principal learning.Principal, studentIDs []string) (learning.StudentCleanupResult, error) {
	return s.student.BatchDeleteStudents(operator, principal, studentIDs)
}
func (s *Service) ImportStudents(operator string, principal learning.Principal, rows []learning.StudentUpsertRequest) (learning.StudentImportResult, error) {
	return s.student.ImportStudents(operator, principal, rows)
}
func (s *Service) StudentGrants(principal learning.Principal, id string) ([]learning.StudentGrant, error) {
	return s.grant.StudentGrants(principal, id)
}
func (s *Service) StudentLearningRecords(principal learning.Principal, id string) ([]learning.StudentLearningRecord, error) {
	return s.student.StudentLearningRecords(principal, id)
}
func (s *Service) StudentTutoringAssignments(principal learning.Principal, id string) ([]learning.TutoringAssignment, error) {
	return s.student.StudentTutoringAssignments(principal, id)
}
func (s *Service) CreateTutoringAssignment(operator string, principal learning.Principal, studentID string, req learning.TutoringAssignmentCreateRequest) (learning.TutoringAssignment, error) {
	return s.student.CreateTutoringAssignment(operator, principal, studentID, req)
}
func (s *Service) EndTutoringAssignment(operator string, principal learning.Principal, id string, req learning.TutoringAssignmentEndRequest) (learning.TutoringAssignment, error) {
	return s.student.EndTutoringAssignment(operator, principal, id, req)
}
func (s *Service) TransferTutoringAssignment(operator string, principal learning.Principal, id string, req learning.TutoringAssignmentTransferRequest) (learning.TutoringAssignment, error) {
	return s.student.TransferTutoringAssignment(operator, principal, id, req)
}
func (s *Service) StudentScores(principal learning.Principal, id string) ([]learning.StudentScoreSummary, error) {
	return s.student.StudentScores(principal, id)
}
func (s *Service) CreateStudentScore(operator string, principal learning.Principal, studentID string, req learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error) {
	return s.student.CreateStudentScore(operator, principal, studentID, req)
}
func (s *Service) UpdateStudentScore(operator string, principal learning.Principal, studentID string, scoreID string, req learning.StudentScoreUpsertRequest) (learning.StudentScoreRecord, error) {
	return s.student.UpdateStudentScore(operator, principal, studentID, scoreID, req)
}
func (s *Service) StudentHome(principal learning.Principal) (learning.StudentHome, error) {
	return s.student.StudentHome(principal)
}
func (s *Service) StudentRecommendations(principal learning.Principal) ([]learning.StudentPackageRecommendation, error) {
	return s.student.StudentRecommendations(principal)
}
func (s *Service) LaunchCampaign(principal learning.Principal) (*learning.LaunchCampaign, error) {
	return s.student.LaunchCampaign(principal)
}
func (s *Service) CreateClassReservation(operator string, principal learning.Principal, req learning.ClassReservationRequest) (learning.ClassReservationIntent, error) {
	return s.student.CreateClassReservation(operator, principal, req)
}
func (s *Service) ClassReservations(principal learning.Principal) []learning.ClassReservationIntent {
	return s.student.ClassReservations(principal)
}
func (s *Service) UpdateClassReservation(operator string, principal learning.Principal, id string, req learning.ClassReservationUpdateRequest) (learning.ClassReservationIntent, error) {
	return s.student.UpdateClassReservation(operator, principal, id, req)
}
func (s *Service) ConfirmStudentSubscription(operator string, principal learning.Principal, req learning.StudentSubscriptionRequest) (learning.SubscriptionReminder, error) {
	return s.student.ConfirmStudentSubscription(operator, principal, req)
}
func (s *Service) UpdateStudentProfile(operator string, principal learning.Principal, req learning.StudentProfileUpdateRequest) (learning.Student, error) {
	return s.student.UpdateStudentProfile(operator, principal, req)
}

func (s *Service) StudentCourseDetail(principal learning.Principal, courseID string) (learning.StudentCourseDetail, error) {
	return s.student.StudentCourseDetail(principal, courseID)
}
func (s *Service) StudentMaterial(principal learning.Principal, materialID string) (learning.Material, error) {
	return s.student.StudentMaterial(principal, materialID)
}
func (s *Service) StudentMaterialPreviewFile(principal learning.Principal, materialID string) (learning.FileAsset, error) {
	return s.student.StudentMaterialPreviewFile(principal, materialID)
}
func (s *Service) StudentHomework(principal learning.Principal, homeworkID string) (learning.Homework, error) {
	return s.student.StudentHomework(principal, homeworkID)
}
func (s *Service) RecordStudentSecurityEvent(operator string, principal learning.Principal, req learning.SecurityEventRequest) error {
	return s.student.RecordStudentSecurityEvent(operator, principal, req)
}
func (s *Service) CreateSubmission(operator string, principal learning.Principal, req learning.SubmissionRequest) (learning.Submission, error) {
	return s.student.CreateSubmission(operator, principal, req)
}
func (s *Service) StudentSubmission(principal learning.Principal, id string) (learning.Submission, error) {
	return s.student.StudentSubmission(principal, id)
}
func (s *Service) StudentStudy(principal learning.Principal) (learning.StudentStudyBoard, error) {
	return s.student.StudentStudy(principal)
}
func (s *Service) StudentTasks(principal learning.Principal) ([]learning.StudentTask, error) {
	return s.student.StudentTasks(principal)
}
func (s *Service) StudentGrowth(principal learning.Principal) ([]learning.StudentLearningRecord, error) {
	return s.student.StudentGrowth(principal)
}
func (s *Service) StudentOwnScores(principal learning.Principal) ([]learning.StudentScoreSummary, error) {
	return s.student.StudentOwnScores(principal)
}
func (s *Service) StudentBadges(principal learning.Principal) ([]learning.Badge, error) {
	return s.student.StudentBadges(principal)
}
func (s *Service) StudentFavorites(principal learning.Principal) ([]learning.Favorite, error) {
	return s.student.StudentFavorites(principal)
}
func (s *Service) AddFavorite(operator string, principal learning.Principal, req learning.FavoriteRequest) (learning.Favorite, error) {
	return s.student.AddFavorite(operator, principal, req)
}
func (s *Service) RemoveFavorite(operator string, principal learning.Principal, id string) error {
	return s.student.RemoveFavorite(operator, principal, id)
}
