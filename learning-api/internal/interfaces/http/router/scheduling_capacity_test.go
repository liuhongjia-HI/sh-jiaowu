package router_test

import (
	"net/http"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestScheduleCapacityHTTPRejectsWithoutWriting(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, err := app.store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 60, StartDate: time.Now().AddDate(0, 0, 30).Format("2006-01-02"),
		StartTime: "16:00", EndTime: "17:00", StudentIDs: []string{"stu-001", "stu-002"}, IgnoreWarnings: true,
	}
	before := len(app.store.ScheduleClasses(p))
	var preview learning.SchedulePreview
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/preview", token, req, http.StatusOK, &preview)
	if preview.CanSave {
		t.Fatal("oversized class passed preview")
	}
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", token, req, http.StatusBadRequest, nil)
	if len(app.store.ScheduleClasses(p)) != before {
		t.Fatal("rejected request wrote class")
	}
	req.ClassType, req.ExpectedStudentCount = "1V2", 3
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", token, req, http.StatusBadRequest, nil)
	if len(app.store.ScheduleClasses(p)) != before {
		t.Fatal("oversized plan wrote class")
	}
	req.ExpectedStudentCount = 0
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/preview", token, req, http.StatusOK, &preview)
	if !preview.CanSave {
		t.Fatalf("valid request blocked: %#v", preview)
	}
	var created learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", token, req, http.StatusOK, &created)
	if created.Capacity != 2 || created.ExpectedStudentCount != 2 || len(created.Students) != 2 {
		t.Fatalf("response: %#v", created)
	}
	var lessons []learning.ScheduleClass
	app.doJSON(t, http.MethodGet, "/api/schedule-classes", token, nil, http.StatusOK, &lessons)
	for _, lesson := range lessons {
		if lesson.ID == created.ID {
			if len(lesson.Students) != 2 || lesson.Students[0].ID != "stu-001" || lesson.Students[1].ID != "stu-002" {
				t.Fatalf("saved students changed: %#v", lesson)
			}
			return
		}
	}
	t.Fatal("created lesson absent from reload")
}
