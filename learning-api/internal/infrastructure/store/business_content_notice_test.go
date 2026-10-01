package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
)

func TestBusinessHomeworkPublicationScopeAndTransitions(t *testing.T) {
	s := NewMemoryStore()
	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	s.students = append(s.students, learning.Student{ID: "no-access", Name: "无权限学生", Grade: "ZZZ", AccountStatus: "正常"})
	called := false
	s.officialNoticeSender = func(learning.Notice) error { called = true; return nil }
	req := learning.HomeworkUpdateRequest{Title: "发布流程测试", CourseID: "course-g05-english-s1-q1", LessonID: "course-g05-english-s1-q1-lesson-1", Deadline: "2030-10-30", Status: string(learning.StatusDraft)}
	id := "hw-g05-english-s1-q1"
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != 0 {
		t.Fatal("draft created publication event")
	}
	req.Status = string(learning.StatusEnabled)
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	count := len(s.businessNoticeEvents)
	if count == 0 || called || len(s.businessNoticeTasks) != 0 {
		t.Fatal("missing station publication or used generic template")
	}
	for _, event := range s.businessNoticeEvents {
		if event.Kind != learning.NoticeHomeworkPublished || event.StudentID == "no-access" || event.RelatedID != id {
			t.Fatal("publication reached wrong recipient")
		}
		if _, err := s.studentHomeworkUnlocked(learning.Principal{StudentID: event.StudentID}, id); err != nil {
			t.Fatal("recipient has no actual content access")
		}
	}
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != count {
		t.Fatal("repeated save duplicated publication")
	}
	req.Status = string(learning.StatusDisabled)
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	req.Status = string(learning.StatusEnabled)
	if _, err := s.UpdateHomework("test", teacher, id, req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != count*2 {
		t.Fatal("true republication did not create new batch")
	}
}

func TestBusinessReviewCompletionWaitsForFinalApproval(t *testing.T) {
	s := NewMemoryStore()
	teacher, err := s.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	req := learning.ReviewCompleteRequest{Score: 80, TeacherComment: "需要复核", FinalStatus: "待复核"}
	if _, err := s.CompleteReview("test", teacher, "rev-001", req); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != 0 || len(s.businessNoticeTasks) != 0 {
		t.Fatal("recheck impersonated final approval")
	}
	req.FinalStatus = "已批改"
	sub, err := s.CompleteReview("test", teacher, "rev-001", req)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != 1 || len(s.businessNoticeTasks) != 0 {
		t.Fatal("wrong final completion outbox")
	}
	event := s.businessNoticeEvents[0]
	if event.Kind != learning.NoticeReviewCompleted || event.RelatedID != sub.ID || event.StudentID != sub.StudentID {
		t.Fatal("wrong submission completion link")
	}
	if _, err := s.CompleteReview("test", teacher, "rev-001", req); err == nil || len(s.businessNoticeEvents) != 1 {
		t.Fatal("repeat approval created another event")
	}
}
