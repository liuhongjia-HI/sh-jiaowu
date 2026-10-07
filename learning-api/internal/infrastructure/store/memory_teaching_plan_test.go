package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestTeachingPlanScopeAndIsolation(t *testing.T) {
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	teacher, _ := s.PrincipalByUserID("user-teacher")
	asset := func(id string) learning.FileAsset {
		return learning.FileAsset{ID: id, FileName: id + ".pdf", OriginalPath: "/tmp/" + id + ".pdf", PreviewPath: "/tmp/" + id + ".pdf", PreviewStatus: "可预览"}
	}
	adminPlan, err := s.CreateTeachingPlan("admin", admin, learning.TeachingPlanUploadRequest{Title: "五年级英语教案", Grade: "五年级", Subject: "english", File: asset("admin-plan-file")})
	if err != nil {
		t.Fatal(err)
	}
	teacherPlan, err := s.CreateTeachingPlan("teacher", teacher, learning.TeachingPlanUploadRequest{Title: "教师教案", Grade: "五年级", Subject: "english", File: asset("teacher-plan-file")})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []learning.Principal{admin, teacher} {
		if got := s.TeachingPlans(p); len(got.Plans) != 2 {
			t.Fatalf("%s visible plans = %d", p.UserID, len(got.Plans))
		}
		for _, id := range []string{adminPlan.ID, teacherPlan.ID} {
			if _, err := s.TeachingPlanFile(p, id); err != nil {
				t.Fatalf("%s cannot open %s: %v", p.UserID, id, err)
			}
		}
	}
	other := teacher
	other.LearningSpaceIDs = nil
	other.TeacherLibrary = &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "math", Grade: "五年级"}}}
	if got := s.TeachingPlans(other); len(got.Plans) != 0 {
		t.Fatal("other subject teacher saw plans")
	}
	if _, err := s.TeachingPlanFile(other, adminPlan.ID); err == nil {
		t.Fatal("other subject teacher opened file")
	}
	student := learning.Principal{UserID: "student", Roles: []learning.Role{learning.RoleStudent}}
	if got := s.TeachingPlans(student); len(got.Plans) != 0 || got.CanUpload {
		t.Fatal("student saw plans")
	}
	if _, err := s.TeachingPlanFile(student, adminPlan.ID); err == nil {
		t.Fatal("student opened file")
	}
	if _, err := s.ContentFile(student, "admin-plan-file"); err == nil {
		t.Fatal("generic file route exposed plan")
	}
	if _, err := s.CreateTeachingPlan("teacher", other, learning.TeachingPlanUploadRequest{Grade: "五年级", Subject: "english", File: asset("wrong-scope")}); err == nil {
		t.Fatal("other subject teacher uploaded plan")
	}
	if _, err := s.CreateTeachingPlan("student", student, learning.TeachingPlanUploadRequest{Grade: "五年级", Subject: "english", File: asset("student-file")}); err == nil {
		t.Fatal("student uploaded plan")
	}
	// Read-only matching teachers can see both uploaders' plans but cannot upload.
	readOnly := teacher
	readOnly.CanUploadHandout = false
	readOnly.TeacherLibrary = &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Subject: "english", Grade: "五年级"}}}
	if got := s.TeachingPlans(readOnly); len(got.Plans) != 2 || got.CanUpload {
		t.Fatal("read-only teacher access is wrong")
	}
	if _, err := s.CreateTeachingPlan("teacher", readOnly, learning.TeachingPlanUploadRequest{Grade: "五年级", Subject: "english", File: asset("readonly-file")}); err == nil {
		t.Fatal("read-only teacher uploaded plan")
	}
	for index := range s.previewJobs {
		if s.previewJobs[index].FileID == "admin-plan-file" {
			s.previewJobs[index].Status = "转换失败"
			s.previewJobs[index].AttemptCount = 3
			stored := s.fileAssets["admin-plan-file"]
			stored.PreviewStatus = "转换失败"
			s.fileAssets["admin-plan-file"] = stored
		}
	}
	if err := s.RetryTeachingPlanPreview("reader", readOnly, adminPlan.ID); err == nil {
		t.Fatal("read-only teacher retried preview")
	}
	if err := s.RetryTeachingPlanPreview("teacher", teacher, adminPlan.ID); err != nil {
		t.Fatal(err)
	}
	if s.fileAssets["admin-plan-file"].PreviewStatus != "待转换" {
		t.Fatal("retry did not reset preview state")
	}
}

func TestTeachingPlanCourseWithoutChapterKeepsScopeAndReferences(t *testing.T) {
	s, p, course, target := directorySyncFixture(t)
	plan, err := s.CreateTeachingPlan("管理员", p, learning.TeachingPlanUploadRequest{Title: "课程通用教案", Grade: course.Grade, Subject: course.Subject, CourseID: course.ID, File: learning.FileAsset{ID: "course-general-file", FileName: "general.pdf", OriginalPath: "/tmp/general.pdf", PreviewStatus: "可预览"}})
	if err != nil || plan.CourseID != course.ID || plan.LessonID != "" || plan.Semester != "S1" || plan.Phase != "Q1" {
		t.Fatalf("course-only upload: %#v %v", plan, err)
	}
	course.Curriculum = []learning.CurriculumNode{{ID: "replacement-unit", Type: learning.CurriculumUnit, Name: "替换目录", SortOrder: 1}}
	if _, err = s.UpdateCourse("管理员", p, course.ID, learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: course.LearningSpaceID, Curriculum: course.Curriculum}); err != nil {
		t.Fatalf("course-general plan blocked directory edit: %v", err)
	}
	if _, err = s.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{CourseID: course.ID, LessonID: "replacement-unit"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{CourseID: course.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateTeachingPlanChapter("管理员", p, plan.ID, learning.TeachingPlanChapterRequest{LessonID: "replacement-unit"}); err == nil {
		t.Fatal("orphan chapter accepted")
	}
	// The common plan still blocks changing its course's teaching scope.
	if _, err = s.UpdateCourse("管理员", p, course.ID, learning.CourseUpsertRequest{Name: course.Name, LearningSpaceID: "space-g05-english-s2-q1", Curriculum: course.Curriculum}); err == nil {
		t.Fatal("general plan allowed teaching scope drift")
	}
	target.Curriculum = course.Curriculum
	if _, err = s.UpdateCourse("管理员", p, target.ID, learning.CourseUpsertRequest{Name: target.Name, LearningSpaceID: target.LearningSpaceID, Curriculum: target.Curriculum}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportCourseFamily("管理员", p, learning.CourseFamilyImportRequest{Name: "课程通用教案系列", CourseIDs: []string{course.ID, target.ID}}); err != nil {
		t.Fatalf("course-general plan blocked equivalent family: %v", err)
	}
	student := learning.Principal{UserID: "student", Roles: []learning.Role{learning.RoleStudent}}
	if got := s.TeachingPlans(student); len(got.Plans) > 0 || len(got.Courses) > 0 {
		t.Fatal("student received internal plan courses")
	}
}
