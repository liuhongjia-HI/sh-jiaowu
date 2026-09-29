package router_test

import (
	"net/http"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestCourseFamilyAPICreateEditAndAddDynamicLevel(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	admin := app.loginAdmin(t, "13800000002")
	student := app.loginStudent(t)
	req := learning.CourseFamilyCreateRequest{Name: "五年级英文共享目录 API", LearningSpaceIDs: []string{"space-g05-english-s1-q1-splus", "space-g05-english-s1-q1-h"}, Curriculum: apiTestCurriculum("family-api")}
	app.doJSON(t, http.MethodPost, "/api/course-families", student, req, http.StatusForbidden, nil)
	var family learning.CourseFamily
	app.doJSON(t, http.MethodPost, "/api/course-families", admin, req, http.StatusOK, &family)
	if family.ID == "" || len(family.Courses) != 2 {
		t.Fatalf("created family = %#v", family)
	}
	var listed []learning.CourseFamily
	app.doJSON(t, http.MethodGet, "/api/course-families", admin, nil, http.StatusOK, &listed)
	if len(listed) != 1 || listed[0].ID != family.ID {
		t.Fatalf("listed families = %#v", listed)
	}
	var added learning.Course
	app.doJSON(t, http.MethodPost, "/api/course-families/"+family.ID+"/courses", admin, learning.CourseFamilyAddCourseRequest{LearningSpaceID: "space-g05-english-s1-q1"}, http.StatusOK, &added)
	if added.FamilyID != family.ID || added.Curriculum[2].ID != family.Curriculum[2].ID {
		t.Fatalf("added course = %#v", added)
	}
	curriculum := apiTestCurriculum("family-api")
	curriculum[2].Name = "Renamed Lesson"
	var updated learning.CourseFamily
	app.doJSON(t, http.MethodPut, "/api/course-families/"+family.ID, admin, learning.CourseFamilyUpdateRequest{Name: family.Name, Curriculum: curriculum}, http.StatusOK, &updated)
	if len(updated.Courses) != 3 {
		t.Fatalf("updated family = %#v", updated)
	}
	var courses []learning.Course
	app.doJSON(t, http.MethodGet, "/api/courses", admin, nil, http.StatusOK, &courses)
	count := 0
	for _, course := range courses {
		if course.FamilyID == family.ID {
			count++
			if course.Curriculum[2].Name != "Renamed Lesson" {
				t.Fatalf("stale course curriculum = %#v", course)
			}
		}
	}
	if count != 3 {
		t.Fatalf("got %d family members, want 3", count)
	}
}
