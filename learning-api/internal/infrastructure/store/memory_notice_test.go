package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestCreateNoticeAndStudentNoticeFiltering(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "英语阅读挑战已发布",
		Target:  "五年级英语班",
		Summary: "今天完成 S1 Q1 阅读挑战。",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if notice.ID == "" || notice.Status != "已发送" {
		t.Fatalf("unexpected notice: %#v", notice)
	}

	englishStudent, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(englishStudent)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if !noticeListContains(home.Notices, notice.ID) {
		t.Fatalf("expected English student to see notice, got %#v", home.Notices)
	}

	phoneNotice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "课",
		Title:   "一对一提醒",
		Target:  "185****9069",
		Summary: "请确认本周课程安排。",
	})
	if err != nil {
		t.Fatalf("expected phone-targeted notice creation to succeed: %v", err)
	}
	home, err = store.StudentHome(englishStudent)
	if err != nil {
		t.Fatalf("expected student home after phone notice: %v", err)
	}
	if !noticeListContains(home.Notices, phoneNotice.ID) {
		t.Fatalf("expected student to see phone-targeted notice, got %#v", home.Notices)
	}
}

func TestStudentHomeExposesMiniProgramSubscribeTemplates(t *testing.T) {
	store := NewMemoryStore()
	store.UseMiniProgramSubscribeTemplates([]string{"vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0", " ", "tpl-review"})
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if !home.SubscriptionReminder.TemplateConfigured || home.SubscriptionReminder.Enabled {
		t.Fatalf("expected configured subscription reminder, got %#v", home.SubscriptionReminder)
	}
	if len(home.SubscriptionReminder.TemplateIDs) != 2 || home.SubscriptionReminder.TemplateIDs[0] != "vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0" || home.SubscriptionReminder.TemplateIDs[1] != "tpl-review" {
		t.Fatalf("expected compact template IDs, got %#v", home.SubscriptionReminder.TemplateIDs)
	}
	todo, ok := findStoreStudentTodo(home.TodayTodos, "subscribe")
	if !ok || todo.ActionText != "开启提醒" || todo.Status != "建议开启" {
		t.Fatalf("expected subscribe todo to be actionable, got %#v", home.TodayTodos)
	}
	reminder, err := store.ConfirmStudentSubscription("小明", student, learning.StudentSubscriptionRequest{TemplateIDs: []string{"vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0"}})
	if err != nil {
		t.Fatalf("expected subscription confirmation to succeed: %v", err)
	}
	if !reminder.Enabled || reminder.ActionText != "已开启" {
		t.Fatalf("expected enabled reminder after confirmation, got %#v", reminder)
	}
	home, err = store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home after subscription: %v", err)
	}
	if _, ok := findStoreStudentTodo(home.TodayTodos, "subscribe"); ok {
		t.Fatalf("subscribe todo should be removed after confirmation, got %#v", home.TodayTodos)
	}
}

func TestOfficialAccountNoticeRequiresRecipientAndConfiguration(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	withoutOpenID, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "课",
		Title:   "明天上课提醒",
		Target:  "小明",
		Summary: "明天 19:00 上课。",
		Channel: "公众号模板消息",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if withoutOpenID.Status != "待配置" || !strings.Contains(withoutOpenID.FailureReason, "openid") {
		t.Fatalf("expected missing openid to be tracked, got %#v", withoutOpenID)
	}
	withOpenID, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "明天上课提醒",
		Target:          "小明",
		Summary:         "明天 19:00 上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-001",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if withOpenID.Status != "待配置" || !strings.Contains(withOpenID.FailureReason, "WECHAT_OFFICIAL_ACCOUNT") {
		t.Fatalf("expected missing official account config to be tracked, got %#v", withOpenID)
	}

	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if noticeListContains(home.Notices, withOpenID.ID) || noticeListContains(home.Notices, withoutOpenID.ID) {
		t.Fatalf("student should not see raw official account notices, got %#v", home.Notices)
	}
	if !noticeListContains(home.Notices, stationNoticeID(withOpenID.ID)) || !noticeListContains(home.Notices, stationNoticeID(withoutOpenID.ID)) {
		t.Fatalf("student should see station history even when official account notice is retryable, got %#v", home.Notices)
	}

	store.officialNoticeSender = func(notice learning.Notice) error { return nil }
	retried, err := store.RetryNotice("运营教务", ops, withOpenID.ID)
	if err != nil {
		t.Fatalf("expected retry to succeed after configuration: %v", err)
	}
	if retried.Status != "已发送" {
		t.Fatalf("expected retried notice to be sent, got %#v", retried)
	}
	home, err = store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home after retry: %v", err)
	}
	if noticeListContains(home.Notices, withOpenID.ID) || !noticeListContains(home.Notices, stationNoticeID(withOpenID.ID)) {
		t.Fatalf("student should keep seeing station notice after successful retry, got %#v", home.Notices)
	}
}

func TestOfficialAccountNoticeSendAndRetry(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	sent := 0
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent++
		if notice.RecipientOpenID == "" {
			t.Fatal("expected recipient openid")
		}
		return nil
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "明天上课提醒",
		Target:          "小明",
		Summary:         "明天 19:00 上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-001",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if notice.Status != "已发送" || notice.FailureReason != "" || sent != 1 {
		t.Fatalf("expected sent notice, got notice=%#v sent=%d", notice, sent)
	}
	retried, err := store.RetryNotice("运营教务", ops, notice.ID)
	if err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if retried.Status != "已发送" || retried.RetryCount != 1 || sent != 2 {
		t.Fatalf("expected retried sent notice, got notice=%#v sent=%d", retried, sent)
	}
}

func TestNoticeRecordsVisibleToOpsAndRetryScopeForTeachers(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "明晚提醒",
		Target:          "小明",
		Summary:         "明天 19:00 上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-001",
	})
	if err != nil {
		t.Fatalf("expected ops notice creation to succeed: %v", err)
	}
	if !noticeListContains(store.Notices(ops), notice.ID) {
		t.Fatalf("expected ops to see all notice records, got %#v", store.Notices(ops))
	}
	store.officialNoticeSender = func(notice learning.Notice) error { return nil }
	retried, err := store.RetryNotice("运营教务", ops, notice.ID)
	if err != nil {
		t.Fatalf("expected ops retry to succeed: %v", err)
	}
	if retried.RetryCount != 1 || retried.Status != "已发送" {
		t.Fatalf("expected ops retry to update record, got %#v", retried)
	}

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	mathNotice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "数学课提醒",
		Target:          "五年级数学班",
		Summary:         "明天上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-002",
	})
	if err != nil {
		t.Fatalf("expected math notice creation to succeed: %v", err)
	}
	if noticeListContains(store.Notices(teacher), mathNotice.ID) {
		t.Fatalf("English teacher should not see math notice records")
	}
	if _, err := store.RetryNotice("英语老师", teacher, mathNotice.ID); err == nil || !strings.Contains(err.Error(), "不能补发") {
		t.Fatalf("expected teacher retry outside scope to be blocked, got %v", err)
	}
}

func TestOfficialAccountNoticeUsesStudentLinkedOpenID(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-linked-openid"
		}
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	var sentTo string
	store.officialNoticeSender = func(notice learning.Notice) error {
		sentTo = notice.RecipientOpenID
		return nil
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "课",
		Title:   "明天上课提醒",
		Target:  "小明",
		Summary: "明天 19:00 上课。",
		Channel: "公众号模板消息",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if notice.Status != "已发送" || notice.RecipientOpenID != "oa-linked-openid" || sentTo != "oa-linked-openid" {
		t.Fatalf("expected linked official account openid to be used, notice=%#v sentTo=%q", notice, sentTo)
	}
}

func TestCompleteReviewAutoNoticeUsesOfficialAccountDelivery(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-review-openid"
		}
	}
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	var sent learning.Notice
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent = notice
		return nil
	}
	_, err = store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          95,
		TeacherComment: "阅读依据找得很准，继续保持。",
		Reward:         "阅读小星星",
		FinalStatus:    "已批改",
	})
	if err != nil {
		t.Fatalf("expected review completion to succeed: %v", err)
	}
	notice := store.notices[0]
	if notice.Channel != "公众号模板消息" || notice.Status != "已发送" || notice.RecipientOpenID != "oa-review-openid" {
		t.Fatalf("expected auto review notice to use official account delivery, got %#v", notice)
	}
	if notice.RelatedType != "review" || notice.RelatedID != "rev-001" {
		t.Fatalf("expected review notice to keep related object, got %#v", notice)
	}
	if sent.ID != notice.ID || sent.RecipientOpenID != "oa-review-openid" {
		t.Fatalf("expected sender to receive review notice, sent=%#v notice=%#v", sent, notice)
	}
}

func TestCompleteReviewAutoNoticeTracksMissingOfficialAccountConfiguration(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-review-openid"
		}
	}
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	_, err = store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          90,
		TeacherComment: "批改完成，请查看反馈。",
		FinalStatus:    "已批改",
	})
	if err != nil {
		t.Fatalf("expected review completion to succeed: %v", err)
	}
	notice := store.notices[0]
	if notice.Channel != "公众号模板消息" || notice.Status != "待配置" || !strings.Contains(notice.FailureReason, "WECHAT_OFFICIAL_ACCOUNT") {
		t.Fatalf("expected missing official account config to be tracked, got %#v", notice)
	}
}

func TestRemindStudentCreatesRetryableOfficialAccountNotice(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	result, err := store.RemindStudent("运营教务", ops, "stu-001")
	if err != nil {
		t.Fatalf("expected reminder to succeed: %v", err)
	}
	notice := store.notices[0]
	if result.NoticeID != notice.ID {
		t.Fatalf("expected result to return notice id, result=%#v notice=%#v", result, notice)
	}
	if notice.Channel != "公众号模板消息" || notice.Status != "待配置" || notice.RelatedType != "student" || notice.RelatedID != "stu-001" {
		t.Fatalf("expected reminder notice to be pending official account delivery, got %#v", notice)
	}
	if !strings.Contains(notice.FailureReason, "openid") {
		t.Fatalf("expected missing openid reason, got %#v", notice)
	}

	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-remind-openid"
		}
	}
	sent := 0
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent++
		if notice.RecipientOpenID != "oa-remind-openid" {
			t.Fatalf("expected retry to resolve linked openid, got %#v", notice)
		}
		return nil
	}
	retried, err := store.RetryNotice("运营教务", ops, notice.ID)
	if err != nil {
		t.Fatalf("expected reminder retry to succeed: %v", err)
	}
	if retried.Status != "已发送" || retried.RetryCount != 1 || sent != 1 {
		t.Fatalf("expected retried reminder to be sent, notice=%#v sent=%d", retried, sent)
	}
}

func TestCreateHomeworkPublishesOfficialAccountNotices(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" || store.students[index].ID == "stu-002" || store.students[index].ID == "stu-003" {
			store.students[index].OfficialAccountOpenID = "oa-" + store.students[index].ID
		}
	}
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	courses := store.Courses(teacher)
	if len(courses) == 0 {
		t.Fatal("expected teacher courses")
	}
	sent := 0
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent++
		if notice.Channel != "公众号模板消息" || notice.RecipientOpenID == "" {
			t.Fatalf("expected official account notice, got %#v", notice)
		}
		return nil
	}
	homework, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{
		Title:    "英语阅读发布通知测试",
		CourseID: courses[0].ID,
		Deadline: "2026-06-30",
		Status:   string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected homework creation to succeed: %v", err)
	}
	if sent != 3 {
		t.Fatalf("expected notices for three English students, sent=%d notices=%#v", sent, store.notices[:3])
	}
	for _, notice := range store.notices[:3] {
		if notice.RelatedType != "homework" || notice.RelatedID != homework.ID || notice.Status != "已发送" {
			t.Fatalf("expected published homework notice, got %#v", notice)
		}
		if len(notice.ID) > mysqlIndexedExternalIDLength {
			t.Fatalf("official homework notice ID is too long for notices.external_id: length=%d id=%q", len(notice.ID), notice.ID)
		}
		stationID := stationNoticeID(notice.ID)
		if len(stationID) > mysqlIndexedExternalIDLength {
			t.Fatalf("station homework notice ID is too long for notices.external_id: length=%d id=%q", len(stationID), stationID)
		}
	}
}

func TestUpdateSettingValidatesAndLogs(t *testing.T) {
	store := NewMemoryStore()

	settings, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{
		Key:   "downloadPolicy",
		Value: "允许下载已发布学习资料",
	})
	if err != nil {
		t.Fatalf("expected setting update to succeed: %v", err)
	}
	if settings["downloadPolicy"] != "允许下载已发布学习资料" {
		t.Fatalf("expected updated setting, got %#v", settings)
	}
	if store.logs[0].Action != "修改系统设置" || store.logs[0].Target != "下载规则" {
		t.Fatalf("expected setting update log, got %#v", store.logs[0])
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "downloadPolicy"}); err == nil {
		t.Fatal("expected empty setting value to be rejected")
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "unknown", Value: "x"}); err == nil {
		t.Fatal("expected unknown setting key to be rejected")
	}
}

func TestCreateParentNoticeUsesUnifiedOfficialAccountNotice(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-parent-openid"
		}
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	var sent learning.Notice
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent = notice
		return nil
	}
	order, err := store.CreateCommercialOrder("运营教务", ops, learning.CommercialOrderCreateRequest{
		StudentID:   "stu-001",
		PackageID:   enabledPackageIDForGrade(store, "五年级"),
		AmountCent:  128000,
		LessonTotal: 20,
	})
	if err != nil {
		t.Fatalf("expected commercial order to be created: %v", err)
	}
	notice, err := store.CreateParentNotice("运营教务", ops, learning.ParentNoticeCreateRequest{
		OrderID: order.ID,
		Title:   "续费提醒",
		Content: "课包剩余不多，建议提前确认续费安排。",
	})
	if err != nil {
		t.Fatalf("expected parent notice to be created: %v", err)
	}
	if notice.Status != "已发送" || notice.NoticeID == "" || notice.Channel != "公众号模板消息" || notice.FailureReason != "" {
		t.Fatalf("expected parent notice to track official account delivery, got %#v", notice)
	}
	latest := store.notices[0]
	if latest.ID != notice.NoticeID || latest.RelatedType != "commercial_order" || latest.RelatedID != order.ID {
		t.Fatalf("expected unified commercial notice record, parent=%#v latest=%#v", notice, latest)
	}
	if latest.RecipientOpenID != "oa-parent-openid" || sent.ID != latest.ID {
		t.Fatalf("expected official account sender to receive linked openid, sent=%#v latest=%#v", sent, latest)
	}
}

func TestSystemReadinessTracksExternalConfiguration(t *testing.T) {
	store := NewMemoryStore()

	readiness := store.SystemReadiness()
	if readiness.TotalCount == 0 || readiness.ReadyCount != 0 {
		t.Fatalf("expected initial readiness to require configuration, got %#v", readiness)
	}
	assertReadinessStatus(t, readiness, "miniProgramDomainStatus", "missing")
	assertReadinessStatus(t, readiness, "productionApiDomain", "missing")
	assertReadinessStatus(t, readiness, "officialAccountConfig", "missing")
	assertReadinessStatus(t, readiness, "studentOfficialAccountOpenID", "warning")

	for index := range store.students {
		if store.students[index].ID == "stu-001" || store.students[index].ID == "stu-002" || store.students[index].ID == "stu-003" {
			store.students[index].OfficialAccountOpenID = "oa-" + store.students[index].ID
		}
	}
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "miniProgramDomainStatus", Value: "已完成"})
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "officialAccountBindingStatus", Value: "已完成"})
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "templateMessageStatus", Value: "已完成"})
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "productionApiDomain", Value: "https://api.starlineeducation.com.cn"})
	store.UseOfficialAccountAPI("oa-appid", "oa-secret", "tpl-id")

	readiness = store.SystemReadiness()
	if readiness.ReadyCount != readiness.TotalCount {
		t.Fatalf("expected all readiness items to pass, got %#v", readiness)
	}
}

func assertReadinessStatus(t *testing.T, readiness learning.SystemReadiness, key, want string) {
	t.Helper()
	for _, item := range readiness.Items {
		if item.Key == key {
			if item.Status != want {
				t.Fatalf("readiness %s status = %q want %q item=%#v", key, item.Status, want, item)
			}
			return
		}
	}
	t.Fatalf("readiness item %s not found in %#v", key, readiness)
}

func TestTeacherNoticeScopeIsRestricted(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	if _, err := store.CreateNotice("英语老师", teacher, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "数学挑战已发布",
		Target:  "五年级数学班",
		Summary: "今天完成数学图形挑战。",
	}); err == nil {
		t.Fatal("expected English teacher to be blocked from sending math notice")
	}
	if _, err := store.CreateNotice("英语老师", teacher, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "英语阅读挑战已发布",
		Target:  "五年级英语班",
		Summary: "今天完成英语阅读挑战。",
	}); err != nil {
		t.Fatalf("expected English teacher to send English notice: %v", err)
	}
}
