package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Banners() []learning.Banner              { return s.banner.Banners() }
func (s *Service) ActiveStudentBanners() []learning.Banner { return s.banner.ActiveStudentBanners() }
func (s *Service) CreateBanner(operator string, req learning.BannerUpsertRequest) (learning.Banner, error) {
	return s.banner.CreateBanner(operator, req)
}
func (s *Service) UpdateBanner(operator string, id string, req learning.BannerUpsertRequest) (learning.Banner, error) {
	return s.banner.UpdateBanner(operator, id, req)
}
func (s *Service) DeleteBanner(operator string, id string) error {
	return s.banner.DeleteBanner(operator, id)
}
