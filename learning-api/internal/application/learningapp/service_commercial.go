package learningapp

import "starline/learning-api/internal/domain/learning"

func (s *Service) CommercialSummary(p learning.Principal) learning.CommercialSummary {
	return s.commercial.CommercialSummary(p)
}
func (s *Service) CommercialOrders(p learning.Principal) []learning.CommercialOrder {
	return s.commercial.CommercialOrders(p)
}
func (s *Service) CreateCommercialOrder(o string, p learning.Principal, r learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error) {
	return s.commercial.CreateCommercialOrder(o, p, r)
}
func (s *Service) CreatePayment(o string, p learning.Principal, id string, r learning.PaymentCreateRequest) (learning.PaymentRecord, error) {
	return s.commercial.CreatePayment(o, p, id, r)
}
func (s *Service) CreateRefund(o string, p learning.Principal, id string, r learning.RefundCreateRequest) (learning.RefundRecord, error) {
	return s.commercial.CreateRefund(o, p, id, r)
}
func (s *Service) CreateContract(o string, p learning.Principal, id string, r learning.ContractCreateRequest) (learning.ContractRecord, error) {
	return s.commercial.CreateContract(o, p, id, r)
}
func (s *Service) CreateInvoice(o string, p learning.Principal, id string, r learning.InvoiceCreateRequest) (learning.InvoiceRecord, error) {
	return s.commercial.CreateInvoice(o, p, id, r)
}
func (s *Service) CreateLessonConsumption(o string, p learning.Principal, r learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error) {
	return s.commercial.CreateLessonConsumption(o, p, r)
}
func (s *Service) CreateRenewalReminder(o string, p learning.Principal, r learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error) {
	return s.commercial.CreateRenewalReminder(o, p, r)
}
func (s *Service) CreateParentNotice(o string, p learning.Principal, r learning.ParentNoticeCreateRequest) (learning.ParentNotice, error) {
	return s.commercial.CreateParentNotice(o, p, r)
}
