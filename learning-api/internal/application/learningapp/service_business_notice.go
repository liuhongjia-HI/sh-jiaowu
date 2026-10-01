package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) BusinessNoticeBindings() []learning.BusinessNoticeBinding {
	return s.notice.BusinessNoticeBindings()
}
func (s *Service) UpdateBusinessNoticeBinding(operator string, req learning.BusinessNoticeBinding) ([]learning.BusinessNoticeBinding, error) {
	return s.notice.UpdateBusinessNoticeBinding(operator, req)
}
func (s *Service) BusinessNoticeTasks() []learning.BusinessNoticeTask {
	return s.notice.BusinessNoticeTasks()
}
func (s *Service) RetryBusinessNotice(operator, id string) (learning.BusinessNoticeTask, error) {
	return s.notice.RetryBusinessNotice(operator, id)
}
func (s *Service) BusinessNoticeDetail(principal learning.Principal, id string) (learning.BusinessNoticeDetail, error) {
	return s.notice.BusinessNoticeDetail(principal, id)
}
func (s *Service) HandleOfficialDeliveryReceipt(openID, messageID, status string) error {
	return s.notice.HandleOfficialDeliveryReceipt(openID, messageID, status)
}
func (s *Service) RefreshOfficialFollower(openID string) error {
	return s.notice.RefreshOfficialFollower(openID)
}
