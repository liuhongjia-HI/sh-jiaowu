package learning

type CommercialOrder struct {
	ID                 string `json:"id"`
	OrderNo            string `json:"orderNo"`
	StudentID          string `json:"studentId"`
	StudentName        string `json:"studentName"`
	PackageID          string `json:"packageId"`
	PackageName        string `json:"packageName"`
	AmountCent         int    `json:"amountCent"`
	PaidAmountCent     int    `json:"paidAmountCent"`
	RefundedAmountCent int    `json:"refundedAmountCent"`
	LessonTotal        int    `json:"lessonTotal"`
	LessonConsumed     int    `json:"lessonConsumed"`
	Status             string `json:"status"`
	ContractStatus     string `json:"contractStatus"`
	InvoiceStatus      string `json:"invoiceStatus"`
	CreatedAt          string `json:"createdAt"`
}

type CommercialOrderCreateRequest struct {
	StudentID   string `json:"studentId"`
	PackageID   string `json:"packageId"`
	AmountCent  int    `json:"amountCent"`
	LessonTotal int    `json:"lessonTotal"`
	Remark      string `json:"remark"`
}

type PaymentRecord struct {
	ID            string `json:"id"`
	OrderID       string `json:"orderId"`
	AmountCent    int    `json:"amountCent"`
	Method        string `json:"method"`
	TransactionNo string `json:"transactionNo"`
	PaidAt        string `json:"paidAt"`
	Status        string `json:"status"`
}

type PaymentCreateRequest struct {
	AmountCent    int    `json:"amountCent"`
	Method        string `json:"method"`
	TransactionNo string `json:"transactionNo"`
}

type RefundRecord struct {
	ID         string `json:"id"`
	OrderID    string `json:"orderId"`
	AmountCent int    `json:"amountCent"`
	Reason     string `json:"reason"`
	RefundedAt string `json:"refundedAt"`
	Status     string `json:"status"`
}

type RefundCreateRequest struct {
	AmountCent int    `json:"amountCent"`
	Reason     string `json:"reason"`
}

// RefundAndSuspendRequest 只用于全额退费并停止该学生后续服务。
// 部分退款仍使用 RefundCreateRequest，避免误停仍有其他服务的学生。
type RefundAndSuspendRequest struct {
	AmountCent int    `json:"amountCent"`
	Reason     string `json:"reason"`
}

type RefundSuspensionResult struct {
	Refund                  RefundRecord `json:"refund"`
	Student                 Student      `json:"student"`
	RevokedGrantCount       int          `json:"revokedGrantCount"`
	RemovedFutureClassCount int          `json:"removedFutureClassCount"`
}

type ContractRecord struct {
	ID       string `json:"id"`
	OrderID  string `json:"orderId"`
	Title    string `json:"title"`
	Signer   string `json:"signer"`
	SignedAt string `json:"signedAt"`
	Status   string `json:"status"`
}

type ContractCreateRequest struct {
	Title  string `json:"title"`
	Signer string `json:"signer"`
}

type InvoiceRecord struct {
	ID         string `json:"id"`
	OrderID    string `json:"orderId"`
	Title      string `json:"title"`
	TaxNo      string `json:"taxNo"`
	AmountCent int    `json:"amountCent"`
	InvoiceNo  string `json:"invoiceNo"`
	IssuedAt   string `json:"issuedAt"`
	Status     string `json:"status"`
}

type InvoiceCreateRequest struct {
	Title      string `json:"title"`
	TaxNo      string `json:"taxNo"`
	AmountCent int    `json:"amountCent"`
	InvoiceNo  string `json:"invoiceNo"`
}

type LessonConsumption struct {
	ID              string `json:"id"`
	OrderID         string `json:"orderId"`
	StudentID       string `json:"studentId"`
	ScheduleClassID string `json:"scheduleClassId"`
	LessonCount     int    `json:"lessonCount"`
	ConsumedAt      string `json:"consumedAt"`
	Remark          string `json:"remark"`
}

type LessonConsumptionCreateRequest struct {
	OrderID         string `json:"orderId"`
	ScheduleClassID string `json:"scheduleClassId"`
	LessonCount     int    `json:"lessonCount"`
	Remark          string `json:"remark"`
}

type RenewalReminder struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	StudentID string `json:"studentId"`
	Reason    string `json:"reason"`
	DueAt     string `json:"dueAt"`
	Status    string `json:"status"`
}

type RenewalReminderCreateRequest struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
	DueAt   string `json:"dueAt"`
}

type ParentNotice struct {
	ID            string `json:"id"`
	OrderID       string `json:"orderId"`
	StudentID     string `json:"studentId"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	SentAt        string `json:"sentAt"`
	Status        string `json:"status"`
	NoticeID      string `json:"noticeId,omitempty"`
	Channel       string `json:"channel,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
}

type ParentNoticeCreateRequest struct {
	OrderID string `json:"orderId"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type CommercialSummary struct {
	OrderCount        int `json:"orderCount"`
	PaidOrderCount    int `json:"paidOrderCount"`
	RevenueCent       int `json:"revenueCent"`
	RefundCent        int `json:"refundCent"`
	LessonRemainCount int `json:"lessonRemainCount"`
	RenewalTodoCount  int `json:"renewalTodoCount"`
}
