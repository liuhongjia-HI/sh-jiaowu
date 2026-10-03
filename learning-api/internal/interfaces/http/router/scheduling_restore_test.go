package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func TestScheduleRestoreHTTPRechecksSlotAtSave(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, _ := app.store.PrincipalByUserID("user-super")
	date := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	req := learning.ScheduleClassCreateRequest{CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main", ClassType: "1V1", DurationMinutes: 90, StartDate: date, StartTime: "19:00", EndTime: "20:30", StudentIDs: []string{"stu-001"}, IgnoreWarnings: true}
	original, err := app.store.CreateScheduleClass("管理员", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.store.CancelScheduleClassScope("管理员", p, original.ID, "this"); err != nil {
		t.Fatal(err)
	}
	var preview learning.SchedulePreview
	base := "/api/schedule-classes/" + original.ID
	app.doJSON(t, http.MethodPost, base+"/restore-preview", token, map[string]any{}, http.StatusOK, &preview)
	if !preview.CanSave {
		t.Fatalf("unexpected precheck: %#v", preview)
	}
	occupying, err := app.store.CreateScheduleClass("管理员", p, req)
	if err != nil {
		t.Fatal(err)
	}
	app.doJSON(t, http.MethodPost, base+"/restore", token, map[string]any{"ignoreWarnings": true}, http.StatusBadRequest, nil)
	for _, lesson := range app.store.ScheduleClasses(p) {
		if lesson.ID == original.ID && lesson.Status != "已取消" {
			t.Fatal("stale preview allowed occupied restore")
		}
	}
	if _, err := app.store.CancelScheduleClassScope("管理员", p, occupying.ID, "this"); err != nil {
		t.Fatal(err)
	}
	var restored learning.ScheduleClass
	app.doJSON(t, http.MethodPost, base+"/restore", token, map[string]any{"ignoreWarnings": true}, http.StatusOK, &restored)
	if restored.ID != original.ID || restored.Status != original.Status {
		t.Fatalf("restore response changed identity: %#v", restored)
	}
	app.doJSON(t, http.MethodPost, base+"/restore", token, map[string]any{}, http.StatusBadRequest, nil)
	studentToken := app.loginStudent(t)
	app.doJSON(t, http.MethodPost, base+"/restore", studentToken, map[string]any{}, http.StatusForbidden, nil)
}
