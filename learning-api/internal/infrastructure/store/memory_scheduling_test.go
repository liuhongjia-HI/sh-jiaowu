package store

import (
	"sort"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestScheduleClassCanReserveTimeWithoutRegisteredStudents(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:             "course-g05-english-s1-q1",
		TeacherID:            "user-teacher",
		CampusID:             "campus-main",
		ClassType:            "1V2",
		DurationMinutes:      90,
		StartTime:            "19:00",
		EndTime:              "20:30",
		StartDate:            "2026-06-03",
		ExpectedStudentCount: 2,
		ReservationNote:      "待家长确认学生名单",
	}
	created, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected pending reservation to succeed: %v", err)
	}
	if created.Status != "待确认" || len(created.Students) != 0 {
		t.Fatalf("expected pending class without students, got %#v", created)
	}
	if created.ExpectedStudentCount != 2 || created.ReservationNote != req.ReservationNote {
		t.Fatalf("expected reservation metadata to be kept, got %#v", created)
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err == nil || !strings.Contains(err.Error(), "老师该时间已有课程") {
		t.Fatalf("expected pending reservation to lock teacher time, got %v", err)
	}
}

func TestScheduleClassKeepsRoomMetadataWithoutBlocking(t *testing.T) {
	store := NewMemoryStore()
	store.users = append(store.users, learning.User{
		ID:               "user-teacher-room",
		Name:             "同教室老师",
		Phone:            "13800000009",
		AccountStatus:    "正常",
		Roles:            []learning.Role{learning.RoleTeacher},
		CampusID:         "campus-main",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
	})
	store.availability = append(store.availability, learning.AvailabilitySlot{
		ID: "av-teacher-room", OwnerType: "teacher", OwnerID: "user-teacher-room", OwnerName: "同教室老师",
		DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-06-01", EndDate: "2026-08-31",
	})
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		RoomName:        "A101",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
	}
	created, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected first schedule to succeed: %v", err)
	}
	if created.CampusID != "campus-main" || created.RoomName != "A101" {
		t.Fatalf("expected room metadata to be stored, got %#v", created)
	}

	req.TeacherID = "user-teacher-room"
	req.StudentIDs = []string{"stu-002"}
	second, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected same room to be allowed because room resources are not blocking, got %v", err)
	}
	if second.RoomName != "A101" {
		t.Fatalf("expected room metadata to be kept, got %#v", second)
	}
}

func TestScheduleClassResolvesTermFromAcademicCalendar(t *testing.T) {
	store := NewMemoryStore()
	// 用一份确定的校历覆盖种子数据，避免用例结果随运行日期漂移。
	// 种子数据里 user-teacher 的可授课时段固定在 2026-06-01 至 2026-08-31，
	// 所以校历也配成覆盖这个窗口，避免用例卡在“老师该时间不可授课”上。
	store.settings["academicCalendar"] = `[
		{"academicYear":"2025.2026学年","semester":"S2 第二学期","startDate":"2026-02-01","endDate":"2026-07-15"},
		{"academicYear":"2025.2026学年","semester":"暑期学期","startDate":"2026-07-16","endDate":"2026-08-31"}
	]`
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-10",
	}
	created, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected schedule to succeed: %v", err)
	}
	if created.AcademicYear != "2025.2026学年" || created.Semester != "S2 第二学期" {
		t.Fatalf("expected term resolved from calendar, got %#v", created)
	}

	// 建班后再调整校历，历史排课的学年归属不应该跟着漂移——只有真的改了
	// 开课日期或课程才重新判定，见 updateScheduleClassUnlocked。
	store.settings["academicCalendar"] = `[
		{"academicYear":"2026.2027学年","semester":"S2 第二学期","startDate":"2026-02-01","endDate":"2026-07-15"}
	]`
	req.RoomName = "A102"
	updated, err := store.UpdateScheduleClass("运营教务", ops, created.ID, req)
	if err != nil {
		t.Fatalf("expected update to succeed: %v", err)
	}
	if updated.AcademicYear != "2025.2026学年" || updated.Semester != "S2 第二学期" {
		t.Fatalf("expected term to stay fixed across unrelated edits, got %#v", updated)
	}

	// 开课日期落不进任何校历学期（例如假期班）时，退回课程所属学习空间的
	// 学期标签 + 开课日期本身的 7 月 1 日规则，而不是阻塞建班。
	req2 := req
	req2.RoomName = ""
	req2.StartDate = "2026-07-22"
	holiday, err := store.CreateScheduleClass("运营教务", ops, req2)
	if err != nil {
		t.Fatalf("expected holiday schedule to succeed: %v", err)
	}
	if holiday.AcademicYear != "2026.2027学年" || holiday.Semester != "S1" {
		t.Fatalf("expected fallback term derived from course space + start date, got %#v", holiday)
	}
}

func TestScheduleClassRejectsStudentConflict(t *testing.T) {
	store := NewMemoryStore()
	store.users = append(store.users,
		learning.User{
			ID:               "user-teacher-alt",
			Name:             "英语代课老师",
			Phone:            "13800000010",
			AccountStatus:    "正常",
			Roles:            []learning.Role{learning.RoleTeacher},
			CampusID:         "campus-main",
			LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		},
	)
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		RoomName:        "A101",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err != nil {
		t.Fatalf("expected first schedule to succeed: %v", err)
	}

	conflictReq := req
	conflictReq.TeacherID = "user-teacher-alt"
	conflictReq.RoomName = "A102"
	if _, err := store.CreateScheduleClass("运营教务", ops, conflictReq); err == nil || !strings.Contains(err.Error(), "小明 该时间已有课程") {
		t.Fatalf("expected student schedule conflict, got %v", err)
	}
}

func TestScheduleClassRejectsTeacherUnavailableTime(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		RoomName:        "A101",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-04",
		StudentIDs:      []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err == nil || !strings.Contains(err.Error(), "超出 英语老师 的可上课时间") {
		t.Fatalf("expected teacher unavailable time to be rejected, got %v", err)
	}
}

func TestScheduleClassConflictHonorsDateRanges(t *testing.T) {
	store := NewMemoryStore()
	store.users = append(store.users, learning.User{
		ID:               "user-teacher-room",
		Name:             "同教室老师",
		Phone:            "13800000009",
		AccountStatus:    "正常",
		Roles:            []learning.Role{learning.RoleTeacher},
		CampusID:         "campus-main",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
	})
	store.availability = append(store.availability,
		learning.AvailabilitySlot{ID: "av-teacher-fall", OwnerType: "teacher", OwnerID: "user-teacher", OwnerName: "英语老师", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-09-01", EndDate: "2026-12-31"},
		learning.AvailabilitySlot{ID: "av-teacher-room-fall", OwnerType: "teacher", OwnerID: "user-teacher-room", OwnerName: "同教室老师", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-09-01", EndDate: "2026-12-31"},
		learning.AvailabilitySlot{ID: "av-stu-001-fall", OwnerType: "student", OwnerID: "stu-001", OwnerName: "小明", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-09-01", EndDate: "2026-12-31"},
		learning.AvailabilitySlot{ID: "av-stu-002-fall", OwnerType: "student", OwnerID: "stu-002", OwnerName: "Lucy", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-09-01", EndDate: "2026-12-31"},
	)
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		RoomName:        "A101",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err != nil {
		t.Fatalf("expected summer schedule to succeed: %v", err)
	}

	fallReq := req
	fallReq.StartDate = "2026-09-02"
	if _, err := store.CreateScheduleClass("运营教务", ops, fallReq); err != nil {
		t.Fatalf("expected same time outside date range to be allowed: %v", err)
	}

	overlapReq := req
	overlapReq.StartDate = "2026-06-03"
	if _, err := store.CreateScheduleClass("运营教务", ops, overlapReq); err == nil || !strings.Contains(err.Error(), "老师该时间已有课程") {
		t.Fatalf("expected overlapping teacher conflict, got %v", err)
	}

	roomReq := fallReq
	roomReq.TeacherID = "user-teacher-room"
	roomReq.StudentIDs = []string{"stu-002"}
	if _, err := store.CreateScheduleClass("运营教务", ops, roomReq); err != nil {
		t.Fatalf("expected overlapping fall room metadata not to block scheduling, got %v", err)
	}
}

func TestScheduleClassAutoOfficialAccountNotices(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-schedule-openid"
		}
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	sent := make([]learning.Notice, 0)
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent = append(sent, notice)
		if notice.Channel != "公众号模板消息" || notice.RecipientOpenID != "oa-schedule-openid" {
			t.Fatalf("expected official account schedule notice, got %#v", notice)
		}
		return nil
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		RoomName:        "A101",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
	}
	created, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected schedule creation to succeed: %v", err)
	}
	assertScheduleNotice(t, findScheduleOfficialNotice(t, store.notices, created.ID, "课程已安排"), created.ID, "课程已安排", "周三")

	req.StartDate = "2026-06-06"
	req.StartTime = "09:00"
	req.EndTime = "10:30"
	updated, err := store.UpdateScheduleClass("运营教务", ops, created.ID, req)
	if err != nil {
		t.Fatalf("expected schedule update to succeed: %v", err)
	}
	assertScheduleNotice(t, findScheduleOfficialNotice(t, store.notices, updated.ID, "课程调整提醒"), updated.ID, "课程调整提醒", "周六")

	cancelled, err := store.CancelScheduleClass("运营教务", ops, created.ID)
	if err != nil {
		t.Fatalf("expected schedule cancellation to succeed: %v", err)
	}
	assertScheduleNotice(t, findScheduleOfficialNotice(t, store.notices, cancelled.ID, "课程取消提醒"), cancelled.ID, "课程取消提醒", "周六")
	if len(sent) != 3 {
		t.Fatalf("expected create/update/cancel to send three notices, sent=%d", len(sent))
	}
}

func findScheduleOfficialNotice(t *testing.T, notices []learning.Notice, scheduleID, title string) learning.Notice {
	t.Helper()
	for _, notice := range notices {
		if notice.RelatedType == "schedule" && notice.RelatedID == scheduleID && notice.Title == title && notice.Channel == "公众号模板消息" {
			return notice
		}
	}
	t.Fatalf("expected official account schedule notice %q for %s, got %#v", title, scheduleID, notices)
	return learning.Notice{}
}

func assertScheduleNotice(t *testing.T, notice learning.Notice, scheduleID, title, summaryPart string) {
	t.Helper()
	if notice.RelatedType != "schedule" || notice.RelatedID != scheduleID {
		t.Fatalf("expected schedule related notice, got %#v", notice)
	}
	if notice.Title != title || notice.Status != "已发送" || notice.Target != "小明" {
		t.Fatalf("unexpected schedule notice state, got %#v", notice)
	}
	if !strings.Contains(notice.Summary, summaryPart) || !strings.Contains(notice.Summary, "老师：英语老师") {
		t.Fatalf("expected schedule summary to include time and teacher, got %#v", notice)
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
		Title:    "可编辑学习资料",
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
	if created.Grade != "五年级" || created.Semester != "S1" || created.Subject != "英文" {
		t.Fatalf("expected created material to include learning dimensions, got %#v", created)
	}
	if created.Type != "学习资料" {
		t.Fatalf("expected material type to use unified naming, got %#v", created)
	}
	for index := range store.materials {
		if store.materials[index].ID == created.ID {
			store.materials[index].Status = learning.Status("已发布")
			store.materials[index].PublishStatus = "已发布"
		}
	}
	materials := store.Materials(teacher)
	if !materialHasLearningDimensions(materials, created.ID) {
		t.Fatalf("expected admin material list to include learning dimensions, got %#v", materials)
	}

	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	before, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("expected student study board: %v", err)
	}
	if before.Student.ID != "stu-001" || len(before.Student.OpenedPackages) == 0 {
		t.Fatalf("expected study board to include student package state, got %#v", before.Student)
	}
	if !materialVisible(before.Materials, created.ID) {
		t.Fatalf("expected created material to be visible to student: %#v", before.Materials)
	}
	if !materialHasLearningDimensions(before.Materials, created.ID) {
		t.Fatalf("expected student material to include learning dimensions, got %#v", before.Materials)
	}
	favorite, err := store.AddFavorite("小明", student, learning.FavoriteRequest{
		TargetType: "material",
		TargetID:   created.ID,
	})
	if err != nil {
		t.Fatalf("expected visible material to be favorited: %v", err)
	}
	favorites, err := store.StudentFavorites(student)
	if err != nil {
		t.Fatalf("expected favorites before disable: %v", err)
	}
	if !favoriteListContains(favorites, favorite.ID) {
		t.Fatalf("expected visible material favorite to be returned, got %#v", favorites)
	}

	if _, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "草稿学习资料",
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
	favoritesAfterDraft, err := store.StudentFavorites(student)
	if err != nil {
		t.Fatalf("expected favorites after draft: %v", err)
	}
	if favoriteListContains(favoritesAfterDraft, favorite.ID) {
		t.Fatalf("expected draft material favorite to be hidden from student: %#v", favoritesAfterDraft)
	}
	adminAfterDraft := store.Materials(teacher)
	if !materialVisible(adminAfterDraft, created.ID) {
		t.Fatalf("expected draft material to remain visible in admin list: %#v", adminAfterDraft)
	}

	if _, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "停用学习资料",
		CourseID: courses[0].ID,
		Chapter:  "第一章",
		Status:   learning.StatusDisabled,
	}); err != nil {
		t.Fatalf("expected material disable to succeed: %v", err)
	}
	disabledStudy, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("expected student study board after disable: %v", err)
	}
	if materialVisible(disabledStudy.Materials, created.ID) {
		t.Fatalf("expected disabled material to be hidden from student: %#v", disabledStudy.Materials)
	}
	favoritesAfterDisable, err := store.StudentFavorites(student)
	if err != nil {
		t.Fatalf("expected favorites after disable: %v", err)
	}
	if favoriteListContains(favoritesAfterDisable, favorite.ID) {
		t.Fatalf("expected disabled material favorite to be hidden from student: %#v", favoritesAfterDisable)
	}
	adminAfterDisable := store.Materials(teacher)
	if !materialVisible(adminAfterDisable, created.ID) {
		t.Fatalf("expected disabled material to remain visible in admin list: %#v", adminAfterDisable)
	}

	published, err := store.UpdateMaterial("英语老师", teacher, created.ID, learning.MaterialUpdateRequest{
		Title:    "已发布学习资料",
		CourseID: courses[0].ID,
		Chapter:  "第一章",
		Status:   learning.Status("已发布"),
	})
	if err != nil {
		t.Fatalf("expected published material status to be accepted: %v", err)
	}
	if published.PublishStatus != "已发布" || published.Status != learning.StatusEnabled {
		t.Fatalf("expected published material to normalize to enabled internally, got %#v", published)
	}
	visibleAgain, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("expected student study board after republish: %v", err)
	}
	if !materialVisible(visibleAgain.Materials, created.ID) {
		t.Fatalf("expected republished material to be visible to student: %#v", visibleAgain.Materials)
	}
	favoritesAfterRepublish, err := store.StudentFavorites(student)
	if err != nil {
		t.Fatalf("expected favorites after republish: %v", err)
	}
	if !favoriteListContains(favoritesAfterRepublish, favorite.ID) {
		t.Fatalf("expected republished material favorite to be visible again, got %#v", favoritesAfterRepublish)
	}
}

// 上传错了、传重了这类误操作，之前唯一的补救是编辑成"停用"——但列表的
// 状态列显示的是转换状态不是发布状态，停用了列表上完全看不出来，运营
// 只会觉得"这功能没用"。这条测试锁的是删除的真实效果：从管理端列表和
// 学生端都消失，且不留一个点开 404 的收藏项。
func TestDeleteMaterialRemovesItFromAdminAndStudentAndCleansFavorites(t *testing.T) {
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
		Title:    "待删除学习资料",
		CourseID: courses[0].ID,
		File: learning.FileAsset{
			ID:            "file-delete-test",
			FileName:      "material.pdf",
			FileType:      "PDF",
			PreviewStatus: "可预览",
		},
	})
	if err != nil {
		t.Fatalf("expected material creation to succeed: %v", err)
	}
	for index := range store.materials {
		if store.materials[index].ID == created.ID {
			store.materials[index].Status = learning.Status("已发布")
			store.materials[index].PublishStatus = "已发布"
		}
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	favorite, err := store.AddFavorite("小明", student, learning.FavoriteRequest{TargetType: "material", TargetID: created.ID})
	if err != nil {
		t.Fatalf("expected visible material to be favorited: %v", err)
	}

	if err := store.DeleteMaterial("英语老师", teacher, "material-does-not-exist"); err == nil {
		t.Fatal("expected deleting a nonexistent material to fail")
	}

	if err := store.DeleteMaterial("英语老师", teacher, created.ID); err != nil {
		t.Fatalf("expected delete to succeed: %v", err)
	}

	adminAfterDelete := store.Materials(teacher)
	if materialVisible(adminAfterDelete, created.ID) {
		t.Fatalf("expected deleted material to disappear from admin list, got %#v", adminAfterDelete)
	}
	studyAfterDelete, err := store.StudentStudy(student)
	if err != nil {
		t.Fatalf("expected student study board after delete: %v", err)
	}
	if materialVisible(studyAfterDelete.Materials, created.ID) {
		t.Fatalf("expected deleted material to disappear from student view, got %#v", studyAfterDelete.Materials)
	}
	favoritesAfterDelete, err := store.StudentFavorites(student)
	if err != nil {
		t.Fatalf("expected favorites after delete: %v", err)
	}
	if favoriteListContains(favoritesAfterDelete, favorite.ID) {
		t.Fatalf("expected the dangling favorite to be cleaned up, got %#v", favoritesAfterDelete)
	}

	if err := store.DeleteMaterial("英语老师", teacher, created.ID); err == nil {
		t.Fatal("expected deleting an already-deleted material to fail")
	}
}

// 重复排课应展开成一节一条的课次，而不是压成一条记录。
func TestCreateRepeatingScheduleExpandsIntoLessons(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	req := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-03",
		StudentIDs:      []string{"stu-001"},
		Repeat:          &learning.ScheduleRepeat{Freq: "weekly", Interval: 1, Count: 4},
	}
	first, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected repeating schedule to succeed: %v", err)
	}
	if first.SeriesID == "" {
		t.Fatal("重复排课应带上 SeriesID，否则后续无法按系列批量调整")
	}
	lessons := seriesLessons(store, first.SeriesID)
	if len(lessons) != 4 {
		t.Fatalf("每周一节共 4 节，应生成 4 条课次，实际 %d", len(lessons))
	}
	want := []string{"2026-06-03", "2026-06-10", "2026-06-17", "2026-06-24"}
	for i, lesson := range lessons {
		if lesson.LessonDate != want[i] {
			t.Fatalf("第 %d 节应在 %s，实际 %s", i+1, want[i], lesson.LessonDate)
		}
		if lesson.StartDate != lesson.LessonDate || lesson.EndDate != lesson.LessonDate {
			t.Fatalf("课次的起止日期必须等于上课日期，实际 %#v", lesson)
		}
	}
}

// 整批校验通过才落库：任何一节排不下，整批都不能留在课表里。
func TestCreateRepeatingScheduleIsAllOrNothing(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	base := learning.ScheduleClassCreateRequest{
		CourseID:        "course-g05-english-s1-q1",
		TeacherID:       "user-teacher",
		CampusID:        "campus-main",
		ClassType:       "1V1",
		DurationMinutes: 90,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-17",
		StudentIDs:      []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, base); err != nil {
		t.Fatalf("expected single lesson to succeed: %v", err)
	}
	before := len(store.scheduleClasses)

	// 6/17 已经有课，这一批的第三节必然撞上。
	repeating := base
	repeating.StartDate = "2026-06-03"
	repeating.Repeat = &learning.ScheduleRepeat{Freq: "weekly", Interval: 1, Count: 4}
	if _, err := store.CreateScheduleClass("运营教务", ops, repeating); err == nil {
		t.Fatal("批次内有一节撞课时必须整批拒绝")
	}
	if len(store.scheduleClasses) != before {
		t.Fatalf("整批拒绝后不应残留半截课表，课次数从 %d 变成 %d", before, len(store.scheduleClasses))
	}
}

// 这是重构要修掉的现网问题：拖动重复课程里的某一节，
// 不能把整个系列一起挪走。
func TestUpdateScopeThisOnlyMovesOneLesson(t *testing.T) {
	store, ops, seriesID := seedWeeklySeries(t)
	lessons := seriesLessons(store, seriesID)
	target := lessons[1] // 2026-06-10

	req := lessonUpdateRequest(target)
	req.StartDate = seriesDatePlusDays(1, 1)
	req.EditScope = learning.EditScopeThis
	req.IgnoreWarnings = true
	moved, err := store.UpdateScheduleClass("运营教务", ops, target.ID, req)
	if err != nil {
		t.Fatalf("expected single lesson move to succeed: %v", err)
	}
	if moved.LessonDate != seriesDatePlusDays(1, 1) {
		t.Fatalf("这一节应挪到 %s，实际 %s", seriesDatePlusDays(1, 1), moved.LessonDate)
	}
	if !moved.Detached {
		t.Fatal("单独改过的课次应标记为已脱离系列")
	}
	after := seriesLessons(store, seriesID)
	want := []string{seriesDate(0), seriesDatePlusDays(1, 1), seriesDate(2), seriesDate(3)}
	for i, lesson := range after {
		if lesson.LessonDate != want[i] {
			t.Fatalf("只应挪动一节，第 %d 节却变成了 %s（期望 %s）", i+1, lesson.LessonDate, want[i])
		}
	}
}

// 「此课次及后续」按整体平移处理，且不碰这节之前的课次。
func TestUpdateScopeThisAndFutureShiftsRemainingLessons(t *testing.T) {
	store, ops, seriesID := seedWeeklySeries(t)
	lessons := seriesLessons(store, seriesID)
	target := lessons[1] // 2026-06-10

	req := lessonUpdateRequest(target)
	req.StartDate = seriesDatePlusDays(1, 1)
	req.EditScope = learning.EditScopeThisAndFuture
	req.IgnoreWarnings = true
	if _, err := store.UpdateScheduleClass("运营教务", ops, target.ID, req); err != nil {
		t.Fatalf("expected series shift to succeed: %v", err)
	}
	after := seriesLessons(store, seriesID)
	// 后三节整体 +1 天，第一节不动。
	want := []string{seriesDate(0), seriesDatePlusDays(1, 1), seriesDatePlusDays(2, 1), seriesDatePlusDays(3, 1)}
	for i, lesson := range after {
		if lesson.LessonDate != want[i] {
			t.Fatalf("第 %d 节应为 %s，实际 %s", i+1, want[i], lesson.LessonDate)
		}
	}
}

// 已经单独调整过的课次不再跟随系列的批量改动。
func TestSeriesUpdateSkipsDetachedLesson(t *testing.T) {
	store, ops, seriesID := seedWeeklySeries(t)
	lessons := seriesLessons(store, seriesID)

	detachReq := lessonUpdateRequest(lessons[2])
	detachReq.StartDate = seriesDatePlusDays(2, 2)
	detachReq.EditScope = learning.EditScopeThis
	detachReq.IgnoreWarnings = true
	if _, err := store.UpdateScheduleClass("运营教务", ops, lessons[2].ID, detachReq); err != nil {
		t.Fatalf("expected detach to succeed: %v", err)
	}

	shiftReq := lessonUpdateRequest(lessons[0])
	shiftReq.StartDate = seriesDatePlusDays(0, 1)
	shiftReq.EditScope = learning.EditScopeAll
	shiftReq.IgnoreWarnings = true
	if _, err := store.UpdateScheduleClass("运营教务", ops, lessons[0].ID, shiftReq); err != nil {
		t.Fatalf("expected series shift to succeed: %v", err)
	}

	after := seriesLessons(store, seriesID)
	// 未脱离的三节整体 +1 天；已脱离的那节停在自己被挪到的位置上。
	want := []string{seriesDatePlusDays(0, 1), seriesDatePlusDays(1, 1), seriesDatePlusDays(2, 2), seriesDatePlusDays(3, 1)}
	for i, lesson := range after {
		if lesson.LessonDate != want[i] {
			t.Fatalf("第 %d 节应为 %s，实际 %s（已脱离的那节不应跟着平移）", i+1, want[i], lesson.LessonDate)
		}
	}
}

// 重复课次必须显式指定修改范围。不给就报错，而不是替用户挑一个——
// 「拖一节课挪走整学期」正是因为以前没有这个必答项。
func TestUpdateRepeatingLessonRequiresExplicitScope(t *testing.T) {
	store, ops, seriesID := seedWeeklySeries(t)
	lessons := seriesLessons(store, seriesID)

	req := lessonUpdateRequest(lessons[1])
	req.StartDate = seriesDatePlusDays(1, 1)
	req.IgnoreWarnings = true
	_, err := store.UpdateScheduleClass("运营教务", ops, lessons[1].ID, req)
	if err == nil || !strings.Contains(err.Error(), "请选择修改范围") {
		t.Fatalf("重复课次未指定修改范围时必须拒绝，实际 %v", err)
	}
}

// 单次课没有系列，不该逼用户选范围。
func TestUpdateSingleLessonNeedsNoScope(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	created, err := store.CreateScheduleClass("运营教务", ops, learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 90, StartTime: "19:00", EndTime: "20:30",
		StartDate: "2026-06-03", StudentIDs: []string{"stu-001"},
	})
	if err != nil {
		t.Fatalf("expected single lesson to succeed: %v", err)
	}
	req := lessonUpdateRequest(created)
	req.StartDate = "2026-06-10"
	if _, err := store.UpdateScheduleClass("运营教务", ops, created.ID, req); err != nil {
		t.Fatalf("单次课不该要求修改范围: %v", err)
	}
}

// 可上课时间是软提醒：默认仍然拦，确认后放行并留痕。
func TestAvailabilityIsSoftWarningWithOverrideTrail(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	// 2026-06-04 是周四，种子里没有任何人填报过周四的可上课时间。
	req := learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 90, StartTime: "19:00", EndTime: "20:30",
		StartDate: "2026-06-04", StudentIDs: []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err == nil {
		t.Fatal("未确认时越出可上课时间仍应拦下")
	}

	req.IgnoreWarnings = true
	created, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("确认后应放行（管理员可能已线下约好时间）: %v", err)
	}
	if created.OverrideNote == "" || !strings.Contains(created.OverrideNote, "可上课时间") {
		t.Fatalf("越界必须留痕，实际 OverrideNote=%q", created.OverrideNote)
	}
}

// 「整个系列」和「此课次及后续」都只动未来的课次，边界是当天。
// 所以用例里的日期必须相对今天算，不能写死——写死的未来日期迟早会变成过去，
// 用例会在某天毫无征兆地开始失败。
func seriesBaseDate() time.Time {
	return time.Now().AddDate(0, 0, 30)
}

func seriesDate(weekOffset int) string {
	return seriesBaseDate().AddDate(0, 0, 7*weekOffset).Format("2006-01-02")
}

func seriesDatePlusDays(weekOffset, days int) string {
	return seriesBaseDate().AddDate(0, 0, 7*weekOffset+days).Format("2006-01-02")
}

func seedWeeklySeries(t *testing.T) (*MemoryStore, learning.Principal, string) {
	t.Helper()
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	// IgnoreWarnings：这些日期落在种子可上课时间窗口之外，
	// 这里要验的是系列调整语义，不是可上课时间匹配。
	first, err := store.CreateScheduleClass("运营教务", ops, learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 90, StartTime: "19:00", EndTime: "20:30",
		StartDate: seriesDate(0), StudentIDs: []string{"stu-001"}, IgnoreWarnings: true,
		Repeat: &learning.ScheduleRepeat{Freq: "weekly", Interval: 1, Count: 4},
	})
	if err != nil {
		t.Fatalf("expected weekly series to succeed: %v", err)
	}
	return store, ops, first.SeriesID
}

// seriesLessons 按上课日期升序返回一个系列的全部课次。
func seriesLessons(store *MemoryStore, seriesID string) []learning.ScheduleClass {
	out := make([]learning.ScheduleClass, 0, 8)
	for _, item := range store.scheduleClasses {
		if item.SeriesID == seriesID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LessonDate < out[j].LessonDate })
	return out
}

func lessonUpdateRequest(lesson learning.ScheduleClass) learning.ScheduleClassCreateRequest {
	studentIDs := make([]string, 0, len(lesson.Students))
	for _, student := range lesson.Students {
		studentIDs = append(studentIDs, student.ID)
	}
	return learning.ScheduleClassCreateRequest{
		CourseID:        lesson.CourseID,
		TeacherID:       lesson.TeacherID,
		CampusID:        lesson.CampusID,
		RoomName:        lesson.RoomName,
		ClassType:       lesson.ClassType,
		DurationMinutes: lesson.DurationMinutes,
		StartTime:       lesson.StartTime,
		EndTime:         lesson.EndTime,
		StartDate:       lesson.LessonDate,
		StudentIDs:      studentIDs,
	}
}
