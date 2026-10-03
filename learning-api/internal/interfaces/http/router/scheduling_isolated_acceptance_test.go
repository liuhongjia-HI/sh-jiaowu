package router_test

import (
	"net/http"
	"net/http/httptest"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

// Owns only temporary memory, identities and storage; external notifications stay disabled.
func TestUnderfilledScheduleTeacherStudentTCPAndCancel(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	server := httptest.NewServer(app.server.transport.handler)
	defer server.Close()
	app.server.URL = server.URL
	http.DefaultClient.Transport = server.Client().Transport
	admin := app.loginAdmin(t, "13800000001")
	teacher := app.loginAdmin(t, "13800000004")
	student := app.loginStudent(t)
	req := learning.ScheduleClassCreateRequest{CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main", ClassType: "1V4", DurationMinutes: 60, StartDate: time.Now().AddDate(0, 0, 45).Format("2006-01-02"), StartTime: "16:00", EndTime: "17:00", StudentIDs: []string{"stu-001"}, IgnoreWarnings: true}
	var lesson learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", admin, req, http.StatusOK, &lesson)
	defer func() {
		app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/cancel", admin, map[string]string{"editScope": "this"}, http.StatusOK, nil)
	}()
	if lesson.Status != "已确认" || lesson.Capacity != 4 || lesson.ExpectedStudentCount != 1 {
		t.Fatalf("underfilled blocked: %#v", lesson)
	}
	assertSeen := func(token, path string, canceled bool) {
		t.Helper()
		var lessons []learning.ScheduleClass
		app.doJSON(t, http.MethodGet, path, token, nil, http.StatusOK, &lessons)
		for _, item := range lessons {
			if item.ID == lesson.ID {
				if (item.Status == "已取消") != canceled {
					t.Fatalf("unexpected status: %#v", item)
				}
				return
			}
		}
		if !canceled {
			t.Fatalf("lesson absent at %s", path)
		}
	}
	assertSeen(teacher, "/api/schedule-classes", false)
	assertSeen(student, "/api/student/schedule", false)
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/cancel", student, map[string]string{"editScope": "this"}, http.StatusForbidden, nil)
	var canceled learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/cancel", admin, map[string]string{"editScope": "this"}, http.StatusOK, &canceled)
	if canceled.Status != "已取消" {
		t.Fatal("test lesson not canceled")
	}
	assertSeen(teacher, "/api/schedule-classes", true)
	assertSeen(student, "/api/student/schedule", true)
	// Verify the scheduling-only completion action with the same isolated roles.
	req.StartDate = time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	app.doJSON(t, http.MethodPost, "/api/schedule-classes", admin, req, http.StatusOK, &lesson)
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/completed", student, map[string]any{}, http.StatusForbidden, nil)
	var completed learning.ScheduleClass
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/completed", teacher, map[string]any{}, http.StatusOK, &completed)
	if completed.Status != "已上课" {
		t.Fatalf("completion failed: %#v", completed)
	}
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/completed", teacher, map[string]any{}, http.StatusOK, &completed)
	assertSeen(teacher, "/api/schedule-classes", false)
	assertSeen(student, "/api/student/schedule", false)
	app.doJSON(t, http.MethodPost, "/api/schedule-classes/"+lesson.ID+"/cancel", admin, map[string]string{"editScope": "this"}, http.StatusOK, &canceled)
	assertSeen(teacher, "/api/schedule-classes", true)
	assertSeen(student, "/api/student/schedule", true)

}
