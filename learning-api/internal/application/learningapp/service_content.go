package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Courses(p learning.Principal) []learning.Course { return s.content.Courses(p) }
func (s *Service) CreateCourse(o string, p learning.Principal, r learning.CourseUpsertRequest) (learning.Course, error) {
	return s.content.CreateCourse(o, p, r)
}
func (s *Service) UpdateCourse(o string, p learning.Principal, id string, r learning.CourseUpsertRequest) (learning.Course, error) {
	return s.content.UpdateCourse(o, p, id, r)
}
func (s *Service) Questions(p learning.Principal, q learning.QuestionBankQuery) []learning.QuestionBankItem {
	return s.content.Questions(p, q)
}
func (s *Service) CreateQuestion(o string, p learning.Principal, r learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	return s.content.CreateQuestion(o, p, r)
}
func (s *Service) UpdateQuestion(o string, p learning.Principal, id string, r learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	return s.content.UpdateQuestion(o, p, id, r)
}
func (s *Service) Materials(p learning.Principal, q learning.MaterialQuery) []learning.Material {
	return s.content.Materials(p, q)
}
func (s *Service) CreateMaterial(o string, p learning.Principal, r learning.MaterialUploadRequest) (learning.Material, error) {
	return s.content.CreateMaterial(o, p, r)
}
func (s *Service) UpdateMaterial(o string, p learning.Principal, id string, r learning.MaterialUpdateRequest) (learning.Material, error) {
	return s.content.UpdateMaterial(o, p, id, r)
}
func (s *Service) ReorderMaterials(o string, p learning.Principal, r learning.MaterialReorderRequest) error {
	return s.content.ReorderMaterials(o, p, r)
}
func (s *Service) DeleteMaterial(o string, p learning.Principal, id string) error {
	return s.content.DeleteMaterial(o, p, id)
}
func (s *Service) Homework(p learning.Principal) []learning.Homework { return s.content.Homework(p) }
func (s *Service) HomeworkSubmissions(p learning.Principal, id string) (learning.HomeworkSubmissionSummary, error) {
	return s.content.HomeworkSubmissions(p, id)
}
func (s *Service) CreateHomework(o string, p learning.Principal, r learning.HomeworkUploadRequest) (learning.Homework, error) {
	return s.content.CreateHomework(o, p, r)
}
func (s *Service) UpdateHomework(o string, p learning.Principal, id string, r learning.HomeworkUpdateRequest) (learning.Homework, error) {
	return s.content.UpdateHomework(o, p, id, r)
}
func (s *Service) DeleteHomework(o string, p learning.Principal, id string) error {
	return s.content.DeleteHomework(o, p, id)
}
func (s *Service) ContentFile(p learning.Principal, id string) (learning.FileAsset, error) {
	return s.content.ContentFile(p, id)
}
func (s *Service) RecoverPreviewJobs() error { return s.content.RecoverPreviewJobs() }
func (s *Service) ClaimPreviewJob() (learning.PreviewJob, bool, error) {
	return s.content.ClaimPreviewJob()
}
func (s *Service) PreviewJobFile(id string) (learning.FileAsset, error) {
	return s.content.PreviewJobFile(id)
}
func (s *Service) CompletePreviewJob(id string, result learning.PreviewResult) error {
	return s.content.CompletePreviewJob(id, result)
}
func (s *Service) FailPreviewJob(id, message string) error {
	return s.content.FailPreviewJob(id, message)
}
func (s *Service) MarkPreviewFileMissing(fileID, message string) error {
	return s.content.MarkPreviewFileMissing(fileID, message)
}
func (s *Service) RetryPreviewJob(operator string, principal learning.Principal, fileID string) error {
	return s.content.RetryPreviewJob(operator, principal, fileID)
}
func (s *Service) Reviews(p learning.Principal) []learning.Review { return s.content.Reviews(p) }
func (s *Service) AssignReview(o string, p learning.Principal, id string, r learning.ReviewAssignRequest) (learning.Review, error) {
	return s.content.AssignReview(o, p, id, r)
}
func (s *Service) CompleteReview(o string, p learning.Principal, id string, r learning.ReviewCompleteRequest) (learning.Submission, error) {
	return s.content.CompleteReview(o, p, id, r)
}
