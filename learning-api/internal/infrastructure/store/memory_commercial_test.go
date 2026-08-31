package store

import (
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestRefundAndSuspendRemovesOnlyFutureStudentSchedules(t *testing.T) {
	store := NewMemoryStore()
	operator, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	store.commercialOrders = append(store.commercialOrders, learning.CommercialOrder{
		ID: "order-refund-suspend", StudentID: "stu-001", StudentName: "小明", PackageID: "pkg-g05-english-s1-full", PackageName: "五年级英语套餐",
		AmountCent: 128000, PaidAmountCent: 128000, LessonTotal: 10, Status: "已支付",
	})
	futureDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	pastDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	store.scheduleClasses = []learning.ScheduleClass{
		{ID: "class-future", LessonDate: futureDate, StartDate: futureDate, Status: "已确认", Students: []learning.CandidateStudent{{ID: "stu-001", Name: "小明"}, {ID: "stu-003", Name: "小航"}}},
		{ID: "class-history", LessonDate: pastDate, StartDate: pastDate, Status: "已确认", Students: []learning.CandidateStudent{{ID: "stu-001", Name: "小明"}}},
	}

	result, err := store.RefundAndSuspendStudent("运营教务", operator, "order-refund-suspend", learning.RefundAndSuspendRequest{
		AmountCent: 128000, Reason: "家长线下退款",
	})
	if err != nil {
		t.Fatalf("refund and suspend: %v", err)
	}
	if result.RemovedFutureClassCount != 1 || len(store.scheduleClasses[0].Students) != 1 || store.scheduleClasses[0].Students[0].ID != "stu-003" {
		t.Fatalf("expected only the future schedule membership to be removed: %#v", store.scheduleClasses)
	}
	if len(store.scheduleClasses[1].Students) != 1 || store.scheduleClasses[1].Students[0].ID != "stu-001" {
		t.Fatalf("historical schedule must be preserved: %#v", store.scheduleClasses[1])
	}
}
