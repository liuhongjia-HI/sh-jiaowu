package store

func commercialRows(s *MemoryStore) []persistenceRow {
	rows := make([]persistenceRow, 0, len(s.commercialOrders)+len(s.payments)+len(s.refunds)+len(s.contracts)+len(s.invoices)+len(s.lessonConsumptions)+len(s.renewalReminders)+len(s.parentNotices))
	for _, item := range s.commercialOrders {
		rows = append(rows, simpleRow("commercial_orders", "id", item.ID, `INSERT INTO commercial_orders (id, order_no, student_id, student_name, package_id, package_name, amount_cent, paid_amount_cent, refunded_amount_cent, lesson_total, lesson_consumed, status, contract_status, invoice_status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_no=VALUES(order_no), student_id=VALUES(student_id), student_name=VALUES(student_name), package_id=VALUES(package_id), package_name=VALUES(package_name), amount_cent=VALUES(amount_cent), paid_amount_cent=VALUES(paid_amount_cent), refunded_amount_cent=VALUES(refunded_amount_cent), lesson_total=VALUES(lesson_total), lesson_consumed=VALUES(lesson_consumed), status=VALUES(status), contract_status=VALUES(contract_status), invoice_status=VALUES(invoice_status)`, item.ID, item.OrderNo, item.StudentID, item.StudentName, item.PackageID, item.PackageName, item.AmountCent, item.PaidAmountCent, item.RefundedAmountCent, item.LessonTotal, item.LessonConsumed, item.Status, item.ContractStatus, item.InvoiceStatus, nullableDateTime(item.CreatedAt)))
	}
	for _, item := range s.payments {
		rows = append(rows, simpleRow("commercial_payments", "id", item.ID, `INSERT INTO commercial_payments (id, order_id, amount_cent, method, transaction_no, paid_at, status) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), amount_cent=VALUES(amount_cent), method=VALUES(method), transaction_no=VALUES(transaction_no), paid_at=VALUES(paid_at), status=VALUES(status)`, item.ID, item.OrderID, item.AmountCent, item.Method, item.TransactionNo, nullableDateTime(item.PaidAt), item.Status))
	}
	for _, item := range s.refunds {
		rows = append(rows, simpleRow("commercial_refunds", "id", item.ID, `INSERT INTO commercial_refunds (id, order_id, amount_cent, reason, refunded_at, status) VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), amount_cent=VALUES(amount_cent), reason=VALUES(reason), refunded_at=VALUES(refunded_at), status=VALUES(status)`, item.ID, item.OrderID, item.AmountCent, item.Reason, nullableDateTime(item.RefundedAt), item.Status))
	}
	for _, item := range s.contracts {
		rows = append(rows, simpleRow("commercial_contracts", "id", item.ID, `INSERT INTO commercial_contracts (id, order_id, title, signer, signed_at, status) VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), title=VALUES(title), signer=VALUES(signer), signed_at=VALUES(signed_at), status=VALUES(status)`, item.ID, item.OrderID, item.Title, item.Signer, nullableDateTime(item.SignedAt), item.Status))
	}
	for _, item := range s.invoices {
		rows = append(rows, simpleRow("commercial_invoices", "id", item.ID, `INSERT INTO commercial_invoices (id, order_id, title, tax_no, amount_cent, invoice_no, issued_at, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), title=VALUES(title), tax_no=VALUES(tax_no), amount_cent=VALUES(amount_cent), invoice_no=VALUES(invoice_no), issued_at=VALUES(issued_at), status=VALUES(status)`, item.ID, item.OrderID, item.Title, item.TaxNo, item.AmountCent, item.InvoiceNo, nullableDateTime(item.IssuedAt), item.Status))
	}
	for _, item := range s.lessonConsumptions {
		rows = append(rows, simpleRow("lesson_consumptions", "id", item.ID, `INSERT INTO lesson_consumptions (id, order_id, student_id, schedule_class_id, lesson_count, consumed_at, remark) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), student_id=VALUES(student_id), schedule_class_id=VALUES(schedule_class_id), lesson_count=VALUES(lesson_count), consumed_at=VALUES(consumed_at), remark=VALUES(remark)`, item.ID, item.OrderID, item.StudentID, item.ScheduleClassID, item.LessonCount, nullableDateTime(item.ConsumedAt), item.Remark))
	}
	for _, item := range s.renewalReminders {
		rows = append(rows, simpleRow("renewal_reminders", "id", item.ID, `INSERT INTO renewal_reminders (id, order_id, student_id, reason, due_at, status) VALUES (?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), student_id=VALUES(student_id), reason=VALUES(reason), due_at=VALUES(due_at), status=VALUES(status)`, item.ID, item.OrderID, item.StudentID, item.Reason, item.DueAt, item.Status))
	}
	for _, item := range s.parentNotices {
		rows = append(rows, simpleRow("parent_notices", "id", item.ID, `INSERT INTO parent_notices (id, order_id, student_id, title, content, sent_at, status, notice_id, channel, failure_reason) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE order_id=VALUES(order_id), student_id=VALUES(student_id), title=VALUES(title), content=VALUES(content), sent_at=VALUES(sent_at), status=VALUES(status), notice_id=VALUES(notice_id), channel=VALUES(channel), failure_reason=VALUES(failure_reason)`, item.ID, item.OrderID, item.StudentID, item.Title, item.Content, nullableDateTime(item.SentAt), item.Status, item.NoticeID, item.Channel, item.FailureReason))
	}
	return rows
}
