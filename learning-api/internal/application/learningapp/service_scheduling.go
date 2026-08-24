package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) Availability(p learning.Principal, typ, id string) ([]learning.AvailabilitySlot, error) {
	return s.scheduling.Availability(p, typ, id)
}
func (s *Service) AvailabilityOverview(p learning.Principal) []learning.AvailabilitySlot {
	return s.scheduling.AvailabilityOverview(p)
}
func (s *Service) SaveAvailability(o string, p learning.Principal, r learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error) {
	return s.scheduling.SaveAvailability(o, p, r)
}
func (s *Service) ScheduleCandidates(p learning.Principal, r learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error) {
	return s.scheduling.ScheduleCandidates(p, r)
}
func (s *Service) ScheduleClasses(p learning.Principal) []learning.ScheduleClass {
	return s.scheduling.ScheduleClasses(p)
}
func (s *Service) CreateScheduleClass(o string, p learning.Principal, r learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	return s.scheduling.CreateScheduleClass(o, p, r)
}
func (s *Service) UpdateScheduleClass(o string, p learning.Principal, id string, r learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	return s.scheduling.UpdateScheduleClass(o, p, id, r)
}
func (s *Service) CancelScheduleClass(o string, p learning.Principal, id string) (learning.ScheduleClass, error) {
	return s.scheduling.CancelScheduleClass(o, p, id)
}
func (s *Service) ReviewScheduleClass(o string, p learning.Principal, id string, approve bool, reason string) (learning.ScheduleClass, error) {
	return s.scheduling.ReviewScheduleClass(o, p, id, approve, reason)
}
func (s *Service) PendingScheduleClasses(p learning.Principal) []learning.ScheduleClass {
	return s.scheduling.PendingScheduleClasses(p)
}
func (s *Service) StudentSchedule(p learning.Principal) ([]learning.ScheduleClass, error) {
	return s.scheduling.StudentSchedule(p)
}
