package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Dashboard(p learning.Principal) learning.DashboardOverview {
	return s.system.Dashboard(p)
}
func (s *Service) SystemReadiness() learning.SystemReadiness { return s.system.SystemReadiness() }
func (s *Service) Packages() []learning.Package              { return s.grant.Packages() }
func (s *Service) CreatePackage(o string, r learning.PackageUpsertRequest) (learning.Package, error) {
	return s.grant.CreatePackage(o, r)
}
func (s *Service) UpdatePackage(o, id string, r learning.PackageUpsertRequest) (learning.Package, error) {
	return s.grant.UpdatePackage(o, id, r)
}
func (s *Service) DeletePackage(operator, id string) error {
	return s.grant.DeletePackage(operator, id)
}
func (s *Service) LearningSpaces() []learning.LearningSpace { return s.grant.LearningSpaces() }
func (s *Service) GrantPreview(studentID, packageID string) (learning.GrantPreview, error) {
	return s.grant.GrantPreview(studentID, packageID)
}
func (s *Service) DirectGrantPeriodDefault() learning.DirectGrantPeriodDefault {
	return s.grant.DirectGrantPeriodDefault()
}
func (s *Service) CreateGrant(operator string, req learning.GrantCreateRequest) (learning.GrantPreview, error) {
	return s.grant.CreateGrant(operator, req)
}
func (s *Service) RevokePackageGrant(operator, studentID, packageID string) (learning.GrantRevokeResult, error) {
	return s.grant.RevokePackageGrant(operator, studentID, packageID)
}
func (s *Service) CreateDirectGrant(operator string, req learning.DirectGrantCreateRequest) (learning.DirectGrantResult, error) {
	return s.grant.CreateDirectGrant(operator, req)
}
func (s *Service) ReplaceDirectGrant(operator string, req learning.DirectGrantReplaceRequest) (learning.DirectGrantResult, error) {
	return s.grant.ReplaceDirectGrant(operator, req)
}
