package store

import (
	"reflect"
	"testing"
	"time"
)

func TestScheduleCompletedIsSchedulingOnly(t *testing.T) {
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	teacher, _ := s.PrincipalByUserID("user-teacher")
	req := teacherLessonRequest()
	req.StartDate = time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	req.IgnoreWarnings = true
	lesson, err := s.CreateScheduleClass("测试", admin, req)
	if err != nil {
		t.Fatal(err)
	}
	grants := append([]packageGrant(nil), s.grants...)
	beforeFeedback := len(s.lessonFeedbacks)
	got, err := s.MarkScheduleClassCompleted("老师", teacher, lesson.ID)
	if err != nil || got.Status != "已上课" {
		t.Fatalf("mark: %#v %v", got, err)
	}
	if !reflect.DeepEqual(grants, s.grants) || len(s.lessonFeedbacks) != beforeFeedback {
		t.Fatal("completion changed grants or feedback")
	}
	other := teacher
	other.UserID = "not-assigned-teacher"
	if _, err := s.MarkScheduleClassCompleted("其他老师", other, lesson.ID); err == nil {
		t.Fatal("unassigned teacher completed lesson")
	}
	if _, err := s.UpdateScheduleClass("教务", admin, lesson.ID, req); err == nil {
		t.Fatal("completed lesson rescheduled")
	}
	again, err := s.MarkScheduleClassCompleted("老师", teacher, lesson.ID)
	if err != nil || again.ID != lesson.ID {
		t.Fatalf("repeat: %v", err)
	}
	req.StartDate = time.Now().AddDate(0, 0, 40).Format("2006-01-02")
	future, err := s.CreateScheduleClass("测试", admin, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.MarkScheduleClassCompleted("老师", teacher, future.ID); err == nil {
		t.Fatal("future class marked completed")
	}
	if _, err = s.CancelScheduleClassScope("清理", admin, future.ID, "this"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.MarkScheduleClassCompleted("老师", teacher, future.ID); err == nil {
		t.Fatal("canceled class marked completed")
	}
	if _, err = s.CancelScheduleClassScope("清理", admin, lesson.ID, "this"); err != nil {
		t.Fatal(err)
	}
}
