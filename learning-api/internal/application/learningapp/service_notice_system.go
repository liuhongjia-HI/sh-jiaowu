package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Notices(p learning.Principal) []learning.Notice { return s.notice.Notices(p) }
func (s *Service) MarkStudentNoticeRead(p learning.Principal, id string) (learning.Notice, error) {
	return s.notice.MarkStudentNoticeRead(p, id)
}
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
func (s *Service) Subjects() []learning.SubjectMetadata { return s.system.Subjects() }
func (s *Service) UpdateSubjectMetadata(o, id string, r learning.SubjectMetadataUpdateRequest) (learning.SubjectMetadata, error) {
	return s.system.UpdateSubjectMetadata(o, id, r)
}
func (s *Service) DeleteSubjectMetadata(o, id string) error {
	return s.system.DeleteSubjectMetadata(o, id)
}
func (s *Service) GradeSubjects() []learning.GradeSubjectMetadata { return s.system.GradeSubjects() }
func (s *Service) UpdateGradeSubjects(o string, r learning.GradeSubjectCatalogUpdateRequest) ([]learning.GradeSubjectMetadata, error) {
	return s.system.UpdateGradeSubjects(o, r)
}

func (s *Service) MarkAllStudentNoticesRead(p learning.Principal) ([]learning.Notice, error) {
	return s.notice.MarkAllStudentNoticesRead(p)
}

func (s *Service) OfficialTemplates() []learning.OfficialTemplate {
	return s.notice.OfficialTemplates()
}
func (s *Service) SyncOfficialTemplates(o string) ([]learning.OfficialTemplate, error) {
	return s.notice.SyncOfficialTemplates(o)
}
func (s *Service) PreviewOfficialAudience(r learning.OfficialAudiencePreviewRequest) (learning.OfficialAudiencePreview, error) {
	return s.notice.PreviewOfficialAudience(r)
}
func (s *Service) OfficialCampaigns() []learning.OfficialCampaign {
	return s.notice.OfficialCampaigns()
}
func (s *Service) OfficialCampaign(id string) (learning.OfficialCampaignDetail, error) {
	return s.notice.OfficialCampaign(id)
}
func (s *Service) CreateOfficialCampaign(o string, r learning.OfficialCampaignCreateRequest) (learning.OfficialCampaign, error) {
	return s.notice.CreateOfficialCampaign(o, r)
}
func (s *Service) RetryOfficialCampaign(o, id string) (learning.OfficialCampaign, error) {
	return s.notice.RetryOfficialCampaign(o, id)
}
func (s *Service) WechatSettings() learning.WechatSettings { return s.system.WechatSettings() }
func (s *Service) UpdateWechatSettings(o string, r learning.WechatSettingsUpdateRequest) (learning.WechatSettings, error) {
	return s.system.UpdateWechatSettings(o, r)
}
func (s *Service) SyncOfficialFollowers(o string) (learning.OfficialFollowerSyncResult, error) {
	return s.notice.SyncOfficialFollowers(o)
}
func (s *Service) VerifyOfficialCallback(signature, timestamp, nonce string) bool {
	return s.notice.VerifyOfficialCallback(signature, timestamp, nonce)
}
func (s *Service) DecryptOfficialCallback(signature, timestamp, nonce, encrypted string) ([]byte, error) {
	return s.notice.DecryptOfficialCallback(signature, timestamp, nonce, encrypted)
}
func (s *Service) HandleOfficialCallback(openID, event string, createTime int64) error {
	return s.notice.HandleOfficialCallback(openID, event, createTime)
}
