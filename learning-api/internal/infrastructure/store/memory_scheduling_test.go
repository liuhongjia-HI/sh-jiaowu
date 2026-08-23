package store

import (
	"strings"
	"testing"

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
		DayOfWeek:            3,
		StartTime:            "19:00",
		EndTime:              "20:30",
		StartDate:            "2026-06-01",
		EndDate:              "2026-08-31",
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
		DayOfWeek:       3,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
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
		DayOfWeek:       3,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-10",
		EndDate:         "2026-07-10",
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
	req2.StartDate = "2026-07-20"
	req2.EndDate = "2026-08-20"
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
		DayOfWeek:       3,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
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
		DayOfWeek:       4,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
		StudentIDs:      []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err == nil || !strings.Contains(err.Error(), "老师该时间不可授课") {
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
		DayOfWeek:       3,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
		StudentIDs:      []string{"stu-001"},
	}
	if _, err := store.CreateScheduleClass("运营教务", ops, req); err != nil {
		t.Fatalf("expected summer schedule to succeed: %v", err)
	}

	fallReq := req
	fallReq.StartDate = "2026-09-01"
	fallReq.EndDate = "2026-12-31"
	if _, err := store.CreateScheduleClass("运营教务", ops, fallReq); err != nil {
		t.Fatalf("expected same time outside date range to be allowed: %v", err)
	}

	overlapReq := req
	overlapReq.StartDate = "2026-08-01"
	overlapReq.EndDate = "2026-09-30"
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
		DayOfWeek:       3,
		StartTime:       "19:00",
		EndTime:         "20:30",
		StartDate:       "2026-06-01",
		EndDate:         "2026-08-31",
		StudentIDs:      []string{"stu-001"},
	}
	created, err := store.CreateScheduleClass("运营教务", ops, req)
	if err != nil {
		t.Fatalf("expected schedule creation to succeed: %v", err)
	}
	assertScheduleNotice(t, findScheduleOfficialNotice(t, store.notices, created.ID, "课程已安排"), created.ID, "课程已安排", "周三")

	req.DayOfWeek = 6
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
