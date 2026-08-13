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

	activePackageID := packageID(4, "语文", 0, "full")
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
	if stored.Name != "小明" || stored.SchoolName != "星河小学" || stored.EnrollmentGrade != "五年级" || stored.LearningStatus != "待开通" || stored.BindStatus != "已绑定" {
		t.Fatalf("unexpected created student: %#v", stored)
	}
	home, err := store.StudentHome(user)
	if err != nil {
		t.Fatalf("expected newly authorized student to enter mini program home: %v", err)
	}
	if home.Student.ID != user.StudentID || home.Student.Grade != "五年级" || len(home.Materials) != 0 || len(home.PendingHomework) != 0 {
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

	// 年级不是学生自助资料的一部分：即使提交了一个和当前推导年级不同的值，
	// 也必须被忽略，年级只能通过管理端订正入学基准来改变。
	basic, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{
		StudentName:  " 小明 ",
		Grade:        " 九年级 ",
		SchoolName:   " 星光小学 ",
		GuardianName: " 小明妈妈 ",
	})
	if err != nil {
		t.Fatalf("expected basic profile update to succeed: %v", err)
	}
	if basic.Name != "小明" || basic.Grade != "五年级" || basic.SchoolName != "星光小学" || basic.GuardianName != "小明妈妈" {
		t.Fatalf("student self-service profile update must ignore grade, got %#v", basic)
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

	store.phoneResolver = func(code string) (string, error) {
		if code != "phone-code" {
			t.Fatalf("unexpected phone code: %q", code)
		}
		return "19900000000", nil
	}
	phoneUpdated, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{PhoneCode: " phone-code "})
	if err != nil {
		t.Fatalf("expected phone authorization to update profile without school fields: %v", err)
	}
	if phoneUpdated.Phone != "19900000000" {
		t.Fatalf("expected resolved phone in student profile, got %#v", phoneUpdated)
	}
	user, ok := store.findUserByStudentID("stu-001")
	if !ok || user.Phone != "19900000000" {
		t.Fatalf("expected student user phone to stay synchronized, got %#v", user)
	}

	store.phoneResolver = func(string) (string, error) { return "13600002201", nil }
	if _, err := store.UpdateStudentProfile("学生本人", student, learning.StudentProfileUpdateRequest{PhoneCode: "duplicate-phone-code"}); err == nil || !strings.Contains(err.Error(), "手机号已绑定其他学生") {
		t.Fatalf("expected duplicate authorized phone to be rejected, got %v", err)
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

	for index := range store.users {
		if store.users[index].ID == "user-teacher" {
			store.users[index].AccountStatus = "停用"
		}
	}
	for _, slot := range store.AvailabilityOverview(ops) {
		if slot.OwnerType == "teacher" && slot.OwnerID == "user-teacher" {
			t.Fatalf("disabled teacher availability should be hidden: %#v", slot)
		}
	}
}

func TestSchedulingRejectsDisabledTeacher(t *testing.T) {
	store := NewMemoryStore()
	for index := range store.users {
		if store.users[index].ID == "user-teacher" {
			store.users[index].AccountStatus = "停用"
		}
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("expected ops principal: %v", err)
	}
	candidates, err := store.ScheduleCandidates(ops, learning.ScheduleCandidateRequest{
		Subject: "英语", Grade: "五年级", ClassType: "1V1", DurationMinutes: 90,
		StartDate: "2026-06-01", EndDate: "2026-08-31",
	})
	if err != nil {
		t.Fatalf("expected candidate query to succeed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("disabled teacher must not produce schedule candidates: %#v", candidates)
	}

	_, err = store.CreateScheduleClass("运营教务", ops, learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 90, DayOfWeek: 3, StartTime: "19:00", EndTime: "20:30",
		StartDate: "2026-06-01", EndDate: "2026-08-31", StudentIDs: []string{"stu-001"},
	})
	if err == nil || !strings.Contains(err.Error(), "教师账号已停用") {
		t.Fatalf("expected disabled teacher class creation to be rejected, got %v", err)
	}

	for index := range store.users {
		if store.users[index].ID == "user-teacher" {
			store.users[index].AccountStatus = "正常"
		}
	}
	existing, err := store.CreateScheduleClass("运营教务", ops, learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 90, DayOfWeek: 3, StartTime: "19:00", EndTime: "20:30",
		StartDate: "2026-06-01", EndDate: "2026-08-31", StudentIDs: []string{"stu-001"},
	})
	if err != nil {
		t.Fatalf("expected active teacher class creation to succeed: %v", err)
	}
	for index := range store.users {
		if store.users[index].ID == "user-teacher" {
			store.users[index].AccountStatus = "停用"
		}
	}
	if _, err := store.UpdateScheduleClass("运营教务", ops, existing.ID, learning.ScheduleClassCreateRequest{
		CourseID: "course-g05-english-s1-q1", TeacherID: "user-teacher", CampusID: "campus-main",
		ClassType: "1V1", DurationMinutes: 90, DayOfWeek: 3, StartTime: "19:00", EndTime: "20:30",
		StartDate: "2026-06-01", EndDate: "2026-08-31", StudentIDs: []string{"stu-001"},
	}); err == nil || !strings.Contains(err.Error(), "教师账号已停用") {
		t.Fatalf("expected disabled teacher schedule update to be rejected, got %v", err)
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
	if !containsString(demo.Subjects, "英文") {
		t.Fatalf("expected teacher subjects to include 英文, got %#v", demo.Subjects)
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
