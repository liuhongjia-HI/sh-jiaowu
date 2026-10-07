package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

const businessNoticeSettingsKey = "wechat.businessNotice.bindings"

var businessNoticeLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*3600)
	}
	return loc
}()

func businessNoticeHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
func businessTime(t time.Time) string          { return t.In(businessNoticeLocation).Format(time.RFC3339) }
func parseBusinessTime(value string) time.Time { t, _ := time.Parse(time.RFC3339, value); return t }
func businessLessonStart(item learning.ScheduleClass) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04", firstNonEmpty(item.LessonDate, item.StartDate)+" "+item.StartTime, businessNoticeLocation)
	return t
}
func businessLessonVersion(item learning.ScheduleClass) string {
	return businessNoticeHash(item.ID, firstNonEmpty(item.LessonDate, item.StartDate), item.StartTime, item.EndTime)
}
func businessLessonEffective(item learning.ScheduleClass) bool {
	return item.Status == "已确认" && item.AuditStatus == learning.AuditApproved
}

func defaultBusinessNoticeBindings() []learning.BusinessNoticeBinding {
	bindings := []learning.BusinessNoticeBinding{
		{Kind: learning.NoticeScheduleConfirmed, Title: "排课确认", RequiredFields: map[string]string{"thing1": "课程名称", "thing2": "上课学生", "time4": "上课时间"}},
		{Kind: learning.NoticeScheduleChanged, Title: "调课成功", RequiredFields: map[string]string{"thing12": "课程名称", "time2": "调前时间", "time4": "调后时间", "thing7": "学员姓名"}},
		{Kind: learning.NoticeScheduleReminder, Title: "课前提醒", RequiredFields: map[string]string{"thing1": "课程名称", "time2": "上课时间", "thing14": "参与人员"}, AvailableFields: map[string]string{"course_name": "课程名称", "student_name": "学生姓名", "lesson_time": "上课时间"}},
		{Kind: learning.NoticeScheduleCancelled, Title: "课程取消", RequiredFields: map[string]string{}, AvailableFields: map[string]string{"course_name": "原课程名称", "student_name": "学生姓名", "lesson_time": "原上课时间", "cancelled_at": "取消时间", "lesson_count": "取消课次数量"}},
		{Kind: learning.NoticeHomeworkSubmitted, Title: "作业提交成功", RequiredFields: map[string]string{"thing6": "课程名称", "thing2": "作业名称", "time4": "作业提交时间"}},
		{Kind: learning.NoticeReviewException, Title: "作业批改异常", RequiredFields: map[string]string{"thing7": "作业名称", "thing5": "班级", "const2": "异常原因"}},
		{Kind: learning.NoticeHomeworkPublished, Title: "作业发布", RequiredFields: map[string]string{}, AvailableFields: map[string]string{"course_name": "课程名称", "homework_title": "作业名称", "student_name": "学生姓名", "published_at": "发布时间", "deadline_at": "截止时间"}},
		{Kind: learning.NoticeReviewCompleted, Title: "批改完成", RequiredFields: map[string]string{}, AvailableFields: map[string]string{"course_name": "课程名称", "homework_title": "作业名称", "student_name": "学生姓名", "reviewed_at": "完成时间", "score": "最终得分"}},
		{Kind: learning.NoticeMaterialsPublished, Title: "资料批量发布", RequiredFields: map[string]string{}, AvailableFields: materialNoticeFields()},
		{Kind: learning.NoticeTeachingPlansUploaded, Title: "教师教案上传", RequiredFields: map[string]string{}, AvailableFields: map[string]string{"resource_title": "教案名称", "resource_count": "教案数量", "published_at": "上传时间", "teacher_name": "教师姓名", "teaching_scope": "年级学科"}},
	}
	for i := range bindings {
		bindings[i].TriggerReady = true
	}
	return bindings
}

func materialNoticeFields() map[string]string {
	return map[string]string{"course_name": "课程名称", "resource_title": "资料名称", "resource_count": "资料数量", "published_at": "发布时间", "student_name": "学生姓名"}
}

func materialNoticeMappingAllowed(key, value string) bool {
	if strings.HasPrefix(key, "thing") {
		return value == "course_name" || value == "resource_title" || value == "student_name"
	}
	if strings.HasPrefix(key, "time") || strings.HasPrefix(key, "date") {
		return value == "published_at"
	}
	return strings.HasPrefix(key, "number") && value == "resource_count"
}

func resourceNoticeMappingAllowed(kind, key, value string) bool {
	if kind == learning.NoticeScheduleReminder {
		return strings.HasPrefix(key, "thing") && (value == "course_name" || value == "student_name") || (strings.HasPrefix(key, "time") || strings.HasPrefix(key, "date")) && value == "lesson_time"
	}
	if kind == learning.NoticeHomeworkPublished || kind == learning.NoticeReviewCompleted {
		if strings.HasPrefix(key, "thing") {
			return value == "course_name" || value == "homework_title" || value == "student_name"
		}
		if strings.HasPrefix(key, "time") || strings.HasPrefix(key, "date") {
			if kind == learning.NoticeHomeworkPublished {
				return value == "published_at" || value == "deadline_at"
			}
			return value == "reviewed_at"
		}
		return kind == learning.NoticeReviewCompleted && strings.HasPrefix(key, "number") && value == "score"
	}
	if kind == learning.NoticeScheduleCancelled {
		if strings.HasPrefix(key, "thing") {
			return value == "course_name" || value == "student_name"
		}
		if strings.HasPrefix(key, "time") || strings.HasPrefix(key, "date") {
			return value == "lesson_time" || value == "cancelled_at"
		}
		return strings.HasPrefix(key, "number") && value == "lesson_count"
	}
	if kind != learning.NoticeTeachingPlansUploaded {
		return materialNoticeMappingAllowed(key, value)
	}
	if strings.HasPrefix(key, "thing") {
		return value == "resource_title" || value == "teacher_name" || value == "teaching_scope"
	}
	if strings.HasPrefix(key, "time") || strings.HasPrefix(key, "date") {
		return value == "published_at"
	}
	return strings.HasPrefix(key, "number") && value == "resource_count"
}

func validTeacherNoticeOrigin(origin string) bool {
	target, err := url.Parse(origin)
	return err == nil && target.Scheme == "https" && target.Hostname() != "" && target.User == nil && (target.Path == "" || target.Path == "/") && target.RawQuery == "" && !target.ForceQuery && target.Fragment == "" && !strings.ContainsAny(origin, "\r\n#")
}

func deriveMaterialNoticeFields(binding *learning.BusinessNoticeBinding) {
	if binding.Kind == learning.NoticeScheduleReminder && len(binding.FieldMappings) == 0 {
		return
	}
	binding.RequiredFields = map[string]string{}
	for key, value := range binding.FieldMappings {
		binding.RequiredFields[key] = binding.AvailableFields[value]
	}
}

func (s *MemoryStore) businessNoticeBindingsUnlocked() []learning.BusinessNoticeBinding {
	bindings := defaultBusinessNoticeBindings()
	var saved []learning.BusinessNoticeBinding
	_ = json.Unmarshal([]byte(s.settings[businessNoticeSettingsKey]), &saved)
	for i := range bindings {
		for _, item := range saved {
			if item.Kind == bindings[i].Kind {
				bindings[i].TemplateID = item.TemplateID
				bindings[i].Enabled = item.Enabled
				bindings[i].EnabledAt = item.EnabledAt
				bindings[i].ApprovedReasons = item.ApprovedReasons
				bindings[i].StudentIDs = item.StudentIDs
				bindings[i].GuardianIDs = item.GuardianIDs
				bindings[i].TeacherIDs = item.TeacherIDs
				bindings[i].WebOrigin = item.WebOrigin
				if bindings[i].AvailableFields != nil {
					bindings[i].FieldMappings = cloneMap(item.FieldMappings)
					deriveMaterialNoticeFields(&bindings[i])
				}
			}
		}
		bindings[i].Reason = s.validateBusinessNoticeBinding(bindings[i])
		bindings[i].Ready = bindings[i].Reason == ""
	}
	return bindings
}

func (s *MemoryStore) validateBusinessNoticeBinding(binding learning.BusinessNoticeBinding) string {
	if len(binding.RequiredFields) == 0 {
		return "需先准备匹配模板和字段映射"
	}
	var template *learning.OfficialTemplate
	for i := range s.officialTemplates {
		if s.officialTemplates[i].ID == binding.TemplateID && s.officialTemplates[i].Status == "启用" {
			template = &s.officialTemplates[i]
			break
		}
	}
	if template == nil {
		return "请选择已同步的模板"
	}
	if binding.Kind == learning.NoticeScheduleReminder && (!strings.Contains(template.Title, "课") || !strings.Contains(template.Title, "提醒")) {
		return "课前提醒需使用上课或课程提醒模板，不能使用预约结果模板"
	}
	fields := parseOfficialTemplateFields(template.Content)
	if len(fields) == 0 {
		fields = template.Fields
	}
	keys := map[string]bool{}
	for _, field := range fields {
		keys[field.Key] = true
	}
	for key := range binding.RequiredFields {
		if !keys[key] {
			return "模板缺少字段 " + key
		}
	}
	for key := range keys {
		if _, ok := binding.RequiredFields[key]; !ok {
			return "模板包含尚未映射的字段 " + key
		}
	}
	if binding.AvailableFields != nil {
		for key, value := range binding.FieldMappings {
			if !resourceNoticeMappingAllowed(binding.Kind, key, value) {
				return "资料通知字段映射无效：" + key
			}
		}
	}
	if binding.Kind == learning.NoticeTeachingPlansUploaded {
		if !validTeacherNoticeOrigin(binding.WebOrigin) {
			return "请填写教师网页的 HTTPS 域名"
		}
		if !binding.TriggerReady {
			return "教案提醒暂不可开启，可先保存配置"
		}
	}
	if binding.Kind == learning.NoticeReviewException && len(binding.ApprovedReasons) == 0 {
		return "请先配置已通过微信审核的异常原因"
	}
	return ""
}

func (s *MemoryStore) BusinessNoticeBindings() []learning.BusinessNoticeBinding {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.businessNoticeBindingsUnlocked()
}
func (s *MemoryStore) UpdateBusinessNoticeBinding(operator string, req learning.BusinessNoticeBinding) ([]learning.BusinessNoticeBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) ([]learning.BusinessNoticeBinding, error) {
		bindings := work.businessNoticeBindingsUnlocked()
		found := false
		for i := range bindings {
			if bindings[i].Kind != req.Kind {
				continue
			}
			found = true
			binding := bindings[i]
			binding.TemplateID = strings.TrimSpace(req.TemplateID)
			binding.Enabled = req.Enabled
			if binding.AvailableFields != nil {
				binding.FieldMappings = cloneMap(req.FieldMappings)
				if len(binding.FieldMappings) > 20 {
					return nil, errors.New("资料通知模板最多映射 20 个字段")
				}
				for key, value := range binding.FieldMappings {
					if !resourceNoticeMappingAllowed(binding.Kind, key, value) {
						return nil, errors.New("资料通知字段映射无效：" + key)
					}
				}
				deriveMaterialNoticeFields(&binding)
			}
			binding.StudentIDs = compactStrings(req.StudentIDs)
			binding.GuardianIDs = compactStrings(req.GuardianIDs)
			if len(binding.GuardianIDs) > 500 {
				return nil, errors.New("试运行家长最多 500 位")
			}
			for _, id := range binding.GuardianIDs {
				found := false
				for _, guardian := range work.guardians {
					if guardian.ID == id {
						found = true
					}
				}
				if !found {
					return nil, errors.New("试运行家长不存在")
				}
			}
			if binding.Kind == learning.NoticeTeachingPlansUploaded {
				if len(req.StudentIDs) != 0 || len(req.GuardianIDs) != 0 {
					return nil, errors.New("教师教案通知不能选择学生或家长接收范围")
				}
				if len(req.TeacherIDs) > 500 {
					return nil, errors.New("试运行教师最多 500 名")
				}
				binding.TeacherIDs = []string{}
				for _, id := range compactStrings(req.TeacherIDs) {
					binding.TeacherIDs = appendUnique(binding.TeacherIDs, id)
				}
				binding.WebOrigin = strings.TrimSpace(req.WebOrigin)
				if binding.WebOrigin != "" && !validTeacherNoticeOrigin(binding.WebOrigin) {
					return nil, errors.New("教师网页地址必须为不含路径、参数或凭据的 HTTPS 域名")
				}
				for _, id := range binding.TeacherIDs {
					teacher, err := work.principalByUserIDUnlocked(id)
					if err != nil || !hasRole(teacher.Roles, learning.RoleTeacher) {
						return nil, errors.New("试运行教师不存在或已停用")
					}
				}
			}
			binding.ApprovedReasons = compactStrings(req.ApprovedReasons)
			for _, reason := range binding.ApprovedReasons {
				if len([]rune(reason)) > 20 {
					return nil, errors.New("异常原因最多20个字符")
				}
			}
			for _, id := range binding.StudentIDs {
				if _, ok := work.findStudent(id); !ok {
					return nil, errors.New("试运行学生不存在")
				}
			}
			binding.Reason = work.validateBusinessNoticeBinding(binding)
			binding.Ready = binding.Reason == ""
			if binding.Enabled && !binding.Ready {
				return nil, errors.New(binding.Reason)
			}
			if binding.Enabled && (!bindings[i].Enabled || binding.EnabledAt == "") {
				binding.EnabledAt = businessTime(time.Now())
			}
			bindings[i] = binding
		}
		if !found {
			return nil, errors.New("通知业务类型不存在")
		}
		work.settings[businessNoticeSettingsKey] = mustJSON(bindings)
		work.prependLog(operator, "配置自动通知", req.Kind)
		return work.businessNoticeBindingsUnlocked(), nil
	})
}

func (s *MemoryStore) bindingForBusinessKind(kind string) learning.BusinessNoticeBinding {
	for _, binding := range s.businessNoticeBindingsUnlocked() {
		if binding.Kind == kind {
			return binding
		}
	}
	return learning.BusinessNoticeBinding{}
}
func bindingStudentIncluded(binding learning.BusinessNoticeBinding, id string) bool {
	if len(binding.StudentIDs) == 0 {
		return true
	}
	for _, item := range binding.StudentIDs {
		if item == id {
			return true
		}
	}
	return false
}

func (s *MemoryStore) businessRecipients(studentID string) []officialAudienceTarget {
	student, ok := s.findStudent(studentID)
	if !ok || student.AccountStatus != "正常" {
		return nil
	}
	out := []officialAudienceTarget{}
	seen := map[string]bool{}
	for _, relation := range s.guardianStudents {
		if relation.StudentID != studentID || relation.Status != learning.GuardianStudentActive {
			continue
		}
		target, reason := s.reachableOfficialGuardian(relation.GuardianID)
		if reason != "" || seen[target.OpenID] {
			continue
		}
		seen[target.OpenID] = true
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenID < out[j].OpenID })
	return out
}

func (s *MemoryStore) collectScheduleBusinessEvents(before *MemoryStore, now time.Time) {
	if s.businessScheduleSnapshots == nil {
		s.businessScheduleSnapshots = map[string]learning.ScheduleClass{}
	}
	oldClasses := map[string]learning.ScheduleClass{}
	for _, item := range before.scheduleClasses {
		oldClasses[item.ID] = item
	}
	batch := "batch-" + now.Format("20060102150405.000000000")
	groups := map[string]*learning.BusinessNoticeEvent{}
	for _, item := range s.scheduleClasses {
		old, existed := oldClasses[item.ID]
		settled, hasSettled := s.businessScheduleSnapshots[item.ID]
		if !hasSettled && existed && businessLessonEffective(old) {
			settled = cloneScheduleClass(old)
			s.businessScheduleSnapshots[item.ID] = settled
			hasSettled = true
		}
		if existed && reflect.DeepEqual(old, item) {
			continue
		}
		kind := ""
		if item.Status == "已取消" && hasSettled && old.Status != "已取消" {
			kind = learning.NoticeScheduleCancelled
		} else if businessLessonEffective(item) {
			if !hasSettled || old.Status == "已取消" {
				kind = learning.NoticeScheduleConfirmed
			} else if businessLessonVersion(settled) != businessLessonVersion(item) {
				kind = learning.NoticeScheduleChanged
			}
			s.businessScheduleSnapshots[item.ID] = cloneScheduleClass(item)
		}
		if kind == "" {
			continue
		}
		students := item.Students
		if kind == learning.NoticeScheduleCancelled {
			students = settled.Students
		}
		for _, candidate := range students {
			student, ok := s.findStudent(candidate.ID)
			if !ok {
				continue
			}
			groupKey := kind + "|" + firstNonEmpty(item.SeriesID, item.ID) + "|" + candidate.ID
			event := groups[groupKey]
			if event == nil {
				event = &learning.BusinessNoticeEvent{ID: businessNoticeHash(batch, groupKey), BatchID: batch, Kind: kind, StudentID: student.ID, StudentName: student.Name, SeriesID: item.SeriesID, RelatedID: item.ID, CreatedAt: businessTime(now), Lessons: []learning.BusinessLessonChange{}}
				groups[groupKey] = event
			}
			change := learning.BusinessLessonChange{After: cloneScheduleClass(item)}
			if hasSettled {
				previous := cloneScheduleClass(settled)
				change.Before = &previous
			}
			event.Lessons = append(event.Lessons, change)
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		event := groups[key]
		sort.Slice(event.Lessons, func(i, j int) bool {
			return businessLessonStart(event.Lessons[i].After).Before(businessLessonStart(event.Lessons[j].After))
		})
		first := event.Lessons[0].After
		event.Title = s.bindingForBusinessKind(event.Kind).Title
		event.Summary = firstNonEmpty(first.CourseName, first.Name) + " / " + scheduleNoticeSummary(first, event.Title)
		if len(event.Lessons) > 1 {
			event.Summary += fmt.Sprintf(" / 共%d次课", len(event.Lessons))
		}
		expiry := businessLessonStart(first)
		if event.Kind == learning.NoticeScheduleCancelled {
			expiry = now.Add(24 * time.Hour)
		}
		event.ExpiresAt = businessTime(expiry)
		s.addBusinessNoticeEvent(*event, now, now, true)
	}
}

func (s *MemoryStore) addBusinessNoticeEvent(event learning.BusinessNoticeEvent, due, now time.Time, station bool) {
	for _, old := range s.businessNoticeEvents {
		if old.ID == event.ID {
			return
		}
	}
	s.businessNoticeEvents = append(s.businessNoticeEvents, event)
	if station {
		s.notices = append([]learning.Notice{{ID: "business-" + event.ID, Type: businessNoticeCategory(event.Kind), Title: event.Title, Target: event.StudentName, Summary: event.Summary, Channel: "站内通知", Status: "已发送", RelatedType: "business", RelatedID: event.ID, RecipientStudentID: event.StudentID}}, s.notices...)
	}
	binding := s.bindingForBusinessKind(event.Kind)
	if !binding.Enabled || !bindingStudentIncluded(binding, event.StudentID) || !parseBusinessTime(binding.EnabledAt).Before(now.Add(time.Nanosecond)) {
		return
	}
	recipients := s.businessRecipients(event.StudentID)
	if len(recipients) == 0 {
		s.businessNoticeTasks = append(s.businessNoticeTasks, learning.BusinessNoticeTask{ID: businessNoticeHash(event.ID, "unreachable"), EventID: event.ID, Kind: event.Kind, StudentID: event.StudentID, StudentName: event.StudentName, Status: "不可触达", FailureReason: "家长未关注公众号或身份未匹配", TemplateID: binding.TemplateID, DueAt: businessTime(due), ExpiresAt: event.ExpiresAt, CreatedAt: businessTime(now)})
		return
	}
	for _, target := range recipients {
		if !bindingGuardianIncluded(binding, target.GuardianID) {
			continue
		}
		id := businessNoticeHash(event.ID, event.StudentID, target.OpenID)
		task := learning.BusinessNoticeTask{ID: id, EventID: event.ID, Kind: event.Kind, StudentID: event.StudentID, StudentName: event.StudentName, GuardianID: target.GuardianID, GuardianName: target.GuardianName, OpenID: target.OpenID, TemplateID: binding.TemplateID, Status: "待发送", DueAt: businessTime(due), ExpiresAt: event.ExpiresAt, CreatedAt: businessTime(now), ClientMessageID: id, PagePath: "pages/notice-detail/index?id=" + event.ID}
		task.Values = businessNoticeValues(event)
		if event.Kind == learning.NoticeScheduleReminder {
			task.Values = reminderNoticeValues(binding, event)
		}
		if event.Kind == learning.NoticeHomeworkPublished || event.Kind == learning.NoticeReviewCompleted {
			task.Values = s.homeworkNotificationValues(binding, event)
		}
		if event.Kind == learning.NoticeScheduleCancelled {
			task.Values = cancellationNoticeValues(binding, event)
		}
		if !binding.Ready {
			task.Status = "配置错误"
			task.FailureReason = binding.Reason
		}
		s.businessNoticeTasks = append(s.businessNoticeTasks, task)
	}
}

func businessShortName(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 20 {
		return string(runes[:19]) + "…"
	}
	return value
}
func businessLessonTime(item learning.ScheduleClass) string {
	return firstNonEmpty(item.LessonDate, item.StartDate) + " " + item.StartTime + "~" + item.EndTime
}
func businessNoticeValues(event learning.BusinessNoticeEvent) map[string]string {
	if len(event.Lessons) == 0 {
		return cloneMap(event.Values)
	}
	change := event.Lessons[0]
	course := businessShortName(firstNonEmpty(change.After.CourseName, change.After.Name))
	student := businessShortName(event.StudentName)
	switch event.Kind {
	case learning.NoticeScheduleConfirmed:
		return map[string]string{"thing1": course, "thing2": student, "time4": businessLessonTime(change.After)}
	case learning.NoticeScheduleChanged:
		if change.Before == nil {
			return nil
		}
		return map[string]string{"thing12": course, "time2": businessLessonTime(*change.Before), "time4": businessLessonTime(change.After), "thing7": student}
	case learning.NoticeScheduleReminder:
		return map[string]string{"thing1": course, "time2": businessLessonTime(change.After), "thing14": student}
	}
	return nil
}

func businessReminderDue(start time.Time) time.Time {
	due := start.Add(-2 * time.Hour).In(businessNoticeLocation)
	if due.Hour() < 8 || due.Hour() > 21 || (due.Hour() == 21 && due.Minute() > 0) {
		due = time.Date(start.In(businessNoticeLocation).Year(), start.In(businessNoticeLocation).Month(), start.In(businessNoticeLocation).Day(), 20, 0, 0, 0, businessNoticeLocation).AddDate(0, 0, -1)
	}
	return due
}

func (s *MemoryStore) planBusinessReminders(now time.Time) {
	binding := s.bindingForBusinessKind(learning.NoticeScheduleReminder)
	if !binding.Enabled {
		return
	}
	for _, item := range s.scheduleClasses {
		start := businessLessonStart(item)
		if !businessLessonEffective(item) || !start.After(now) {
			continue
		}
		for _, candidate := range item.Students {
			if !bindingStudentIncluded(binding, candidate.ID) {
				continue
			}
			student, ok := s.findStudent(candidate.ID)
			if !ok || student.AccountStatus != "正常" {
				continue
			}
			id := businessNoticeHash(learning.NoticeScheduleReminder, item.ID, businessLessonVersion(item), candidate.ID, binding.EnabledAt)
			due := businessReminderDue(start)
			if due.Before(now) {
				due = businessNextReminderWindow(now)
			}
			event := learning.BusinessNoticeEvent{ID: id, BatchID: id, Kind: learning.NoticeScheduleReminder, StudentID: student.ID, StudentName: student.Name, RelatedID: item.ID, SeriesID: item.SeriesID, Title: "课前提醒", Summary: scheduleNoticeSummary(item, "即将上课"), Lessons: []learning.BusinessLessonChange{{After: cloneScheduleClass(item)}}, CreatedAt: businessTime(now), ExpiresAt: businessTime(start)}
			s.addBusinessNoticeEvent(event, due, now, false)
		}
	}
}

func (s *MemoryStore) BusinessNoticeTasks() []learning.BusinessNoticeTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := cloneBusinessNoticeValue(s.businessNoticeTasks)
	if out == nil {
		out = []learning.BusinessNoticeTask{}
	}
	for i := range out {
		out[i].Retryable = s.businessTaskCanRetry(out[i], time.Now())
		out[i].OpenID = ""
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *MemoryStore) BusinessNoticeDetail(principal learning.Principal, id string) (learning.BusinessNoticeDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, event := range s.businessNoticeEvents {
		if event.ID != id {
			continue
		}
		allowed := false
		if principal.GuardianID != "" {
			guardian, found := s.guardianByID(principal.GuardianID)
			if !found || guardian.AccountStatus != "正常" {
				break
			}
			for _, relation := range s.guardianStudents {
				if relation.GuardianID == principal.GuardianID && relation.StudentID == event.StudentID && relation.Status == learning.GuardianStudentActive {
					allowed = true
				}
			}
		} else {
			allowed = principal.StudentID == event.StudentID
		}
		student, ok := s.findStudent(event.StudentID)
		if !allowed || !ok || student.AccountStatus != "正常" {
			break
		}
		if event.Kind == learning.NoticeReviewException {
			review, valid := s.currentReviewException(event)
			if !valid {
				return learning.BusinessNoticeDetail{}, errors.New("该批改异常已解除或更新，请查看最新作业状态")
			}
			if _, err := s.studentHomeworkUnlocked(learning.Principal{StudentID: event.StudentID}, review.HomeworkID); err != nil {
				return learning.BusinessNoticeDetail{}, errors.New("作业不存在或访问权限已失效")
			}
		}
		if event.Kind == learning.NoticeMaterialsPublished {
			items := s.materialBatchAccessibleResources(event.StudentID, event.ResourceIDs)
			if len(items) == 0 {
				break
			}
			event = cloneBusinessNoticeValue(event)
			event.Values = nil
			event.ResourceIDs = nil
			for _, item := range items {
				event.ResourceIDs = append(event.ResourceIDs, item.ID)
			}
			event.Summary = fmt.Sprintf("%s / 共%d份资料", items[0].Course, len(items))
			return learning.BusinessNoticeDetail{Event: event, CurrentMaterials: items, NoticeID: event.StationNoticeID, CanSwitch: principal.StudentID != event.StudentID}, nil
		}
		detail := learning.BusinessNoticeDetail{Event: cloneBusinessNoticeValue(event), CurrentLessons: []learning.ScheduleClass{}, CanSwitch: principal.StudentID != event.StudentID}
		for _, change := range event.Lessons {
			for _, current := range s.scheduleClasses {
				if current.ID == change.After.ID {
					detail.CurrentLessons = append(detail.CurrentLessons, businessLessonForStudent(current, event.StudentID))
				}
			}
		}
		for i := range detail.Event.Lessons {
			detail.Event.Lessons[i].After = businessLessonForStudent(detail.Event.Lessons[i].After, event.StudentID)
			if detail.Event.Lessons[i].Before != nil {
				filtered := businessLessonForStudent(*detail.Event.Lessons[i].Before, event.StudentID)
				detail.Event.Lessons[i].Before = &filtered
			}
		}
		return detail, nil
	}
	return learning.BusinessNoticeDetail{}, errors.New("通知不存在或无权查看")
}

// Parents see only their child's membership, never other participants or staff notes.
func businessLessonForStudent(item learning.ScheduleClass, studentID string) learning.ScheduleClass {
	item = cloneScheduleClass(item)
	students := []learning.CandidateStudent{}
	for _, student := range item.Students {
		if student.ID == studentID {
			students = append(students, student)
		}
	}
	item.Students = students
	item.ReservationNote = ""
	item.OverrideNote = ""
	item.AuditReason = ""
	item.AuditedBy = ""
	item.CreatedBy = ""
	return item
}

func businessNextReminderWindow(now time.Time) time.Time {
	local := now.In(businessNoticeLocation)
	if local.Hour() < 8 {
		return time.Date(local.Year(), local.Month(), local.Day(), 8, 0, 0, 0, businessNoticeLocation)
	}
	if local.Hour() > 21 || (local.Hour() == 21 && (local.Minute() > 0 || local.Second() > 0)) {
		return time.Date(local.Year(), local.Month(), local.Day(), 8, 0, 0, 0, businessNoticeLocation).AddDate(0, 0, 1)
	}
	return now
}

func businessNoticeCategory(kind string) string {
	if strings.HasPrefix(kind, "review_") {
		return "评"
	}
	if strings.HasPrefix(kind, "homework_") {
		return "练"
	}
	return "课"
}
func (s *MemoryStore) addSubmissionBusinessEvent(submission learning.Submission, homework learning.Homework, now time.Time) {
	student, ok := s.findStudent(submission.StudentID)
	if !ok {
		return
	}
	courseName := homework.Course
	if courseName == "" {
		if course, found := s.findCourse(homework.CourseID); found {
			courseName = course.Name
		}
	}
	submittedAt, err := time.ParseInLocation("2006-01-02 15:04:05", submission.CreatedAt, businessNoticeLocation)
	if err != nil {
		return
	}
	id := businessNoticeHash(learning.NoticeHomeworkSubmitted, submission.ID)
	event := learning.BusinessNoticeEvent{ID: id, BatchID: id, Kind: learning.NoticeHomeworkSubmitted, StudentID: student.ID, StudentName: student.Name, RelatedID: submission.ID, Title: "作业提交成功", Summary: homework.Title + " / 提交成功", CreatedAt: businessTime(submittedAt), ExpiresAt: businessTime(submittedAt.Add(24 * time.Hour)), Values: map[string]string{"thing6": businessShortName(courseName), "thing2": businessShortName(homework.Title), "time4": submittedAt.Format("2006-01-02 15:04:05")}}
	s.addBusinessNoticeEvent(event, now, now, true)
}

// External delivery requires an explicitly configured template and field mapping.
// No event falls back to the legacy generic official template.
func (s *MemoryStore) addHomeworkPublishedBusinessEvents(homework learning.Homework, now time.Time) {
	batch := businessNoticeHash(learning.NoticeHomeworkPublished, homework.ID, now.Format(time.RFC3339Nano))
	expires := now.Add(7 * 24 * time.Hour)
	if deadline, valid := homeworkNotificationDeadline(homework); valid && deadline.Before(expires) {
		expires = deadline
	}
	for _, student := range s.students {
		if student.AccountStatus != "正常" {
			continue
		}
		if _, err := s.studentHomeworkUnlocked(learning.Principal{StudentID: student.ID}, homework.ID); err != nil {
			continue
		}
		event := learning.BusinessNoticeEvent{ID: businessNoticeHash(batch, student.ID), BatchID: batch, Kind: learning.NoticeHomeworkPublished, StudentID: student.ID, StudentName: student.Name, RelatedID: homework.ID, Title: "作业已发布", Summary: homework.Title + " / 请按时完成", CreatedAt: businessTime(now), ExpiresAt: businessTime(expires)}
		s.addBusinessNoticeEvent(event, now, now, true)
	}
}

func (s *MemoryStore) addReviewCompletedBusinessEvent(submission learning.Submission, homework learning.Homework, now time.Time) {
	student, ok := s.findStudent(submission.StudentID)
	if !ok || student.AccountStatus != "正常" || submission.Status != "已批改" {
		return
	}
	if _, err := s.studentHomeworkUnlocked(learning.Principal{StudentID: student.ID}, homework.ID); err != nil {
		return
	}
	id := businessNoticeHash(learning.NoticeReviewCompleted, submission.ID)
	event := learning.BusinessNoticeEvent{ID: id, BatchID: id, Kind: learning.NoticeReviewCompleted, StudentID: student.ID, StudentName: student.Name, RelatedID: submission.ID, Title: "作业批改完成", Summary: homework.Title + " / 查看老师评语", CreatedAt: businessTime(now), ExpiresAt: businessTime(now.Add(7 * 24 * time.Hour))}
	s.addBusinessNoticeEvent(event, now, now, true)
}

func bindingGuardianIncluded(binding learning.BusinessNoticeBinding, id string) bool {
	if len(binding.GuardianIDs) == 0 {
		return true
	}
	for _, selected := range binding.GuardianIDs {
		if selected == id {
			return true
		}
	}
	return false
}
