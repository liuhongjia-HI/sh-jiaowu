package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) MaterialDownloadSelection(p learning.Principal, r learning.MaterialDownloadScope) (learning.MaterialDownloadSelection, error) {
	return s.content.MaterialDownloadSelection(p, r)
}
func (s *Service) CreateMaterialDownload(p learning.Principal, r learning.MaterialDownloadScope) (learning.MaterialDownloadJob, error) {
	return s.content.CreateMaterialDownload(p, r)
}
func (s *Service) MaterialDownloads(p learning.Principal) []learning.MaterialDownloadJob {
	return s.content.MaterialDownloads(p)
}
func (s *Service) MaterialDownloadArchive(p learning.Principal, id string) (string, error) {
	return s.content.MaterialDownloadArchive(p, id)
}
func (s *Service) RetryMaterialDownload(p learning.Principal, id string) (learning.MaterialDownloadJob, error) {
	scope, err := s.content.RetryMaterialDownload(p, id)
	if err != nil {
		return learning.MaterialDownloadJob{}, err
	}
	return s.content.CreateMaterialDownload(p, scope)
}
func (s *Service) RecoverMaterialDownloads() error { return s.content.RecoverMaterialDownloads() }
func (s *Service) ClaimMaterialDownload() (learning.MaterialDownloadJob, []learning.MaterialDownloadFile, bool, error) {
	return s.content.ClaimMaterialDownload()
}
func (s *Service) FinishMaterialDownload(id, archivePath, reason string) error {
	return s.content.FinishMaterialDownload(id, archivePath, reason)
}
func (s *Service) ExpiredMaterialArchives() ([]string, error) {
	return s.content.ExpiredMaterialArchives()
}

func (s *Service) InvalidateMaterialDownload(p learning.Principal, id string) error {
	return s.content.InvalidateMaterialDownload(p, id)
}

func (s *Service) AcknowledgeMaterialArchiveRemoval(path string) error {
	return s.content.AcknowledgeMaterialArchiveRemoval(path)
}
