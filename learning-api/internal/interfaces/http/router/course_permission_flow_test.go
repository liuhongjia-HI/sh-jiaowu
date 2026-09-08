package router_test

import (
	"net/http"
	"starline/learning-api/internal/domain/learning"
	"testing"
)

// The real router/auth/service runs against a disposable store, without MySQL writes.
func TestCoursePermissionActivationAndRevocationFlow(t *testing.T) {
	app := newTestApp(t)
	defer app.close()
	admin := app.loginAdmin(t, "13800000002")
	var student learning.Student
	app.doJSON(t, http.MethodPost, "/api/students", admin, learning.StudentUpsertRequest{Name: "权限闭环测试", Phone: "18600009876", Grade: "五年级", SchoolName: "测试学校", AccountStatus: "正常"}, http.StatusOK, &student)
	token := app.login(t, "/api/auth/demo-student-login", map[string]string{"phone": student.Phone, "password": "123456"})
	principal, err := app.store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	courseID := "course-g05-english-s1-q1"
	nodes := apiTestCurriculum(courseID)
	nodes = append(nodes, learning.CurriculumNode{ID: courseID + "-lesson-2", ParentID: courseID + "-chapter-1", Type: learning.CurriculumLesson, Name: "第一章第二节", SortOrder: 2}, learning.CurriculumNode{ID: courseID + "-chapter-2", ParentID: courseID + "-unit-1", Type: learning.CurriculumChapter, Name: "第二章", SortOrder: 2}, learning.CurriculumNode{ID: courseID + "-lesson-3", ParentID: courseID + "-chapter-2", Type: learning.CurriculumLesson, Name: "第二章第一节", SortOrder: 1})
	app.doJSON(t, http.MethodPut, "/api/courses/"+courseID, admin, learning.CourseUpsertRequest{Name: "英文权限验收", LearningSpaceID: "space-g05-english-s1-q1", Curriculum: nodes, Status: learning.StatusEnabled}, http.StatusOK, nil)
	materialIDs, homeworkIDs := []string{}, []string{}
	for _, lesson := range []string{"-lesson-1", "-lesson-2", "-lesson-3"} {
		material, err := app.store.CreateMaterial("测试", principal, learning.MaterialUploadRequest{Title: lesson + "讲义", CourseID: courseID, LessonID: courseID + lesson, TagCode: "HD"})
		if err != nil {
			t.Fatal(err)
		}
		materialIDs = append(materialIDs, material.ID)
		homework, err := app.store.CreateHomework("测试", principal, learning.HomeworkUploadRequest{Title: lesson + "习题", CourseID: courseID, LessonID: courseID + lesson, TagCode: "HW", Status: string(learning.StatusEnabled)})
		if err != nil {
			t.Fatal(err)
		}
		homeworkIDs = append(homeworkIDs, homework.ID)
	}
	check := func(t *testing.T, full bool) {
		t.Helper()
		var study learning.StudentStudyBoard
		app.doJSON(t, http.MethodGet, "/api/student/study", token, nil, http.StatusOK, &study)
		if len(study.Subjects) != 5 {
			t.Fatalf("subject catalog count=%d", len(study.Subjects))
		}
		for _, subject := range study.Subjects {
			if !subject.CanOpen {
				t.Fatalf("subject should be browsable: %#v", subject)
			}
			if subject.Subject != "英文" && subject.AccessState != "preview" {
				t.Fatalf("English grant must not unlock another subject: %#v", subject)
			}
		}
		var detail learning.StudentCourseDetail
		app.doJSON(t, http.MethodGet, "/api/student/study/"+courseID, token, nil, http.StatusOK, &detail)
		locks := 0
		for _, station := range detail.Stations {
			if station.Status == "未开通" {
				locks++
				if station.Icon != "🔒" || station.MaterialID != "" || station.HomeworkID != "" {
					t.Fatalf("unsafe lock: %#v", station)
				}
			}
		}
		if (!full && locks != 2) || (full && locks != 0) {
			t.Fatalf("full=%v locked lessons=%d", full, locks)
		}
		for i := range materialIDs {
			status := http.StatusOK
			if i > 0 && !full {
				status = http.StatusBadRequest
				for _, material := range detail.Materials {
					if material.ID == materialIDs[i] {
						t.Fatal("locked material leaked into readable list")
					}
				}
				for _, homework := range detail.Homework {
					if homework.ID == homeworkIDs[i] {
						t.Fatal("locked homework leaked into readable list")
					}
				}
			}
			app.doJSON(t, http.MethodGet, "/api/student/materials/"+materialIDs[i], token, nil, status, nil)
			app.doJSON(t, http.MethodGet, "/api/student/homework/"+homeworkIDs[i], token, nil, status, nil)
			if i > 0 && !full {
				for _, suffix := range []string{"/preview", "/download"} {
					deniedStatus := http.StatusBadRequest
					if suffix == "/download" {
						deniedStatus = http.StatusForbidden
					}
					app.doJSON(t, http.MethodGet, "/api/student/materials/"+materialIDs[i]+suffix, token, nil, deniedStatus, nil)
				}
			}
		}
	}
	t.Run("unopened", func(t *testing.T) { check(t, false) })
	grant := learning.DirectGrantCreateRequest{StudentID: student.ID, LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, ContentTypeCodes: []string{"course"}}
	app.doJSON(t, http.MethodPost, "/api/grants/direct", token, grant, http.StatusForbidden, nil)
	app.doJSON(t, http.MethodPost, "/api/grants/direct", admin, grant, http.StatusOK, nil)
	t.Run("opened_same_student_token", func(t *testing.T) { check(t, true) })
	app.doJSON(t, http.MethodPut, "/api/grants/direct", admin, learning.DirectGrantReplaceRequest{StudentID: student.ID, Selections: []learning.DirectGrantSelection{}}, http.StatusOK, nil)
	t.Run("revoked_same_student_token", func(t *testing.T) { check(t, false) })
}
