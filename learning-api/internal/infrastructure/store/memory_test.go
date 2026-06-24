package store

import (
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestLoginWithDemoStudentPassword(t *testing.T) {
	store := NewMemoryStore()

	if _, err := store.LoginWithDemoStudentPassword("18500009069", "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}

	student, err := store.LoginWithDemoStudentPassword("18500009069", demoLoginPassword)
	if err != nil {
		t.Fatalf("expected demo student login to succeed: %v", err)
	}
	if student.UserID != "user-student-001" || !hasRole(student.Roles, learning.RoleStudent) {
		t.Fatalf("unexpected student principal: %#v", student)
	}

	if _, err := store.LoginWithDemoStudentPassword("13800000001", demoLoginPassword); err == nil {
		t.Fatal("expected admin account to be rejected by student demo login")
	}
}

func TestLoginWithWechatCodeBindsStudentByPhone(t *testing.T) {
	store := NewMemoryStore()

	unmatched, err := store.LoginWithWechatCode("new-openid", "", "")
	if err != nil {
		t.Fatalf("expected unbound wechat login to create a student account: %v", err)
	}
	if unmatched.StudentID == "" || !hasRole(unmatched.Roles, learning.RoleStudent) {
		t.Fatalf("unexpected unmatched student principal: %#v", unmatched)
	}
	home, err := store.StudentHome(unmatched)
	if err != nil {
		t.Fatalf("expected unmatched student to see empty home: %v", err)
	}
	if len(home.Materials) != 0 || len(home.PendingHomework) != 0 || home.ContinueCourse.ID != "" {
		t.Fatalf("expected unmatched student to have no opened content, got %#v", home)
	}

	student, err := store.LoginWithWechatCode("student", "18500009069", "")
	if err != nil {
		t.Fatalf("expected phone binding to succeed: %v", err)
	}
	if student.UserID != "user-student-001" || !hasRole(student.Roles, learning.RoleStudent) {
		t.Fatalf("unexpected student principal: %#v", student)
	}

	again, err := store.LoginWithWechatCode("student", "", "")
	if err != nil {
		t.Fatalf("expected bound wechat login to succeed: %v", err)
	}
	if again.UserID != student.UserID {
		t.Fatalf("expected same student after binding, got %#v", again)
	}
}

func TestUpdateStudentProfile(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	if _, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{
		Nickname: "小星星",
	}); err == nil {
		t.Fatal("expected missing avatar to be rejected")
	}

	if _, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{
		AvatarURL: "https://example.com/avatar.png",
	}); err == nil {
		t.Fatal("expected missing nickname to be rejected")
	}

	updated, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{
		Nickname:  " 小星星 ",
		AvatarURL: " https://example.com/avatar.png ",
	})
	if err != nil {
		t.Fatalf("expected profile update to succeed: %v", err)
	}
	if updated.Nickname != "小星星" || updated.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected updated profile: %#v", updated)
	}

	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if home.Student.Nickname != updated.Nickname || home.Student.AvatarURL != updated.AvatarURL {
		t.Fatalf("expected student home to include profile, got %#v", home.Student)
	}
}

func TestPhoneBindingMergesWechatOnlyStudent(t *testing.T) {
	store := NewMemoryStore()

	wechatOnly, err := store.LoginWithWechatCode("student", "", "")
	if err != nil {
		t.Fatalf("expected wechat-only login to succeed: %v", err)
	}
	if wechatOnly.UserID == "user-student-001" {
		t.Fatalf("expected temporary wechat student, got seeded student: %#v", wechatOnly)
	}

	student, err := store.LoginWithWechatCode("student", "18500009069", "")
	if err != nil {
		t.Fatalf("expected phone binding to merge into seeded student: %v", err)
	}
	if student.UserID != "user-student-001" {
		t.Fatalf("unexpected student principal after merge: %#v", student)
	}
	for _, user := range store.users {
		if user.ID == wechatOnly.UserID {
			t.Fatal("temporary user should be removed after phone binding")
		}
	}
	again, err := store.LoginWithWechatCode("student", "", "")
	if err != nil {
		t.Fatalf("expected merged openID to login: %v", err)
	}
	if again.UserID != "user-student-001" {
		t.Fatalf("expected seeded student after merge, got %#v", again)
	}
}

func TestRealWechatBindingCanReplaceDemoOpenID(t *testing.T) {
	store := NewMemoryStore()
	for i := range store.users {
		if store.users[i].ID == "user-student-001" {
			store.users[i].OpenID = "demo-student"
			break
		}
	}
	store.wechatResolver = func(code string) (string, error) {
		return "real-" + code, nil
	}

	student, err := store.LoginWithWechatCode("wx-code", "18500009069", "")
	if err != nil {
		t.Fatalf("expected real wechat binding to replace demo openID: %v", err)
	}
	if student.UserID != "user-student-001" {
		t.Fatalf("unexpected student principal: %#v", student)
	}

	again, err := store.LoginWithWechatCode("wx-code", "", "")
	if err != nil {
		t.Fatalf("expected replaced openID to login: %v", err)
	}
	if again.UserID != student.UserID {
		t.Fatalf("expected same student after replacing demo openID, got %#v", again)
	}
}

func TestLoginWithAdminPassword(t *testing.T) {
	store := NewMemoryStore()

	admin, err := store.LoginWithAdminPassword("13800000001", demoLoginPassword)
	if err != nil {
		t.Fatalf("expected admin login to succeed: %v", err)
	}
	if admin.UserID != "user-super" || !hasRole(admin.Roles, learning.RoleSuperAdmin) {
		t.Fatalf("unexpected admin principal: %#v", admin)
	}

	teacher, err := store.LoginWithAdminPassword("13800000004", demoLoginPassword)
	if err != nil {
		t.Fatalf("expected teacher login to succeed: %v", err)
	}
	if teacher.UserID != "user-teacher" || !hasRole(teacher.Roles, learning.RoleTeacher) {
		t.Fatalf("unexpected teacher principal: %#v", teacher)
	}

	if _, err := store.LoginWithAdminPassword("13800000001", "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}

	if _, err := store.LoginWithAdminPassword("18500009069", demoLoginPassword); err == nil {
		t.Fatal("expected student account to be rejected by admin login")
	}

	store.users[0].AccountStatus = "停用"
	if _, err := store.LoginWithAdminPassword("13800000001", demoLoginPassword); err == nil {
		t.Fatal("expected disabled admin account to fail")
	}
}

func TestAvailabilityOverviewRespectsVisibleOwners(t *testing.T) {
	store := NewMemoryStore()

	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	opsSlots := store.AvailabilityOverview(ops)
	if len(opsSlots) != len(store.availability) {
		t.Fatalf("expected ops to see all seeded availability slots, got %d want %d", len(opsSlots), len(store.availability))
	}

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	teacherSlots := store.AvailabilityOverview(teacher)
	if len(teacherSlots) == 0 {
		t.Fatal("expected teacher overview to include visible slots")
	}
	for _, slot := range teacherSlots {
		if slot.OwnerType != "teacher" || slot.OwnerID != teacher.UserID {
			t.Fatalf("teacher should only see own availability: %#v", slot)
		}
	}
}

func TestTeachersIncludesGradeSubjectSummary(t *testing.T) {
	store := NewMemoryStore()

	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	teachers := store.Teachers(admin)
	if len(teachers) == 0 {
		t.Fatal("expected seeded teachers")
	}

	var demo learning.Teacher
	for _, teacher := range teachers {
		if teacher.ID == "user-teacher" {
			demo = teacher
			break
		}
	}
	if demo.ID == "" {
		t.Fatal("expected demo teacher")
	}
	if !containsString(demo.Grades, "五年级") {
		t.Fatalf("expected teacher grades to include 五年级, got %#v", demo.Grades)
	}
	if !containsString(demo.Subjects, "英语") {
		t.Fatalf("expected teacher subjects to include 英语, got %#v", demo.Subjects)
	}
}

func TestCompleteReviewCreatesStudentSubmission(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	before := len(store.Reviews(teacher))
	submission, err := store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          95,
		TeacherComment: "阅读依据找得很准，继续保持。",
		Reward:         "阅读小星星",
	})
	if err != nil {
		t.Fatalf("expected review completion to succeed: %v", err)
	}
	if submission.StudentID != "stu-001" || submission.HomeworkID != "hw-g05-english-s1-mid" {
		t.Fatalf("unexpected submission linkage: %#v", submission)
	}
	if got := len(store.Reviews(teacher)); got != before-1 {
		t.Fatalf("expected pending reviews to decrease, got %d want %d", got, before-1)
	}

	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	result, err := store.StudentSubmission(student, submission.ID)
	if err != nil {
		t.Fatalf("expected student to see review result: %v", err)
	}
	if result.TeacherComment != "阅读依据找得很准，继续保持。" || result.Score != 95 {
		t.Fatalf("unexpected result content: %#v", result)
	}
}

func TestCompleteReviewRequiresReviewPermission(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	teacher.CanReview = false

	_, err = store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          90,
		TeacherComment: "继续保持。",
	})
	if err == nil {
		t.Fatal("expected teacher without review permission to fail")
	}
}

func TestCreatePackageSupportsGrantPreview(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语专项题包",
		AcademicYear:     "2026 学年",
		Grade:            "五年级",
		Semester:         "第一学期",
		Subject:          "英语",
		PhaseScope:       "期中前",
		PackageType:      "题",
		Summary:          "只开放期中前英语练习。",
		LearningSpaceIDs: []string{"space-g05-english-s1-mid"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	if !containsString(pkg.LearningSpaceIDs, "space-g05-english-s1-mid") {
		t.Fatalf("expected package response to include learning space ids: %#v", pkg.LearningSpaceIDs)
	}
	if !containsString(pkg.ContentTypeCodes, "question") {
		t.Fatalf("expected package response to include content type codes: %#v", pkg.ContentTypeCodes)
	}

	preview, err := store.GrantPreview("stu-001", pkg.ID)
	if err != nil {
		t.Fatalf("expected grant preview for created package: %v", err)
	}
	if len(preview.OpenHomework) == 0 {
		t.Fatalf("expected preview to include opened homework: %#v", preview)
	}
	if len(preview.OpenCourses) != 0 || len(preview.OpenMaterials) != 0 {
		t.Fatalf("question-only package should not open courses/materials: %#v", preview)
	}
}

func TestUpdatePackageRefreshesExistingGrantAccess(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语可编辑套餐",
		AcademicYear:     "2026 学年",
		Grade:            "五年级",
		Semester:         "第一学期",
		Subject:          "英语",
		PhaseScope:       "期中前",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-mid"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err != nil {
		t.Fatalf("expected grant creation to succeed: %v", err)
	}

	_, err = store.UpdatePackage("运营教务", pkg.ID, learning.PackageUpsertRequest{
		Name:             "五年级英语可编辑套餐",
		AcademicYear:     "2026 学年",
		Grade:            "五年级",
		Semester:         "第一学期",
		Subject:          "英语",
		PhaseScope:       "全学期",
		PackageType:      "题+讲义",
		LearningSpaceIDs: []string{"space-g05-english-s1-mid", "space-g05-english-s1-final"},
		ContentTypeCodes: []string{"question", "handout"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package update to succeed: %v", err)
	}

	spaceIDs := store.studentAccessibleSpaceIDs("stu-001")
	if !containsString(spaceIDs, "space-g05-english-s1-final") {
		t.Fatalf("expected existing grant access to refresh after package update, got %#v", spaceIDs)
	}
	materials := store.materialsForStudent("stu-001")
	foundFinalMaterial := false
	for _, material := range materials {
		if material.LearningSpaceID == "space-g05-english-s1-final" {
			foundFinalMaterial = true
			break
		}
	}
	if !foundFinalMaterial {
		t.Fatalf("expected updated package to open final material, got %#v", materials)
	}
}

func TestGrantPreviewMarksExistingActiveGrant(t *testing.T) {
	store := NewMemoryStore()

	preview, err := store.GrantPreview("stu-001", "pkg-g05-english-s1-full")
	if err != nil {
		t.Fatalf("expected grant preview to succeed: %v", err)
	}
	if !preview.AlreadyOpened {
		t.Fatalf("expected existing active grant to be marked, got %#v", preview)
	}
	if preview.ExistingUntil != "2027-05-22" {
		t.Fatalf("expected existing grant expiry, got %q", preview.ExistingUntil)
	}

	created, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语未开通套餐",
		AcademicYear:     "2026 学年",
		Grade:            "五年级",
		Semester:         "第一学期",
		Subject:          "英语",
		PhaseScope:       "期中前",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-mid"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	nextPreview, err := store.GrantPreview("stu-001", created.ID)
	if err != nil {
		t.Fatalf("expected new package preview to succeed: %v", err)
	}
	if nextPreview.AlreadyOpened || nextPreview.ExistingUntil != "" {
		t.Fatalf("expected unopened package preview, got %#v", nextPreview)
	}
}

func TestCreateCourseRespectsTeacherScope(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	course, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "五年级英语阅读拓展课",
		LearningSpaceID: "space-g05-english-s1-mid",
		ChapterCount:    6,
		Status:          learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected teacher to create course in own scope: %v", err)
	}
	if course.Subject != "英语" || course.Grade != "五年级" || course.MaterialNum != 0 || course.HomeworkNum != 0 {
		t.Fatalf("unexpected course: %#v", course)
	}

	if _, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "五年级数学拓展课",
		LearningSpaceID: "space-g05-math-s1-mid",
		ChapterCount:    6,
		Status:          learning.StatusEnabled,
	}); err == nil {
		t.Fatal("expected teacher to be blocked from another subject")
	}
}

func TestUpdateCourseSyncsContentReferences(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	updated, err := store.UpdateCourse("超级管理员", admin, "course-g05-english-s1-mid", learning.CourseUpsertRequest{
		Name:            "五年级英语期中阅读精讲课",
		LearningSpaceID: "space-g05-english-s1-mid",
		ChapterCount:    10,
		Status:          learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected course update to succeed: %v", err)
	}
	if updated.ChapterCount != 10 || updated.MaterialNum == 0 || updated.HomeworkNum == 0 {
		t.Fatalf("unexpected updated course counts: %#v", updated)
	}
	for _, material := range store.materials {
		if material.CourseID == updated.ID && material.Course != updated.Name {
			t.Fatalf("expected material course name to sync: %#v", material)
		}
	}
	for _, homework := range store.homework {
		if homework.CourseID == updated.ID && homework.Course != updated.Name {
			t.Fatalf("expected homework course name to sync: %#v", homework)
		}
	}
}

func TestCreateNoticeAndStudentNoticeFiltering(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "英语阅读挑战已发布",
		Target:  "五年级英语班",
		Summary: "今天完成期中前阅读挑战。",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if notice.ID == "" || notice.Status != "已发送" {
		t.Fatalf("unexpected notice: %#v", notice)
	}

	englishStudent, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(englishStudent)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if !noticeListContains(home.Notices, notice.ID) {
		t.Fatalf("expected English student to see notice, got %#v", home.Notices)
	}
}

func TestUpdateSettingValidatesAndLogs(t *testing.T) {
	store := NewMemoryStore()

	settings, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{
		Key:   "downloadPolicy",
		Value: "允许下载已发布讲义",
	})
	if err != nil {
		t.Fatalf("expected setting update to succeed: %v", err)
	}
	if settings["downloadPolicy"] != "允许下载已发布讲义" {
		t.Fatalf("expected updated setting, got %#v", settings)
	}
	if store.logs[0].Action != "修改系统设置" || store.logs[0].Target != "下载规则" {
		t.Fatalf("expected setting update log, got %#v", store.logs[0])
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "downloadPolicy"}); err == nil {
		t.Fatal("expected empty setting value to be rejected")
	}
	if _, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "unknown", Value: "x"}); err == nil {
		t.Fatal("expected unknown setting key to be rejected")
	}
}

func TestTeacherNoticeScopeIsRestricted(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	if _, err := store.CreateNotice("英语老师", teacher, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "数学挑战已发布",
		Target:  "五年级数学班",
		Summary: "今天完成数学图形挑战。",
	}); err == nil {
		t.Fatal("expected English teacher to be blocked from sending math notice")
	}
	if _, err := store.CreateNotice("英语老师", teacher, learning.NoticeCreateRequest{
		Type:    "练",
		Title:   "英语阅读挑战已发布",
		Target:  "五年级英语班",
		Summary: "今天完成英语阅读挑战。",
	}); err != nil {
		t.Fatalf("expected English teacher to send English notice: %v", err)
	}
}

func TestUpdateMaterialDraftHidesFromStudent(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	courses := store.Courses(teacher)
	if len(courses) == 0 {
		t.Fatal("expected teacher to see courses")
	}
	created, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "可编辑讲义",
		CourseID: courses[0].ID,
		File: learning.FileAsset{
			ID:            "file-test-material",
			FileName:      "material.pdf",
			FileType:      "PDF",
			PreviewStatus: "可预览",
		},
	})
	if err != nil {
		t.Fatalf("expected material creation to succeed: %v", err)
	}

	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	before, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("expected student study board: %v", err)
	}
	if !materialVisible(before.Materials, created.ID) {
		t.Fatalf("expected created material to be visible to student: %#v", before.Materials)
	}

	if _, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "草稿讲义",
		CourseID: courses[0].ID,
		Chapter:  "第一章",
		Status:   learning.StatusDraft,
	}); err != nil {
		t.Fatalf("expected material update to succeed: %v", err)
	}
	after, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("expected student study board after update: %v", err)
	}
	if materialVisible(after.Materials, created.ID) {
		t.Fatalf("expected draft material to be hidden from student: %#v", after.Materials)
	}
}

func TestUpdateHomeworkRejectsTeacherOutsideScope(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	courses := store.Courses(teacher)
	if len(courses) == 0 {
		t.Fatal("expected teacher to see courses")
	}
	var outside learning.Homework
	for _, item := range store.homework {
		if !containsString(teacher.LearningSpaceIDs, item.LearningSpaceID) {
			outside = item
			break
		}
	}
	if outside.ID == "" {
		t.Fatal("expected seeded homework outside teacher scope")
	}
	if _, err := store.UpdateHomework("英语老师", teacher, outside.ID, learning.HomeworkUpdateRequest{
		Title:    "跨范围题目",
		CourseID: courses[0].ID,
		Status:   string(learning.StatusEnabled),
	}); err == nil {
		t.Fatal("expected cross-scope homework update to fail")
	}
}

func TestDisabledHomeworkIsHiddenFromStudent(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	if _, err := store.UpdateHomework("英语老师", teacher, "hw-g05-english-s1-mid", learning.HomeworkUpdateRequest{
		Title:    "停用练习",
		CourseID: "course-g05-english-s1-mid",
		Deadline: "2026-10-30",
		Status:   string(learning.StatusDisabled),
	}); err != nil {
		t.Fatalf("expected homework update to succeed: %v", err)
	}

	tasks, err := store.StudentTasks(student)
	if err != nil {
		t.Fatalf("expected student tasks: %v", err)
	}
	for _, task := range tasks {
		if task.ID == "hw-g05-english-s1-mid" {
			t.Fatalf("disabled homework should be hidden from student tasks: %#v", task)
		}
	}
}

func TestQuestionBankReusableByGradeSemesterSubjectAndHomeworkReviewFlow(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	item, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Grade: "五年级", Semester: "第一学期", Subject: "英语", Type: "multiple",
		Stem: "哪些做法有助于英语阅读？", Options: []string{"圈关键词", "完全不读题", "复查答案"},
		Answers: []string{"圈关键词", "复查答案"}, Score: 10, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected question creation to succeed: %v", err)
	}
	created, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{
		Title: "题库组卷练习", CourseID: "course-g05-english-s1-mid", LearningSpaceID: "space-g05-english-s1-mid",
		Deadline: "2026-11-01", Status: string(learning.StatusEnabled), QuestionIDs: []string{item.ID},
	})
	if err != nil {
		t.Fatalf("expected homework creation to succeed: %v", err)
	}
	if created.QuestionNum != 1 || created.Questions[0].ID != item.ID {
		t.Fatalf("unexpected homework questions: %#v", created)
	}
	submission, err := store.CreateSubmission("学生", student, learning.SubmissionRequest{
		HomeworkID: created.ID,
		Answers:    []learning.SubmissionAnswer{{QuestionID: item.ID, Choices: []string{"复查答案", "圈关键词"}}},
	})
	if err != nil {
		t.Fatalf("expected submission to succeed: %v", err)
	}
	if submission.Status != "已批改" || submission.Score != 100 {
		t.Fatalf("expected all-objective homework to auto grade: %#v", submission)
	}
}

func TestTextQuestionSubmissionCreatesPendingReview(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	submission, err := store.CreateSubmission("学生", student, learning.SubmissionRequest{
		HomeworkID: "hw-g05-english-s1-mid",
		Answers: []learning.SubmissionAnswer{
			{QuestionID: "q1", Choice: "A"},
			{QuestionID: "q2", Text: "今天学会了抓中心句。"},
		},
	})
	if err != nil {
		t.Fatalf("expected submission to succeed: %v", err)
	}
	if submission.Status != "待批改" || submission.ObjectiveScore == 0 {
		t.Fatalf("expected text homework to be pending review with objective score: %#v", submission)
	}
	if len(store.reviews) == 0 || store.reviews[0].SubmissionID != submission.ID {
		t.Fatalf("expected pending review for submission, reviews=%#v", store.reviews)
	}
}

func noticeListContains(notices []learning.Notice, id string) bool {
	for _, notice := range notices {
		if notice.ID == id {
			return true
		}
	}
	return false
}

func materialVisible(items []learning.Material, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
