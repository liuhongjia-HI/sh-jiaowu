package store

import (
	"strings"
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

func TestStudentRecommendationsExcludeOpenedAndRankEligiblePackages(t *testing.T) {
	store := NewMemoryStore()
	principal, err := store.PrincipalByUserID("user-student-003")
	if err != nil {
		t.Fatalf("load student principal: %v", err)
	}

	activePackageID := packageID(4, "数学", 0, "full")
	store.grants = append(store.grants, packageGrant{
		ID: "grant-recommendation-active", StudentID: "stu-003", PackageID: activePackageID,
		StartsAt: "2026-01-01", EndsAt: "2027-01-01", EffectiveUntil: "2027-01-01", Status: "active",
	})
	store.syncSpaceAccessForGrant(store.grants[len(store.grants)-1])
	store.packages = append(store.packages,
		learning.Package{ID: "pkg-recommend-disabled", Name: "停用课程套餐", AcademicYear: "2025.2026学年", Grade: "五年级", Semester: "S1", Subject: "英语", Status: learning.StatusDisabled},
		learning.Package{ID: "pkg-recommend-empty", Name: "空课程套餐", AcademicYear: "2025.2026学年", Grade: "五年级", Semester: "S1", Subject: "科学", Status: learning.StatusEnabled},
	)
	store.learningSpaces = append(store.learningSpaces, learningSpace{ID: "space-g05-science-s1-q1", AcademicYear: "2025.2026学年", Grade: "五年级", Semester: "S1", Subject: "科学", Phase: "Q1 期中", Name: "五年级科学S1Q1期中", Status: learning.StatusEnabled})
	store.packageSpaces = append(store.packageSpaces,
		packageSpace{PackageID: "pkg-recommend-disabled", LearningSpaceID: "space-g05-english-s1-q1"},
		packageSpace{PackageID: "pkg-recommend-empty", LearningSpaceID: "space-g05-science-s1-q1"},
	)
	store.contentTypes = append(store.contentTypes,
		packageContentType{PackageID: "pkg-recommend-disabled", ContentType: "course"},
		packageContentType{PackageID: "pkg-recommend-empty", ContentType: "course"},
	)

	recommendations, err := store.StudentRecommendations(principal)
	if err != nil {
		t.Fatalf("student recommendations: %v", err)
	}
	if len(recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	if len(recommendations) > 3 {
		t.Fatalf("expected at most 3 recommendations, got %d", len(recommendations))
	}
	if !recommendations[0].SameLearningSpace {
		t.Fatalf("expected same-space recommendation first, got %#v", recommendations[0])
	}
	for _, item := range recommendations {
		if item.PackageID == packageID(4, "英语", 0, "question_handout") || item.PackageID == activePackageID {
			t.Fatalf("active package must not be recommended: %#v", item)
		}
		if item.PackageID == packageID(4, "语文", 0, "question") {
			t.Fatalf("question-only package must not be recommended: %#v", item)
		}
		if item.PackageID == "pkg-recommend-disabled" || item.PackageID == "pkg-recommend-empty" {
			t.Fatalf("disabled or empty package must not be recommended: %#v", item)
		}
		if item.CourseCount+item.MaterialCount == 0 {
			t.Fatalf("empty package must not be recommended: %#v", item)
		}
	}

	for index := range store.grants {
		if store.grants[index].ID == "grant-recommendation-active" {
			store.grants[index].EndsAt = "2025-01-01"
			store.grants[index].EffectiveUntil = "2025-01-01"
		}
	}
	recommendations, err = store.StudentRecommendations(principal)
	if err != nil {
		t.Fatalf("student recommendations after expiry: %v", err)
	}
	if !hasRecommendation(recommendations, activePackageID) {
		t.Fatalf("expired package should be recommendable again, got %#v", recommendations)
	}
}

func hasRecommendation(items []learning.StudentPackageRecommendation, packageID string) bool {
	for _, item := range items {
		if item.PackageID == packageID {
			return true
		}
	}
	return false
}

func TestLoginWithWechatCodeBindsStudentByPhone(t *testing.T) {
	store := NewMemoryStore()

	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "new-openid"}); err == nil || !strings.Contains(err.Error(), "微信账号未绑定") {
		t.Fatalf("expected unbound wechat login to require phone binding, got %v", err)
	}

	student, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "student", Phone: "18500009069", StudentName: "小明", SchoolName: "星河小学", Grade: "五年级"})
	if err != nil {
		t.Fatalf("expected phone binding to succeed: %v", err)
	}
	if student.UserID != "user-student-001" || !hasRole(student.Roles, learning.RoleStudent) {
		t.Fatalf("unexpected student principal: %#v", student)
	}

	again, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "student"})
	if err != nil {
		t.Fatalf("expected bound wechat login to succeed: %v", err)
	}
	if again.UserID != student.UserID {
		t.Fatalf("expected same student after binding, got %#v", again)
	}

	retry, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "student", Phone: "18500009069", StudentName: "小明", SchoolName: "星河小学", Grade: "五年级"})
	if err != nil {
		t.Fatalf("expected repeated binding from same wechat to be idempotent: %v", err)
	}
	if retry.UserID != student.UserID {
		t.Fatalf("expected repeated binding to return same student, got %#v", retry)
	}
}

func TestWechatStudentBindingValidatesProfile(t *testing.T) {
	store := NewMemoryStore()
	for i := range store.students {
		if store.students[i].ID == "stu-001" {
			store.students[i].SchoolName = "星河小学"
		}
	}

	cases := []struct {
		name string
		req  learning.WechatLoginRequest
		want string
	}{
		{
			name: "wrong name",
			req:  learning.WechatLoginRequest{Code: "student-a", Phone: "18500009069", StudentName: "小红", Grade: "五年级", SchoolName: "星河小学"},
			want: "学生姓名与后台档案不一致",
		},
		{
			name: "wrong grade",
			req:  learning.WechatLoginRequest{Code: "student-b", Phone: "18500009069", StudentName: "小明", Grade: "四年级", SchoolName: "星河小学"},
			want: "年级与后台档案不一致",
		},
		{
			name: "wrong school",
			req:  learning.WechatLoginRequest{Code: "student-c", Phone: "18500009069", StudentName: "小明", Grade: "五年级", SchoolName: "另一所学校"},
			want: "学校与后台档案不一致",
		},
		{
			name: "missing school",
			req:  learning.WechatLoginRequest{Code: "student-d", Phone: "18500009069", StudentName: "小明", Grade: "五年级"},
			want: "请填写学生姓名、学校和年级",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.LoginWithWechatCode(tt.req); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}

	student, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "student", Phone: "18500009069", StudentName: "小明", Grade: "五年级", SchoolName: "星河小学",
	})
	if err != nil {
		t.Fatalf("expected verified binding to succeed: %v", err)
	}
	if student.UserID != "user-student-001" {
		t.Fatalf("unexpected student after verified binding: %#v", student)
	}
	stored, ok := store.findRawStudent("stu-001")
	if !ok || stored.SchoolName != "星河小学" || stored.BindStatus != "已绑定" {
		t.Fatalf("expected binding profile to be persisted, got %#v", stored)
	}
}

func TestWechatStudentBindingCreatesStudentForUnmatchedPhone(t *testing.T) {
	store := NewMemoryStore()
	user, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "missing-phone", Phone: "19900000000", StudentName: "小明", SchoolName: "星河小学", Grade: "五年级",
	})
	if err != nil {
		t.Fatalf("expected unmatched phone to create a student account: %v", err)
	}
	if user.StudentID == "" || user.Phone != "19900000000" || !hasRole(user.Roles, learning.RoleStudent) {
		t.Fatalf("unexpected principal for new wechat student: %#v", user)
	}
	stored, ok := store.findRawStudent(user.StudentID)
	if !ok {
		t.Fatalf("expected created student %q to be persisted in store", user.StudentID)
	}
	if stored.Name != "小明" || stored.SchoolName != "星河小学" || stored.Grade != "五年级" || stored.LearningStatus != "待开通" || stored.BindStatus != "已绑定" {
		t.Fatalf("unexpected created student: %#v", stored)
	}
	home, err := store.StudentHome(user)
	if err != nil {
		t.Fatalf("expected newly authorized student to enter mini program home: %v", err)
	}
	if home.Student.ID != user.StudentID || len(home.Materials) != 0 || len(home.PendingHomework) != 0 {
		t.Fatalf("new student should enter without learning permissions, got %#v", home)
	}
	again, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "missing-phone"})
	if err != nil {
		t.Fatalf("expected repeated login with same wechat to succeed: %v", err)
	}
	if again.StudentID != user.StudentID {
		t.Fatalf("expected repeated login to return the same student, got %#v", again)
	}
}

func TestWechatStudentBindingUsesExistingMaskedPhoneStudent(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	spaceID := "space-g05-english-s1-q1"
	store.students = []learning.Student{{
		ID:             "stu-acceptance",
		Name:           "王同学",
		Grade:          "五年级",
		Phone:          "185****3993",
		SchoolName:     "星河小学",
		AccountStatus:  "正常",
		LearningStatus: "已开通",
		BindStatus:     "待绑定",
	}}
	store.packages = []learning.Package{{
		ID: "pkg-acceptance", Name: "验收五年级英语套餐", AcademicYear: "2025.2026学年",
		Grade: "五年级", Semester: "上学期", Subject: "英语", PhaseScope: "第一阶段",
		PackageType: "课程+资料+练习", Status: learning.StatusEnabled,
	}}
	store.packageSpaces = []packageSpace{{PackageID: "pkg-acceptance", LearningSpaceID: spaceID}}
	store.contentTypes = []packageContentType{
		{PackageID: "pkg-acceptance", ContentType: "course"},
		{PackageID: "pkg-acceptance", ContentType: "handout"},
		{PackageID: "pkg-acceptance", ContentType: "question"},
	}
	store.grants = []packageGrant{{
		ID: "grant-acceptance", StudentID: "stu-acceptance", PackageID: "pkg-acceptance",
		StartsAt: "2026-07-01", EndsAt: "2027-07-01", Status: "active",
	}}
	store.syncSpaceAccessForGrant(store.grants[0])
	store.courses = []learning.Course{{
		ID: "course-acceptance", Name: "验收五年级英语课程-07110752", Grade: "五年级", Subject: "英语",
		LearningSpaceID: spaceID, Status: learning.StatusEnabled,
	}}
	store.materials = []learning.Material{{
		ID: "mat-acceptance", Title: "验收英语资料-07110806", CourseID: "course-acceptance",
		LearningSpaceID: spaceID, PublishStatus: "已发布", Status: learning.StatusEnabled,
	}}
	store.homework = []learning.Homework{{
		ID: "hw-acceptance", Title: "验收英语小练习-07110752", CourseID: "course-acceptance",
		LearningSpaceID: spaceID, PublishStatus: "已发布", Status: string(learning.StatusEnabled),
	}}
	studentCount := len(store.students)

	principal, err := store.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "acceptance-openid", Phone: "18500003993", StudentName: "王同学", SchoolName: "星河小学", Grade: "五年级",
	})
	if err != nil {
		t.Fatalf("expected masked phone student to bind with real wechat phone: %v", err)
	}
	if principal.StudentID != "stu-acceptance" || principal.UserID != "user-stu-acceptance" || principal.Phone != "18500003993" {
		t.Fatalf("expected original student principal with real phone, got %#v", principal)
	}
	if len(store.students) != studentCount {
		t.Fatalf("masked phone binding should not create duplicate students, got %d want %d", len(store.students), studentCount)
	}
	stored, ok := store.findRawStudent("stu-acceptance")
	if !ok || stored.BindStatus != "已绑定" || stored.Phone != "185****3993" {
		t.Fatalf("expected original masked student to be bound without replacing archived phone, got %#v", stored)
	}
	user, ok := store.findUserByStudentID("stu-acceptance")
	if !ok || user.OpenID != "demo-acceptance-openid" || user.Phone != "18500003993" {
		t.Fatalf("expected user to be created with real phone and openid, got %#v", user)
	}
	home, err := store.StudentHome(principal)
	if err != nil {
		t.Fatalf("expected original opened student to enter home: %v", err)
	}
	if home.Student.ID != "stu-acceptance" || home.ContinueCourse.Name != "验收五年级英语课程-07110752" {
		t.Fatalf("expected original opened package course on home, got %#v", home)
	}
	if len(home.Materials) != 1 || home.Materials[0].Title != "验收英语资料-07110806" {
		t.Fatalf("expected original opened material on home, got %#v", home.Materials)
	}
	if len(home.PendingHomework) != 1 || home.PendingHomework[0].Title != "验收英语小练习-07110752" {
		t.Fatalf("expected original opened homework on home, got %#v", home.PendingHomework)
	}
}

func TestWechatStudentBindingRejectsDuplicatePhone(t *testing.T) {
	duplicateStore := NewMemoryStore()
	var duplicate learning.User
	for _, user := range duplicateStore.users {
		if user.ID == "user-student-001" {
			duplicate = user
			break
		}
	}
	duplicate.ID = "user-duplicate-phone"
	duplicate.StudentID = "stu-duplicate-phone"
	duplicate.OpenID = ""
	duplicateStore.users = append(duplicateStore.users, duplicate)
	if _, err := duplicateStore.LoginWithWechatCode(learning.WechatLoginRequest{
		Code: "duplicate-phone", Phone: duplicate.Phone, StudentName: "任意学生", SchoolName: "星河小学", Grade: "五年级",
	}); err == nil || !strings.Contains(err.Error(), "手机号匹配到多个账号") {
		t.Fatalf("expected duplicate phone to be rejected, got %v", err)
	}
}

func TestOperationLogIDsAreUniqueForRapidWrites(t *testing.T) {
	ids := map[string]bool{}
	for i := 0; i < 200; i++ {
		id := newOperationLogID()
		if ids[id] {
			t.Fatalf("operation log id should be unique, got duplicate %q", id)
		}
		ids[id] = true
		if !strings.HasPrefix(id, "log-") {
			t.Fatalf("operation log id should keep log prefix, got %q", id)
		}
	}
}

func TestUpdateStudentProfile(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	basic, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{
		StudentName:  " 小明 ",
		Grade:        " 五年级 ",
		SchoolName:   " 星光小学 ",
		GuardianName: " 小明妈妈 ",
	})
	if err != nil {
		t.Fatalf("expected basic profile update to succeed: %v", err)
	}
	if basic.Name != "小明" || basic.Grade != "五年级" || basic.SchoolName != "星光小学" || basic.GuardianName != "小明妈妈" {
		t.Fatalf("unexpected basic profile: %#v", basic)
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
	if updated.SchoolName != "星光小学" {
		t.Fatalf("profile update should keep existing school, got %#v", updated)
	}
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].SchoolName = ""
		}
	}
	if _, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{
		StudentName: "小明",
		Grade:       "五年级",
	}); err == nil || !strings.Contains(err.Error(), "请填写学校") {
		t.Fatalf("expected empty school to be rejected, got %v", err)
	}

	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if home.Student.Nickname != updated.Nickname || home.Student.AvatarURL != updated.AvatarURL {
		t.Fatalf("expected student home to include profile, got %#v", home.Student)
	}
}

func TestDashboardUsesRealProductionCounts(t *testing.T) {
	store := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	store.students = []learning.Student{{ID: "stu-prod", Name: "王同学", Grade: "五年级", Phone: "18500003993", AccountStatus: "正常"}}
	store.packages = []learning.Package{{ID: "pkg-prod", Name: "验收套餐", Grade: "五年级", Status: learning.StatusEnabled}}
	store.grants = []packageGrant{{ID: "grant-prod", StudentID: "stu-prod", PackageID: "pkg-prod", StartsAt: "2026-07-01", EndsAt: "2027-07-01", Status: "active"}}
	store.materials = []learning.Material{{ID: "mat-prod", Title: "验收资料", ViewCount: 7, PublishStatus: "已发布", Status: learning.StatusEnabled}}
	store.homework = []learning.Homework{{ID: "hw-prod", Title: "验收练习", PublishStatus: "已发布", Status: string(learning.StatusEnabled)}}
	store.reviews = []learning.Review{{ID: "rev-prod", Status: "待批改"}}

	overview := store.Dashboard()
	if overview.OpenedStudents != 1 || overview.PackageCount != 1 || overview.PendingReviews != 1 || overview.MaterialViews != 7 || overview.UnpublishedFiles != 0 {
		t.Fatalf("dashboard should use real counts, got %#v", overview)
	}
}

func TestStudentsIncludeLatestSubmissionStatus(t *testing.T) {
	store := NewMemoryStore()
	store.submissions["sub-old"] = learning.Submission{
		ID:         "sub-old",
		HomeworkID: "hw-g05-english-s1-q1",
		StudentID:  "stu-001",
		TaskTitle:  "旧练习",
		Status:     "待批改",
		CreatedAt:  "2026-06-20 19:00:00",
	}
	store.submissions["sub-latest"] = learning.Submission{
		ID:         "sub-latest",
		HomeworkID: "hw-g05-english-s1-q1",
		StudentID:  "stu-001",
		TaskTitle:  "最新练习",
		Status:     "已批改",
		CreatedAt:  "2026-06-24 20:15:00",
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}

	rows := store.Students(ops, learning.StudentQuery{Keyword: "小明"})
	if len(rows) != 1 {
		t.Fatalf("expected one student row, got %#v", rows)
	}
	if rows[0].LastSubmissionStatus != "已批改" || rows[0].LastSubmittedAt != "2026-06-24 20:15:00" {
		t.Fatalf("expected latest submission status on list row, got %#v", rows[0])
	}

	detail, err := store.StudentDetail(ops, "stu-001")
	if err != nil {
		t.Fatalf("expected student detail: %v", err)
	}
	if detail.Student.LastSubmissionStatus != rows[0].LastSubmissionStatus || detail.Student.LastSubmittedAt != rows[0].LastSubmittedAt {
		t.Fatalf("expected detail to include latest submission status, detail=%#v row=%#v", detail.Student, rows[0])
	}
}

func TestStudentScoreRecords(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("expected admin principal: %v", err)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}

	if _, err := store.CreateStudentScore("管理员", admin, "stu-001", learning.StudentScoreUpsertRequest{
		Subject:   "数学",
		ExamName:  "入学测评",
		ExamDate:  "2026-06-01",
		Score:     101,
		FullScore: 100,
	}); err == nil {
		t.Fatal("expected score greater than full score to be rejected")
	}

	first, err := store.CreateStudentScore("管理员", admin, "stu-001", learning.StudentScoreUpsertRequest{
		Subject:        "数学",
		ExamType:       "单元测",
		ExamName:       "入学测评",
		ExamDate:       "2026-06-01",
		Score:          72,
		FullScore:      100,
		AverageScore:   70,
		TeacherComment: "先补计算准确率。",
	})
	if err != nil {
		t.Fatalf("expected first score to be created: %v", err)
	}
	if first.ID == "" || first.StudentID != "stu-001" || first.ExamType != "单元测" {
		t.Fatalf("unexpected first score: %#v", first)
	}
	if _, err := store.CreateStudentScore("管理员", admin, "stu-001", learning.StudentScoreUpsertRequest{
		Subject:   "数学",
		ExamType:  "随堂考",
		ExamName:  "未支持类型",
		ExamDate:  "2026-06-10",
		Score:     80,
		FullScore: 100,
	}); err == nil || !strings.Contains(err.Error(), "考试类型") {
		t.Fatalf("expected invalid exam type to be rejected, got %v", err)
	}
	if _, err := store.CreateStudentScore("学生本人", student, "stu-001", learning.StudentScoreUpsertRequest{
		Subject:   "数学",
		ExamName:  "自填成绩",
		ExamDate:  "2026-06-02",
		Score:     80,
		FullScore: 100,
	}); err == nil {
		t.Fatal("expected student role to be rejected from score entry")
	}
	latest, err := store.CreateStudentScore("管理员", admin, "stu-001", learning.StudentScoreUpsertRequest{
		Subject:        "数学",
		ExamType:       "期中",
		ExamName:       "阶段测评",
		ExamDate:       "2026-06-20",
		Score:          86,
		FullScore:      100,
		AverageScore:   78,
		TeacherComment: `<strong onclick="bad()">继续巩固计算准确率。</strong><script>alert(1)</script><img src="javascript:alert(1)" />`,
	})
	if err != nil {
		t.Fatalf("expected latest score to be created: %v", err)
	}
	if strings.Contains(latest.TeacherComment, "script") || strings.Contains(latest.TeacherComment, "onclick") || strings.Contains(latest.TeacherComment, "javascript:") {
		t.Fatalf("expected teacher comment rich text to be sanitized, got %q", latest.TeacherComment)
	}
	if !strings.Contains(latest.TeacherComment, "<strong>继续巩固计算准确率。</strong>") {
		t.Fatalf("expected safe rich text to be preserved, got %q", latest.TeacherComment)
	}
	_, err = store.CreateStudentScore("管理员", admin, "stu-001", learning.StudentScoreUpsertRequest{
		Subject:        "英语",
		ExamType:       "单元测",
		ExamName:       "阅读测评",
		ExamDate:       "2026-06-05",
		Score:          94,
		FullScore:      100,
		AverageScore:   88,
		TeacherComment: "阅读保持稳定。",
	})
	if err != nil {
		t.Fatalf("expected other subject score to be created: %v", err)
	}

	summaries, err := store.StudentOwnScores(student)
	if err != nil {
		t.Fatalf("expected student scores: %v", err)
	}
	if len(summaries) != 2 || summaries[0].LatestRecord == nil || summaries[0].LatestRecord.ID != latest.ID {
		t.Fatalf("unexpected score summaries: %#v", summaries)
	}
	if summaries[0].Subject != "数学" {
		t.Fatalf("expected most recent subject to be first, got %#v", summaries)
	}
	if summaries[0].LatestRecord.ExamType != "期中" {
		t.Fatalf("expected latest exam type to be preserved, got %#v", summaries[0].LatestRecord)
	}
	if summaries[0].Improvement != 14 || summaries[0].ImprovementPct != 14 {
		t.Fatalf("unexpected improvement: %#v", summaries[0])
	}
	if summaries[0].ProblemPoint == "" || !strings.Contains(summaries[0].ProblemPoint, "数学") {
		t.Fatalf("expected problem point to be generated, got %#v", summaries[0])
	}
	if strings.Contains(summaries[0].Description, "<") || !strings.Contains(summaries[0].Description, "继续巩固计算准确率") {
		t.Fatalf("expected summary description to use plain text advice, got %#v", summaries[0])
	}
	if summaries[0].NextStep != latest.TeacherComment {
		t.Fatalf("expected next step to use teacher comment, got %#v", summaries[0])
	}
	records, err := store.StudentGrowth(student)
	if err != nil {
		t.Fatalf("expected growth records: %v", err)
	}
	foundScore := false
	for _, record := range records {
		if record.Type == "成绩" && record.Score == 86 && record.FullScore == 100 {
			foundScore = true
			break
		}
	}
	if !foundScore {
		t.Fatalf("expected growth to include latest score, got %#v", records)
	}
}

func TestPhoneBindingDoesNotCreateTemporaryStudent(t *testing.T) {
	store := NewMemoryStore()

	userCount := len(store.users)
	studentCount := len(store.students)
	if _, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "student"}); err == nil || !strings.Contains(err.Error(), "微信账号未绑定") {
		t.Fatalf("expected wechat-only login to be rejected before binding, got %v", err)
	}
	if len(store.users) != userCount || len(store.students) != studentCount {
		t.Fatalf("wechat-only login should not create temporary accounts, users=%d/%d students=%d/%d", len(store.users), userCount, len(store.students), studentCount)
	}

	student, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "student", Phone: "18500009069", StudentName: "小明", SchoolName: "星河小学", Grade: "五年级"})
	if err != nil {
		t.Fatalf("expected phone binding into seeded student: %v", err)
	}
	if student.UserID != "user-student-001" {
		t.Fatalf("unexpected student principal after binding: %#v", student)
	}
	again, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "student"})
	if err != nil {
		t.Fatalf("expected bound openID to login: %v", err)
	}
	if again.UserID != "user-student-001" {
		t.Fatalf("expected seeded student after binding, got %#v", again)
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

	student, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "wx-code", Phone: "18500009069", StudentName: "小明", SchoolName: "星河小学", Grade: "五年级"})
	if err != nil {
		t.Fatalf("expected real wechat binding to replace demo openID: %v", err)
	}
	if student.UserID != "user-student-001" {
		t.Fatalf("unexpected student principal: %#v", student)
	}

	again, err := store.LoginWithWechatCode(learning.WechatLoginRequest{Code: "wx-code"})
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
	if submission.StudentID != "stu-001" || submission.HomeworkID != "hw-g05-english-s1-q1" {
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

func TestCompleteReviewCanRouteToRecheckBeforeFinalApproval(t *testing.T) {
	store := NewMemoryStore()

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	before := len(store.Reviews(teacher))
	submission, err := store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          88,
		TeacherComment: "先进入复核，确认建议是否完整。",
		Reward:         "复核星章",
		FinalStatus:    "待复核",
	})
	if err != nil {
		t.Fatalf("expected review recheck to succeed: %v", err)
	}
	if submission.Status != "待复核" {
		t.Fatalf("expected submission to wait for recheck, got %#v", submission)
	}
	reviews := store.Reviews(teacher)
	if len(reviews) != before {
		t.Fatalf("expected recheck review to stay on board, got %d want %d", len(reviews), before)
	}
	var recheck learning.Review
	for _, item := range reviews {
		if item.ID == "rev-001" {
			recheck = item
			break
		}
	}
	if recheck.Status != "待复核" || recheck.SubmissionID != submission.ID || recheck.TeacherComment == "" {
		t.Fatalf("expected review to move to recheck column, got %#v", recheck)
	}
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	result, err := store.StudentSubmission(student, submission.ID)
	if err != nil {
		t.Fatalf("expected student to see recheck feedback: %v", err)
	}
	if result.Status != "待复核" || result.TeacherComment == "" {
		t.Fatalf("unexpected student recheck result: %#v", result)
	}

	final, err := store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          90,
		TeacherComment: "复核通过，按这个方案继续练习。",
		Reward:         "阅读小星星",
		FinalStatus:    "已批改",
	})
	if err != nil {
		t.Fatalf("expected final approval to succeed: %v", err)
	}
	if final.Status != "已批改" {
		t.Fatalf("expected final status, got %#v", final)
	}
	if got := len(store.Reviews(teacher)); got != before-1 {
		t.Fatalf("expected final approval to remove review, got %d want %d", got, before-1)
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
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		Summary:          "只开放 S1 Q1 英语练习。",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}
	if !containsString(pkg.LearningSpaceIDs, "space-g05-english-s1-q1") {
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

func TestCreatePackageRejectsCrossAcademicYearSpace(t *testing.T) {
	store := NewMemoryStore()
	store.learningSpaces = append(store.learningSpaces, learningSpace{
		ID:           "space-g05-english-s1-next-year",
		AcademicYear: "2026.2027学年",
		Grade:        "五年级",
		Semester:     "S1",
		Subject:      "英语",
		Phase:        "Q1",
		Name:         "2026.2027学年 五年级 S1 英语 Q1",
		Status:       learning.StatusEnabled,
	})

	if _, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语跨学年套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-next-year"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	}); err == nil || !strings.Contains(err.Error(), "学年") {
		t.Fatalf("expected cross academic year package space to be rejected, got %v", err)
	}
}

func TestUpdatePackageRefreshesExistingGrantAccess(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语可编辑套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
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
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "全学期",
		PackageType:      "题+学习资料",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1", "space-g05-english-s1-q2"},
		ContentTypeCodes: []string{"question", "handout"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package update to succeed: %v", err)
	}

	spaceIDs := store.studentAccessibleSpaceIDs("stu-001")
	if !containsString(spaceIDs, "space-g05-english-s1-q2") {
		t.Fatalf("expected existing grant access to refresh after package update, got %#v", spaceIDs)
	}
	materials := store.materialsForStudent("stu-001")
	foundFinalMaterial := false
	for _, material := range materials {
		if material.LearningSpaceID == "space-g05-english-s1-q2" {
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
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
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

func TestGrantPreviewRejectsPackageOutsideStudentGrade(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "六年级英语错配套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "六年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g06-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}

	if _, err := store.GrantPreview("stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "不能给五年级学生开通") {
		t.Fatalf("expected grade mismatch preview error, got %v", err)
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "不能给五年级学生开通") {
		t.Fatalf("expected grade mismatch grant error, got %v", err)
	}
}

func TestGrantPreviewRejectsDisabledPackage(t *testing.T) {
	store := NewMemoryStore()

	pkg, err := store.CreatePackage("运营教务", learning.PackageUpsertRequest{
		Name:             "五年级英语停用套餐",
		AcademicYear:     "2025.2026学年",
		Grade:            "五年级",
		Semester:         "S1",
		Subject:          "英语",
		PhaseScope:       "Q1",
		PackageType:      "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"},
		Status:           learning.StatusDisabled,
	})
	if err != nil {
		t.Fatalf("expected package creation to succeed: %v", err)
	}

	if _, err := store.GrantPreview("stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "套餐当前未启用") {
		t.Fatalf("expected disabled package preview error, got %v", err)
	}
	if _, err := store.CreateGrant("运营教务", "stu-001", pkg.ID); err == nil || !strings.Contains(err.Error(), "套餐当前未启用") {
		t.Fatalf("expected disabled package grant error, got %v", err)
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
		LearningSpaceID: "space-g05-english-s1-q1",
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
		LearningSpaceID: "space-g05-math-s1-q1",
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
	updated, err := store.UpdateCourse("超级管理员", admin, "course-g05-english-s1-q1", learning.CourseUpsertRequest{
		Name:            "五年级英语期中阅读精讲课",
		LearningSpaceID: "space-g05-english-s1-q1",
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
		Summary: "今天完成 S1 Q1 阅读挑战。",
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

	phoneNotice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "课",
		Title:   "一对一提醒",
		Target:  "185****9069",
		Summary: "请确认本周课程安排。",
	})
	if err != nil {
		t.Fatalf("expected phone-targeted notice creation to succeed: %v", err)
	}
	home, err = store.StudentHome(englishStudent)
	if err != nil {
		t.Fatalf("expected student home after phone notice: %v", err)
	}
	if !noticeListContains(home.Notices, phoneNotice.ID) {
		t.Fatalf("expected student to see phone-targeted notice, got %#v", home.Notices)
	}
}

func TestStudentHomeExposesMiniProgramSubscribeTemplates(t *testing.T) {
	store := NewMemoryStore()
	store.UseMiniProgramSubscribeTemplates([]string{"vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0", " ", "tpl-review"})
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if !home.SubscriptionReminder.TemplateConfigured || home.SubscriptionReminder.Enabled {
		t.Fatalf("expected configured subscription reminder, got %#v", home.SubscriptionReminder)
	}
	if len(home.SubscriptionReminder.TemplateIDs) != 2 || home.SubscriptionReminder.TemplateIDs[0] != "vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0" || home.SubscriptionReminder.TemplateIDs[1] != "tpl-review" {
		t.Fatalf("expected compact template IDs, got %#v", home.SubscriptionReminder.TemplateIDs)
	}
	todo, ok := findStoreStudentTodo(home.TodayTodos, "subscribe")
	if !ok || todo.ActionText != "开启提醒" || todo.Status != "建议开启" {
		t.Fatalf("expected subscribe todo to be actionable, got %#v", home.TodayTodos)
	}
	reminder, err := store.ConfirmStudentSubscription("小明", student, learning.StudentSubscriptionRequest{TemplateIDs: []string{"vePubb0t7OgxNsZA0J3s60urpzf8_XJjLH4JhPynHd0"}})
	if err != nil {
		t.Fatalf("expected subscription confirmation to succeed: %v", err)
	}
	if !reminder.Enabled || reminder.ActionText != "已开启" {
		t.Fatalf("expected enabled reminder after confirmation, got %#v", reminder)
	}
	home, err = store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home after subscription: %v", err)
	}
	if _, ok := findStoreStudentTodo(home.TodayTodos, "subscribe"); ok {
		t.Fatalf("subscribe todo should be removed after confirmation, got %#v", home.TodayTodos)
	}
}

func TestOfficialAccountNoticeRequiresRecipientAndConfiguration(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	withoutOpenID, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "课",
		Title:   "明天上课提醒",
		Target:  "小明",
		Summary: "明天 19:00 上课。",
		Channel: "公众号模板消息",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if withoutOpenID.Status != "待配置" || !strings.Contains(withoutOpenID.FailureReason, "openid") {
		t.Fatalf("expected missing openid to be tracked, got %#v", withoutOpenID)
	}
	withOpenID, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "明天上课提醒",
		Target:          "小明",
		Summary:         "明天 19:00 上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-001",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if withOpenID.Status != "待配置" || !strings.Contains(withOpenID.FailureReason, "WECHAT_OFFICIAL_ACCOUNT") {
		t.Fatalf("expected missing official account config to be tracked, got %#v", withOpenID)
	}

	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	home, err := store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home: %v", err)
	}
	if noticeListContains(home.Notices, withOpenID.ID) || noticeListContains(home.Notices, withoutOpenID.ID) {
		t.Fatalf("student should not see raw official account notices, got %#v", home.Notices)
	}
	if !noticeListContains(home.Notices, stationNoticeID(withOpenID.ID)) || !noticeListContains(home.Notices, stationNoticeID(withoutOpenID.ID)) {
		t.Fatalf("student should see station history even when official account notice is retryable, got %#v", home.Notices)
	}

	store.officialNoticeSender = func(notice learning.Notice) error { return nil }
	retried, err := store.RetryNotice("运营教务", ops, withOpenID.ID)
	if err != nil {
		t.Fatalf("expected retry to succeed after configuration: %v", err)
	}
	if retried.Status != "已发送" {
		t.Fatalf("expected retried notice to be sent, got %#v", retried)
	}
	home, err = store.StudentHome(student)
	if err != nil {
		t.Fatalf("expected student home after retry: %v", err)
	}
	if noticeListContains(home.Notices, withOpenID.ID) || !noticeListContains(home.Notices, stationNoticeID(withOpenID.ID)) {
		t.Fatalf("student should keep seeing station notice after successful retry, got %#v", home.Notices)
	}
}

func TestOfficialAccountNoticeSendAndRetry(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	sent := 0
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent++
		if notice.RecipientOpenID == "" {
			t.Fatal("expected recipient openid")
		}
		return nil
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "明天上课提醒",
		Target:          "小明",
		Summary:         "明天 19:00 上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-001",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if notice.Status != "已发送" || notice.FailureReason != "" || sent != 1 {
		t.Fatalf("expected sent notice, got notice=%#v sent=%d", notice, sent)
	}
	retried, err := store.RetryNotice("运营教务", ops, notice.ID)
	if err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if retried.Status != "已发送" || retried.RetryCount != 1 || sent != 2 {
		t.Fatalf("expected retried sent notice, got notice=%#v sent=%d", retried, sent)
	}
}

func TestNoticeRecordsVisibleToOpsAndRetryScopeForTeachers(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "明晚提醒",
		Target:          "小明",
		Summary:         "明天 19:00 上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-001",
	})
	if err != nil {
		t.Fatalf("expected ops notice creation to succeed: %v", err)
	}
	if !noticeListContains(store.Notices(ops), notice.ID) {
		t.Fatalf("expected ops to see all notice records, got %#v", store.Notices(ops))
	}
	store.officialNoticeSender = func(notice learning.Notice) error { return nil }
	retried, err := store.RetryNotice("运营教务", ops, notice.ID)
	if err != nil {
		t.Fatalf("expected ops retry to succeed: %v", err)
	}
	if retried.RetryCount != 1 || retried.Status != "已发送" {
		t.Fatalf("expected ops retry to update record, got %#v", retried)
	}

	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	mathNotice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:            "课",
		Title:           "数学课提醒",
		Target:          "五年级数学班",
		Summary:         "明天上课。",
		Channel:         "公众号模板消息",
		RecipientOpenID: "oa-openid-002",
	})
	if err != nil {
		t.Fatalf("expected math notice creation to succeed: %v", err)
	}
	if noticeListContains(store.Notices(teacher), mathNotice.ID) {
		t.Fatalf("English teacher should not see math notice records")
	}
	if _, err := store.RetryNotice("英语老师", teacher, mathNotice.ID); err == nil || !strings.Contains(err.Error(), "不能补发") {
		t.Fatalf("expected teacher retry outside scope to be blocked, got %v", err)
	}
}

func TestOfficialAccountNoticeUsesStudentLinkedOpenID(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-linked-openid"
		}
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	var sentTo string
	store.officialNoticeSender = func(notice learning.Notice) error {
		sentTo = notice.RecipientOpenID
		return nil
	}
	notice, err := store.CreateNotice("运营教务", ops, learning.NoticeCreateRequest{
		Type:    "课",
		Title:   "明天上课提醒",
		Target:  "小明",
		Summary: "明天 19:00 上课。",
		Channel: "公众号模板消息",
	})
	if err != nil {
		t.Fatalf("expected notice creation to succeed: %v", err)
	}
	if notice.Status != "已发送" || notice.RecipientOpenID != "oa-linked-openid" || sentTo != "oa-linked-openid" {
		t.Fatalf("expected linked official account openid to be used, notice=%#v sentTo=%q", notice, sentTo)
	}
}

func TestCompleteReviewAutoNoticeUsesOfficialAccountDelivery(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-review-openid"
		}
	}
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	var sent learning.Notice
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent = notice
		return nil
	}
	_, err = store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          95,
		TeacherComment: "阅读依据找得很准，继续保持。",
		Reward:         "阅读小星星",
		FinalStatus:    "已批改",
	})
	if err != nil {
		t.Fatalf("expected review completion to succeed: %v", err)
	}
	notice := store.notices[0]
	if notice.Channel != "公众号模板消息" || notice.Status != "已发送" || notice.RecipientOpenID != "oa-review-openid" {
		t.Fatalf("expected auto review notice to use official account delivery, got %#v", notice)
	}
	if notice.RelatedType != "review" || notice.RelatedID != "rev-001" {
		t.Fatalf("expected review notice to keep related object, got %#v", notice)
	}
	if sent.ID != notice.ID || sent.RecipientOpenID != "oa-review-openid" {
		t.Fatalf("expected sender to receive review notice, sent=%#v notice=%#v", sent, notice)
	}
}

func TestCompleteReviewAutoNoticeTracksMissingOfficialAccountConfiguration(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-review-openid"
		}
	}
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	_, err = store.CompleteReview("英语老师", teacher, "rev-001", learning.ReviewCompleteRequest{
		Score:          90,
		TeacherComment: "批改完成，请查看反馈。",
		FinalStatus:    "已批改",
	})
	if err != nil {
		t.Fatalf("expected review completion to succeed: %v", err)
	}
	notice := store.notices[0]
	if notice.Channel != "公众号模板消息" || notice.Status != "待配置" || !strings.Contains(notice.FailureReason, "WECHAT_OFFICIAL_ACCOUNT") {
		t.Fatalf("expected missing official account config to be tracked, got %#v", notice)
	}
}

func TestRemindStudentCreatesRetryableOfficialAccountNotice(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	result, err := store.RemindStudent("运营教务", ops, "stu-001")
	if err != nil {
		t.Fatalf("expected reminder to succeed: %v", err)
	}
	notice := store.notices[0]
	if result.NoticeID != notice.ID {
		t.Fatalf("expected result to return notice id, result=%#v notice=%#v", result, notice)
	}
	if notice.Channel != "公众号模板消息" || notice.Status != "待配置" || notice.RelatedType != "student" || notice.RelatedID != "stu-001" {
		t.Fatalf("expected reminder notice to be pending official account delivery, got %#v", notice)
	}
	if !strings.Contains(notice.FailureReason, "openid") {
		t.Fatalf("expected missing openid reason, got %#v", notice)
	}

	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-remind-openid"
		}
	}
	sent := 0
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent++
		if notice.RecipientOpenID != "oa-remind-openid" {
			t.Fatalf("expected retry to resolve linked openid, got %#v", notice)
		}
		return nil
	}
	retried, err := store.RetryNotice("运营教务", ops, notice.ID)
	if err != nil {
		t.Fatalf("expected reminder retry to succeed: %v", err)
	}
	if retried.Status != "已发送" || retried.RetryCount != 1 || sent != 1 {
		t.Fatalf("expected retried reminder to be sent, notice=%#v sent=%d", retried, sent)
	}
}

func TestCreateHomeworkPublishesOfficialAccountNotices(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" || store.students[index].ID == "stu-002" || store.students[index].ID == "stu-003" {
			store.students[index].OfficialAccountOpenID = "oa-" + store.students[index].ID
		}
	}
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}
	courses := store.Courses(teacher)
	if len(courses) == 0 {
		t.Fatal("expected teacher courses")
	}
	sent := 0
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent++
		if notice.Channel != "公众号模板消息" || notice.RecipientOpenID == "" {
			t.Fatalf("expected official account notice, got %#v", notice)
		}
		return nil
	}
	homework, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{
		Title:    "英语阅读发布通知测试",
		CourseID: courses[0].ID,
		Deadline: "2026-06-30",
		Status:   string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected homework creation to succeed: %v", err)
	}
	if sent != 3 {
		t.Fatalf("expected notices for three English students, sent=%d notices=%#v", sent, store.notices[:3])
	}
	for _, notice := range store.notices[:3] {
		if notice.RelatedType != "homework" || notice.RelatedID != homework.ID || notice.Status != "已发送" {
			t.Fatalf("expected published homework notice, got %#v", notice)
		}
	}
}

func TestUpdateSettingValidatesAndLogs(t *testing.T) {
	store := NewMemoryStore()

	settings, err := store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{
		Key:   "downloadPolicy",
		Value: "允许下载已发布学习资料",
	})
	if err != nil {
		t.Fatalf("expected setting update to succeed: %v", err)
	}
	if settings["downloadPolicy"] != "允许下载已发布学习资料" {
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

func TestCreateParentNoticeUsesUnifiedOfficialAccountNotice(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.students {
		if store.students[index].ID == "stu-001" {
			store.students[index].OfficialAccountOpenID = "oa-parent-openid"
		}
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	var sent learning.Notice
	store.officialNoticeSender = func(notice learning.Notice) error {
		sent = notice
		return nil
	}
	order, err := store.CreateCommercialOrder("运营教务", ops, learning.CommercialOrderCreateRequest{
		StudentID:   "stu-001",
		PackageID:   enabledPackageIDForGrade(store, "五年级"),
		AmountCent:  128000,
		LessonTotal: 20,
	})
	if err != nil {
		t.Fatalf("expected commercial order to be created: %v", err)
	}
	notice, err := store.CreateParentNotice("运营教务", ops, learning.ParentNoticeCreateRequest{
		OrderID: order.ID,
		Title:   "续费提醒",
		Content: "课包剩余不多，建议提前确认续费安排。",
	})
	if err != nil {
		t.Fatalf("expected parent notice to be created: %v", err)
	}
	if notice.Status != "已发送" || notice.NoticeID == "" || notice.Channel != "公众号模板消息" || notice.FailureReason != "" {
		t.Fatalf("expected parent notice to track official account delivery, got %#v", notice)
	}
	latest := store.notices[0]
	if latest.ID != notice.NoticeID || latest.RelatedType != "commercial_order" || latest.RelatedID != order.ID {
		t.Fatalf("expected unified commercial notice record, parent=%#v latest=%#v", notice, latest)
	}
	if latest.RecipientOpenID != "oa-parent-openid" || sent.ID != latest.ID {
		t.Fatalf("expected official account sender to receive linked openid, sent=%#v latest=%#v", sent, latest)
	}
}

func TestSystemReadinessTracksExternalConfiguration(t *testing.T) {
	store := NewMemoryStore()

	readiness := store.SystemReadiness()
	if readiness.TotalCount == 0 || readiness.ReadyCount != 0 {
		t.Fatalf("expected initial readiness to require configuration, got %#v", readiness)
	}
	assertReadinessStatus(t, readiness, "miniProgramDomainStatus", "missing")
	assertReadinessStatus(t, readiness, "productionApiDomain", "missing")
	assertReadinessStatus(t, readiness, "officialAccountConfig", "missing")
	assertReadinessStatus(t, readiness, "studentOfficialAccountOpenID", "warning")

	for index := range store.students {
		if store.students[index].ID == "stu-001" || store.students[index].ID == "stu-002" || store.students[index].ID == "stu-003" {
			store.students[index].OfficialAccountOpenID = "oa-" + store.students[index].ID
		}
	}
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "miniProgramDomainStatus", Value: "已完成"})
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "officialAccountBindingStatus", Value: "已完成"})
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "templateMessageStatus", Value: "已完成"})
	_, _ = store.UpdateSetting("校区管理员", learning.SettingUpdateRequest{Key: "productionApiDomain", Value: "https://api.starlineeducation.com.cn"})
	store.UseOfficialAccountAPI("oa-appid", "oa-secret", "tpl-id")

	readiness = store.SystemReadiness()
	if readiness.ReadyCount != readiness.TotalCount {
		t.Fatalf("expected all readiness items to pass, got %#v", readiness)
	}
}

func assertReadinessStatus(t *testing.T, readiness learning.SystemReadiness, key, want string) {
	t.Helper()
	for _, item := range readiness.Items {
		if item.Key == key {
			if item.Status != want {
				t.Fatalf("readiness %s status = %q want %q item=%#v", key, item.Status, want, item)
			}
			return
		}
	}
	t.Fatalf("readiness item %s not found in %#v", key, readiness)
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
	if created.AcademicYear == "" || created.Grade != "五年级" || created.Semester != "S1" || created.Subject != "英语" {
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
	store.submissions["sub-hidden-homework"] = learning.Submission{
		ID:             "sub-hidden-homework",
		HomeworkID:     "hw-g05-english-s1-q1",
		StudentID:      "stu-001",
		TaskTitle:      "停用前已提交练习",
		Score:          88,
		TeacherComment: "继续保持。",
		Status:         "已批改",
		CreatedAt:      "2026-06-24 20:00:00",
	}
	beforeGrowth, err := store.StudentGrowth(student)
	if err != nil {
		t.Fatalf("expected growth before homework disable: %v", err)
	}
	if !learningRecordContains(beforeGrowth, "growth-sub-hidden-homework") {
		t.Fatalf("expected visible homework submission in growth before disable, got %#v", beforeGrowth)
	}

	if _, err := store.UpdateHomework("英语老师", teacher, "hw-g05-english-s1-q1", learning.HomeworkUpdateRequest{
		Title:    "停用练习",
		CourseID: "course-g05-english-s1-q1",
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
		if task.ID == "hw-g05-english-s1-q1" {
			t.Fatalf("disabled homework should be hidden from student tasks: %#v", task)
		}
	}
	afterGrowth, err := store.StudentGrowth(student)
	if err != nil {
		t.Fatalf("expected growth after homework disable: %v", err)
	}
	if learningRecordContains(afterGrowth, "growth-sub-hidden-homework") {
		t.Fatalf("disabled homework submission should be hidden from student growth: %#v", afterGrowth)
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
		Grade: "五年级", Semester: "S1", Subject: "英语", Type: "multiple",
		Stem: "哪些做法有助于英语阅读？", Options: []string{"圈关键词", "完全不读题", "复查答案"},
		Answers: []string{"圈关键词", "复查答案"}, Score: 10, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected question creation to succeed: %v", err)
	}
	created, err := store.CreateHomework("英语老师", teacher, learning.HomeworkUploadRequest{
		Title: "题库组卷练习", CourseID: "course-g05-english-s1-q1", LearningSpaceID: "space-g05-english-s1-q1",
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

func TestQuestionBankSupportsRichTextStem(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}

	item, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Grade: "五年级", Semester: "S1", Subject: "英语", Type: "single",
		Stem:    `<strong onclick="bad()">阅读图片</strong><img src="https://example.com/q.png" /><script>alert(1)</script><iframe src="https://example.com"></iframe>选择正确答案`,
		Options: []string{"A", "B"}, Answer: "A", Score: 10, Status: string(learning.StatusEnabled),
	})
	if err != nil {
		t.Fatalf("expected rich text question creation to succeed: %v", err)
	}
	if !strings.Contains(item.Stem, "<img") {
		t.Fatalf("expected rich text stem to be preserved, got %q", item.Stem)
	}
	if strings.Contains(item.Stem, "script") || strings.Contains(item.Stem, "onclick") || strings.Contains(item.Stem, "iframe") {
		t.Fatalf("expected unsafe rich text to be removed, got %q", item.Stem)
	}
	if strings.Contains(item.Title, "<") || !strings.Contains(item.Title, "阅读图片") {
		t.Fatalf("expected generated title to use plain text, got %q", item.Title)
	}
}

func TestTeacherQuestionBankScopeLimitedByLearningSpaces(t *testing.T) {
	store := NewMemoryStore()
	teacher, err := store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatalf("expected teacher principal: %v", err)
	}

	if _, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
		Grade: "五年级", Semester: "S1", Subject: "英语", Type: "single",
		Stem: "Which one is an English word?", Options: []string{"apple", "数学"},
		Answer: "apple", Score: 10, Status: string(learning.StatusEnabled),
	}); err != nil {
		t.Fatalf("expected teacher to create question in own scope: %v", err)
	}

	cases := []struct {
		name     string
		grade    string
		semester string
		subject  string
	}{
		{name: "another subject", grade: "五年级", semester: "S1", subject: "数学"},
		{name: "another grade", grade: "六年级", semester: "S1", subject: "英语"},
		{name: "another semester", grade: "五年级", semester: "S2", subject: "英语"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.CreateQuestion("英语老师", teacher, learning.QuestionBankUpsertRequest{
				Grade: tt.grade, Semester: tt.semester, Subject: tt.subject, Type: "single",
				Stem: "超出老师负责范围的题目", Options: []string{"A", "B"},
				Answer: "A", Score: 10, Status: string(learning.StatusEnabled),
			})
			if err == nil || !strings.Contains(err.Error(), "不能维护未负责范围的题库") {
				t.Fatalf("expected scope error, got %v", err)
			}
		})
	}
}

func TestTextQuestionSubmissionCreatesPendingReview(t *testing.T) {
	store := NewMemoryStore()
	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("expected student principal: %v", err)
	}
	submission, err := store.CreateSubmission("学生", student, learning.SubmissionRequest{
		HomeworkID: "hw-g05-english-s1-q1",
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

func findStoreStudentTodo(todos []learning.StudentTodo, todoType string) (learning.StudentTodo, bool) {
	for _, todo := range todos {
		if todo.Type == todoType {
			return todo, true
		}
	}
	return learning.StudentTodo{}, false
}

func favoriteListContains(favorites []learning.Favorite, id string) bool {
	for _, favorite := range favorites {
		if favorite.ID == id {
			return true
		}
	}
	return false
}

func learningRecordContains(records []learning.StudentLearningRecord, id string) bool {
	for _, record := range records {
		if record.ID == id {
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

func materialHasLearningDimensions(items []learning.Material, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return item.AcademicYear != "" && item.Grade != "" && item.Semester != "" && item.Subject != ""
		}
	}
	return false
}

func enabledPackageIDForGrade(store *MemoryStore, grade string) string {
	for _, pkg := range store.packages {
		if pkg.Grade == grade && pkg.Status == learning.StatusEnabled {
			return pkg.ID
		}
	}
	return ""
}
