package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Notices(p learning.Principal) []learning.Notice { return s.notice.Notices(p) }
func (s *Service) CreateNotice(o string, p learning.Principal, r learning.NoticeCreateRequest) (learning.Notice, error) {
	return s.notice.CreateNotice(o, p, r)
}
func (s *Service) RetryNotice(o string, p learning.Principal, id string) (learning.Notice, error) {
	return s.notice.RetryNotice(o, p, id)
}
func (s *Service) Logs() []learning.OperationLog { return s.notice.Logs() }
func (s *Service) StudentPermissions() []learning.StudentPermissionSummary {
	return s.notice.StudentPermissions()
}
func (s *Service) PackagePermissions() []learning.PackagePermissionSummary {
	return s.notice.PackagePermissions()
}
func (s *Service) ContentPermissions() []learning.ContentPermissionSummary {
	return s.notice.ContentPermissions()
}
func (s *Service) Settings() map[string]string { return s.system.Settings() }
func (s *Service) UpdateSetting(o string, r learning.SettingUpdateRequest) (map[string]string, error) {
	return s.system.UpdateSetting(o, r)
}
