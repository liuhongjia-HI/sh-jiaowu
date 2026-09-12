package store

import (
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestCreateMaterialNotifiesCurrentCourseStudentsOnly(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	course, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "内容更新通知课程",
		LearningSpaceID: "space-g05-english-s1-q1",
		Status:          learning.StatusEnabled,
		Curriculum:      testCurriculum("notice-course"),
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}

	if _, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "HD_第一课讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		File:     learning.FileAsset{ID: "file-notice-hd", FileName: "hd.pdf", FileType: "PDF"},
	}); err != nil {
		t.Fatalf("create material: %v", err)
	}

	wantTitle := "G5英文课已上传新内容，请查看"
	gotByStudent := map[string]learning.Notice{}
	for _, notice := range store.notices {
		if notice.RelatedType != "course" || notice.RelatedID != course.ID {
			continue
		}
		if notice.Title != wantTitle || notice.Summary != "请查看" || notice.Type != "课" || notice.Channel != "站内通知" {
			t.Fatalf("unexpected course content notice: %#v", notice)
		}
		if notice.RecipientStudentID == "" {
			t.Fatalf("course content notice must bind a student: %#v", notice)
		}
		gotByStudent[notice.RecipientStudentID] = notice
	}
	if _, ok := gotByStudent["stu-001"]; !ok {
		t.Fatalf("expected current English student stu-001 to be notified, got %#v", gotByStudent)
	}
	if _, ok := gotByStudent["stu-002"]; !ok {
		t.Fatalf("expected current English student stu-002 to be notified, got %#v", gotByStudent)
	}
	if _, ok := gotByStudent["stu-003"]; !ok {
		t.Fatalf("expected current English student stu-003 to be notified, got %#v", gotByStudent)
	}

	xiaoming, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(xiaoming)
	if err != nil {
		t.Fatalf("student home: %v", err)
	}
	if !noticeListContains(home.Notices, gotByStudent["stu-001"].ID) {
		t.Fatalf("current student should see the course content notice, got %#v", home.Notices)
	}
	for _, notice := range home.Notices {
		if notice.RecipientStudentID != "stu-001" {
			t.Fatalf("student inbox leaked another student's notice: %#v", notice)
		}
		if notice.RelatedType != "course" {
			t.Fatalf("student inbox should only include course content notices, got %#v", notice)
		}
	}

	lateStudent, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "新来的学生", Phone: "13900009999", Grade: "五年级", AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("create late student: %v", err)
	}
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{
		StudentID: lateStudent.ID, PackageID: packageID(4, "英文", 0, "full"),
	}); err != nil {
		t.Fatalf("open package for late student: %v", err)
	}
	if got := store.noticesForStudent(lateStudent); len(got) != 0 {
		t.Fatalf("new student should not see historical course notices, got %#v", got)
	}

	mathOnly, err := store.CreateStudent("超级管理员", admin, learning.StudentUpsertRequest{
		Name: "只开通数学", Phone: "13900008888", Grade: "五年级", AccountStatus: "正常",
	})
	if err != nil {
		t.Fatalf("create math student: %v", err)
	}
	if _, err := store.CreateGrant("运营教务", learning.GrantCreateRequest{
		StudentID: mathOnly.ID, PackageID: packageID(4, "数学", 0, "question_handout"),
	}); err != nil {
		t.Fatalf("open math package: %v", err)
	}

	if _, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "Blank_第一课讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "Blank",
		File:     learning.FileAsset{ID: "file-notice-blank", FileName: "blank.pdf", FileType: "PDF"},
	}); err != nil {
		t.Fatalf("create second material: %v", err)
	}

	lateNotices := store.noticesForStudent(lateStudent)
	if len(lateNotices) != 1 {
		t.Fatalf("late student should only see notices after joining, got %#v", lateNotices)
	}
	if lateNotices[0].RelatedID != course.ID || lateNotices[0].RecipientStudentID != lateStudent.ID {
		t.Fatalf("late student notice mismatch: %#v", lateNotices[0])
	}
	if got := store.noticesForStudent(mathOnly); len(got) != 0 {
		t.Fatalf("math-only student should not receive English course notices, got %#v", got)
	}

	xiaomingHome, err := store.StudentHome(xiaoming)
	if err != nil {
		t.Fatalf("student home after second upload: %v", err)
	}
	courseNoticeCount := 0
	for _, notice := range xiaomingHome.Notices {
		if notice.RelatedType == "course" && notice.RelatedID == course.ID {
			courseNoticeCount++
		}
	}
	if courseNoticeCount != 2 {
		t.Fatalf("each upload should append a notice for current students, got %d from %#v", courseNoticeCount, xiaomingHome.Notices)
	}
}

func TestUpdateMaterialDraftDoesNotNotifyUntilPublished(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	course, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "草稿发布通知课程",
		LearningSpaceID: "space-g05-english-s1-q1",
		Status:          learning.StatusEnabled,
		Curriculum:      testCurriculum("notice-draft"),
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	created, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "HD_草稿讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		File:     learning.FileAsset{ID: "file-notice-draft", FileName: "draft.pdf", FileType: "PDF"},
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	firstCount := countCourseNotices(store, course.ID, "stu-001")
	if firstCount != 1 {
		t.Fatalf("published upload should notify once, got %d", firstCount)
	}

	if _, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "HD_草稿讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		Status:   learning.StatusDraft,
	}); err != nil {
		t.Fatalf("set draft: %v", err)
	}
	if count := countCourseNotices(store, course.ID, "stu-001"); count != firstCount {
		t.Fatalf("switching to draft should not notify, got %d", count)
	}

	if _, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "HD_草稿讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		Status:   learning.StatusEnabled,
	}); err != nil {
		t.Fatalf("publish material: %v", err)
	}
	if count := countCourseNotices(store, course.ID, "stu-001"); count != firstCount+1 {
		t.Fatalf("publishing a draft should notify once more, got %d", count)
	}

	if _, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "HD_草稿讲义改名",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		Status:   learning.StatusEnabled,
	}); err != nil {
		t.Fatalf("rename published material: %v", err)
	}
	if count := countCourseNotices(store, course.ID, "stu-001"); count != firstCount+1 {
		t.Fatalf("editing an already published material should not notify, got %d", count)
	}
}

func TestReplaceMaterialAppendsAnotherCourseNotice(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	course, err := store.CreateCourse("英语老师", teacher, learning.CourseUpsertRequest{
		Name:            "替换资料通知课程",
		LearningSpaceID: "space-g05-english-s1-q1",
		Status:          learning.StatusEnabled,
		Curriculum:      testCurriculum("notice-replace"),
	})
	if err != nil {
		t.Fatalf("create course: %v", err)
	}
	if _, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "HD_替换讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		File:     learning.FileAsset{ID: "file-notice-old", FileName: "old.pdf", FileType: "PDF"},
	}); err != nil {
		t.Fatalf("create material: %v", err)
	}
	if _, err := store.CreateMaterial("英语老师", teacher, learning.MaterialUploadRequest{
		Title:    "HD_替换讲义",
		CourseID: course.ID,
		LessonID: firstLessonID(course),
		TagCode:  "HD",
		File:     learning.FileAsset{ID: "file-notice-new", FileName: "new.pdf", FileType: "PDF"},
	}); err != nil {
		t.Fatalf("replace material: %v", err)
	}
	if count := countCourseNotices(store, course.ID, "stu-001"); count != 2 {
		t.Fatalf("replacing published content should append another notice, got %d", count)
	}
}

func TestStudentInboxIgnoresGradeWideHistoricalNotices(t *testing.T) {
	store := NewMemoryStore()
	store.notices = []learning.Notice{
		{ID: "legacy-grade", Type: "练", Title: "英语阅读挑战已发布", Target: "五年级英语班", Summary: "今天的小挑战别忘啦", Status: "已发送"},
		{ID: "legacy-all", Type: "通知", Title: "全体通知", Target: "全部学生", Summary: "请查看", Status: "已发送"},
	}
	student := learning.Student{ID: "stu-001", Name: "小明", Grade: "五年级", AccountStatus: "正常"}
	if got := store.noticesForStudent(student); len(got) != 0 {
		t.Fatalf("legacy grade-wide notices must not appear in student inbox, got %#v", got)
	}
}

func countCourseNotices(store *MemoryStore, courseID, studentID string) int {
	count := 0
	for _, notice := range store.notices {
		if notice.RelatedType == "course" && notice.RelatedID == courseID && notice.RecipientStudentID == studentID {
			count++
		}
	}
	return count
}

func TestCourseContentNoticeTitleUsesGradeCode(t *testing.T) {
	store := NewMemoryStore()
	title := store.courseContentNoticeTitle(learning.Course{Grade: "六年级", Subject: "科学"})
	if title != "G6科学课已上传新内容，请查看" {
		t.Fatalf("unexpected notice title %q", title)
	}
	if !strings.Contains(title, "请查看") {
		t.Fatalf("notice title should ask the student to check the course, got %q", title)
	}
}
