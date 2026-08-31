package store

import (
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) commercialSummaryUnlocked(principal learning.Principal) learning.CommercialSummary {
	orders := s.commercialOrdersUnlocked(principal)
	summary := learning.CommercialSummary{OrderCount: len(orders)}
	for _, order := range orders {
		if order.PaidAmountCent > 0 {
			summary.PaidOrderCount++
		}
		summary.RevenueCent += order.PaidAmountCent
		summary.RefundCent += order.RefundedAmountCent
		summary.LessonRemainCount += maxInt(0, order.LessonTotal-order.LessonConsumed)
	}
	for _, reminder := range s.renewalReminders {
		if reminder.Status == "待跟进" && s.canSeeCommercialStudent(principal, reminder.StudentID) {
			summary.RenewalTodoCount++
		}
	}
	return summary
}

func (s *MemoryStore) commercialOrdersUnlocked(principal learning.Principal) []learning.CommercialOrder {
	out := make([]learning.CommercialOrder, 0)
	for _, order := range s.commercialOrders {
		if s.canSeeCommercialStudent(principal, order.StudentID) {
			out = append(out, order)
		}
	}
	return out
}

func (s *MemoryStore) createCommercialOrderUnlocked(operator string, principal learning.Principal, req learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.CommercialOrder, error) {
			return work.createCommercialOrderUnlocked(operator, principal, req)
		})
	}
	if !canManageCommercial(principal) {
		return learning.CommercialOrder{}, errors.New("没有权限创建订单")
	}
	req.StudentID = strings.TrimSpace(req.StudentID)
	req.PackageID = strings.TrimSpace(req.PackageID)
	if req.AmountCent <= 0 {
		return learning.CommercialOrder{}, errors.New("订单金额必须大于 0")
	}
	if req.LessonTotal <= 0 {
		return learning.CommercialOrder{}, errors.New("请填写课时数")
	}
	student, err := s.visibleStudent(principal, req.StudentID)
	if err != nil {
		return learning.CommercialOrder{}, err
	}
	pkg, ok := s.findPackage(req.PackageID)
	if !ok || pkg.Status != learning.StatusEnabled {
		return learning.CommercialOrder{}, errors.New("请选择可售学习套餐")
	}
	now := time.Now()
	order := learning.CommercialOrder{
		ID:             "order-" + now.Format("20060102150405.000000000"),
		OrderNo:        "SL" + now.Format("20060102150405"),
		StudentID:      student.ID,
		StudentName:    student.Name,
		PackageID:      pkg.ID,
		PackageName:    pkg.Name,
		AmountCent:     req.AmountCent,
		LessonTotal:    req.LessonTotal,
		Status:         "待支付",
		ContractStatus: "待签署",
		InvoiceStatus:  "未开票",
		CreatedAt:      now.Format("2006-01-02 15:04:05"),
	}
	s.commercialOrders = append([]learning.CommercialOrder{order}, s.commercialOrders...)
	s.prependLogDetail(operator, "创建订单", order.StudentName+" / "+order.PackageName, "金额分: "+itoa(order.AmountCent))
	return order, nil
}

func (s *MemoryStore) createPaymentUnlocked(operator string, principal learning.Principal, orderID string, req learning.PaymentCreateRequest) (learning.PaymentRecord, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.PaymentRecord, error) {
			return work.createPaymentUnlocked(operator, principal, orderID, req)
		})
	}
	if !canManageCommercial(principal) {
		return learning.PaymentRecord{}, errors.New("没有权限登记收款")
	}
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.PaymentRecord{}, err
	}
	req.Method = strings.TrimSpace(req.Method)
	req.TransactionNo = strings.TrimSpace(req.TransactionNo)
	if req.AmountCent <= 0 {
		return learning.PaymentRecord{}, errors.New("收款金额必须大于 0")
	}
	if order.PaidAmountCent+req.AmountCent > order.AmountCent {
		return learning.PaymentRecord{}, errors.New("收款金额不能超过订单金额")
	}
	if req.Method == "" {
		req.Method = "线下收款"
	}
	now := time.Now()
	payment := learning.PaymentRecord{ID: "pay-" + now.Format("20060102150405.000000000"), OrderID: order.ID, AmountCent: req.AmountCent, Method: req.Method, TransactionNo: req.TransactionNo, PaidAt: now.Format("2006-01-02 15:04:05"), Status: "已确认"}
	order.PaidAmountCent += req.AmountCent
	if order.PaidAmountCent >= order.AmountCent {
		order.Status = "已支付"
	} else {
		order.Status = "部分支付"
	}
	s.commercialOrders[index] = order
	s.payments = append([]learning.PaymentRecord{payment}, s.payments...)
	s.prependLogDetail(operator, "登记收款", order.StudentName+" / "+order.PackageName, "金额分: "+itoa(payment.AmountCent))
	return payment, nil
}

func (s *MemoryStore) createRefundUnlocked(operator string, principal learning.Principal, orderID string, req learning.RefundCreateRequest) (learning.RefundRecord, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.RefundRecord, error) {
			return work.createRefundUnlocked(operator, principal, orderID, req)
		})
	}
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.RefundRecord{}, err
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.AmountCent <= 0 {
		return learning.RefundRecord{}, errors.New("退款金额必须大于 0")
	}
	if order.RefundedAmountCent+req.AmountCent > order.PaidAmountCent {
		return learning.RefundRecord{}, errors.New("退款金额不能超过实收金额")
	}
	if req.Reason == "" {
		req.Reason = "家长申请退款"
	}
	now := time.Now()
	refund := learning.RefundRecord{ID: "refund-" + now.Format("20060102150405.000000000"), OrderID: order.ID, AmountCent: req.AmountCent, Reason: req.Reason, RefundedAt: now.Format("2006-01-02 15:04:05"), Status: "已退款"}
	order.RefundedAmountCent += req.AmountCent
	if order.RefundedAmountCent >= order.PaidAmountCent {
		order.Status = "已退款"
	} else {
		order.Status = "部分退款"
	}
	s.commercialOrders[index] = order
	s.refunds = append([]learning.RefundRecord{refund}, s.refunds...)
	s.prependLogDetail(operator, "登记退款", order.StudentName+" / "+order.PackageName, req.Reason)
	return refund, nil
}

func (s *MemoryStore) refundAndSuspendStudentUnlocked(operator string, principal learning.Principal, orderID string, req learning.RefundAndSuspendRequest) (learning.RefundSuspensionResult, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.RefundSuspensionResult, error) {
			return work.refundAndSuspendStudentUnlocked(operator, principal, orderID, req)
		})
	}
	if !canManageCommercial(principal) {
		return learning.RefundSuspensionResult{}, errors.New("没有权限办理退款停学")
	}
	_, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.RefundSuspensionResult{}, err
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		return learning.RefundSuspensionResult{}, errors.New("请填写退款停学原因")
	}
	if req.AmountCent <= 0 || order.RefundedAmountCent+req.AmountCent != order.PaidAmountCent {
		return learning.RefundSuspensionResult{}, errors.New("退款停学必须退清该订单的剩余实收金额；部分退款请使用普通退款")
	}
	student, err := s.visibleStudent(principal, order.StudentID)
	if err != nil {
		return learning.RefundSuspensionResult{}, err
	}
	if student.AccountStatus == "停用" {
		return learning.RefundSuspensionResult{}, errors.New("该学生账号已停用")
	}
	if s.studentHasOtherActivePackageGrant(student.ID, order.PackageID) {
		return learning.RefundSuspensionResult{}, errors.New("该学生还有其他有效套餐，不能随单停用账号；请使用普通退款或先确认全部服务已结束")
	}

	refund, err := s.createRefundUnlocked(operator, principal, orderID, learning.RefundCreateRequest{AmountCent: req.AmountCent, Reason: req.Reason})
	if err != nil {
		return learning.RefundSuspensionResult{}, err
	}
	revoked := s.revokePackageGrantsForRefund(student.ID, order.PackageID)
	removedClasses := s.removeStudentFromFutureScheduleClasses(student.ID)
	for index := range s.students {
		if s.students[index].ID == student.ID {
			s.students[index].AccountStatus = "停用"
			s.syncStudentUser(s.students[index])
			student = s.decorateStudent(s.students[index])
			break
		}
	}
	s.prependLogDetail(operator, "退款停学", student.Name+" / "+order.PackageName, "退款单: "+refund.ID+"；原因: "+req.Reason+"；收回套餐权限: "+itoa(revoked)+"；移除后续课次: "+itoa(removedClasses))
	return learning.RefundSuspensionResult{Refund: refund, Student: student, RevokedGrantCount: revoked, RemovedFutureClassCount: removedClasses}, nil
}

func (s *MemoryStore) studentHasOtherActivePackageGrant(studentID, excludedPackageID string) bool {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.PackageID != excludedPackageID && grantActive(grant) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) revokePackageGrantsForRefund(studentID, packageID string) int {
	revokedGrantIDs := make(map[string]bool)
	for index := range s.grants {
		grant := &s.grants[index]
		if grant.StudentID != studentID || grant.PackageID != packageID || grant.Status == "revoked" {
			continue
		}
		grant.Status = "revoked"
		revokedGrantIDs[grant.ID] = true
	}
	if len(revokedGrantIDs) == 0 {
		return 0
	}
	remainingAccess := make([]learningSpaceAccess, 0, len(s.spaceAccess))
	for _, access := range s.spaceAccess {
		if !revokedGrantIDs[access.PackageGrantID] {
			remainingAccess = append(remainingAccess, access)
		}
	}
	s.spaceAccess = remainingAccess
	return len(revokedGrantIDs)
}

func (s *MemoryStore) removeStudentFromFutureScheduleClasses(studentID string) int {
	today := time.Now().Format("2006-01-02")
	removed := 0
	for index := range s.scheduleClasses {
		item := &s.scheduleClasses[index]
		lessonDate := item.LessonDate
		if lessonDate == "" {
			lessonDate = item.StartDate
		}
		if lessonDate <= today || item.Status == "已取消" {
			continue
		}
		students := make([]learning.CandidateStudent, 0, len(item.Students))
		for _, candidate := range item.Students {
			if candidate.ID == studentID {
				removed++
				continue
			}
			students = append(students, candidate)
		}
		item.Students = students
	}
	return removed
}

func (s *MemoryStore) createContractUnlocked(operator string, principal learning.Principal, orderID string, req learning.ContractCreateRequest) (learning.ContractRecord, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ContractRecord, error) {
			return work.createContractUnlocked(operator, principal, orderID, req)
		})
	}
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.ContractRecord{}, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Signer = strings.TrimSpace(req.Signer)
	if req.Title == "" {
		req.Title = order.PackageName + " 学习服务协议"
	}
	if req.Signer == "" {
		req.Signer = order.StudentName + "家长"
	}
	now := time.Now()
	contract := learning.ContractRecord{ID: "contract-" + now.Format("20060102150405.000000000"), OrderID: order.ID, Title: req.Title, Signer: req.Signer, SignedAt: now.Format("2006-01-02 15:04:05"), Status: "已签署"}
	order.ContractStatus = "已签署"
	s.commercialOrders[index] = order
	s.contracts = append([]learning.ContractRecord{contract}, s.contracts...)
	s.prependLog(operator, "签署合同", order.StudentName+" / "+contract.Title)
	return contract, nil
}

func (s *MemoryStore) createInvoiceUnlocked(operator string, principal learning.Principal, orderID string, req learning.InvoiceCreateRequest) (learning.InvoiceRecord, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.InvoiceRecord, error) {
			return work.createInvoiceUnlocked(operator, principal, orderID, req)
		})
	}
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.InvoiceRecord{}, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.InvoiceNo = strings.TrimSpace(req.InvoiceNo)
	if req.Title == "" {
		return learning.InvoiceRecord{}, errors.New("请输入发票抬头")
	}
	available := order.PaidAmountCent - order.RefundedAmountCent
	if req.AmountCent <= 0 {
		req.AmountCent = available
	}
	if req.AmountCent <= 0 || req.AmountCent > available {
		return learning.InvoiceRecord{}, errors.New("开票金额不能超过订单可开票金额")
	}
	now := time.Now()
	invoice := learning.InvoiceRecord{ID: "invoice-" + now.Format("20060102150405.000000000"), OrderID: order.ID, Title: req.Title, TaxNo: strings.TrimSpace(req.TaxNo), AmountCent: req.AmountCent, InvoiceNo: req.InvoiceNo, IssuedAt: now.Format("2006-01-02 15:04:05"), Status: "已开票"}
	order.InvoiceStatus = "已开票"
	s.commercialOrders[index] = order
	s.invoices = append([]learning.InvoiceRecord{invoice}, s.invoices...)
	s.prependLogDetail(operator, "开具发票", order.StudentName+" / "+invoice.Title, "金额分: "+itoa(invoice.AmountCent))
	return invoice, nil
}

func (s *MemoryStore) createLessonConsumptionUnlocked(operator string, principal learning.Principal, req learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.LessonConsumption, error) {
			return work.createLessonConsumptionUnlocked(operator, principal, req)
		})
	}
	index, order, err := s.commercialOrderForWrite(principal, req.OrderID)
	if err != nil {
		return learning.LessonConsumption{}, err
	}
	if req.LessonCount <= 0 {
		return learning.LessonConsumption{}, errors.New("课消数必须大于 0")
	}
	if order.LessonConsumed+req.LessonCount > order.LessonTotal {
		return learning.LessonConsumption{}, errors.New("课消不能超过订单剩余课时")
	}
	now := time.Now()
	item := learning.LessonConsumption{ID: "lesson-" + now.Format("20060102150405.000000000"), OrderID: order.ID, StudentID: order.StudentID, ScheduleClassID: strings.TrimSpace(req.ScheduleClassID), LessonCount: req.LessonCount, ConsumedAt: now.Format("2006-01-02 15:04:05"), Remark: strings.TrimSpace(req.Remark)}
	order.LessonConsumed += req.LessonCount
	if order.LessonTotal-order.LessonConsumed <= 3 && order.Status != "已退款" {
		order.Status = "待续费"
	}
	s.commercialOrders[index] = order
	s.lessonConsumptions = append([]learning.LessonConsumption{item}, s.lessonConsumptions...)
	s.prependLogDetail(operator, "登记课消", order.StudentName+" / "+order.PackageName, "课时: "+itoa(item.LessonCount))
	return item, nil
}

func (s *MemoryStore) createRenewalReminderUnlocked(operator string, principal learning.Principal, req learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.RenewalReminder, error) {
			return work.createRenewalReminderUnlocked(operator, principal, req)
		})
	}
	_, order, err := s.commercialOrderForWrite(principal, req.OrderID)
	if err != nil {
		return learning.RenewalReminder{}, err
	}
	req.Reason = strings.TrimSpace(req.Reason)
	req.DueAt = strings.TrimSpace(req.DueAt)
	if req.Reason == "" {
		req.Reason = "课时即将用完"
	}
	if req.DueAt == "" {
		req.DueAt = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}
	item := learning.RenewalReminder{ID: "renewal-" + time.Now().Format("20060102150405.000000000"), OrderID: order.ID, StudentID: order.StudentID, Reason: req.Reason, DueAt: req.DueAt, Status: "待跟进"}
	s.renewalReminders = append([]learning.RenewalReminder{item}, s.renewalReminders...)
	s.prependLog(operator, "创建续费提醒", order.StudentName+" / "+item.Reason)
	return item, nil
}

func (s *MemoryStore) createParentNoticeUnlocked(operator string, principal learning.Principal, req learning.ParentNoticeCreateRequest) (learning.ParentNotice, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ParentNotice, error) {
			return work.createParentNoticeUnlocked(operator, principal, req)
		})
	}
	_, order, err := s.commercialOrderForWrite(principal, req.OrderID)
	if err != nil {
		return learning.ParentNotice{}, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || req.Content == "" {
		return learning.ParentNotice{}, errors.New("请输入通知标题和内容")
	}
	now := time.Now()
	noticeID := "parent-notice-" + now.Format("20060102150405.000000000")
	notice := learning.Notice{
		ID:              noticeID,
		Type:            "续",
		Title:           req.Title,
		Target:          order.StudentName,
		Summary:         req.Content,
		Channel:         "公众号模板消息",
		RecipientOpenID: s.officialAccountOpenIDForStudent(order.StudentID),
		RelatedType:     "commercial_order",
		RelatedID:       order.ID,
	}
	notice = s.deliverNotice(notice)
	item := learning.ParentNotice{
		ID:            noticeID,
		OrderID:       order.ID,
		StudentID:     order.StudentID,
		Title:         req.Title,
		Content:       req.Content,
		SentAt:        now.Format("2006-01-02 15:04:05"),
		Status:        notice.Status,
		NoticeID:      notice.ID,
		Channel:       notice.Channel,
		FailureReason: notice.FailureReason,
	}
	s.parentNotices = append([]learning.ParentNotice{item}, s.parentNotices...)
	s.prependNoticeRecord(notice)
	s.prependLog(operator, "发送家长通知", order.StudentName+" / "+item.Title)
	return item, nil
}
