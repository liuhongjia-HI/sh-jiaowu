package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func homeworkNotificationsFixture(t *testing.T, kind string) (*MemoryStore, learning.Principal) {
	t.Helper()
	s := NewMemoryStore()
	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	s.guardians = []learning.Guardian{{ID: "content-parent", Name: "测试家长", UnionID: "content-union", AccountStatus: "正常"}, {ID: "other-parent", UnionID: "other-union", AccountStatus: "正常"}}
	s.guardianStudents = []learning.GuardianStudent{{GuardianID: "content-parent", StudentID: "stu-001", Status: learning.GuardianStudentActive}, {GuardianID: "other-parent", StudentID: "stu-001", Status: learning.GuardianStudentActive}}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "content-open", UnionID: "content-union", Subscribed: true}, {OpenID: "other-open", UnionID: "other-union", Subscribed: true}}
	source := "published_at"
	content := "课程：{{thing1.DATA}}\n作业：{{thing2.DATA}}\n学生：{{thing3.DATA}}\n时间：{{time4.DATA}}"
	mapping := map[string]string{"thing1": "course_name", "thing2": "homework_title", "thing3": "student_name", "time4": source}
	if kind == learning.NoticeReviewCompleted {
		mapping["time4"] = "reviewed_at"
		mapping["number5"] = "score"
		content += "\n得分：{{number5.DATA}}"
	}
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "content-fixture", Title: "隔离业务模板", Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
	if _, err = s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: kind, TemplateID: "content-fixture", Enabled: true, StudentIDs: []string{"stu-001"}, GuardianIDs: []string{"content-parent"}, FieldMappings: mapping}); err != nil {
		t.Fatal(err)
	}
	return s, teacher
}
func TestBusinessHomeworkExternalPublicationTransitionsAndRevocation(t *testing.T) {
	s, teacher := homeworkNotificationsFixture(t, learning.NoticeHomeworkPublished)
	id := "hw-g05-english-s1-q1"
	req := learning.HomeworkUpdateRequest{Title: "发布通知验证", CourseID: "course-g05-english-s1-q1", LessonID: "course-g05-english-s1-q1-lesson-1", Deadline: "2030-10-30", Status: string(learning.StatusDraft)}
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 0 {
		t.Fatal("draft pushed")
	}
	req.Status = string(learning.StatusEnabled)
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatal("publication target not isolated")
	}
	task := s.businessNoticeTasks[0]
	if task.GuardianID != "content-parent" || task.StudentID != "stu-001" || task.Values["thing2"] != req.Title || task.Values["time4"] == "" {
		t.Fatal("incorrect publication fields")
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "" {
		t.Fatal(reason)
	}
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatal("repeated save duplicated external task")
	}
	req.Status = string(learning.StatusDisabled)
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason == "" {
		t.Fatal("withdrawn homework still deliverable")
	}
	sends := 0
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sends++; return "unexpected", nil }
	if err := s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 0 {
		t.Fatal("revoked publication sent")
	}
}
func TestBusinessReviewExternalFinalCompletionAndInvalidResult(t *testing.T) {
	s, teacher := homeworkNotificationsFixture(t, learning.NoticeReviewCompleted)
	req := learning.ReviewCompleteRequest{Score: 80, TeacherComment: "复核测试", FinalStatus: "待复核"}
	if _, err := s.CompleteReview("test", teacher, "rev-001", req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 0 {
		t.Fatal("pending review pushed final result")
	}
	req.FinalStatus = "已批改"
	sub, err := s.CompleteReview("test", teacher, "rev-001", req)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatal("final completion target not isolated")
	}
	task := s.businessNoticeTasks[0]
	if task.Values["number5"] != "80" || task.Values["time4"] == "" {
		t.Fatal("result fields incorrect")
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "" {
		t.Fatal(reason)
	}
	if _, err = s.CompleteReview("test", teacher, "rev-001", req); err == nil || len(s.businessNoticeTasks) != 1 {
		t.Fatal("duplicate review generated another message")
	}
	sub.Status = "待复核"
	s.submissions[sub.ID] = sub
	if reason := s.businessTaskValidity(task, time.Now()); reason == "" {
		t.Fatal("changed result remained deliverable")
	}
	sub.Status = "已批改"
	sub.StudentID = "stu-002"
	s.submissions[sub.ID] = sub
	if reason := s.businessTaskValidity(task, time.Now()); reason == "" {
		t.Fatal("changed result owner remained deliverable")
	}
}

func TestBusinessHomeworkDeadlinePermissionAndFieldChanges(t *testing.T) {
	s, teacher := homeworkNotificationsFixture(t, learning.NoticeHomeworkPublished)
	id := "hw-g05-english-s1-q1"
	req := learning.HomeworkUpdateRequest{Title: "通知有效性测试", CourseID: "course-g05-english-s1-q1", LessonID: "course-g05-english-s1-q1-lesson-1", Deadline: "2030-10-30", Status: string(learning.StatusDraft)}
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	req.Status = string(learning.StatusEnabled)
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	task := s.businessNoticeTasks[0]
	for i := range s.homework {
		if s.homework[i].ID == id {
			s.homework[i].DeadlineAt = time.Now().Add(-time.Minute).Format(time.RFC3339)
		}
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "作业已超过截止时间" {
		t.Fatal("changed deadline did not block pending task")
	}
	for i := range s.homework {
		if s.homework[i].ID == id {
			s.homework[i].DeadlineAt = "2030-10-30T23:59:59+08:00"
			s.homework[i].Title = "修改后的作业"
		}
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "作业内容或通知字段映射已变更" {
		t.Fatal("stale homework fields retained")
	}
	for i := range s.homework {
		if s.homework[i].ID == id {
			s.homework[i].Title = req.Title
		}
	}
	s.grants = nil
	// Remove the public preview source as well: access intentionally survives a
	// package ending while the first lesson remains publicly previewable.
	s.courses = nil
	if _, err := s.studentHomeworkUnlocked(learning.Principal{StudentID: "stu-001"}, id); err == nil {
		t.Fatal("fixture still grants public preview access")
	}
	if reason := s.businessTaskValidity(task, time.Now()); reason != "作业访问权限已失效" {
		t.Fatal("revoked homework permission retained")
	}
	deadline, ok := homeworkNotificationDeadline(learning.Homework{Deadline: "2030-10-30"})
	if !ok || deadline.In(businessNoticeLocation).Format("2006-01-02 15:04:05") != "2030-10-30 23:59:59" {
		t.Fatal("date-only deadline conversion incorrect")
	}
}

func TestBusinessHomeworkMissingDeadlineMappingIsNotSent(t *testing.T) {
	s, _ := homeworkNotificationsFixture(t, learning.NoticeHomeworkPublished)
	content := "作业：{{thing2.DATA}}\n截止：{{time4.DATA}}"
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "deadline-fixture", Title: "截止时间测试", Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
	b := s.bindingForBusinessKind(learning.NoticeHomeworkPublished)
	b.TemplateID = "deadline-fixture"
	b.FieldMappings = map[string]string{"thing2": "homework_title", "time4": "deadline_at"}
	if _, err := s.UpdateBusinessNoticeBinding("test", b); err != nil {
		t.Fatal(err)
	}
	for i := range s.homework {
		if s.homework[i].ID == "hw-g05-english-s1-q1" {
			s.homework[i].Deadline = ""
			s.homework[i].DeadlineAt = ""
			s.addHomeworkPublishedBusinessEvents(s.homework[i], time.Now())
			break
		}
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatal("missing pending fixture")
	}
	if reason := s.businessTaskValidity(s.businessNoticeTasks[0], time.Now()); reason != "作业通知缺少字段：time4" {
		t.Fatal(reason)
	}
}
