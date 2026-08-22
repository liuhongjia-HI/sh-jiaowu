package store

func engagementRows(s *MemoryStore) []persistenceRow {
	rows := make([]persistenceRow, 0, len(s.notices)+len(s.logs)+len(s.settings)+len(s.favorites)+len(s.subscriptionPreferences))
	for _, item := range s.notices {
		rows = append(rows, simpleRow("notices", "external_id", item.ID, `INSERT INTO notices (external_id, notice_type, title, target, content, channel, recipient_open_id, status, failure_reason, related_type, related_id, retry_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE notice_type=VALUES(notice_type), title=VALUES(title), target=VALUES(target), content=VALUES(content), channel=VALUES(channel), recipient_open_id=VALUES(recipient_open_id), status=VALUES(status), failure_reason=VALUES(failure_reason), related_type=VALUES(related_type), related_id=VALUES(related_id), retry_count=VALUES(retry_count)`, item.ID, item.Type, item.Title, item.Target, item.Summary, item.Channel, item.RecipientOpenID, item.Status, item.FailureReason, item.RelatedType, item.RelatedID, item.RetryCount))
	}
	for _, item := range s.logs {
		rows = append(rows, simpleRow("operation_logs", "external_id", item.ID, `INSERT INTO operation_logs (external_id, operator_id, operator_name, action, target, ip, user_agent, detail, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE operator_id=VALUES(operator_id), operator_name=VALUES(operator_name), action=VALUES(action), target=VALUES(target), ip=VALUES(ip), user_agent=VALUES(user_agent), detail=VALUES(detail), created_at=VALUES(created_at)`, item.ID, item.OperatorID, item.Operator, item.Action, item.Target, item.IP, item.UserAgent, item.Detail, nullableDateTime(item.Time)))
	}
	for key, value := range s.settings {
		rows = append(rows, simpleRow("system_settings", "setting_key", key, `INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value)`, key, value))
	}
	for _, item := range s.favorites {
		rows = append(rows, simpleRow("student_favorites", "id", item.ID, `INSERT INTO student_favorites (id, student_id, target_type, target_id, title, course, created_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE student_id=VALUES(student_id), target_type=VALUES(target_type), target_id=VALUES(target_id), title=VALUES(title), course=VALUES(course), created_at=VALUES(created_at)`, item.ID, item.StudentID, item.TargetType, item.TargetID, item.Title, item.Course, nullableDateTime(item.CreatedAt)))
	}
	for _, item := range s.subscriptionPreferences {
		rows = append(rows, simpleRow("student_subscriptions", "student_id", item.StudentID, `INSERT INTO student_subscriptions (student_id, enabled, template_ids_json, updated_at) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE enabled=VALUES(enabled), template_ids_json=VALUES(template_ids_json), updated_at=VALUES(updated_at)`, item.StudentID, item.Enabled, mustJSON(item.TemplateIDs), nullableDateTime(item.UpdatedAt)))
	}
	for _, item := range s.banners {
		rows = append(rows, simpleRow("banners", "id", item.ID, `INSERT INTO banners (id, image_url, title, link_type, link_value, sort_order, starts_at, ends_at, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE image_url=VALUES(image_url), title=VALUES(title), link_type=VALUES(link_type), link_value=VALUES(link_value), sort_order=VALUES(sort_order), starts_at=VALUES(starts_at), ends_at=VALUES(ends_at), enabled=VALUES(enabled), created_at=VALUES(created_at)`, item.ID, item.ImageURL, item.Title, item.LinkType, item.LinkValue, item.SortOrder, item.StartsAt, item.EndsAt, item.Enabled, item.CreatedAt))
	}
	return rows
}
