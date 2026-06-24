package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type MemoryStore struct {
	users              []learning.User
	packages           []learning.Package
	students           []learning.Student
	learningSpaces     []learningSpace
	packageSpaces      []packageSpace
	contentTypes       []packageContentType
	spaceAccess        []learningSpaceAccess
	courses            []learning.Course
	questionBank       []learning.QuestionBankItem
	materials          []learning.Material
	homework           []learning.Homework
	fileAssets         map[string]learning.FileAsset
	reviews            []learning.Review
	notices            []learning.Notice
	logs               []learning.OperationLog
	settings           map[string]string
	grants             []packageGrant
	availability       []learning.AvailabilitySlot
	scheduleClasses    []learning.ScheduleClass
	commercialOrders   []learning.CommercialOrder
	payments           []learning.PaymentRecord
	refunds            []learning.RefundRecord
	contracts          []learning.ContractRecord
	invoices           []learning.InvoiceRecord
	lessonConsumptions []learning.LessonConsumption
	renewalReminders   []learning.RenewalReminder
	parentNotices      []learning.ParentNotice
	submissions        map[string]learning.Submission
	favorites          map[string]learning.Favorite
	wechatResolver     func(code string) (string, error)
	phoneResolver      func(phoneCode string) (string, error)
	db                 *sql.DB
}

type Options struct {
	SeedDemoData           bool
	SkipBaseData           bool
	BootstrapAdminName     string
	BootstrapAdminPhone    string
	BootstrapAdminPassword string
}

type packageGrant struct {
	ID             string
	StudentID      string
	PackageID      string
	StartsAt       string
	EndsAt         string
	Status         string
	EffectiveUntil string
}

type learningSpace struct {
	ID           string
	AcademicYear string
	Grade        string
	Subject      string
	Semester     string
	Phase        string
	Name         string
	Status       learning.Status
}

type packageSpace struct {
	PackageID       string
	LearningSpaceID string
}

type packageContentType struct {
	PackageID   string
	ContentType string
}

type learningSpaceAccess struct {
	StudentID       string
	LearningSpaceID string
	PackageGrantID  string
	StartsAt        string
	EndsAt          string
	Status          string
}

const demoLoginPassword = "123456"

func mustPasswordHash(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithOptions(Options{SeedDemoData: true})
}

func NewMemoryStoreWithOptions(options Options) *MemoryStore {
	adminPasswordHash := mustPasswordHash(demoLoginPassword)
	store := &MemoryStore{
		fileAssets:  map[string]learning.FileAsset{},
		submissions: map[string]learning.Submission{},
		favorites:   map[string]learning.Favorite{},
		settings:    map[string]string{},
	}
	if !options.SkipBaseData {
		store.seedBaseDictionaries()
	}
	if options.SeedDemoData {
		store.seedDemoUsers(adminPasswordHash)
		seedPermissionDemoData(store)
		seedSchedulingDemoData(store)
	} else if strings.TrimSpace(options.BootstrapAdminPhone) != "" && strings.TrimSpace(options.BootstrapAdminPassword) != "" {
		store.seedBootstrapAdmin(options)
	}
	return store
}

func (s *MemoryStore) seedBootstrapAdmin(options Options) {
	name := strings.TrimSpace(options.BootstrapAdminName)
	if name == "" {
		name = "超级管理员"
	}
	s.users = []learning.User{{
		ID:            "user-super",
		Name:          name,
		Phone:         strings.TrimSpace(options.BootstrapAdminPhone),
		OpenID:        "bootstrap-super",
		PasswordHash:  mustPasswordHash(options.BootstrapAdminPassword),
		AccountStatus: "正常",
		Roles:         []learning.Role{learning.RoleSuperAdmin},
		CampusScopes:  []string{"campus-main"},
	}}
}

func (s *MemoryStore) seedBaseDictionaries() {
	s.settings = map[string]string{
		"academicYear":   "2026 学年",
		"grades":         "G1-G12",
		"semesters":      "第一学期 / 第二学期 / 暑期班",
		"watermarkRule":  "昵称 + 手机尾号 + 时间",
		"downloadPolicy": "默认不可下载",
	}
}

func (s *MemoryStore) seedDemoUsers(adminPasswordHash string) {
	s.users = []learning.User{
		{ID: "user-super", Name: "超级管理员", Phone: "13800000001", OpenID: "demo-super", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleSuperAdmin}, CampusScopes: []string{"campus-main"}},
		{ID: "user-campus", Name: "校区管理员", Phone: "13800000002", OpenID: "demo-campus", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleCampusAdmin}, CampusID: "campus-main", CampusScopes: []string{"campus-main"}},
		{ID: "user-ops", Name: "运营教务", Phone: "13800000003", OpenID: "demo-ops", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleOpsStaff}, CampusID: "campus-main", CampusScopes: []string{"campus-main"}},
		{ID: "user-teacher", Name: "英语老师", Phone: "13800000004", OpenID: "demo-teacher", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleTeacher}, CampusID: "campus-main", LearningSpaceIDs: []string{"space-g05-english-s1-mid", "space-g05-english-s1-final"}, CanUploadHandout: true, CanUploadQuestion: true, CanReview: true},
		{ID: "user-student-001", Name: "小明", Phone: "18500009069", OpenID: "", AccountStatus: "正常", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-001", CampusID: "campus-main"},
		{ID: "user-student-002", Name: "Lucy", Phone: "13600002201", OpenID: "", AccountStatus: "待提醒", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-002", CampusID: "campus-main"},
		{ID: "user-student-003", Name: "小航", Phone: "13700003303", OpenID: "", AccountStatus: "正常", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-003", CampusID: "campus-main"},
	}
	s.students = []learning.Student{
		{ID: "stu-001", Name: "小明", Grade: "五年级", Phone: "185****9069", LearningStatus: "连续7天", AccountStatus: "正常", StreakDays: 7, AverageScore: 92, BadgeCount: 5, BindStatus: "待绑定", LastStudyAt: "2026-05-22 18:20:00", EffectiveUntil: "2027-05-22"},
		{ID: "stu-002", Name: "Lucy", Grade: "五年级", Phone: "136****2201", LearningStatus: "今日未学", AccountStatus: "待提醒", StreakDays: 3, AverageScore: 86, BadgeCount: 3, BindStatus: "待绑定", LastStudyAt: "2026-05-21 19:10:00", EffectiveUntil: "2027-05-22"},
		{ID: "stu-003", Name: "小航", Grade: "五年级", Phone: "137****3303", LearningStatus: "刚开通", AccountStatus: "正常", StreakDays: 1, AverageScore: 80, BadgeCount: 1, BindStatus: "待绑定", LastStudyAt: "2026-05-22 20:00:00", EffectiveUntil: "2027-05-22"},
	}
	s.reviews = []learning.Review{
		{ID: "rev-001", StudentID: "stu-001", HomeworkID: "hw-g05-english-s1-mid", StudentName: "小明", PackageName: "英语班", Homework: "阅读挑战", SystemScore: 86, TeacherComment: "阅读理解整体不错，注意把答案依据写完整。", Reward: "阅读小星星", Status: "待评语"},
		{ID: "rev-002", StudentID: "stu-002", HomeworkID: "hw-g05-math-s1-mid", StudentName: "Lucy", PackageName: "数学班", Homework: "图形挑战", SystemScore: 78, TeacherComment: "图形思路基本正确，错题建议再画一遍辅助线。", Reward: "图形探索徽章", Status: "待复核"},
	}
	s.notices = []learning.Notice{
		{ID: "notice-001", Type: "练", Title: "英语阅读挑战已发布", Target: "英语班 86 名学生", Summary: "今天的小挑战别忘啦", Status: "已发送"},
		{ID: "notice-002", Type: "评", Title: "批改完成提醒", Target: "小明", Summary: "老师反馈已经准备好了", Status: "自动发送"},
	}
	s.logs = []learning.OperationLog{
		{ID: "log-001", Operator: "本地开发", Action: "初始化权限演示", Target: "完整学习空间与三种套餐", Time: "2026-05-22 09:30:00"},
	}
}

func (s *MemoryStore) ConnectSchedulingDB(dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}
	s.db = db
	if err := s.ensureSchedulingTables(); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	if err := s.bootstrapSchedulingData(); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	return nil
}

func seedSchedulingDemoData(s *MemoryStore) {
	s.availability = []learning.AvailabilitySlot{
		{ID: "av-teacher-1", OwnerType: "teacher", OwnerID: "user-teacher", OwnerName: "英语老师", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-06-01", EndDate: "2026-08-31", Remark: "晚间可排英语小班"},
		{ID: "av-teacher-2", OwnerType: "teacher", OwnerID: "user-teacher", OwnerName: "英语老师", DayOfWeek: 6, StartTime: "09:00", EndTime: "12:00", StartDate: "2026-06-01", EndDate: "2026-08-31"},
		// 周三晚上三人时间一致，可凑满 1V3；周六时间分散，用于演示协调。
		{ID: "av-stu-1", OwnerType: "student", OwnerID: "stu-001", OwnerName: "小明", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-06-01", EndDate: "2026-08-31"},
		{ID: "av-stu-2", OwnerType: "student", OwnerID: "stu-002", OwnerName: "Lucy", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-06-01", EndDate: "2026-08-31"},
		{ID: "av-stu-3", OwnerType: "student", OwnerID: "stu-003", OwnerName: "小航", DayOfWeek: 3, StartTime: "19:00", EndTime: "21:00", StartDate: "2026-06-01", EndDate: "2026-08-31"},
		{ID: "av-stu-4", OwnerType: "student", OwnerID: "stu-001", OwnerName: "小明", DayOfWeek: 6, StartTime: "09:00", EndTime: "11:00", StartDate: "2026-06-01", EndDate: "2026-08-31"},
		{ID: "av-stu-5", OwnerType: "student", OwnerID: "stu-002", OwnerName: "Lucy", DayOfWeek: 6, StartTime: "10:00", EndTime: "12:00", StartDate: "2026-06-01", EndDate: "2026-08-31"},
	}
}

type packageTypeSpec struct {
	Code         string
	Label        string
	Summary      string
	ContentTypes []string
}

var demoGrades = []string{"一年级", "二年级", "三年级", "四年级", "五年级", "六年级", "七年级", "八年级", "九年级", "十年级", "十一年级", "十二年级"}

var demoSubjects = []string{"语文", "数学", "英语", "物理", "化学", "地理", "历史", "政治", "生物"}

// elementarySubjects 小学阶段（1-6 年级）实际开设的学科，只有语文、数学、英语。
var elementarySubjects = map[string]bool{"语文": true, "数学": true, "英语": true}

// subjectAppliesToGrade 判断某年级是否开设该学科。小学没有物理、化学、生物、
// 地理、政治、历史，这些学科从初中（七年级，gradeIndex 6）才开始，借此避免
// 生成「一年级物理」这类无效组合，减少数据量与存储空间。
func subjectAppliesToGrade(gradeIndex int, subject string) bool {
	if gradeIndex < 6 { // 一年级到六年级
		return elementarySubjects[subject]
	}
	return true
}

var demoSemesters = []string{"第一学期", "第二学期"}

var demoPhases = []string{"期中前", "期末前"}

var demoPackageTypes = []packageTypeSpec{
	{Code: "question", Label: "题", Summary: "只开放题", ContentTypes: []string{"question"}},
	{Code: "question_handout", Label: "题+讲义", Summary: "开放题和讲义", ContentTypes: []string{"question", "handout"}},
	{Code: "full", Label: "课程+题+讲义", Summary: "开放课程、题和讲义", ContentTypes: []string{"course", "question", "handout"}},
}

func seedPermissionDemoData(s *MemoryStore) {
	const academicYear = "2026 学年"
	for gradeIndex, grade := range demoGrades {
		for _, subject := range demoSubjects {
			if !subjectAppliesToGrade(gradeIndex, subject) {
				continue
			}
			for semesterIndex, semester := range demoSemesters {
				for phaseIndex, phase := range demoPhases {
					spaceID := learningSpaceID(gradeIndex, subject, semesterIndex, phaseIndex)
					spaceName := grade + subject + semester + phase
					courseName := spaceName + "课程"
					s.learningSpaces = append(s.learningSpaces, learningSpace{
						ID: spaceID, AcademicYear: academicYear, Grade: grade, Subject: subject,
						Semester: semester, Phase: phase, Name: spaceName, Status: learning.StatusEnabled,
					})
					s.courses = append(s.courses, learning.Course{
						ID: courseID(spaceID), Name: courseName, Subject: subject, Grade: grade, LearningSpaceID: spaceID,
						ChapterCount: 8, MaterialNum: 1, HomeworkNum: 1, Status: learning.StatusEnabled,
					})
					s.materials = append(s.materials, learning.Material{
						ID: materialID(spaceID), Title: spaceName + "核心讲义", CourseID: courseID(spaceID), Course: courseName, LearningSpaceID: spaceID,
						Chapter: "基础巩固", Type: "讲义", ViewCount: demoViewCount(gradeIndex, semesterIndex, phaseIndex),
						OwnerTeacherID: "teacher-" + subjectSlug(subject), OwnerTeacherName: subject + "老师", PublishStatus: "已发布", Status: learning.StatusEnabled,
					})
					questions := s.ensureDemoQuestionBank(grade, semester, subject)
					s.homework = append(s.homework, learning.Homework{
						ID: homeworkID(spaceID), Title: spaceName + "练习题", PackageName: subject + "题",
						CourseID: courseID(spaceID), Course: courseName, LearningSpaceID: spaceID, Grade: grade, Semester: semester, Subject: subject,
						QuestionNum: len(questions), QuestionIDs: questionIDs(questions), Questions: questions, Deadline: demoDeadline(semesterIndex, phaseIndex),
						SubmittedNum: 0, TotalNum: 0, OwnerTeacherID: "teacher-" + subjectSlug(subject),
						OwnerTeacherName: subject + "老师", PublishStatus: "已发布", Status: "已发布",
					})
				}

				for _, pkgType := range demoPackageTypes {
					pkgID := packageID(gradeIndex, subject, semesterIndex, pkgType.Code)
					s.packages = append(s.packages, learning.Package{
						ID: pkgID, Name: academicYear + " " + grade + " " + semester + " " + subject + " " + pkgType.Label,
						AcademicYear: academicYear, Grade: grade, Semester: semester, Subject: subject,
						PhaseScope: "全学期", PackageType: pkgType.Label, Summary: pkgType.Summary, Status: learning.StatusEnabled,
					})
					for phaseIndex := range demoPhases {
						s.packageSpaces = append(s.packageSpaces, packageSpace{
							PackageID:       pkgID,
							LearningSpaceID: learningSpaceID(gradeIndex, subject, semesterIndex, phaseIndex),
						})
					}
					for _, contentType := range pkgType.ContentTypes {
						s.contentTypes = append(s.contentTypes, packageContentType{PackageID: pkgID, ContentType: contentType})
					}
				}
			}
		}
	}

	s.grants = []packageGrant{
		{ID: "grant-001", StudentID: "stu-001", PackageID: packageID(4, "英语", 0, "full"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		{ID: "grant-002", StudentID: "stu-002", PackageID: packageID(4, "数学", 0, "question_handout"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		{ID: "grant-003", StudentID: "stu-003", PackageID: packageID(4, "语文", 0, "question"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		// Lucy、小航 同时开通 五年级英语，用于演示「同年级同学科」成班与协调。
		{ID: "grant-004", StudentID: "stu-002", PackageID: packageID(4, "英语", 0, "full"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		{ID: "grant-005", StudentID: "stu-003", PackageID: packageID(4, "英语", 0, "question_handout"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
	}
	for _, grant := range s.grants {
		s.syncSpaceAccessForGrant(grant)
		if pkg, ok := s.findPackage(grant.PackageID); ok {
			s.addStudentOpenedPackage(grant.StudentID, pkg.Name)
		}
	}
}

func learningSpaceID(gradeIndex int, subject string, semesterIndex, phaseIndex int) string {
	return "space-g" + twoDigit(gradeIndex+1) + "-" + subjectSlug(subject) + "-s" + strconv.Itoa(semesterIndex+1) + "-" + phaseSlug(phaseIndex)
}

func packageID(gradeIndex int, subject string, semesterIndex int, packageType string) string {
	return "pkg-g" + twoDigit(gradeIndex+1) + "-" + subjectSlug(subject) + "-s" + strconv.Itoa(semesterIndex+1) + "-" + packageType
}

func courseID(spaceID string) string {
	return "course-" + strings.TrimPrefix(spaceID, "space-")
}

func materialID(spaceID string) string {
	return "mat-" + strings.TrimPrefix(spaceID, "space-")
}

func homeworkID(spaceID string) string {
	return "hw-" + strings.TrimPrefix(spaceID, "space-")
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func subjectSlug(subject string) string {
	switch subject {
	case "语文":
		return "chinese"
	case "数学":
		return "math"
	case "英语":
		return "english"
	case "物理":
		return "physics"
	case "化学":
		return "chemistry"
	case "地理":
		return "geography"
	case "历史":
		return "history"
	case "政治":
		return "politics"
	case "生物":
		return "biology"
	default:
		return strings.ToLower(subject)
	}
}

func phaseSlug(phaseIndex int) string {
	if phaseIndex == 0 {
		return "mid"
	}
	return "final"
}

func demoViewCount(gradeIndex, semesterIndex, phaseIndex int) int {
	return 80 + gradeIndex*12 + semesterIndex*8 + phaseIndex*4
}

func demoQuestions(subject string) []learning.Question {
	return []learning.Question{
		{
			ID:      "q1",
			Type:    "single",
			Stem:    "学习" + subject + "时，下面哪种做法更好？",
			Options: []string{"打好基础，多练习多复习", "只看不练，遇到难题就跳过", "完全不复习，全靠考前突击"},
			Answer:  "A",
		},
		{
			ID:   "q2",
			Type: "text",
			Stem: "用一句话说说你今天学到的一个" + subject + "小知识。",
		},
	}
}

func (s *MemoryStore) ensureDemoQuestionBank(grade, semester, subject string) []learning.Question {
	prefix := "qb-" + slugText(grade) + "-" + subjectSlug(subject) + "-s" + semesterNumber(semester)
	existing := make([]learning.Question, 0)
	for _, item := range s.questionBank {
		if item.Grade == grade && item.Semester == semester && item.Subject == subject {
			existing = append(existing, bankItemQuestion(item))
		}
	}
	if len(existing) > 0 {
		return existing
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	for index, question := range demoQuestions(subject) {
		item := learning.QuestionBankItem{
			ID: prefix + "-q" + strconv.Itoa(index+1), Grade: grade, Semester: semester, Subject: subject,
			Type: question.Type, Stem: question.Stem, Options: question.Options, Answer: question.Answer,
			Answers: normalizedQuestionAnswers(question), Score: 100, Status: string(learning.StatusEnabled),
			OwnerTeacherID: "teacher-" + subjectSlug(subject), OwnerTeacherName: subject + "老师", CreatedAt: now, UpdatedAt: now,
		}
		s.questionBank = append(s.questionBank, item)
		existing = append(existing, bankItemQuestion(item))
	}
	return existing
}

func slugText(value string) string {
	replacer := strings.NewReplacer("一年级", "g01", "二年级", "g02", "三年级", "g03", "四年级", "g04", "五年级", "g05", "六年级", "g06", "七年级", "g07", "八年级", "g08", "九年级", "g09", " ", "-")
	return strings.ToLower(replacer.Replace(value))
}

func semesterNumber(value string) string {
	if strings.Contains(value, "第二") {
		return "2"
	}
	return "1"
}

func demoDeadline(semesterIndex, phaseIndex int) string {
	if semesterIndex == 0 {
		if phaseIndex == 0 {
			return "2026-10-30"
		}
		return "2027-01-15"
	}
	if phaseIndex == 0 {
		return "2027-04-30"
	}
	return "2027-06-20"
}

func (s *MemoryStore) Dashboard() learning.DashboardOverview {
	views := 0
	for _, material := range s.materials {
		views += material.ViewCount
	}
	return learning.DashboardOverview{
		OpenedStudents:   286,
		PackageCount:     len(s.packages),
		PendingReviews:   len(s.reviews),
		MaterialViews:    views,
		ExpiringStudents: 3,
		UnpublishedFiles: 2,
	}
}

func (s *MemoryStore) Packages() []learning.Package {
	out := make([]learning.Package, 0, len(s.packages))
	for _, pkg := range s.packages {
		out = append(out, s.decoratePackage(pkg))
	}
	return out
}

func (s *MemoryStore) CreatePackage(operator string, req learning.PackageUpsertRequest) (learning.Package, error) {
	pkg, err := s.packageFromRequest("", req)
	if err != nil {
		return learning.Package{}, err
	}
	if s.packageNameExists("", pkg.Name) {
		return learning.Package{}, errors.New("学习套餐名称已存在")
	}
	pkg.ID = "pkg-custom-" + time.Now().Format("20060102150405.000000000")
	s.packages = append([]learning.Package{pkg}, s.packages...)
	s.replacePackageRelations(pkg.ID, req.LearningSpaceIDs, req.ContentTypeCodes)
	s.prependLog(operator, "创建学习套餐", pkg.Name)
	return s.decoratePackage(pkg), nil
}

func (s *MemoryStore) UpdatePackage(operator string, id string, req learning.PackageUpsertRequest) (learning.Package, error) {
	id = strings.TrimSpace(id)
	pkg, err := s.packageFromRequest(id, req)
	if err != nil {
		return learning.Package{}, err
	}
	if s.packageNameExists(id, pkg.Name) {
		return learning.Package{}, errors.New("学习套餐名称已存在")
	}
	for index := range s.packages {
		if s.packages[index].ID != id {
			continue
		}
		before := s.decoratePackage(s.packages[index])
		s.packages[index] = pkg
		s.replacePackageRelations(id, req.LearningSpaceIDs, req.ContentTypeCodes)
		s.refreshSpaceAccessForPackage(id)
		after := s.decoratePackage(pkg)
		s.prependLogDetail(operator, "编辑学习套餐", pkg.Name, auditChangeDetail(packageAuditSnapshot(before), packageAuditSnapshot(after)))
		return after, nil
	}
	return learning.Package{}, errors.New("学习套餐不存在")
}

func (s *MemoryStore) LearningSpaces() []learning.LearningSpace {
	out := make([]learning.LearningSpace, 0, len(s.learningSpaces))
	for _, space := range s.learningSpaces {
		out = append(out, learning.LearningSpace{
			ID:           space.ID,
			AcademicYear: space.AcademicYear,
			Grade:        space.Grade,
			Subject:      space.Subject,
			Semester:     space.Semester,
			Phase:        space.Phase,
			Name:         space.Name,
			Status:       space.Status,
		})
	}
	return out
}

func (s *MemoryStore) LoginWithWechatCode(code, phone, phoneCode string) (learning.Principal, error) {
	openID, err := s.resolveOpenID(code)
	if err != nil {
		return learning.Principal{}, err
	}
	phone = strings.TrimSpace(phone)
	if phone == "" && strings.TrimSpace(phoneCode) != "" {
		resolvedPhone, err := s.resolvePhoneNumber(phoneCode)
		if err != nil {
			return learning.Principal{}, err
		}
		phone = resolvedPhone
	}
	if phone != "" {
		for i, user := range s.users {
			if user.Phone != phone || !canRebindByPhone(user, s.wechatResolver != nil) {
				continue
			}
			if user.AccountStatus != "正常" {
				return learning.Principal{}, errors.New("账号已停用，请联系管理员")
			}
			s.users[i].OpenID = openID
			s.removeWechatOnlyStudent(openID, s.users[i].StudentID)
			action := "绑定教师微信"
			if hasRole(user.Roles, learning.RoleStudent) {
				action = "绑定学生微信"
			} else if isAdminStaffUser(user) {
				action = "绑定后台人员微信"
			}
			s.prependLog(user.Name, action, user.Name)
			return principalFromUser(s.users[i]), nil
		}
	}
	for _, user := range s.users {
		if user.OpenID != openID {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		return principalFromUser(user), nil
	}
	return s.createWechatStudent(openID, phone), nil
}

func (s *MemoryStore) createWechatStudent(openID, phone string) learning.Principal {
	now := time.Now()
	suffix := now.Format("20060102150405.000000000")
	studentID := "stu-wx-" + suffix
	userID := "user-wx-" + suffix
	student := learning.Student{
		ID:             studentID,
		Name:           "微信用户",
		Nickname:       "",
		AvatarURL:      "",
		Grade:          "待完善",
		Phone:          displayPhone(phone),
		OpenedPackages: []string{},
		LearningStatus: "待开通",
		AccountStatus:  "正常",
		Remark:         "微信授权自动创建",
		BindStatus:     "已绑定",
		LastStudyAt:    "",
	}
	user := learning.User{
		ID:            userID,
		Name:          student.Name,
		Phone:         phone,
		OpenID:        openID,
		AccountStatus: "正常",
		Remark:        student.Remark,
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     studentID,
	}
	s.students = append([]learning.Student{student}, s.students...)
	s.users = append(s.users, user)
	s.prependLog(student.Name, "微信授权登录", "自动创建待开通学生账号")
	return principalFromUser(user)
}

func (s *MemoryStore) removeWechatOnlyStudent(openID, keepStudentID string) {
	removeStudentIDs := map[string]bool{}
	users := make([]learning.User, 0, len(s.users))
	for _, user := range s.users {
		if user.OpenID == openID && user.StudentID != keepStudentID && user.Phone == "" && hasRole(user.Roles, learning.RoleStudent) {
			removeStudentIDs[user.StudentID] = true
			continue
		}
		users = append(users, user)
	}
	if len(removeStudentIDs) == 0 {
		return
	}
	students := make([]learning.Student, 0, len(s.students))
	for _, student := range s.students {
		if removeStudentIDs[student.ID] && student.Remark == "微信授权自动创建" {
			continue
		}
		students = append(students, student)
	}
	s.users = users
	s.students = students
}

func (s *MemoryStore) LoginWithAdminPassword(phone, password string) (learning.Principal, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" || password == "" {
		return learning.Principal{}, errors.New("请输入手机号和密码")
	}
	for _, user := range s.users {
		if user.Phone != phone {
			continue
		}
		if !hasRole(user.Roles, learning.RoleTeacher) && !isAdminStaffUser(user) {
			return learning.Principal{}, errors.New("手机号或密码错误")
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			return learning.Principal{}, errors.New("手机号或密码错误")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("手机号或密码错误")
}

func (s *MemoryStore) ChangePassword(operator string, principal learning.Principal, req learning.PasswordChangeRequest) error {
	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.OldPassword == "" || req.NewPassword == "" {
		return errors.New("请输入原密码和新密码")
	}
	if err := validateNewPassword(req.NewPassword); err != nil {
		return err
	}
	for i := range s.users {
		if s.users[i].ID != principal.UserID {
			continue
		}
		if s.users[i].PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(s.users[i].PasswordHash), []byte(req.OldPassword)) != nil {
			s.prependLogDetail(operator, "修改密码失败", s.users[i].Name, "原密码错误")
			return errors.New("原密码不正确")
		}
		if bcrypt.CompareHashAndPassword([]byte(s.users[i].PasswordHash), []byte(req.NewPassword)) == nil {
			return errors.New("新密码不能和原密码相同")
		}
		s.users[i].PasswordHash = mustPasswordHash(req.NewPassword)
		s.users[i].MustChangePassword = false
		s.users[i].TokenVersion++
		s.prependLogDetail(operator, "修改密码", s.users[i].Name, "用户主动修改密码")
		return nil
	}
	return errors.New("账号不存在，请重新登录")
}

func (s *MemoryStore) ResetPassword(operator string, principal learning.Principal, userID string) (learning.PasswordResetResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return learning.PasswordResetResult{}, errors.New("请选择账号")
	}
	for i := range s.users {
		if s.users[i].ID != userID {
			continue
		}
		if !canResetPassword(principal, s.users[i]) {
			return learning.PasswordResetResult{}, errors.New("没有权限重置该账号密码")
		}
		temp, err := generateTemporaryPassword()
		if err != nil {
			return learning.PasswordResetResult{}, errors.New("临时密码生成失败")
		}
		s.users[i].PasswordHash = mustPasswordHash(temp)
		s.users[i].MustChangePassword = true
		s.users[i].TokenVersion++
		s.prependLogDetail(operator, "重置密码", s.users[i].Name, "已生成临时密码并要求下次登录修改")
		return learning.PasswordResetResult{UserID: s.users[i].ID, TemporaryPassword: temp, MustChangePassword: true}, nil
	}
	return learning.PasswordResetResult{}, errors.New("账号不存在")
}

func (s *MemoryStore) RecordSecurityEvent(operator, action, target, detail string) {
	s.prependLogDetail(operator, action, target, detail)
}

func (s *MemoryStore) LoginWithDemoStudentPassword(phone, password string) (learning.Principal, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" || password != demoLoginPassword {
		return learning.Principal{}, errors.New("手机号或密码错误")
	}
	for _, user := range s.users {
		if user.Phone != phone || !hasRole(user.Roles, learning.RoleStudent) {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("手机号或密码错误")
}

func (s *MemoryStore) PrincipalByUserID(userID string) (learning.Principal, error) {
	for _, user := range s.users {
		if user.ID != userID {
			continue
		}
		if user.AccountStatus != "正常" {
			return learning.Principal{}, errors.New("账号已停用，请联系管理员")
		}
		return principalFromUser(user), nil
	}
	return learning.Principal{}, errors.New("账号不存在，请重新登录")
}

func (s *MemoryStore) AdminStaff() []learning.AdminStaff {
	out := make([]learning.AdminStaff, 0)
	for _, user := range s.users {
		if !isAdminStaffUser(user) {
			continue
		}
		out = append(out, adminStaffFromUser(user))
	}
	return out
}

func (s *MemoryStore) CreateAdminStaff(operator string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	req, err := normalizeAdminStaffRequest(req, false)
	if err != nil {
		return learning.AdminStaff{}, err
	}
	if s.userPhoneExists("", req.Phone) {
		return learning.AdminStaff{}, errors.New("手机号已存在")
	}
	user := learning.User{
		ID:                 "user-admin-" + time.Now().Format("20060102150405"),
		Name:               req.Name,
		Phone:              req.Phone,
		PasswordHash:       mustPasswordHash(demoLoginPassword),
		MustChangePassword: true,
		AccountStatus:      "正常",
		Roles:              []learning.Role{req.Role},
		CampusID:           req.CampusID,
		Remark:             req.Remark,
	}
	s.users = append(s.users, user)
	s.prependLog(operator, "新增管理人员", user.Name+" / "+roleName(req.Role))
	return adminStaffFromUser(user), nil
}

func (s *MemoryStore) UpdateAdminStaff(operator string, principal learning.Principal, id string, req learning.AdminStaffUpsertRequest) (learning.AdminStaff, error) {
	req, err := normalizeAdminStaffRequest(req, true)
	if err != nil {
		return learning.AdminStaff{}, err
	}
	for i := range s.users {
		if s.users[i].ID != id {
			continue
		}
		if !isAdminStaffUser(s.users[i]) {
			return learning.AdminStaff{}, errors.New("只能编辑管理人员账号")
		}
		if id == principal.UserID && req.AccountStatus != "正常" {
			return learning.AdminStaff{}, errors.New("不能停用当前登录账号")
		}
		if s.userPhoneExists(id, req.Phone) {
			return learning.AdminStaff{}, errors.New("手机号已存在")
		}
		wasSuper := hasRole(s.users[i].Roles, learning.RoleSuperAdmin) && s.users[i].AccountStatus == "正常"
		willSuper := req.Role == learning.RoleSuperAdmin && req.AccountStatus == "正常"
		if wasSuper && !willSuper && s.activeSuperAdminCount() <= 1 {
			return learning.AdminStaff{}, errors.New("至少保留一个正常的超级管理员")
		}
		before := adminStaffFromUser(s.users[i])
		s.users[i].Name = req.Name
		s.users[i].Phone = req.Phone
		s.users[i].Roles = []learning.Role{req.Role}
		s.users[i].CampusID = req.CampusID
		s.users[i].AccountStatus = req.AccountStatus
		s.users[i].Remark = req.Remark
		after := adminStaffFromUser(s.users[i])
		s.prependLogDetail(operator, "更新管理人员", s.users[i].Name+" / "+roleName(req.Role), auditChangeDetail(adminStaffAuditSnapshot(before), adminStaffAuditSnapshot(after)))
		return after, nil
	}
	return learning.AdminStaff{}, errors.New("admin staff not found")
}

func (s *MemoryStore) Teachers(principal learning.Principal) []learning.Teacher {
	out := make([]learning.Teacher, 0)
	for _, user := range s.users {
		if !hasRole(user.Roles, learning.RoleTeacher) {
			continue
		}
		if hasRole(principal.Roles, learning.RoleTeacher) && principal.UserID == user.ID {
			out = append(out, s.teacherFromUser(user))
			continue
		}
		if !canManageTeacher(principal, user) {
			continue
		}
		out = append(out, s.teacherFromUser(user))
	}
	return out
}

func (s *MemoryStore) CreateTeacher(operator string, principal learning.Principal, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	req, err := s.normalizeTeacherRequest(principal, req, false)
	if err != nil {
		return learning.Teacher{}, err
	}
	for _, user := range s.users {
		if user.Phone == req.Phone {
			return learning.Teacher{}, errors.New("手机号已存在")
		}
	}
	user := learning.User{
		ID:                 "user-teacher-" + time.Now().Format("20060102150405"),
		Name:               req.Name,
		Phone:              req.Phone,
		PasswordHash:       mustPasswordHash(demoLoginPassword),
		MustChangePassword: true,
		AccountStatus:      "正常",
		Roles:              []learning.Role{learning.RoleTeacher},
		CampusID:           req.CampusID,
		LearningSpaceIDs:   req.LearningSpaceIDs,
		CanUploadHandout:   req.CanUploadHandout,
		CanUploadQuestion:  req.CanUploadQuestion,
		CanReview:          req.CanReview,
		Remark:             req.Remark,
	}
	s.users = append(s.users, user)
	s.prependLog(operator, "新增教师", user.Name)
	return s.teacherFromUser(user), nil
}

func (s *MemoryStore) UpdateTeacher(operator string, principal learning.Principal, id string, req learning.TeacherUpsertRequest) (learning.Teacher, error) {
	req, err := s.normalizeTeacherRequest(principal, req, true)
	if err != nil {
		return learning.Teacher{}, err
	}
	for i := range s.users {
		if s.users[i].ID != id {
			continue
		}
		if !hasRole(s.users[i].Roles, learning.RoleTeacher) {
			return learning.Teacher{}, errors.New("只能编辑教师账号")
		}
		if !canManageTeacher(principal, s.users[i]) {
			return learning.Teacher{}, errors.New("不能管理其他校区教师")
		}
		for _, user := range s.users {
			if user.ID != id && user.Phone == req.Phone {
				return learning.Teacher{}, errors.New("手机号已存在")
			}
		}
		before := s.teacherFromUser(s.users[i])
		s.users[i].Name = req.Name
		s.users[i].Phone = req.Phone
		s.users[i].CampusID = req.CampusID
		s.users[i].LearningSpaceIDs = req.LearningSpaceIDs
		s.users[i].CanUploadHandout = req.CanUploadHandout
		s.users[i].CanUploadQuestion = req.CanUploadQuestion
		s.users[i].CanReview = req.CanReview
		s.users[i].AccountStatus = req.AccountStatus
		s.users[i].Remark = req.Remark
		after := s.teacherFromUser(s.users[i])
		s.prependLogDetail(operator, "更新教师", s.users[i].Name, auditChangeDetail(teacherAuditSnapshot(before), teacherAuditSnapshot(after)))
		return after, nil
	}
	return learning.Teacher{}, errors.New("teacher not found")
}

func (s *MemoryStore) Students(principal learning.Principal, query learning.StudentQuery) []learning.Student {
	students := make([]learning.Student, 0, len(s.students))
	for _, student := range s.students {
		decorated := s.decorateStudent(student)
		if canSeeStudent(principal, decorated, s.coursesForStudent(student.ID)) && matchesStudentQuery(decorated, query) {
			students = append(students, decorated)
		}
	}
	return students
}

func (s *MemoryStore) StudentDetail(principal learning.Principal, id string) (learning.StudentDetail, error) {
	student, err := s.visibleStudent(principal, id)
	if err != nil {
		return learning.StudentDetail{}, err
	}
	grants, _ := s.StudentGrants(principal, id)
	records, _ := s.StudentLearningRecords(principal, id)
	permissions := s.permissionForStudent(student)
	return learning.StudentDetail{
		Student:         student,
		Grants:          grants,
		Permissions:     permissions,
		LearningRecords: records,
		Notices:         s.noticesForStudent(student),
		Logs:            s.logsForStudent(student),
	}, nil
}

func (s *MemoryStore) CreateStudent(operator string, principal learning.Principal, req learning.StudentUpsertRequest) (learning.Student, error) {
	req, err := normalizeStudentRequest(req, false)
	if err != nil {
		return learning.Student{}, err
	}
	if s.phoneExists("", req.Phone) {
		return learning.Student{}, errors.New("手机号已存在")
	}
	id := "stu-" + time.Now().Format("20060102150405")
	student := learning.Student{
		ID:             id,
		Name:           req.Name,
		Nickname:       "",
		AvatarURL:      "",
		Grade:          req.Grade,
		Phone:          req.Phone,
		OpenedPackages: []string{},
		LearningStatus: "未开始",
		AccountStatus:  "正常",
		Remark:         req.Remark,
		BindStatus:     "待绑定",
	}
	s.students = append([]learning.Student{student}, s.students...)
	s.users = append(s.users, learning.User{
		ID:            "user-" + id,
		Name:          req.Name,
		Phone:         req.Phone,
		AccountStatus: "正常",
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     id,
		CampusID:      principal.CampusID,
	})
	s.prependLog(operator, "新增学生", student.Name)
	return s.decorateStudent(student), nil
}

func (s *MemoryStore) UpdateStudent(operator string, principal learning.Principal, id string, req learning.StudentUpsertRequest) (learning.Student, error) {
	req, err := normalizeStudentRequest(req, true)
	if err != nil {
		return learning.Student{}, err
	}
	for i := range s.students {
		if s.students[i].ID != id {
			continue
		}
		if _, err := s.visibleStudent(principal, id); err != nil {
			return learning.Student{}, err
		}
		if s.phoneExists(id, req.Phone) {
			return learning.Student{}, errors.New("手机号已存在")
		}
		before := s.decorateStudent(s.students[i])
		s.students[i].Name = req.Name
		s.students[i].Phone = req.Phone
		s.students[i].Grade = req.Grade
		s.students[i].AccountStatus = req.AccountStatus
		s.students[i].Remark = req.Remark
		s.syncStudentUser(s.students[i])
		after := s.decorateStudent(s.students[i])
		s.prependLogDetail(operator, "更新学生", s.students[i].Name, auditChangeDetail(studentAuditSnapshot(before), studentAuditSnapshot(after)))
		return after, nil
	}
	return learning.Student{}, errors.New("student not found")
}

func (s *MemoryStore) UpdateStudentProfile(operator string, principal learning.Principal, req learning.StudentProfileUpdateRequest) (learning.Student, error) {
	if principal.StudentID == "" {
		return learning.Student{}, errors.New("student account is not bound")
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.AvatarURL = strings.TrimSpace(req.AvatarURL)
	if req.Nickname == "" {
		return learning.Student{}, errors.New("请授权微信昵称")
	}
	if req.AvatarURL == "" {
		return learning.Student{}, errors.New("请授权微信头像")
	}
	if len([]rune(req.Nickname)) > 32 {
		return learning.Student{}, errors.New("昵称最多 32 个字")
	}
	if len(req.AvatarURL) > 1000 {
		return learning.Student{}, errors.New("头像地址过长")
	}
	for i := range s.students {
		if s.students[i].ID != principal.StudentID {
			continue
		}
		if s.students[i].AccountStatus == "停用" {
			return learning.Student{}, errors.New("账号已停用，请联系老师或管理员")
		}
		beforeName := s.students[i].Nickname
		beforeAvatar := s.students[i].AvatarURL
		s.students[i].Nickname = req.Nickname
		s.students[i].AvatarURL = req.AvatarURL
		if beforeName != req.Nickname || beforeAvatar != req.AvatarURL {
			s.prependLog(operator, "更新学生资料", s.students[i].Name)
		}
		return s.decorateStudent(s.students[i]), nil
	}
	return learning.Student{}, errors.New("student not found")
}

func (s *MemoryStore) RemindStudent(operator string, principal learning.Principal, id string) (learning.StudentRemindResult, error) {
	student, err := s.visibleStudent(principal, id)
	if err != nil {
		return learning.StudentRemindResult{}, err
	}
	noticeID := "notice-" + time.Now().Format("20060102150405")
	notice := learning.Notice{
		ID:      noticeID,
		Type:    "提醒",
		Title:   "学习提醒",
		Target:  student.Name,
		Summary: "今天的小挑战别忘啦",
		Status:  "已创建",
	}
	s.notices = append([]learning.Notice{notice}, s.notices...)
	s.prependLog(operator, "提醒学生", student.Name)
	return learning.StudentRemindResult{NoticeID: noticeID, Message: "已创建学习提醒"}, nil
}

func (s *MemoryStore) ImportStudents(operator string, principal learning.Principal, rows []learning.StudentUpsertRequest) learning.StudentImportResult {
	result := learning.StudentImportResult{Errors: []learning.StudentImportRowError{}}
	for index, row := range rows {
		if _, err := s.CreateStudent(operator, principal, row); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, learning.StudentImportRowError{Row: index + 1, Message: err.Error()})
			continue
		}
		result.SuccessCount++
	}
	s.prependLog(operator, "批量导入学生", "成功 "+itoa(result.SuccessCount)+" 条")
	return result
}

func (s *MemoryStore) StudentGrants(principal learning.Principal, id string) ([]learning.StudentGrant, error) {
	if _, err := s.visibleStudent(principal, id); err != nil {
		return nil, err
	}
	grants := make([]learning.StudentGrant, 0)
	today := time.Now().Format("2006-01-02")
	for _, grant := range s.grants {
		if grant.StudentID != id {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if !ok {
			continue
		}
		state := "生效中"
		if grantEndsAt(grant) < today || grant.Status == "revoked" {
			state = "已到期"
		}
		grants = append(grants, learning.StudentGrant{
			StudentID: id, PackageID: pkg.ID, PackageName: pkg.Name, EffectiveUntil: grantEndsAt(grant), PermissionState: state,
		})
	}
	return grants, nil
}

func (s *MemoryStore) StudentLearningRecords(principal learning.Principal, id string) ([]learning.StudentLearningRecord, error) {
	student, err := s.visibleStudent(principal, id)
	if err != nil {
		return nil, err
	}
	records := make([]learning.StudentLearningRecord, 0)
	for _, material := range s.materialsForStudent(id) {
		records = append(records, learning.StudentLearningRecord{
			ID: "learn-" + material.ID, Type: "资料", Title: material.Title, Course: material.Course,
			Status: "已学习", OccurredAt: firstNonEmpty(student.LastStudyAt, "2026-05-22 18:20:00"), Description: "查看课件资料",
		})
	}
	for _, review := range s.reviews {
		if review.StudentName != student.Name {
			continue
		}
		records = append(records, learning.StudentLearningRecord{
			ID: "review-" + review.ID, Type: "小挑战", Title: review.Homework, Course: review.PackageName,
			Status: review.Status, Score: review.SystemScore, OccurredAt: "2026-05-22 20:10:00", Description: "提交后等待老师反馈",
		})
	}
	return records, nil
}

func (s *MemoryStore) CommercialSummary(principal learning.Principal) learning.CommercialSummary {
	orders := s.CommercialOrders(principal)
	summary := learning.CommercialSummary{OrderCount: len(orders)}
	for _, order := range orders {
		if order.PaidAmountCent > 0 {
			summary.PaidOrderCount++
		}
		summary.RevenueCent += order.PaidAmountCent
		summary.RefundCent += order.RefundedAmountCent
		summary.LessonRemainCount += maxInt(0, order.LessonTotal-order.LessonConsumed)
	}
	for _, reminder := range s.renewalReminders {
		if reminder.Status == "待跟进" && s.canSeeCommercialStudent(principal, reminder.StudentID) {
			summary.RenewalTodoCount++
		}
	}
	return summary
}

func (s *MemoryStore) CommercialOrders(principal learning.Principal) []learning.CommercialOrder {
	out := make([]learning.CommercialOrder, 0)
	for _, order := range s.commercialOrders {
		if s.canSeeCommercialStudent(principal, order.StudentID) {
			out = append(out, order)
		}
	}
	return out
}

func (s *MemoryStore) CreateCommercialOrder(operator string, principal learning.Principal, req learning.CommercialOrderCreateRequest) (learning.CommercialOrder, error) {
	if !canManageCommercial(principal) {
		return learning.CommercialOrder{}, errors.New("没有权限创建订单")
	}
	req.StudentID = strings.TrimSpace(req.StudentID)
	req.PackageID = strings.TrimSpace(req.PackageID)
	if req.AmountCent <= 0 {
		return learning.CommercialOrder{}, errors.New("订单金额必须大于 0")
	}
	if req.LessonTotal <= 0 {
		return learning.CommercialOrder{}, errors.New("请填写课时数")
	}
	student, err := s.visibleStudent(principal, req.StudentID)
	if err != nil {
		return learning.CommercialOrder{}, err
	}
	pkg, ok := s.findPackage(req.PackageID)
	if !ok || pkg.Status != learning.StatusEnabled {
		return learning.CommercialOrder{}, errors.New("请选择可售学习套餐")
	}
	now := time.Now()
	order := learning.CommercialOrder{
		ID:             "order-" + now.Format("20060102150405.000000000"),
		OrderNo:        "SL" + now.Format("20060102150405"),
		StudentID:      student.ID,
		StudentName:    student.Name,
		PackageID:      pkg.ID,
		PackageName:    pkg.Name,
		AmountCent:     req.AmountCent,
		LessonTotal:    req.LessonTotal,
		Status:         "待支付",
		ContractStatus: "待签署",
		InvoiceStatus:  "未开票",
		CreatedAt:      now.Format("2006-01-02 15:04:05"),
	}
	s.commercialOrders = append([]learning.CommercialOrder{order}, s.commercialOrders...)
	s.prependLogDetail(operator, "创建订单", order.StudentName+" / "+order.PackageName, "金额分: "+itoa(order.AmountCent))
	return order, nil
}

func (s *MemoryStore) CreatePayment(operator string, principal learning.Principal, orderID string, req learning.PaymentCreateRequest) (learning.PaymentRecord, error) {
	if !canManageCommercial(principal) {
		return learning.PaymentRecord{}, errors.New("没有权限登记收款")
	}
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.PaymentRecord{}, err
	}
	req.Method = strings.TrimSpace(req.Method)
	req.TransactionNo = strings.TrimSpace(req.TransactionNo)
	if req.AmountCent <= 0 {
		return learning.PaymentRecord{}, errors.New("收款金额必须大于 0")
	}
	if order.PaidAmountCent+req.AmountCent > order.AmountCent {
		return learning.PaymentRecord{}, errors.New("收款金额不能超过订单金额")
	}
	if req.Method == "" {
		req.Method = "线下收款"
	}
	now := time.Now()
	payment := learning.PaymentRecord{ID: "pay-" + now.Format("20060102150405.000000000"), OrderID: order.ID, AmountCent: req.AmountCent, Method: req.Method, TransactionNo: req.TransactionNo, PaidAt: now.Format("2006-01-02 15:04:05"), Status: "已确认"}
	order.PaidAmountCent += req.AmountCent
	if order.PaidAmountCent >= order.AmountCent {
		order.Status = "已支付"
		if _, err := s.CreateGrant(operator, order.StudentID, order.PackageID); err != nil && !strings.Contains(err.Error(), "已有生效套餐") {
			return learning.PaymentRecord{}, err
		}
	} else {
		order.Status = "部分支付"
	}
	s.commercialOrders[index] = order
	s.payments = append([]learning.PaymentRecord{payment}, s.payments...)
	s.prependLogDetail(operator, "登记收款", order.StudentName+" / "+order.PackageName, "金额分: "+itoa(payment.AmountCent))
	return payment, nil
}

func (s *MemoryStore) CreateRefund(operator string, principal learning.Principal, orderID string, req learning.RefundCreateRequest) (learning.RefundRecord, error) {
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.RefundRecord{}, err
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.AmountCent <= 0 {
		return learning.RefundRecord{}, errors.New("退款金额必须大于 0")
	}
	if order.RefundedAmountCent+req.AmountCent > order.PaidAmountCent {
		return learning.RefundRecord{}, errors.New("退款金额不能超过实收金额")
	}
	if req.Reason == "" {
		req.Reason = "家长申请退款"
	}
	now := time.Now()
	refund := learning.RefundRecord{ID: "refund-" + now.Format("20060102150405.000000000"), OrderID: order.ID, AmountCent: req.AmountCent, Reason: req.Reason, RefundedAt: now.Format("2006-01-02 15:04:05"), Status: "已退款"}
	order.RefundedAmountCent += req.AmountCent
	if order.RefundedAmountCent >= order.PaidAmountCent {
		order.Status = "已退款"
	} else {
		order.Status = "部分退款"
	}
	s.commercialOrders[index] = order
	s.refunds = append([]learning.RefundRecord{refund}, s.refunds...)
	s.prependLogDetail(operator, "登记退款", order.StudentName+" / "+order.PackageName, req.Reason)
	return refund, nil
}

func (s *MemoryStore) CreateContract(operator string, principal learning.Principal, orderID string, req learning.ContractCreateRequest) (learning.ContractRecord, error) {
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.ContractRecord{}, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Signer = strings.TrimSpace(req.Signer)
	if req.Title == "" {
		req.Title = order.PackageName + " 学习服务协议"
	}
	if req.Signer == "" {
		req.Signer = order.StudentName + "家长"
	}
	now := time.Now()
	contract := learning.ContractRecord{ID: "contract-" + now.Format("20060102150405.000000000"), OrderID: order.ID, Title: req.Title, Signer: req.Signer, SignedAt: now.Format("2006-01-02 15:04:05"), Status: "已签署"}
	order.ContractStatus = "已签署"
	s.commercialOrders[index] = order
	s.contracts = append([]learning.ContractRecord{contract}, s.contracts...)
	s.prependLog(operator, "签署合同", order.StudentName+" / "+contract.Title)
	return contract, nil
}

func (s *MemoryStore) CreateInvoice(operator string, principal learning.Principal, orderID string, req learning.InvoiceCreateRequest) (learning.InvoiceRecord, error) {
	index, order, err := s.commercialOrderForWrite(principal, orderID)
	if err != nil {
		return learning.InvoiceRecord{}, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.InvoiceNo = strings.TrimSpace(req.InvoiceNo)
	if req.Title == "" {
		return learning.InvoiceRecord{}, errors.New("请输入发票抬头")
	}
	available := order.PaidAmountCent - order.RefundedAmountCent
	if req.AmountCent <= 0 {
		req.AmountCent = available
	}
	if req.AmountCent <= 0 || req.AmountCent > available {
		return learning.InvoiceRecord{}, errors.New("开票金额不能超过订单可开票金额")
	}
	now := time.Now()
	invoice := learning.InvoiceRecord{ID: "invoice-" + now.Format("20060102150405.000000000"), OrderID: order.ID, Title: req.Title, TaxNo: strings.TrimSpace(req.TaxNo), AmountCent: req.AmountCent, InvoiceNo: req.InvoiceNo, IssuedAt: now.Format("2006-01-02 15:04:05"), Status: "已开票"}
	order.InvoiceStatus = "已开票"
	s.commercialOrders[index] = order
	s.invoices = append([]learning.InvoiceRecord{invoice}, s.invoices...)
	s.prependLogDetail(operator, "开具发票", order.StudentName+" / "+invoice.Title, "金额分: "+itoa(invoice.AmountCent))
	return invoice, nil
}

func (s *MemoryStore) CreateLessonConsumption(operator string, principal learning.Principal, req learning.LessonConsumptionCreateRequest) (learning.LessonConsumption, error) {
	index, order, err := s.commercialOrderForWrite(principal, req.OrderID)
	if err != nil {
		return learning.LessonConsumption{}, err
	}
	if req.LessonCount <= 0 {
		return learning.LessonConsumption{}, errors.New("课消数必须大于 0")
	}
	if order.LessonConsumed+req.LessonCount > order.LessonTotal {
		return learning.LessonConsumption{}, errors.New("课消不能超过订单剩余课时")
	}
	now := time.Now()
	item := learning.LessonConsumption{ID: "lesson-" + now.Format("20060102150405.000000000"), OrderID: order.ID, StudentID: order.StudentID, ScheduleClassID: strings.TrimSpace(req.ScheduleClassID), LessonCount: req.LessonCount, ConsumedAt: now.Format("2006-01-02 15:04:05"), Remark: strings.TrimSpace(req.Remark)}
	order.LessonConsumed += req.LessonCount
	if order.LessonTotal-order.LessonConsumed <= 3 && order.Status != "已退款" {
		order.Status = "待续费"
	}
	s.commercialOrders[index] = order
	s.lessonConsumptions = append([]learning.LessonConsumption{item}, s.lessonConsumptions...)
	s.prependLogDetail(operator, "登记课消", order.StudentName+" / "+order.PackageName, "课时: "+itoa(item.LessonCount))
	return item, nil
}

func (s *MemoryStore) CreateRenewalReminder(operator string, principal learning.Principal, req learning.RenewalReminderCreateRequest) (learning.RenewalReminder, error) {
	_, order, err := s.commercialOrderForWrite(principal, req.OrderID)
	if err != nil {
		return learning.RenewalReminder{}, err
	}
	req.Reason = strings.TrimSpace(req.Reason)
	req.DueAt = strings.TrimSpace(req.DueAt)
	if req.Reason == "" {
		req.Reason = "课时即将用完"
	}
	if req.DueAt == "" {
		req.DueAt = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}
	item := learning.RenewalReminder{ID: "renewal-" + time.Now().Format("20060102150405.000000000"), OrderID: order.ID, StudentID: order.StudentID, Reason: req.Reason, DueAt: req.DueAt, Status: "待跟进"}
	s.renewalReminders = append([]learning.RenewalReminder{item}, s.renewalReminders...)
	s.prependLog(operator, "创建续费提醒", order.StudentName+" / "+item.Reason)
	return item, nil
}

func (s *MemoryStore) CreateParentNotice(operator string, principal learning.Principal, req learning.ParentNoticeCreateRequest) (learning.ParentNotice, error) {
	_, order, err := s.commercialOrderForWrite(principal, req.OrderID)
	if err != nil {
		return learning.ParentNotice{}, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" || req.Content == "" {
		return learning.ParentNotice{}, errors.New("请输入通知标题和内容")
	}
	now := time.Now()
	item := learning.ParentNotice{ID: "parent-notice-" + now.Format("20060102150405.000000000"), OrderID: order.ID, StudentID: order.StudentID, Title: req.Title, Content: req.Content, SentAt: now.Format("2006-01-02 15:04:05"), Status: "已发送"}
	s.parentNotices = append([]learning.ParentNotice{item}, s.parentNotices...)
	s.notices = append([]learning.Notice{{ID: item.ID, Type: "续", Title: item.Title, Target: order.StudentName, Summary: item.Content, Status: "已发送"}}, s.notices...)
	s.prependLog(operator, "发送家长通知", order.StudentName+" / "+item.Title)
	return item, nil
}

func (s *MemoryStore) Courses(principal learning.Principal) []learning.Course {
	out := make([]learning.Course, 0, len(s.courses))
	for _, course := range s.courses {
		if canSeeCourse(principal, course) {
			out = append(out, s.decorateCourse(course))
		}
	}
	return out
}

func (s *MemoryStore) CreateCourse(operator string, principal learning.Principal, req learning.CourseUpsertRequest) (learning.Course, error) {
	course, err := s.courseFromRequest(principal, "", req)
	if err != nil {
		return learning.Course{}, err
	}
	if s.courseNameExists("", course.Name) {
		return learning.Course{}, errors.New("课程名称已存在")
	}
	course.ID = "course-custom-" + time.Now().Format("20060102150405.000000000")
	s.courses = append([]learning.Course{course}, s.courses...)
	s.prependLog(operator, "创建课程", course.Name)
	return s.decorateCourse(course), nil
}

func (s *MemoryStore) UpdateCourse(operator string, principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error) {
	id = strings.TrimSpace(id)
	course, err := s.courseFromRequest(principal, id, req)
	if err != nil {
		return learning.Course{}, err
	}
	if s.courseNameExists(id, course.Name) {
		return learning.Course{}, errors.New("课程名称已存在")
	}
	for index := range s.courses {
		if s.courses[index].ID != id {
			continue
		}
		before := s.decorateCourse(s.courses[index])
		s.courses[index] = course
		s.syncCourseReferences(course)
		after := s.decorateCourse(course)
		s.prependLogDetail(operator, "编辑课程", course.Name, auditChangeDetail(courseAuditSnapshot(before), courseAuditSnapshot(after)))
		return after, nil
	}
	return learning.Course{}, errors.New("课程不存在")
}

func (s *MemoryStore) Materials(principal learning.Principal) []learning.Material {
	courses := courseNames(s.Courses(principal))
	return s.materialsForCourses(courses)
}

func (s *MemoryStore) CreateMaterial(operator string, principal learning.Principal, req learning.MaterialUploadRequest) (learning.Material, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.Chapter = strings.TrimSpace(req.Chapter)
	if req.Title == "" {
		return learning.Material{}, errors.New("请输入讲义标题")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Material{}, err
	}
	if !canUploadHandout(principal) {
		return learning.Material{}, errors.New("当前账号没有上传讲义权限，请联系管理员开通")
	}
	if req.Chapter == "" {
		req.Chapter = "未分章节"
	}
	asset := req.File
	s.fileAssets[asset.ID] = asset
	item := learning.Material{
		ID:               "material-" + time.Now().Format("20060102150405.000000000"),
		Title:            req.Title,
		CourseID:         course.ID,
		Course:           course.Name,
		LearningSpaceID:  course.LearningSpaceID,
		Chapter:          req.Chapter,
		Type:             "讲义",
		OwnerTeacherID:   principal.UserID,
		OwnerTeacherName: principal.Name,
		PublishStatus:    "已发布",
		Status:           learning.StatusEnabled,
		FileID:           asset.ID,
		FileName:         asset.FileName,
		FileSize:         asset.FileSize,
		FileType:         asset.FileType,
		PreviewStatus:    asset.PreviewStatus,
		PreviewURL:       "/api/files/" + asset.ID + "/preview",
		DownloadURL:      "/api/files/" + asset.ID + "/download",
	}
	s.materials = append([]learning.Material{item}, s.materials...)
	s.prependLog(operator, "上传讲义", item.Title)
	return item, nil
}

func (s *MemoryStore) UpdateMaterial(operator string, principal learning.Principal, id string, req learning.MaterialUpdateRequest) (learning.Material, error) {
	id = strings.TrimSpace(id)
	req.Title = strings.TrimSpace(req.Title)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.Chapter = strings.TrimSpace(req.Chapter)
	if req.Title == "" {
		return learning.Material{}, errors.New("请输入讲义标题")
	}
	if !isContentStatus(req.Status) {
		return learning.Material{}, errors.New("请选择正确的发布状态")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Material{}, err
	}
	if !canUploadHandout(principal) {
		return learning.Material{}, errors.New("当前账号没有维护讲义权限，请联系管理员开通")
	}
	if req.Chapter == "" {
		req.Chapter = "未分章节"
	}
	for index := range s.materials {
		if s.materials[index].ID != id {
			continue
		}
		if !canSeeCourse(principal, learning.Course{ID: s.materials[index].CourseID, LearningSpaceID: s.materials[index].LearningSpaceID}) {
			return learning.Material{}, errors.New("不能维护未负责的讲义")
		}
		before := s.materials[index]
		s.materials[index].Title = req.Title
		s.materials[index].CourseID = course.ID
		s.materials[index].Course = course.Name
		s.materials[index].LearningSpaceID = course.LearningSpaceID
		s.materials[index].Chapter = req.Chapter
		s.materials[index].Status = req.Status
		s.materials[index].PublishStatus = publishStatus(req.Status)
		s.prependLogDetail(operator, "编辑讲义", req.Title, auditChangeDetail(materialAuditSnapshot(before), materialAuditSnapshot(s.materials[index])))
		return s.materials[index], nil
	}
	return learning.Material{}, errors.New("讲义不存在")
}

func (s *MemoryStore) Homework(principal learning.Principal) []learning.Homework {
	courses := courseNames(s.Courses(principal))
	return s.homeworkForCourses(courses)
}

func (s *MemoryStore) Questions(principal learning.Principal) []learning.QuestionBankItem {
	out := make([]learning.QuestionBankItem, 0, len(s.questionBank))
	for _, item := range s.questionBank {
		if canSeeQuestionScope(principal, item.Grade, item.Semester, item.Subject, s.learningSpaces) {
			out = append(out, item)
		}
	}
	return out
}

func (s *MemoryStore) CreateQuestion(operator string, principal learning.Principal, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	item, err := s.questionFromRequest("qb-"+time.Now().Format("20060102150405.000000000"), principal, req)
	if err != nil {
		return learning.QuestionBankItem{}, err
	}
	item.OwnerTeacherID = principal.UserID
	item.OwnerTeacherName = principal.Name
	now := time.Now().Format("2006-01-02 15:04:05")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.questionBank = append([]learning.QuestionBankItem{item}, s.questionBank...)
	s.prependLog(operator, "新增题库题目", item.Grade+" "+item.Semester+" "+item.Subject)
	return item, nil
}

func (s *MemoryStore) UpdateQuestion(operator string, principal learning.Principal, id string, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	id = strings.TrimSpace(id)
	for index := range s.questionBank {
		if s.questionBank[index].ID != id {
			continue
		}
		if !canEditQuestion(principal, s.questionBank[index]) {
			return learning.QuestionBankItem{}, errors.New("只能编辑自己创建或有管理权限的题目")
		}
		item, err := s.questionFromRequest(id, principal, req)
		if err != nil {
			return learning.QuestionBankItem{}, err
		}
		item.OwnerTeacherID = s.questionBank[index].OwnerTeacherID
		item.OwnerTeacherName = s.questionBank[index].OwnerTeacherName
		item.CreatedAt = s.questionBank[index].CreatedAt
		item.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
		before := s.questionBank[index]
		s.questionBank[index] = item
		s.refreshHomeworkQuestionSnapshots(item.ID)
		s.prependLogDetail(operator, "编辑题库题目", item.Stem, auditChangeDetail(map[string]any{"stem": before.Stem, "status": before.Status}, map[string]any{"stem": item.Stem, "status": item.Status}))
		return item, nil
	}
	return learning.QuestionBankItem{}, errors.New("题目不存在")
}

func (s *MemoryStore) questionFromRequest(id string, principal learning.Principal, req learning.QuestionBankUpsertRequest) (learning.QuestionBankItem, error) {
	req.Grade = strings.TrimSpace(req.Grade)
	req.Semester = strings.TrimSpace(req.Semester)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Type = strings.TrimSpace(req.Type)
	req.Stem = strings.TrimSpace(req.Stem)
	req.Answer = strings.TrimSpace(req.Answer)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = string(learning.StatusEnabled)
	}
	if req.Grade == "" || req.Semester == "" || req.Subject == "" {
		return learning.QuestionBankItem{}, errors.New("请选择年级、学期和学科")
	}
	if !canSeeQuestionScope(principal, req.Grade, req.Semester, req.Subject, s.learningSpaces) {
		return learning.QuestionBankItem{}, errors.New("不能维护未负责范围的题库")
	}
	if req.Stem == "" {
		return learning.QuestionBankItem{}, errors.New("请输入题干")
	}
	if req.Type != "single" && req.Type != "multiple" && req.Type != "text" {
		return learning.QuestionBankItem{}, errors.New("请选择正确的题型")
	}
	if !isContentStatus(learning.Status(status)) {
		return learning.QuestionBankItem{}, errors.New("请选择正确的发布状态")
	}
	options := cleanPhrases(req.Options)
	answers := cleanPhrases(req.Answers)
	if req.Type == "single" {
		if len(options) < 2 || req.Answer == "" {
			return learning.QuestionBankItem{}, errors.New("单选题需要至少两个选项和一个正确答案")
		}
		answers = []string{req.Answer}
	}
	if req.Type == "multiple" {
		if len(options) < 2 || len(answers) == 0 {
			return learning.QuestionBankItem{}, errors.New("多选题需要至少两个选项和正确答案")
		}
	}
	if req.Type == "text" {
		options = nil
		answers = nil
		req.Answer = ""
	}
	score := req.Score
	if score <= 0 {
		score = 10
	}
	return learning.QuestionBankItem{
		ID: id, Grade: req.Grade, Semester: req.Semester, Subject: req.Subject, Type: req.Type, Stem: req.Stem,
		Options: options, Answer: req.Answer, Answers: answers, Score: score, Status: status,
	}, nil
}

func (s *MemoryStore) CreateHomework(operator string, principal learning.Principal, req learning.HomeworkUploadRequest) (learning.Homework, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.Deadline = strings.TrimSpace(req.Deadline)
	if req.Title == "" {
		return learning.Homework{}, errors.New("请输入题目标题")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Homework{}, err
	}
	if !canUploadQuestion(principal) {
		return learning.Homework{}, errors.New("当前账号没有上传题目权限，请联系管理员开通")
	}
	questions, err := s.questionsForHomework(course, req.QuestionIDs)
	if err != nil {
		return learning.Homework{}, err
	}
	asset := req.File
	if asset.ID != "" {
		s.fileAssets[asset.ID] = asset
	}
	status := learning.Status(strings.TrimSpace(req.Status))
	if status == "" {
		status = learning.StatusEnabled
	}
	if !isContentStatus(status) {
		return learning.Homework{}, errors.New("请选择正确的发布状态")
	}
	item := learning.Homework{
		ID:               "homework-" + time.Now().Format("20060102150405.000000000"),
		Title:            req.Title,
		PackageName:      course.Subject + "题",
		CourseID:         course.ID,
		Course:           course.Name,
		LearningSpaceID:  course.LearningSpaceID,
		Grade:            course.Grade,
		Semester:         s.semesterForSpace(course.LearningSpaceID),
		Subject:          course.Subject,
		QuestionNum:      len(questions),
		QuestionIDs:      questionIDs(questions),
		Questions:        questions,
		Deadline:         req.Deadline,
		OwnerTeacherID:   principal.UserID,
		OwnerTeacherName: principal.Name,
		PublishStatus:    publishStatus(status),
		Status:           string(status),
		FileID:           asset.ID,
		FileName:         asset.FileName,
		FileSize:         asset.FileSize,
		FileType:         asset.FileType,
		PreviewStatus:    asset.PreviewStatus,
		PreviewURL:       "/api/files/" + asset.ID + "/preview",
		DownloadURL:      "/api/files/" + asset.ID + "/download",
	}
	s.homework = append([]learning.Homework{item}, s.homework...)
	s.prependLog(operator, "上传题目", item.Title)
	return item, nil
}

func (s *MemoryStore) UpdateHomework(operator string, principal learning.Principal, id string, req learning.HomeworkUpdateRequest) (learning.Homework, error) {
	id = strings.TrimSpace(id)
	req.Title = strings.TrimSpace(req.Title)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	req.Deadline = strings.TrimSpace(req.Deadline)
	status := learning.Status(strings.TrimSpace(req.Status))
	if req.Title == "" {
		return learning.Homework{}, errors.New("请输入题目标题")
	}
	if !isContentStatus(status) {
		return learning.Homework{}, errors.New("请选择正确的发布状态")
	}
	course, err := s.courseForUpload(principal, req.CourseID, req.LearningSpaceID)
	if err != nil {
		return learning.Homework{}, err
	}
	if !canUploadQuestion(principal) {
		return learning.Homework{}, errors.New("当前账号没有维护题目权限，请联系管理员开通")
	}
	questions, err := s.questionsForHomework(course, req.QuestionIDs)
	if err != nil {
		return learning.Homework{}, err
	}
	for index := range s.homework {
		if s.homework[index].ID != id {
			continue
		}
		if !canSeeCourse(principal, learning.Course{ID: s.homework[index].CourseID, LearningSpaceID: s.homework[index].LearningSpaceID}) {
			return learning.Homework{}, errors.New("不能维护未负责的题目")
		}
		before := s.homework[index]
		s.homework[index].Title = req.Title
		s.homework[index].PackageName = course.Subject + "题"
		s.homework[index].CourseID = course.ID
		s.homework[index].Course = course.Name
		s.homework[index].LearningSpaceID = course.LearningSpaceID
		s.homework[index].Grade = course.Grade
		s.homework[index].Semester = s.semesterForSpace(course.LearningSpaceID)
		s.homework[index].Subject = course.Subject
		s.homework[index].QuestionIDs = questionIDs(questions)
		s.homework[index].Questions = questions
		s.homework[index].QuestionNum = len(questions)
		s.homework[index].Deadline = req.Deadline
		s.homework[index].Status = string(status)
		s.homework[index].PublishStatus = publishStatus(status)
		s.prependLogDetail(operator, "编辑题目", req.Title, auditChangeDetail(homeworkAuditSnapshot(before), homeworkAuditSnapshot(s.homework[index])))
		return s.homework[index], nil
	}
	return learning.Homework{}, errors.New("题目不存在")
}

func (s *MemoryStore) ContentFile(principal learning.Principal, fileID string) (learning.FileAsset, error) {
	asset, ok := s.fileAssets[fileID]
	if !ok {
		return learning.FileAsset{}, errors.New("文件不存在")
	}
	for _, material := range s.Materials(principal) {
		if material.FileID == fileID {
			return asset, nil
		}
	}
	for _, item := range s.Homework(principal) {
		if item.FileID == fileID {
			return asset, nil
		}
	}
	return learning.FileAsset{}, errors.New("没有权限查看该文件")
}
func (s *MemoryStore) Reviews(principal learning.Principal) []learning.Review {
	subjects := subjectsForCourses(s.Courses(principal))
	out := make([]learning.Review, 0, len(s.reviews))
	for _, review := range s.reviews {
		if canSeeSubject(principal, subjects, review.PackageName) || canSeeSubject(principal, subjects, review.Homework) {
			out = append(out, review)
		}
	}
	return out
}

func (s *MemoryStore) CompleteReview(operator string, principal learning.Principal, id string, req learning.ReviewCompleteRequest) (learning.Submission, error) {
	if !canReviewHomework(principal) {
		return learning.Submission{}, errors.New("当前账号没有批改权限，请联系管理员开通")
	}
	id = strings.TrimSpace(id)
	req.TeacherComment = strings.TrimSpace(req.TeacherComment)
	req.Reward = strings.TrimSpace(req.Reward)
	if req.Score < 0 || req.Score > 100 {
		return learning.Submission{}, errors.New("分数需在 0 到 100 之间")
	}
	if req.TeacherComment == "" {
		return learning.Submission{}, errors.New("请填写给学生看的评语")
	}
	reviewIndex := -1
	var review learning.Review
	for index, item := range s.reviews {
		if item.ID == id {
			reviewIndex = index
			review = item
			break
		}
	}
	if reviewIndex < 0 {
		return learning.Submission{}, errors.New("待批改记录不存在")
	}
	visible := false
	for _, item := range s.Reviews(principal) {
		if item.ID == id {
			visible = true
			break
		}
	}
	if !visible {
		return learning.Submission{}, errors.New("没有权限批改该练习")
	}
	if review.StudentID == "" || review.HomeworkID == "" {
		return learning.Submission{}, errors.New("待批改记录缺少学生或题目信息")
	}
	homework, ok := s.findHomework(review.HomeworkID)
	if !ok {
		return learning.Submission{}, errors.New("题目不存在")
	}
	if req.Reward == "" {
		req.Reward = rewardForScore(req.Score)
	}
	submission, ok := s.submissions[review.SubmissionID]
	if !ok {
		submission = learning.Submission{
			ID:         "sub-review-" + id,
			HomeworkID: homework.ID,
			StudentID:  review.StudentID,
			TaskTitle:  homework.Title,
			CreatedAt:  time.Now().Format("2006-01-02 15:04:05"),
		}
	}
	submission.Score = req.Score
	submission.FinalScore = req.Score
	if submission.ObjectiveScore == 0 {
		submission.ObjectiveScore = review.SystemScore
	}
	submission.TeacherComment = req.TeacherComment
	submission.Reward = req.Reward
	submission.Status = "已批改"
	s.submissions[submission.ID] = submission
	s.reviews = append(s.reviews[:reviewIndex], s.reviews[reviewIndex+1:]...)
	s.notices = append([]learning.Notice{{
		ID:      "notice-review-" + time.Now().Format("20060102150405.000000000"),
		Type:    "评",
		Title:   "批改完成提醒",
		Target:  review.StudentName,
		Summary: homework.Title + "已完成批改，快去查看老师反馈。",
		Status:  "自动发送",
	}}, s.notices...)
	s.prependLog(operator, "完成批改", review.StudentName+" · "+homework.Title)
	return submission, nil
}

func (s *MemoryStore) Notices(principal learning.Principal) []learning.Notice {
	subjects := subjectsForCourses(s.Courses(principal))
	out := make([]learning.Notice, 0, len(s.notices))
	for _, notice := range s.notices {
		if canSeeSubject(principal, subjects, notice.Target) || canSeeSubject(principal, subjects, notice.Title) {
			out = append(out, notice)
		}
	}
	return out
}

func (s *MemoryStore) CreateNotice(operator string, principal learning.Principal, req learning.NoticeCreateRequest) (learning.Notice, error) {
	req.Type = strings.TrimSpace(req.Type)
	req.Title = strings.TrimSpace(req.Title)
	req.Target = strings.TrimSpace(req.Target)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Type == "" {
		req.Type = "通知"
	}
	if req.Title == "" {
		return learning.Notice{}, errors.New("请输入通知标题")
	}
	if req.Target == "" {
		return learning.Notice{}, errors.New("请选择或填写接收对象")
	}
	if req.Summary == "" {
		return learning.Notice{}, errors.New("请输入通知内容")
	}
	if !s.canSendNoticeTo(principal, req.Target, req.Title, req.Summary) {
		return learning.Notice{}, errors.New("不能发送到未负责的学生范围")
	}
	notice := learning.Notice{
		ID:      "notice-" + time.Now().Format("20060102150405.000000000"),
		Type:    req.Type,
		Title:   req.Title,
		Target:  req.Target,
		Summary: req.Summary,
		Status:  "已发送",
	}
	s.notices = append([]learning.Notice{notice}, s.notices...)
	s.prependLog(operator, "发送通知", notice.Target+" / "+notice.Title)
	return notice, nil
}

func (s *MemoryStore) Logs() []learning.OperationLog {
	return append([]learning.OperationLog(nil), s.logs...)
}

func (s *MemoryStore) Settings() map[string]string {
	out := make(map[string]string, len(s.settings))
	for key, value := range s.settings {
		out[key] = value
	}
	return out
}

func (s *MemoryStore) UpdateSetting(operator string, req learning.SettingUpdateRequest) (map[string]string, error) {
	req.Key = strings.TrimSpace(req.Key)
	req.Value = strings.TrimSpace(req.Value)
	if req.Key == "" {
		return nil, errors.New("请选择要修改的设置项")
	}
	if req.Value == "" {
		return nil, errors.New("设置值不能为空")
	}
	if _, ok := s.settings[req.Key]; !ok {
		return nil, errors.New("设置项不存在")
	}
	before := map[string]string{req.Key: s.settings[req.Key]}
	s.settings[req.Key] = req.Value
	after := map[string]string{req.Key: s.settings[req.Key]}
	s.prependLogDetail(operator, "修改系统设置", settingLabel(req.Key), auditChangeDetail(before, after))
	return s.Settings(), nil
}

func (s *MemoryStore) GrantPreview(studentID, packageID string) (learning.GrantPreview, error) {
	student, ok := s.findStudent(studentID)
	if !ok {
		return learning.GrantPreview{}, errors.New("student not found")
	}
	pkg, ok := s.findPackage(packageID)
	if !ok {
		return learning.GrantPreview{}, errors.New("package not found")
	}

	openCourses, openMaterials, openHomework := s.openContentForPackage(pkg)
	alreadyOpened, existingUntil := s.activeGrantState(student.ID, pkg.ID)
	return learning.GrantPreview{
		StudentID: student.ID, PackageID: pkg.ID, StudentName: student.Name, PackageName: pkg.Name,
		AlreadyOpened: alreadyOpened, ExistingUntil: existingUntil,
		LearningSpaces: s.learningSpaceNamesForPackage(pkg.ID), ContentTypes: s.contentTypeLabelsForPackage(pkg.ID),
		OpenCourses: openCourses, OpenMaterials: openMaterials, OpenHomework: openHomework,
		BlockedContent: s.blockedContentForPackage(pkg), EffectiveDefault: "今天起 365 天",
	}, nil
}

func (s *MemoryStore) CreateGrant(operator, studentID, packageID string) (learning.GrantPreview, error) {
	preview, err := s.GrantPreview(studentID, packageID)
	if err != nil {
		return learning.GrantPreview{}, err
	}
	if !s.hasGrant(studentID, packageID) {
		startsAt := time.Now().Format("2006-01-02")
		endsAt := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
		grant := packageGrant{
			ID:             "grant-" + time.Now().Format("20060102150405"),
			StudentID:      studentID,
			PackageID:      packageID,
			StartsAt:       startsAt,
			EndsAt:         endsAt,
			Status:         "active",
			EffectiveUntil: endsAt,
		}
		s.grants = append(s.grants, grant)
		s.syncSpaceAccessForGrant(grant)
		s.addStudentOpenedPackage(studentID, preview.PackageName)
	}
	s.prependLog(operator, "开通套餐", preview.StudentName+" / "+preview.PackageName)
	return preview, nil
}

func (s *MemoryStore) StudentPermissions() []learning.StudentPermissionSummary {
	out := make([]learning.StudentPermissionSummary, 0, len(s.students))
	for _, student := range s.students {
		out = append(out, s.permissionForStudent(student))
	}
	return out
}

func (s *MemoryStore) PackagePermissions() []learning.PackagePermissionSummary {
	out := make([]learning.PackagePermissionSummary, 0, len(s.packages))
	for _, pkg := range s.packages {
		students := make([]string, 0)
		for _, grant := range s.grants {
			if grant.PackageID != pkg.ID || !grantActive(grant) {
				continue
			}
			student, ok := s.findStudent(grant.StudentID)
			if ok {
				students = appendUnique(students, student.Name)
			}
		}
		courses, materials, homework := s.openContentForPackage(pkg)
		out = append(out, learning.PackagePermissionSummary{
			PackageID: pkg.ID, PackageName: pkg.Name, Status: pkg.Status, OpenedStudents: len(students),
			Students: students, LearningSpaces: s.learningSpaceNamesForPackage(pkg.ID), ContentTypes: s.contentTypeLabelsForPackage(pkg.ID),
			OpenCourses: courses, OpenMaterials: materials, OpenHomework: homework,
		})
	}
	return out
}

func (s *MemoryStore) ContentPermissions() []learning.ContentPermissionSummary {
	out := make([]learning.ContentPermissionSummary, 0, len(s.courses)+len(s.materials)+len(s.homework))
	for _, course := range s.courses {
		packages, students := s.audienceForContent(course.LearningSpaceID, "course")
		out = append(out, learning.ContentPermissionSummary{
			ContentID: course.ID, ContentTitle: course.Name, ContentType: "课程", Course: course.Name,
			LearningSpace: s.learningSpaceName(course.LearningSpaceID), Status: string(course.Status),
			OpenedPackages: packages, OpenedStudents: students,
		})
	}
	for _, material := range s.materials {
		packages, students := s.audienceForContent(material.LearningSpaceID, "handout")
		out = append(out, learning.ContentPermissionSummary{
			ContentID: material.ID, ContentTitle: material.Title, ContentType: material.Type, Course: material.Course,
			LearningSpace: s.learningSpaceName(material.LearningSpaceID), OwnerTeacherName: material.OwnerTeacherName,
			Status: string(material.Status), OpenedPackages: packages, OpenedStudents: students,
		})
	}
	for _, item := range s.homework {
		packages, students := s.audienceForContent(item.LearningSpaceID, "question")
		out = append(out, learning.ContentPermissionSummary{
			ContentID: item.ID, ContentTitle: item.Title, ContentType: "小挑战", Course: item.Course,
			LearningSpace: s.learningSpaceName(item.LearningSpaceID), OwnerTeacherName: item.OwnerTeacherName,
			Status: item.Status, OpenedPackages: packages, OpenedStudents: students,
		})
	}
	return out
}

func (s *MemoryStore) StudentHome(principal learning.Principal) (learning.StudentHome, error) {
	if principal.StudentID == "" {
		return learning.StudentHome{}, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return learning.StudentHome{}, errors.New("student not found")
	}
	if student.AccountStatus == "停用" {
		return learning.StudentHome{}, errors.New("账号已停用，请联系老师或管理员")
	}
	courses := s.coursesForStudent(student.ID)
	materials := s.materialsForStudent(student.ID)
	homework := s.homeworkForStudent(student.ID)
	continueCourse := learning.Course{}
	if len(courses) > 0 {
		continueCourse = courses[0]
	}
	if len(materials) == 0 {
		materials = []learning.Material{}
	}
	if len(homework) == 0 {
		homework = []learning.Homework{}
	}
	return learning.StudentHome{
		Student:          student,
		ContinueCourse:   continueCourse,
		ContinueProgress: s.courseProgress(student.ID, continueCourse.ID),
		PendingHomework:  homework,
		Notices:          s.noticesForStudent(student),
		Materials:        materials,
	}, nil
}

// StudentStudy 返回学习页聚合数据：可学课程（带真实进度）与资料。
func (s *MemoryStore) StudentStudy(principal learning.Principal) (learning.StudentStudyBoard, error) {
	if principal.StudentID == "" {
		return learning.StudentStudyBoard{}, errors.New("student account is not bound")
	}
	courses := s.coursesForStudent(principal.StudentID)
	cards := make([]learning.StudentCourseCard, 0, len(courses))
	for _, course := range courses {
		cards = append(cards, learning.StudentCourseCard{
			Course:   course,
			Progress: s.courseProgress(principal.StudentID, course.ID),
		})
	}
	materials := s.materialsForStudent(principal.StudentID)
	if len(materials) == 0 {
		materials = []learning.Material{}
	}
	return learning.StudentStudyBoard{Courses: cards, Materials: materials}, nil
}

// StudentTasks 返回任务列表，studentStatus 由提交记录派生（已完成/待完成）。
func (s *MemoryStore) StudentTasks(principal learning.Principal) ([]learning.StudentTask, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	homework := s.homeworkForStudent(principal.StudentID)
	tasks := make([]learning.StudentTask, 0, len(homework))
	for _, item := range homework {
		task := learning.StudentTask{Homework: item, StudentStatus: "待完成"}
		if sub, ok := s.latestSubmission(principal.StudentID, item.ID); ok {
			if sub.Status == "待批改" {
				task.StudentStatus = "批改中"
			} else {
				task.StudentStatus = "已完成"
			}
			task.Score = sub.Score
			task.SubmissionID = sub.ID
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// courseProgress 复用学习地图站点，计算某课程对当前学生的真实完成度。
func (s *MemoryStore) courseProgress(studentID, courseID string) int {
	if studentID == "" || courseID == "" {
		return 0
	}
	materials := make([]learning.Material, 0)
	for _, material := range s.materialsForStudent(studentID) {
		if material.CourseID == courseID {
			materials = append(materials, material)
		}
	}
	homework := make([]learning.Homework, 0)
	for _, item := range s.homeworkForStudent(studentID) {
		if item.CourseID == courseID {
			homework = append(homework, item)
		}
	}
	return stationProgress(s.buildStations(studentID, materials, homework))
}

// latestSubmission 返回某学生对某小挑战最近一次提交。
func (s *MemoryStore) latestSubmission(studentID, homeworkID string) (learning.Submission, bool) {
	var latest learning.Submission
	found := false
	for _, sub := range s.submissions {
		if sub.StudentID != studentID || sub.HomeworkID != homeworkID {
			continue
		}
		if !found || sub.CreatedAt > latest.CreatedAt {
			latest = sub
			found = true
		}
	}
	return latest, found
}

func (s *MemoryStore) Availability(principal learning.Principal, ownerType, ownerID string) ([]learning.AvailabilitySlot, error) {
	ownerType = strings.TrimSpace(ownerType)
	ownerID = strings.TrimSpace(ownerID)
	if ownerType == "" || ownerID == "" {
		return nil, errors.New("请选择要查看的老师或学生")
	}
	if !s.canManageAvailability(principal, ownerType, ownerID) {
		return nil, errors.New("没有权限查看该可用时间")
	}
	out := make([]learning.AvailabilitySlot, 0)
	for _, slot := range s.availability {
		if slot.OwnerType == ownerType && slot.OwnerID == ownerID {
			out = append(out, slot)
		}
	}
	sortAvailability(out)
	return out, nil
}

func (s *MemoryStore) AvailabilityOverview(principal learning.Principal) []learning.AvailabilitySlot {
	out := make([]learning.AvailabilitySlot, 0)
	for _, slot := range s.availability {
		if s.canManageAvailability(principal, slot.OwnerType, slot.OwnerID) {
			out = append(out, slot)
		}
	}
	sortAvailability(out)
	return out
}

func (s *MemoryStore) SaveAvailability(operator string, principal learning.Principal, req learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error) {
	req.OwnerType = strings.TrimSpace(req.OwnerType)
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	if req.OwnerType == "" || req.OwnerID == "" {
		return nil, errors.New("请选择要维护的老师或学生")
	}
	if !s.canManageAvailability(principal, req.OwnerType, req.OwnerID) {
		return nil, errors.New("没有权限维护该可用时间")
	}
	ownerName, err := s.availabilityOwnerName(req.OwnerType, req.OwnerID)
	if err != nil {
		return nil, err
	}
	slots := make([]learning.AvailabilitySlot, 0, len(req.Slots))
	for index, slot := range req.Slots {
		slot.OwnerType = req.OwnerType
		slot.OwnerID = req.OwnerID
		slot.OwnerName = ownerName
		slot.StartTime = strings.TrimSpace(slot.StartTime)
		slot.EndTime = strings.TrimSpace(slot.EndTime)
		slot.StartDate = strings.TrimSpace(slot.StartDate)
		slot.EndDate = strings.TrimSpace(slot.EndDate)
		slot.Remark = strings.TrimSpace(slot.Remark)
		if slot.DayOfWeek < 1 || slot.DayOfWeek > 7 {
			return nil, errors.New("请选择星期")
		}
		start, ok := parseClock(slot.StartTime)
		if !ok {
			return nil, errors.New("开始时间格式应为 HH:mm")
		}
		end, ok := parseClock(slot.EndTime)
		if !ok || end <= start {
			return nil, errors.New("结束时间必须晚于开始时间")
		}
		slot.ID = "av-" + req.OwnerType + "-" + req.OwnerID + "-" + strconv.Itoa(index+1)
		slots = append(slots, slot)
	}
	next := make([]learning.AvailabilitySlot, 0, len(s.availability)+len(slots))
	for _, slot := range s.availability {
		if slot.OwnerType == req.OwnerType && slot.OwnerID == req.OwnerID {
			continue
		}
		next = append(next, slot)
	}
	next = append(next, slots...)
	if err := s.replaceAvailabilitySlots(req.OwnerType, req.OwnerID, slots); err != nil {
		return nil, err
	}
	s.availability = next
	sortAvailability(slots)
	s.prependLog(operator, "维护可上课时间", ownerName)
	return slots, nil
}

func (s *MemoryStore) ScheduleCandidates(principal learning.Principal, req learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error) {
	req.Subject = strings.TrimSpace(req.Subject)
	req.Grade = strings.TrimSpace(req.Grade)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.ClassType = strings.TrimSpace(req.ClassType)
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 90
	}
	capacity := classCapacity(req.ClassType)
	if capacity <= 0 {
		return nil, errors.New("请选择正确班型")
	}
	minStudents := minClassStudents(capacity)

	// 解析目标课程：优先按「学科 + 年级」入口，其次兼容旧的按课程入口。
	var targetCourses []learning.Course
	if req.CourseID != "" {
		course, err := s.courseForScheduling(principal, req.CourseID)
		if err != nil {
			return nil, err
		}
		targetCourses = []learning.Course{course}
		req.Subject = course.Subject
		req.Grade = course.Grade
	} else {
		if req.Subject == "" || req.Grade == "" {
			return nil, errors.New("请选择学科和年级")
		}
		for _, course := range s.courses {
			if course.Status == learning.StatusEnabled && course.Subject == req.Subject && course.Grade == req.Grade && canSeeCourse(principal, course) {
				targetCourses = append(targetCourses, course)
			}
		}
		if len(targetCourses) == 0 {
			return nil, errors.New("没有该学科 + 年级的可排课程")
		}
	}
	repCourse := targetCourses[0]
	spaceIDs := make([]string, 0, len(targetCourses))
	for _, course := range targetCourses {
		spaceIDs = appendUnique(spaceIDs, course.LearningSpaceID)
	}

	// 可授课老师：teacher 角色、当前账号有权管理、且授课范围覆盖该学科 + 年级。
	teachers := make([]learning.User, 0)
	for _, user := range s.users {
		if !hasRole(user.Roles, learning.RoleTeacher) {
			continue
		}
		if req.TeacherID != "" && user.ID != req.TeacherID {
			continue
		}
		if !canManageTeacher(principal, user) && principal.UserID != user.ID {
			continue
		}
		if !intersects(user.LearningSpaceIDs, spaceIDs) {
			continue
		}
		teachers = append(teachers, user)
	}

	// 适配学生：同年级 + 已开通同学科，确保「只有同年级同学科的才能排一起」。
	eligible := make([]learning.CandidateStudent, 0)
	for _, student := range s.students {
		decorated := s.decorateStudent(student)
		if !canSeeStudent(principal, decorated, s.coursesForStudent(student.ID)) {
			continue
		}
		if decorated.Grade != req.Grade || !s.studentHasSubjectGrade(student.ID, req.Subject, req.Grade) {
			continue
		}
		eligible = append(eligible, learning.CandidateStudent{
			ID: student.ID, Name: decorated.Name, Grade: decorated.Grade, OpenedPackages: decorated.OpenedPackages,
		})
	}

	candidates := make([]learning.ScheduleCandidate, 0)
	for _, teacher := range teachers {
		for _, teacherSlot := range s.ownerAvailability("teacher", teacher.ID) {
			startMin, _ := parseClock(teacherSlot.StartTime)
			endMin, _ := parseClock(teacherSlot.EndTime)
			for candidateStart := startMin; candidateStart+req.DurationMinutes <= endMin; candidateStart += 30 {
				candidateEnd := candidateStart + req.DurationMinutes
				if s.hasScheduleConflict("teacher", teacher.ID, teacherSlot.DayOfWeek, candidateStart, candidateEnd) {
					continue
				}
				available := make([]learning.CandidateStudent, 0)
				missing := make([]learning.CandidateStudent, 0)
				for _, student := range eligible {
					if !s.hasScheduleConflict("student", student.ID, teacherSlot.DayOfWeek, candidateStart, candidateEnd) &&
						s.studentAvailable(student.ID, teacherSlot.DayOfWeek, candidateStart, candidateEnd) {
						available = append(available, student)
					} else {
						missing = append(missing, student)
					}
				}
				// 保留差一两人就能成班的近似方案，交给「协调建议」面板处理。
				if len(available) == 0 {
					continue
				}
				score := len(available) * 20
				if len(available) >= capacity {
					score += 40
				} else if len(available) >= minStudents {
					score += 10
				}
				candidates = append(candidates, learning.ScheduleCandidate{
					ID:                "candidate-" + teacher.ID + "-" + strconv.Itoa(teacherSlot.DayOfWeek) + "-" + minutesToClock(candidateStart),
					DayOfWeek:         teacherSlot.DayOfWeek,
					StartTime:         minutesToClock(candidateStart),
					EndTime:           minutesToClock(candidateEnd),
					TeacherID:         teacher.ID,
					TeacherName:       teacher.Name,
					CourseID:          repCourse.ID,
					CourseName:        repCourse.Name,
					Subject:           req.Subject,
					Grade:             req.Grade,
					ClassType:         req.ClassType,
					Capacity:          capacity,
					AvailableStudents: available,
					MissingStudents:   missing,
					StudentCount:      len(available),
					Score:             score,
					Reason:            candidateReason(len(available), capacity),
				})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].DayOfWeek == candidates[j].DayOfWeek {
				return candidates[i].StartTime < candidates[j].StartTime
			}
			return candidates[i].DayOfWeek < candidates[j].DayOfWeek
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates, nil
}

func (s *MemoryStore) ScheduleClasses(principal learning.Principal) []learning.ScheduleClass {
	out := make([]learning.ScheduleClass, 0)
	for _, item := range s.scheduleClasses {
		if s.canSeeScheduleClass(principal, item) {
			out = append(out, item)
		}
	}
	return out
}

func (s *MemoryStore) CreateScheduleClass(operator string, principal learning.Principal, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	item, err := s.buildScheduleClass(principal, "", req)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	item.ID = "schedule-" + time.Now().Format("20060102150405.000000000")
	item.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	if err := s.insertScheduleClass(item); err != nil {
		return learning.ScheduleClass{}, err
	}
	s.scheduleClasses = append([]learning.ScheduleClass{item}, s.scheduleClasses...)
	s.prependLog(operator, "确认排课", item.Name+" / "+item.TeacherName)
	return item, nil
}

func (s *MemoryStore) buildScheduleClass(principal learning.Principal, exceptID string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.ClassType = strings.TrimSpace(req.ClassType)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 90
	}
	course, err := s.courseForScheduling(principal, req.CourseID)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	teacher, ok := s.findUser(req.TeacherID)
	if !ok || !hasRole(teacher.Roles, learning.RoleTeacher) {
		return learning.ScheduleClass{}, errors.New("请选择老师")
	}
	capacity := classCapacity(req.ClassType)
	if capacity <= 0 {
		return learning.ScheduleClass{}, errors.New("请选择正确班型")
	}
	if len(req.StudentIDs) < minClassStudents(capacity) {
		return learning.ScheduleClass{}, errors.New("学生人数不足，暂不能成班")
	}
	if len(req.StudentIDs) > capacity {
		return learning.ScheduleClass{}, errors.New("学生人数超过班型容量")
	}
	startMin, ok := parseClock(req.StartTime)
	if !ok {
		return learning.ScheduleClass{}, errors.New("开始时间格式应为 HH:mm")
	}
	endMin, ok := parseClock(req.EndTime)
	if !ok || endMin <= startMin {
		return learning.ScheduleClass{}, errors.New("结束时间必须晚于开始时间")
	}
	if req.DayOfWeek < 1 || req.DayOfWeek > 7 {
		return learning.ScheduleClass{}, errors.New("请选择星期")
	}
	if s.hasScheduleConflictExcept("teacher", teacher.ID, req.DayOfWeek, startMin, endMin, exceptID) {
		return learning.ScheduleClass{}, errors.New("老师该时间已有课程")
	}
	students := make([]learning.CandidateStudent, 0, len(req.StudentIDs))
	seen := map[string]bool{}
	for _, studentID := range req.StudentIDs {
		studentID = strings.TrimSpace(studentID)
		if studentID == "" || seen[studentID] {
			continue
		}
		seen[studentID] = true
		student, err := s.visibleStudent(principal, studentID)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		if student.Grade != course.Grade {
			return learning.ScheduleClass{}, errors.New(student.Name + " 与班级年级不一致，只有同年级才能排一起")
		}
		if !s.studentHasSubjectGrade(student.ID, course.Subject, course.Grade) {
			return learning.ScheduleClass{}, errors.New(student.Name + " 未开通该学科，只有同学科才能排一起")
		}
		if !s.studentAvailable(student.ID, req.DayOfWeek, startMin, endMin) {
			return learning.ScheduleClass{}, errors.New(student.Name + " 该时间不可上课")
		}
		if s.hasScheduleConflictExcept("student", student.ID, req.DayOfWeek, startMin, endMin, exceptID) {
			return learning.ScheduleClass{}, errors.New(student.Name + " 该时间已有课程")
		}
		students = append(students, learning.CandidateStudent{ID: student.ID, Name: student.Name, Grade: student.Grade, OpenedPackages: student.OpenedPackages})
	}
	return learning.ScheduleClass{
		Name:            course.Subject + " " + req.ClassType + " 小班",
		CourseID:        course.ID,
		CourseName:      course.Name,
		TeacherID:       teacher.ID,
		TeacherName:     teacher.Name,
		ClassType:       req.ClassType,
		Capacity:        capacity,
		DurationMinutes: req.DurationMinutes,
		DayOfWeek:       req.DayOfWeek,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		StartDate:       req.StartDate,
		EndDate:         req.EndDate,
		Students:        students,
		Status:          "已确认",
	}, nil
}

func (s *MemoryStore) UpdateScheduleClass(operator string, principal learning.Principal, id string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return learning.ScheduleClass{}, errors.New("请选择要调整的课程")
	}
	for index, existing := range s.scheduleClasses {
		if existing.ID != id {
			continue
		}
		if !s.canSeeScheduleClass(principal, existing) {
			return learning.ScheduleClass{}, errors.New("没有权限调整该课程")
		}
		if hasRole(principal.Roles, learning.RoleTeacher) || hasRole(principal.Roles, learning.RoleStudent) {
			return learning.ScheduleClass{}, errors.New("请联系教务调整课程")
		}
		if existing.Status == "已取消" {
			return learning.ScheduleClass{}, errors.New("已取消课程不能调课")
		}
		item, err := s.buildScheduleClass(principal, id, req)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		if err := s.updateScheduleClass(item); err != nil {
			return learning.ScheduleClass{}, err
		}
		s.scheduleClasses[index] = item
		s.prependLogDetail(operator, "调整排课", item.Name+" / "+item.TeacherName, auditChangeDetail(scheduleClassAuditSnapshot(existing), scheduleClassAuditSnapshot(item)))
		return item, nil
	}
	return learning.ScheduleClass{}, errors.New("课程不存在")
}

func (s *MemoryStore) CancelScheduleClass(operator string, principal learning.Principal, id string) (learning.ScheduleClass, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return learning.ScheduleClass{}, errors.New("请选择要取消的课程")
	}
	for index, item := range s.scheduleClasses {
		if item.ID != id {
			continue
		}
		if !s.canSeeScheduleClass(principal, item) {
			return learning.ScheduleClass{}, errors.New("没有权限取消该课程")
		}
		if hasRole(principal.Roles, learning.RoleTeacher) || hasRole(principal.Roles, learning.RoleStudent) {
			return learning.ScheduleClass{}, errors.New("请联系教务调整课程")
		}
		if item.Status == "已取消" {
			return item, nil
		}
		before := item
		item.Status = "已取消"
		if err := s.updateScheduleClassStatus(item.ID, item.Status); err != nil {
			return learning.ScheduleClass{}, err
		}
		s.scheduleClasses[index] = item
		s.prependLogDetail(operator, "取消排课", item.Name+" / "+item.TeacherName, auditChangeDetail(scheduleClassAuditSnapshot(before), scheduleClassAuditSnapshot(item)))
		return item, nil
	}
	return learning.ScheduleClass{}, errors.New("课程不存在")
}

func (s *MemoryStore) StudentSchedule(principal learning.Principal) ([]learning.ScheduleClass, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	out := make([]learning.ScheduleClass, 0)
	for _, item := range s.scheduleClasses {
		for _, student := range item.Students {
			if student.ID == principal.StudentID {
				out = append(out, item)
				break
			}
		}
	}
	return out, nil
}

func (s *MemoryStore) StudentCourseDetail(principal learning.Principal, courseID string) (learning.StudentCourseDetail, error) {
	if principal.StudentID == "" {
		return learning.StudentCourseDetail{}, errors.New("student account is not bound")
	}
	courseID = strings.TrimSpace(courseID)
	var course learning.Course
	found := false
	for _, item := range s.coursesForStudent(principal.StudentID) {
		if item.ID == courseID {
			course = item
			found = true
			break
		}
	}
	if !found {
		return learning.StudentCourseDetail{}, errors.New("课程不存在或未开通")
	}
	materials := make([]learning.Material, 0)
	for _, material := range s.materialsForStudent(principal.StudentID) {
		if material.CourseID == courseID {
			materials = append(materials, material)
		}
	}
	homework := make([]learning.Homework, 0)
	for _, item := range s.homeworkForStudent(principal.StudentID) {
		if item.CourseID == courseID {
			homework = append(homework, item)
		}
	}
	stations := s.buildStations(principal.StudentID, materials, homework)
	return learning.StudentCourseDetail{
		Course:    course,
		Materials: materials,
		Homework:  homework,
		Stations:  stations,
		Progress:  stationProgress(stations),
	}, nil
}

// StudentGrowth 返回成长轨迹：提交记录 + 已学资料，按时间倒序。
func (s *MemoryStore) StudentGrowth(principal learning.Principal) ([]learning.StudentLearningRecord, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	records := make([]learning.StudentLearningRecord, 0)
	for _, sub := range s.submissions {
		if sub.StudentID != principal.StudentID {
			continue
		}
		records = append(records, learning.StudentLearningRecord{
			ID: "growth-" + sub.ID, Type: "小挑战", Title: sub.TaskTitle, Status: sub.Status,
			Score: sub.Score, OccurredAt: sub.CreatedAt, Description: sub.TeacherComment,
		})
	}
	student, _ := s.findStudent(principal.StudentID)
	for _, material := range s.materialsForStudent(principal.StudentID) {
		records = append(records, learning.StudentLearningRecord{
			ID: "growth-mat-" + material.ID, Type: "资料", Title: material.Title, Course: material.Course,
			Status: "已学习", OccurredAt: firstNonEmpty(student.LastStudyAt, "2026-05-22 18:20:00"), Description: "查看课件资料",
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].OccurredAt > records[j].OccurredAt })
	return records, nil
}

// StudentBadges 返回徽章墙，是否获得由学生真实学习数据派生。
func (s *MemoryStore) StudentBadges(principal learning.Principal) ([]learning.Badge, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	student, ok := s.findStudent(principal.StudentID)
	if !ok {
		return nil, errors.New("student not found")
	}
	materialCount := len(s.materialsForStudent(principal.StudentID))
	submissionCount := 0
	hasSubmission := false
	for _, sub := range s.submissions {
		if sub.StudentID == principal.StudentID {
			submissionCount++
			hasSubmission = true
		}
	}
	return []learning.Badge{
		{ID: "badge-reading", Icon: "⭐", Name: "阅读小星星", Desc: "完成第一次小挑战", Obtained: hasSubmission},
		{ID: "badge-streak", Icon: "🔥", Name: "坚持不懈", Desc: "连续学习满 7 天", Obtained: student.StreakDays >= 7},
		{ID: "badge-expert", Icon: "🏅", Name: "学习小达人", Desc: "平均分达到 90", Obtained: student.AverageScore >= 90},
		{ID: "badge-explorer", Icon: "🧭", Name: "探索者", Desc: "学习满 5 份讲义", Obtained: materialCount >= 5},
		{ID: "badge-challenger", Icon: "🎯", Name: "挑战王", Desc: "提交满 3 次小挑战", Obtained: submissionCount >= 3},
	}, nil
}

// StudentFavorites 返回当前学生的收藏列表，按收藏时间倒序。
func (s *MemoryStore) StudentFavorites(principal learning.Principal) ([]learning.Favorite, error) {
	if principal.StudentID == "" {
		return nil, errors.New("student account is not bound")
	}
	out := make([]learning.Favorite, 0)
	for _, fav := range s.favorites {
		if fav.StudentID == principal.StudentID {
			out = append(out, fav)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// AddFavorite 收藏一条内容，校验该内容在学生权限范围内，幂等。
func (s *MemoryStore) AddFavorite(operator string, principal learning.Principal, req learning.FavoriteRequest) (learning.Favorite, error) {
	if principal.StudentID == "" {
		return learning.Favorite{}, errors.New("student account is not bound")
	}
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.TargetID = strings.TrimSpace(req.TargetID)
	var title, course string
	switch req.TargetType {
	case "material":
		material, err := s.StudentMaterial(principal, req.TargetID)
		if err != nil {
			return learning.Favorite{}, err
		}
		title, course = material.Title, material.Course
	case "homework":
		homework, err := s.StudentHomework(principal, req.TargetID)
		if err != nil {
			return learning.Favorite{}, err
		}
		title, course = homework.Title, homework.Course
	default:
		return learning.Favorite{}, errors.New("不支持的收藏类型")
	}
	for _, fav := range s.favorites {
		if fav.StudentID == principal.StudentID && fav.TargetType == req.TargetType && fav.TargetID == req.TargetID {
			return fav, nil
		}
	}
	fav := learning.Favorite{
		ID:         "fav-" + time.Now().Format("20060102150405.000000000"),
		StudentID:  principal.StudentID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Title:      title,
		Course:     course,
		CreatedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
	s.favorites[fav.ID] = fav
	s.prependLog(operator, "收藏内容", title)
	return fav, nil
}

// RemoveFavorite 取消收藏，仅允许删除本人的收藏。
func (s *MemoryStore) RemoveFavorite(operator string, principal learning.Principal, id string) error {
	if principal.StudentID == "" {
		return errors.New("student account is not bound")
	}
	id = strings.TrimSpace(id)
	fav, ok := s.favorites[id]
	if !ok {
		return errors.New("收藏记录不存在")
	}
	if fav.StudentID != principal.StudentID {
		return errors.New("没有权限取消该收藏")
	}
	delete(s.favorites, id)
	s.prependLog(operator, "取消收藏", fav.Title)
	return nil
}

func (s *MemoryStore) StudentMaterial(principal learning.Principal, materialID string) (learning.Material, error) {
	if principal.StudentID == "" {
		return learning.Material{}, errors.New("student account is not bound")
	}
	materialID = strings.TrimSpace(materialID)
	for _, material := range s.materialsForStudent(principal.StudentID) {
		if material.ID == materialID {
			return material, nil
		}
	}
	return learning.Material{}, errors.New("资料不存在或未开通")
}

func (s *MemoryStore) StudentHomework(principal learning.Principal, homeworkID string) (learning.Homework, error) {
	if principal.StudentID == "" {
		return learning.Homework{}, errors.New("student account is not bound")
	}
	homeworkID = strings.TrimSpace(homeworkID)
	for _, item := range s.homeworkForStudent(principal.StudentID) {
		if item.ID == homeworkID {
			return item, nil
		}
	}
	return learning.Homework{}, errors.New("题目不存在或未开通")
}

func (s *MemoryStore) CreateSubmission(operator string, principal learning.Principal, req learning.SubmissionRequest) (learning.Submission, error) {
	homework, err := s.StudentHomework(principal, req.HomeworkID)
	if err != nil {
		return learning.Submission{}, err
	}
	if len(req.Answers) == 0 {
		return learning.Submission{}, errors.New("请先作答再提交")
	}
	score, hasText := gradeSubmission(homework, req.Answers)
	status := "已批改"
	comment := commentForScore(score, homework.Title)
	reward := rewardForScore(score)
	if hasText {
		status = "待批改"
		comment = "客观题已完成，简答题老师正在批改。"
		reward = ""
	}
	submission := learning.Submission{
		ID:             "sub-" + time.Now().Format("20060102150405.000000000"),
		HomeworkID:     homework.ID,
		StudentID:      principal.StudentID,
		TaskTitle:      homework.Title,
		Score:          score,
		ObjectiveScore: score,
		FinalScore:     score,
		TeacherComment: comment,
		Reward:         reward,
		Status:         status,
		CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		Answers:        req.Answers,
	}
	s.submissions[submission.ID] = submission
	if hasText {
		student, _ := s.findStudent(principal.StudentID)
		s.reviews = append([]learning.Review{{
			ID: "rev-" + submission.ID, StudentID: principal.StudentID, HomeworkID: homework.ID, SubmissionID: submission.ID,
			StudentName: student.Name, PackageName: homework.PackageName, Homework: homework.Title, SystemScore: score,
			TeacherComment: "", Status: "待批改",
		}}, s.reviews...)
	}
	s.prependLog(operator, "提交小挑战", homework.Title)
	return submission, nil
}

func (s *MemoryStore) StudentSubmission(principal learning.Principal, id string) (learning.Submission, error) {
	if principal.StudentID == "" {
		return learning.Submission{}, errors.New("student account is not bound")
	}
	id = strings.TrimSpace(id)
	submission, ok := s.submissions[id]
	if !ok {
		return learning.Submission{}, errors.New("提交记录不存在")
	}
	if submission.StudentID != principal.StudentID {
		return learning.Submission{}, errors.New("没有权限查看该批改结果")
	}
	return submission, nil
}

func (s *MemoryStore) hasSubmission(studentID, homeworkID string) bool {
	for _, submission := range s.submissions {
		if submission.StudentID == studentID && submission.HomeworkID == homeworkID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) buildStations(studentID string, materials []learning.Material, homework []learning.Homework) []learning.Station {
	stations := make([]learning.Station, 0, len(materials)+len(homework))
	for i, material := range materials {
		status := "学习中"
		desc := "正在学习，继续加油"
		if i > 0 {
			status = "待挑战"
			desc = "完成上一站后继续阅读"
		}
		stations = append(stations, learning.Station{
			Icon:       "📖",
			Title:      "第 " + strconv.Itoa(len(stations)+1) + " 站 " + material.Title,
			Desc:       desc,
			Status:     status,
			MaterialID: material.ID,
		})
	}
	for _, item := range homework {
		status := "待挑战"
		desc := "完成小挑战即可解锁奖励"
		if s.hasSubmission(studentID, item.ID) {
			status = "已完成"
			desc = "已提交，等待老师反馈"
		}
		stations = append(stations, learning.Station{
			Icon:       "🎯",
			Title:      "第 " + strconv.Itoa(len(stations)+1) + " 站 " + item.Title,
			Desc:       desc,
			Status:     status,
			HomeworkID: item.ID,
		})
	}
	return stations
}

func stationProgress(stations []learning.Station) int {
	if len(stations) == 0 {
		return 0
	}
	done := 0
	for _, station := range stations {
		if station.Status == "已完成" {
			done++
		}
	}
	return done * 100 / len(stations)
}

func gradeSubmission(homework learning.Homework, answers []learning.SubmissionAnswer) (int, bool) {
	if len(homework.Questions) == 0 {
		return 90, false
	}
	answerMap := make(map[string]learning.SubmissionAnswer, len(answers))
	for _, answer := range answers {
		answerMap[answer.QuestionID] = answer
	}
	totalScore := 0
	gotScore := 0
	hasText := false
	for _, question := range homework.Questions {
		score := question.Score
		if score <= 0 {
			score = 10
		}
		totalScore += score
		answer := answerForQuestion(answerMap, question.ID)
		if question.Type == "single" {
			if strings.EqualFold(strings.TrimSpace(answer.Choice), strings.TrimSpace(question.Answer)) {
				gotScore += score
			}
			continue
		}
		if question.Type == "multiple" {
			if sameChoiceSet(answer.Choices, normalizedQuestionAnswers(question)) {
				gotScore += score
			}
			continue
		}
		hasText = true
	}
	if totalScore == 0 {
		return 0, hasText
	}
	return gotScore * 100 / totalScore, hasText
}

func answerForQuestion(answerMap map[string]learning.SubmissionAnswer, questionID string) learning.SubmissionAnswer {
	if answer, ok := answerMap[questionID]; ok {
		return answer
	}
	if index := strings.LastIndex(questionID, "-q"); index >= 0 {
		if answer, ok := answerMap[questionID[index+1:]]; ok {
			return answer
		}
	}
	return learning.SubmissionAnswer{}
}

func commentForScore(score int, title string) string {
	switch {
	case score >= 90:
		return title + "完成得很棒，重点都抓住啦，继续保持！"
	case score >= 60:
		return title + "整体不错，个别地方再细心一点就更好了。"
	default:
		return title + "已经迈出第一步啦，跟着讲义再复习一遍，下次一定更好。"
	}
}

func rewardForScore(score int) string {
	if score >= 90 {
		return "获得「学习之星」徽章 ⭐"
	}
	if score >= 60 {
		return "获得 10 点能量值 ⚡"
	}
	return "完成即可获得 5 点能量值"
}

func (s *MemoryStore) findStudent(id string) (learning.Student, bool) {
	for _, student := range s.students {
		if student.ID == id {
			return s.decorateStudent(student), true
		}
	}
	return learning.Student{}, false
}

func (s *MemoryStore) visibleStudent(principal learning.Principal, id string) (learning.Student, error) {
	student, ok := s.findStudent(id)
	if !ok {
		return learning.Student{}, errors.New("student not found")
	}
	if !canSeeStudent(principal, student, s.coursesForStudent(student.ID)) {
		return learning.Student{}, errors.New("没有权限访问该学生")
	}
	return student, nil
}

func (s *MemoryStore) decorateStudent(student learning.Student) learning.Student {
	if user, ok := s.findUserByStudentID(student.ID); ok && strings.TrimSpace(user.OpenID) != "" {
		student.BindStatus = "已绑定"
	} else if student.BindStatus == "" {
		student.BindStatus = "待绑定"
	}
	effectiveUntil := ""
	packages := make([]string, 0)
	for _, grant := range s.grants {
		if grant.StudentID != student.ID {
			continue
		}
		if grantEndsAt(grant) > effectiveUntil {
			effectiveUntil = grantEndsAt(grant)
		}
		if pkg, ok := s.findPackage(grant.PackageID); ok {
			packages = appendUnique(packages, pkg.Name)
		}
	}
	if effectiveUntil != "" {
		student.EffectiveUntil = effectiveUntil
	}
	if len(packages) > 0 {
		student.OpenedPackages = packages
	}
	return student
}

func (s *MemoryStore) findUserByStudentID(studentID string) (learning.User, bool) {
	for _, user := range s.users {
		if user.StudentID == studentID {
			return user, true
		}
	}
	return learning.User{}, false
}

func (s *MemoryStore) phoneExists(currentStudentID, phone string) bool {
	for _, student := range s.students {
		if student.ID != currentStudentID && student.Phone == phone {
			return true
		}
	}
	for _, user := range s.users {
		if user.StudentID != currentStudentID && user.Phone == phone {
			return true
		}
	}
	return false
}

func (s *MemoryStore) syncStudentUser(student learning.Student) {
	for i := range s.users {
		if s.users[i].StudentID != student.ID {
			continue
		}
		s.users[i].Name = student.Name
		s.users[i].Phone = student.Phone
		s.users[i].AccountStatus = student.AccountStatus
		return
	}
	s.users = append(s.users, learning.User{
		ID:            "user-" + student.ID,
		Name:          student.Name,
		Phone:         student.Phone,
		AccountStatus: student.AccountStatus,
		Roles:         []learning.Role{learning.RoleStudent},
		StudentID:     student.ID,
	})
}

func (s *MemoryStore) permissionForStudent(student learning.Student) learning.StudentPermissionSummary {
	packages := make([]string, 0)
	courses := make([]string, 0)
	materials := make([]string, 0)
	homework := make([]string, 0)
	learningSpaces := make([]string, 0)
	contentTypes := make([]string, 0)
	effectiveUntil := ""
	for _, grant := range s.grants {
		if grant.StudentID != student.ID || !grantActive(grant) {
			continue
		}
		pkg, ok := s.findPackage(grant.PackageID)
		if !ok {
			continue
		}
		packages = appendUnique(packages, pkg.Name)
		learningSpaces = appendUnique(learningSpaces, s.learningSpaceNamesForGrant(grant.ID)...)
		contentTypes = appendUnique(contentTypes, s.contentTypeLabelsForPackage(pkg.ID)...)
		pkgCourses, pkgMaterials, pkgHomework := s.openContentForStudentGrant(grant)
		courses = appendUnique(courses, pkgCourses...)
		materials = appendUnique(materials, pkgMaterials...)
		homework = appendUnique(homework, pkgHomework...)
		if grantEndsAt(grant) > effectiveUntil {
			effectiveUntil = grantEndsAt(grant)
		}
	}
	state := "未开通"
	if len(packages) > 0 {
		state = "生效中"
	}
	return learning.StudentPermissionSummary{
		StudentID: student.ID, StudentName: student.Name, Grade: student.Grade, AccountStatus: student.AccountStatus,
		OpenedPackages: packages, LearningSpaces: learningSpaces, ContentTypes: contentTypes,
		OpenCourses: courses, OpenMaterials: materials, OpenHomework: homework,
		EffectiveUntil: effectiveUntil, PermissionState: state,
	}
}

func (s *MemoryStore) noticesForStudent(student learning.Student) []learning.Notice {
	out := make([]learning.Notice, 0)
	courses := s.coursesForStudent(student.ID)
	subjects := subjectsForCourses(courses)
	for _, notice := range s.notices {
		if noticeMatchesStudent(notice, student, subjects) {
			out = append(out, notice)
		}
	}
	return out
}

func noticeMatchesStudent(notice learning.Notice, student learning.Student, subjects []string) bool {
	target := notice.Target + " " + notice.Title + " " + notice.Summary
	if strings.Contains(notice.Target, "全部") || strings.Contains(target, student.Name) || strings.Contains(target, student.Grade) {
		return true
	}
	for _, pkg := range student.OpenedPackages {
		if pkg != "" && strings.Contains(target, pkg) {
			return true
		}
	}
	for _, subject := range subjects {
		if subject != "" && strings.Contains(target, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) logsForStudent(student learning.Student) []learning.OperationLog {
	out := make([]learning.OperationLog, 0)
	for _, log := range s.logs {
		if strings.Contains(log.Target, student.Name) {
			out = append(out, log)
		}
	}
	return out
}

func (s *MemoryStore) findPackage(id string) (learning.Package, bool) {
	for _, pkg := range s.packages {
		if pkg.ID == id {
			return s.decoratePackage(pkg), true
		}
	}
	return learning.Package{}, false
}

func (s *MemoryStore) findHomework(id string) (learning.Homework, bool) {
	for _, item := range s.homework {
		if item.ID == id {
			return item, true
		}
	}
	return learning.Homework{}, false
}

func (s *MemoryStore) questionsForHomework(course learning.Course, ids []string) ([]learning.Question, error) {
	if len(ids) == 0 {
		return []learning.Question{}, nil
	}
	space, ok := s.findLearningSpace(course.LearningSpaceID)
	if !ok {
		return nil, errors.New("请选择正确的课程范围")
	}
	out := make([]learning.Question, 0, len(ids))
	for _, id := range ids {
		item, ok := s.findQuestionBankItem(id)
		if !ok {
			return nil, errors.New("题库题目不存在")
		}
		if item.Status != string(learning.StatusEnabled) {
			return nil, errors.New("只能选择启用的题库题目")
		}
		if item.Grade != space.Grade || item.Semester != space.Semester || item.Subject != space.Subject {
			return nil, errors.New("题目范围必须和发布课程范围一致")
		}
		out = append(out, bankItemQuestion(item))
	}
	return out, nil
}

func (s *MemoryStore) findQuestionBankItem(id string) (learning.QuestionBankItem, bool) {
	for _, item := range s.questionBank {
		if item.ID == id {
			return item, true
		}
	}
	return learning.QuestionBankItem{}, false
}

func (s *MemoryStore) semesterForSpace(id string) string {
	if space, ok := s.findLearningSpace(id); ok {
		return space.Semester
	}
	return ""
}

func (s *MemoryStore) refreshHomeworkQuestionSnapshots(questionID string) {
	for index := range s.homework {
		if !containsString(s.homework[index].QuestionIDs, questionID) {
			continue
		}
		questions := make([]learning.Question, 0, len(s.homework[index].QuestionIDs))
		for _, id := range s.homework[index].QuestionIDs {
			if item, ok := s.findQuestionBankItem(id); ok {
				questions = append(questions, bankItemQuestion(item))
			}
		}
		s.homework[index].Questions = questions
		s.homework[index].QuestionNum = len(questions)
	}
}

func (s *MemoryStore) packageFromRequest(id string, req learning.PackageUpsertRequest) (learning.Package, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.AcademicYear = strings.TrimSpace(req.AcademicYear)
	req.Grade = strings.TrimSpace(req.Grade)
	req.Semester = strings.TrimSpace(req.Semester)
	req.Subject = strings.TrimSpace(req.Subject)
	req.PhaseScope = strings.TrimSpace(req.PhaseScope)
	req.PackageType = strings.TrimSpace(req.PackageType)
	req.Summary = strings.TrimSpace(req.Summary)
	if req.Name == "" {
		return learning.Package{}, errors.New("请输入学习套餐名称")
	}
	if req.AcademicYear == "" {
		req.AcademicYear = "2026 学年"
	}
	if req.Grade == "" || req.Subject == "" || req.Semester == "" {
		return learning.Package{}, errors.New("请选择年级、学科和学期")
	}
	if req.PhaseScope == "" {
		req.PhaseScope = "全学期"
	}
	if req.PackageType == "" {
		req.PackageType = packageTypeLabel(req.ContentTypeCodes)
	}
	if req.Status == "" {
		req.Status = learning.StatusEnabled
	}
	if req.Status != learning.StatusEnabled && req.Status != learning.StatusDraft && req.Status != learning.StatusDisabled {
		return learning.Package{}, errors.New("套餐状态不正确")
	}
	if len(req.LearningSpaceIDs) == 0 {
		return learning.Package{}, errors.New("请选择套餐开放的学习空间")
	}
	for _, spaceID := range req.LearningSpaceIDs {
		if !s.learningSpaceExists(spaceID) {
			return learning.Package{}, errors.New("学习空间不存在：" + spaceID)
		}
		if !s.learningSpaceMatches(spaceID, req.Grade, req.Subject, req.Semester) {
			return learning.Package{}, errors.New("学习空间需与套餐年级、学科和学期一致")
		}
	}
	if len(req.ContentTypeCodes) == 0 {
		return learning.Package{}, errors.New("请选择套餐开放的内容类型")
	}
	for _, code := range req.ContentTypeCodes {
		if !validContentType(code) {
			return learning.Package{}, errors.New("内容类型不正确：" + code)
		}
	}
	return learning.Package{
		ID:           id,
		Name:         req.Name,
		AcademicYear: req.AcademicYear,
		Grade:        req.Grade,
		Semester:     req.Semester,
		Subject:      req.Subject,
		PhaseScope:   req.PhaseScope,
		PackageType:  req.PackageType,
		Summary:      req.Summary,
		Status:       req.Status,
	}, nil
}

func (s *MemoryStore) packageNameExists(currentID, name string) bool {
	for _, pkg := range s.packages {
		if pkg.ID != currentID && pkg.Name == name {
			return true
		}
	}
	return false
}

func (s *MemoryStore) courseFromRequest(principal learning.Principal, id string, req learning.CourseUpsertRequest) (learning.Course, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.LearningSpaceID = strings.TrimSpace(req.LearningSpaceID)
	if req.Name == "" {
		return learning.Course{}, errors.New("请输入课程名称")
	}
	if req.LearningSpaceID == "" {
		return learning.Course{}, errors.New("请选择课程所属学习空间")
	}
	if req.ChapterCount < 0 {
		return learning.Course{}, errors.New("章节数不能小于 0")
	}
	if req.Status == "" {
		req.Status = learning.StatusEnabled
	}
	if req.Status != learning.StatusEnabled && req.Status != learning.StatusDraft && req.Status != learning.StatusDisabled {
		return learning.Course{}, errors.New("课程状态不正确")
	}
	space, ok := s.findLearningSpace(req.LearningSpaceID)
	if !ok {
		return learning.Course{}, errors.New("学习空间不存在")
	}
	course := learning.Course{
		ID:              id,
		Name:            req.Name,
		Subject:         space.Subject,
		Grade:           space.Grade,
		LearningSpaceID: space.ID,
		ChapterCount:    req.ChapterCount,
		Status:          req.Status,
	}
	if !canSeeCourse(principal, course) {
		return learning.Course{}, errors.New("不能维护未负责的课程范围")
	}
	return course, nil
}

func (s *MemoryStore) courseNameExists(currentID, name string) bool {
	for _, course := range s.courses {
		if course.ID != currentID && course.Name == name {
			return true
		}
	}
	return false
}

func (s *MemoryStore) decorateCourse(course learning.Course) learning.Course {
	materialNum := 0
	for _, material := range s.materials {
		if material.CourseID == course.ID {
			materialNum++
		}
	}
	homeworkNum := 0
	for _, item := range s.homework {
		if item.CourseID == course.ID {
			homeworkNum++
		}
	}
	course.MaterialNum = materialNum
	course.HomeworkNum = homeworkNum
	return course
}

func (s *MemoryStore) syncCourseReferences(course learning.Course) {
	for index := range s.materials {
		if s.materials[index].CourseID == course.ID {
			s.materials[index].Course = course.Name
			s.materials[index].LearningSpaceID = course.LearningSpaceID
		}
	}
	for index := range s.homework {
		if s.homework[index].CourseID == course.ID {
			s.homework[index].Course = course.Name
			s.homework[index].LearningSpaceID = course.LearningSpaceID
		}
	}
}

func (s *MemoryStore) replacePackageRelations(packageID string, learningSpaceIDs []string, contentTypes []string) {
	nextSpaces := make([]packageSpace, 0, len(s.packageSpaces)+len(learningSpaceIDs))
	for _, relation := range s.packageSpaces {
		if relation.PackageID != packageID {
			nextSpaces = append(nextSpaces, relation)
		}
	}
	for _, spaceID := range learningSpaceIDs {
		nextSpaces = append(nextSpaces, packageSpace{PackageID: packageID, LearningSpaceID: spaceID})
	}
	s.packageSpaces = nextSpaces

	nextTypes := make([]packageContentType, 0, len(s.contentTypes)+len(contentTypes))
	for _, item := range s.contentTypes {
		if item.PackageID != packageID {
			nextTypes = append(nextTypes, item)
		}
	}
	for _, code := range contentTypes {
		nextTypes = append(nextTypes, packageContentType{PackageID: packageID, ContentType: code})
	}
	s.contentTypes = nextTypes
}

func (s *MemoryStore) refreshSpaceAccessForPackage(packageID string) {
	activeGrants := make([]packageGrant, 0)
	for _, grant := range s.grants {
		if grant.PackageID == packageID && grantActive(grant) {
			activeGrants = append(activeGrants, grant)
		}
	}
	nextAccess := make([]learningSpaceAccess, 0, len(s.spaceAccess))
	for _, access := range s.spaceAccess {
		remove := false
		for _, grant := range activeGrants {
			if access.PackageGrantID == grant.ID {
				remove = true
				break
			}
		}
		if !remove {
			nextAccess = append(nextAccess, access)
		}
	}
	s.spaceAccess = nextAccess
	for _, grant := range activeGrants {
		s.syncSpaceAccessForGrant(grant)
	}
}

func (s *MemoryStore) decoratePackage(pkg learning.Package) learning.Package {
	pkg.LearningSpaceIDs = s.learningSpaceIDsForPackage(pkg.ID)
	pkg.LearningSpaces = s.learningSpaceNamesForPackage(pkg.ID)
	pkg.ContentTypeCodes = s.contentTypesForPackage(pkg.ID)
	pkg.ContentTypes = s.contentTypeLabelsForPackage(pkg.ID)
	pkg.OpenStudentNum = 0
	for _, grant := range s.grants {
		if grant.PackageID == pkg.ID && grantActive(grant) {
			pkg.OpenStudentNum++
		}
	}
	return pkg
}

func (s *MemoryStore) openContentForPackage(pkg learning.Package) ([]string, []string, []string) {
	courses := make([]string, 0)
	materials := make([]string, 0)
	homework := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForPackage(pkg.ID)
	contentTypes := s.contentTypesForPackage(pkg.ID)
	for _, course := range s.courses {
		if course.Status != learning.StatusEnabled || !containsString(spaceIDs, course.LearningSpaceID) || !containsString(contentTypes, "course") {
			continue
		}
		courses = appendUnique(courses, course.Name)
	}
	for _, material := range s.materials {
		if material.Status == learning.StatusEnabled && containsString(spaceIDs, material.LearningSpaceID) && containsString(contentTypes, "handout") {
			materials = appendUnique(materials, material.Title)
		}
	}
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && containsString(spaceIDs, item.LearningSpaceID) && containsString(contentTypes, "question") {
			homework = appendUnique(homework, item.Title)
		}
	}
	return courses, materials, homework
}

func (s *MemoryStore) openContentForStudentGrant(grant packageGrant) ([]string, []string, []string) {
	courses := make([]string, 0)
	materials := make([]string, 0)
	homework := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
	contentTypes := s.contentTypesForPackage(grant.PackageID)
	for _, course := range s.courses {
		if course.Status == learning.StatusEnabled && containsString(spaceIDs, course.LearningSpaceID) && containsString(contentTypes, "course") {
			courses = appendUnique(courses, course.Name)
		}
	}
	for _, material := range s.materials {
		if material.Status == learning.StatusEnabled && containsString(spaceIDs, material.LearningSpaceID) && containsString(contentTypes, "handout") {
			materials = appendUnique(materials, material.Title)
		}
	}
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && containsString(spaceIDs, item.LearningSpaceID) && containsString(contentTypes, "question") {
			homework = appendUnique(homework, item.Title)
		}
	}
	return courses, materials, homework
}

func (s *MemoryStore) blockedContentForPackage(pkg learning.Package) []string {
	blocked := make([]string, 0)
	spaceIDs := s.learningSpaceIDsForPackage(pkg.ID)
	for _, space := range s.learningSpaces {
		if space.Status == learning.StatusDisabled || containsString(spaceIDs, space.ID) {
			continue
		}
		if space.Grade == pkg.Grade && space.Semester == pkg.Semester {
			blocked = appendUnique(blocked, space.Name)
		}
	}
	return blocked
}

func (s *MemoryStore) audienceForContent(learningSpaceID, contentType string) ([]string, []string) {
	packages := make([]string, 0)
	students := make([]string, 0)
	for _, pkg := range s.packages {
		if !containsString(s.learningSpaceIDsForPackage(pkg.ID), learningSpaceID) || !containsString(s.contentTypesForPackage(pkg.ID), contentType) {
			continue
		}
		packages = appendUnique(packages, pkg.Name)
		for _, grant := range s.grants {
			if grant.PackageID != pkg.ID || !grantActive(grant) || !s.grantOpensSpace(grant.ID, learningSpaceID) {
				continue
			}
			student, ok := s.findStudent(grant.StudentID)
			if ok {
				students = appendUnique(students, student.Name)
			}
		}
	}
	return packages, students
}

func (s *MemoryStore) hasGrant(studentID, packageID string) bool {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.PackageID == packageID && grant.Status != "revoked" {
			return true
		}
	}
	return false
}

func (s *MemoryStore) activeGrantState(studentID, packageID string) (bool, string) {
	for _, grant := range s.grants {
		if grant.StudentID == studentID && grant.PackageID == packageID && grantActive(grant) {
			return true, grantEndsAt(grant)
		}
	}
	return false, ""
}

func (s *MemoryStore) addStudentOpenedPackage(studentID, packageName string) {
	for i := range s.students {
		if s.students[i].ID == studentID {
			s.students[i].OpenedPackages = appendUnique(s.students[i].OpenedPackages, packageName)
			return
		}
	}
}

func (s *MemoryStore) coursesForStudent(studentID string) []learning.Course {
	out := make([]learning.Course, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || !containsString(s.contentTypesForPackage(grant.PackageID), "course") {
			continue
		}
		spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
		for _, course := range s.courses {
			if course.Status == learning.StatusEnabled && containsString(spaceIDs, course.LearningSpaceID) {
				out = appendCourseUnique(out, s.decorateCourse(course))
			}
		}
	}
	return out
}

// studentAccessibleSpaceIDs 返回学生通过有效套餐开通的全部学习空间 ID，不区分内容类型。
func (s *MemoryStore) studentAccessibleSpaceIDs(studentID string) []string {
	out := make([]string, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) {
			continue
		}
		out = appendUnique(out, s.learningSpaceIDsForGrant(grant.ID)...)
	}
	return out
}

// studentHasSubjectGrade 判断学生是否开通了某学科+年级，用于「只有同年级同学科才能排一起」。
func (s *MemoryStore) studentHasSubjectGrade(studentID, subject, grade string) bool {
	for _, id := range s.studentAccessibleSpaceIDs(studentID) {
		for _, space := range s.learningSpaces {
			if space.ID == id && space.Grade == grade && space.Subject == subject {
				return true
			}
		}
	}
	return false
}

func (s *MemoryStore) materialsForStudent(studentID string) []learning.Material {
	out := make([]learning.Material, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || !containsString(s.contentTypesForPackage(grant.PackageID), "handout") {
			continue
		}
		spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
		for _, material := range s.materials {
			if material.Status == learning.StatusEnabled && containsString(spaceIDs, material.LearningSpaceID) {
				out = appendMaterialUnique(out, material)
			}
		}
	}
	return out
}

func (s *MemoryStore) homeworkForStudent(studentID string) []learning.Homework {
	out := make([]learning.Homework, 0)
	for _, grant := range s.grants {
		if grant.StudentID != studentID || !grantActive(grant) || !containsString(s.contentTypesForPackage(grant.PackageID), "question") {
			continue
		}
		spaceIDs := s.learningSpaceIDsForGrant(grant.ID)
		for _, item := range s.homework {
			if homeworkVisible(item.Status) && containsString(spaceIDs, item.LearningSpaceID) {
				out = appendHomeworkUnique(out, item)
			}
		}
	}
	return out
}

func (s *MemoryStore) learningSpaceIDsForPackage(packageID string) []string {
	out := make([]string, 0)
	for _, relation := range s.packageSpaces {
		if relation.PackageID == packageID && s.learningSpaceEnabled(relation.LearningSpaceID) {
			out = appendUnique(out, relation.LearningSpaceID)
		}
	}
	return out
}

func (s *MemoryStore) learningSpaceIDsForGrant(grantID string) []string {
	out := make([]string, 0)
	for _, access := range s.spaceAccess {
		if access.PackageGrantID == grantID && access.Status == "active" && access.EndsAt >= time.Now().Format("2006-01-02") && s.learningSpaceEnabled(access.LearningSpaceID) {
			out = appendUnique(out, access.LearningSpaceID)
		}
	}
	return out
}

func (s *MemoryStore) learningSpaceNamesForPackage(packageID string) []string {
	names := make([]string, 0)
	for _, id := range s.learningSpaceIDsForPackage(packageID) {
		names = appendUnique(names, s.learningSpaceName(id))
	}
	return names
}

func (s *MemoryStore) learningSpaceNamesForGrant(grantID string) []string {
	names := make([]string, 0)
	for _, id := range s.learningSpaceIDsForGrant(grantID) {
		names = appendUnique(names, s.learningSpaceName(id))
	}
	return names
}

func (s *MemoryStore) learningSpaceNames(ids []string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = appendUnique(names, s.learningSpaceName(id))
	}
	return names
}

func (s *MemoryStore) learningSpaceGrades(ids []string) []string {
	grades := make([]string, 0)
	for _, id := range ids {
		for _, space := range s.learningSpaces {
			if space.ID == id {
				grades = appendUnique(grades, space.Grade)
				break
			}
		}
	}
	return grades
}

func (s *MemoryStore) learningSpaceSubjects(ids []string) []string {
	subjects := make([]string, 0)
	for _, id := range ids {
		for _, space := range s.learningSpaces {
			if space.ID == id {
				subjects = appendUnique(subjects, space.Subject)
				break
			}
		}
	}
	return subjects
}

func (s *MemoryStore) learningSpaceName(id string) string {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space.Name
		}
	}
	return id
}

func (s *MemoryStore) learningSpaceEnabled(id string) bool {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space.Status == learning.StatusEnabled
		}
	}
	return false
}

func (s *MemoryStore) learningSpaceExists(id string) bool {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return true
		}
	}
	return false
}

func (s *MemoryStore) findLearningSpace(id string) (learningSpace, bool) {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space, true
		}
	}
	return learningSpace{}, false
}

func (s *MemoryStore) learningSpaceMatches(id, grade, subject, semester string) bool {
	for _, space := range s.learningSpaces {
		if space.ID == id {
			return space.Grade == grade && space.Subject == subject && space.Semester == semester
		}
	}
	return false
}

func (s *MemoryStore) courseForUpload(principal learning.Principal, courseID, learningSpaceID string) (learning.Course, error) {
	for _, course := range s.courses {
		if course.ID != courseID {
			continue
		}
		if learningSpaceID != "" && course.LearningSpaceID != learningSpaceID {
			return learning.Course{}, errors.New("请选择正确的课程范围")
		}
		if !canSeeCourse(principal, course) {
			return learning.Course{}, errors.New("不能上传到未负责的课程")
		}
		if course.Status != learning.StatusEnabled {
			return learning.Course{}, errors.New("课程已停用，不能上传")
		}
		return course, nil
	}
	return learning.Course{}, errors.New("请选择课程")
}

func (s *MemoryStore) contentTypesForPackage(packageID string) []string {
	out := make([]string, 0)
	for _, item := range s.contentTypes {
		if item.PackageID == packageID {
			out = appendUnique(out, item.ContentType)
		}
	}
	return out
}

func (s *MemoryStore) contentTypeLabelsForPackage(packageID string) []string {
	labels := make([]string, 0)
	for _, value := range s.contentTypesForPackage(packageID) {
		labels = appendUnique(labels, contentTypeLabel(value))
	}
	return labels
}

func (s *MemoryStore) grantOpensSpace(grantID, learningSpaceID string) bool {
	return containsString(s.learningSpaceIDsForGrant(grantID), learningSpaceID)
}

func (s *MemoryStore) syncSpaceAccessForGrant(grant packageGrant) {
	for _, relation := range s.packageSpaces {
		if relation.PackageID != grant.PackageID || !s.learningSpaceEnabled(relation.LearningSpaceID) {
			continue
		}
		s.spaceAccess = append(s.spaceAccess, learningSpaceAccess{
			StudentID:       grant.StudentID,
			LearningSpaceID: relation.LearningSpaceID,
			PackageGrantID:  grant.ID,
			StartsAt:        grant.StartsAt,
			EndsAt:          grantEndsAt(grant),
			Status:          grant.Status,
		})
	}
}

func (s *MemoryStore) materialsForCourses(courses []string) []learning.Material {
	out := make([]learning.Material, 0)
	for _, material := range s.materials {
		if material.Status == learning.StatusEnabled && containsString(courses, material.Course) {
			out = append(out, material)
		}
	}
	return out
}

func (s *MemoryStore) homeworkForCourses(courses []string) []learning.Homework {
	out := make([]learning.Homework, 0)
	for _, item := range s.homework {
		if homeworkVisible(item.Status) && containsString(courses, item.Course) {
			out = append(out, item)
		}
	}
	return out
}

func homeworkVisible(status string) bool {
	status = strings.TrimSpace(status)
	return status != string(learning.StatusDraft) && status != string(learning.StatusDisabled)
}

func isContentStatus(status learning.Status) bool {
	return status == learning.StatusEnabled || status == learning.StatusDraft || status == learning.StatusDisabled
}

func publishStatus(status learning.Status) string {
	if status == learning.StatusEnabled {
		return "已发布"
	}
	return string(status)
}

func (s *MemoryStore) prependLog(operator, action, target string) {
	s.prependLogDetail(operator, action, target, "")
}

func auditChangeDetail(before, after any) string {
	payload := map[string]any{
		"before": before,
		"after":  after,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func packageAuditSnapshot(item learning.Package) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"name":             item.Name,
		"academicYear":     item.AcademicYear,
		"grade":            item.Grade,
		"semester":         item.Semester,
		"subject":          item.Subject,
		"phaseScope":       item.PhaseScope,
		"packageType":      item.PackageType,
		"summary":          item.Summary,
		"learningSpaceIds": item.LearningSpaceIDs,
		"contentTypeCodes": item.ContentTypeCodes,
		"status":           item.Status,
	}
}

func adminStaffAuditSnapshot(item learning.AdminStaff) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"name":          item.Name,
		"phone":         item.Phone,
		"role":          item.Role,
		"campusId":      item.CampusID,
		"accountStatus": item.AccountStatus,
		"remark":        item.Remark,
	}
}

func teacherAuditSnapshot(item learning.Teacher) map[string]any {
	return map[string]any{
		"id":                item.ID,
		"name":              item.Name,
		"phone":             item.Phone,
		"campusId":          item.CampusID,
		"learningSpaceIds":  item.LearningSpaceIDs,
		"canUploadHandout":  item.CanUploadHandout,
		"canUploadQuestion": item.CanUploadQuestion,
		"canReview":         item.CanReview,
		"accountStatus":     item.AccountStatus,
		"remark":            item.Remark,
	}
}

func studentAuditSnapshot(item learning.Student) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"name":          item.Name,
		"phone":         item.Phone,
		"grade":         item.Grade,
		"accountStatus": item.AccountStatus,
		"remark":        item.Remark,
	}
}

func courseAuditSnapshot(item learning.Course) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"name":            item.Name,
		"learningSpaceId": item.LearningSpaceID,
		"chapterCount":    item.ChapterCount,
		"status":          item.Status,
	}
}

func materialAuditSnapshot(item learning.Material) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"title":           item.Title,
		"courseId":        item.CourseID,
		"learningSpaceId": item.LearningSpaceID,
		"chapter":         item.Chapter,
		"status":          item.Status,
		"publishStatus":   item.PublishStatus,
	}
}

func homeworkAuditSnapshot(item learning.Homework) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"title":           item.Title,
		"courseId":        item.CourseID,
		"learningSpaceId": item.LearningSpaceID,
		"deadline":        item.Deadline,
		"status":          item.Status,
		"publishStatus":   item.PublishStatus,
	}
}

func scheduleClassAuditSnapshot(item learning.ScheduleClass) map[string]any {
	studentIDs := make([]string, 0, len(item.Students))
	for _, student := range item.Students {
		studentIDs = append(studentIDs, student.ID)
	}
	return map[string]any{
		"id":              item.ID,
		"name":            item.Name,
		"courseId":        item.CourseID,
		"teacherId":       item.TeacherID,
		"classType":       item.ClassType,
		"capacity":        item.Capacity,
		"durationMinutes": item.DurationMinutes,
		"dayOfWeek":       item.DayOfWeek,
		"startTime":       item.StartTime,
		"endTime":         item.EndTime,
		"startDate":       item.StartDate,
		"endDate":         item.EndDate,
		"studentIds":      studentIDs,
		"status":          item.Status,
	}
}

func (s *MemoryStore) prependLogDetail(operator, action, target, detail string) {
	audit := parseAuditOperator(operator)
	if detail != "" {
		audit.Detail = strings.TrimSpace(detail)
	}
	s.logs = append([]learning.OperationLog{{
		ID:         "log-" + time.Now().Format("20060102150405"),
		Operator:   audit.Name,
		OperatorID: audit.ID,
		IP:         audit.IP,
		UserAgent:  audit.UserAgent,
		Action:     action,
		Target:     target,
		Detail:     audit.Detail,
		Time:       time.Now().Format("2006-01-02 15:04:05"),
	}}, s.logs...)
	if s.db != nil {
		_ = s.persistAll()
	}
}

type auditOperatorInfo struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	Detail    string `json:"detail"`
}

func parseAuditOperator(value string) auditOperatorInfo {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "audit:") {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "audit:"))
		if err == nil {
			var info auditOperatorInfo
			if json.Unmarshal(raw, &info) == nil {
				info.Name = strings.TrimSpace(info.Name)
				if info.Name == "" {
					info.Name = "本地开发"
				}
				return info
			}
		}
	}
	if value == "" {
		value = "本地开发"
	}
	return auditOperatorInfo{Name: value}
}

func normalizeStudentRequest(req learning.StudentUpsertRequest, allowStatus bool) (learning.StudentUpsertRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Grade = strings.TrimSpace(req.Grade)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	if req.Name == "" {
		return req, errors.New("请输入学生姓名")
	}
	if req.Phone == "" {
		return req, errors.New("请输入手机号")
	}
	if req.Grade == "" {
		return req, errors.New("请选择年级")
	}
	if !allowStatus || req.AccountStatus == "" {
		req.AccountStatus = "正常"
	}
	if req.AccountStatus != "正常" && req.AccountStatus != "停用" && req.AccountStatus != "待提醒" {
		return req, errors.New("账号状态不正确")
	}
	return req, nil
}

func validateNewPassword(password string) error {
	if len(password) < 8 {
		return errors.New("新密码至少 8 位")
	}
	hasLetter := false
	hasDigit := false
	for _, ch := range password {
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			hasLetter = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("新密码需同时包含字母和数字")
	}
	return nil
}

func generateTemporaryPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return fmt.Sprintf("Starline@%s1", string(out)), nil
}

func matchesStudentQuery(student learning.Student, query learning.StudentQuery) bool {
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" && !strings.Contains(student.Name, keyword) && !strings.Contains(student.Phone, keyword) {
		return false
	}
	if query.Grade != "" && student.Grade != query.Grade {
		return false
	}
	if query.AccountStatus != "" && student.AccountStatus != query.AccountStatus {
		return false
	}
	if query.LearningStatus != "" && student.LearningStatus != query.LearningStatus {
		return false
	}
	if query.PackageState == "已开通" && len(student.OpenedPackages) == 0 {
		return false
	}
	if query.PackageState == "未开通" && len(student.OpenedPackages) > 0 {
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cleanPhrases(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = appendUnique(out, value)
	}
	return out
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}

func principalFromUser(user learning.User) learning.Principal {
	return learning.Principal{
		UserID:             user.ID,
		Name:               user.Name,
		Phone:              user.Phone,
		StudentID:          user.StudentID,
		CampusID:           user.CampusID,
		Roles:              append([]learning.Role(nil), user.Roles...),
		MustChangePassword: user.MustChangePassword,
		TokenVersion:       user.TokenVersion,
		CampusScopes:       append([]string(nil), user.CampusScopes...),
		LearningSpaceIDs:   append([]string(nil), user.LearningSpaceIDs...),
		CanUploadHandout:   user.CanUploadHandout,
		CanUploadQuestion:  user.CanUploadQuestion,
		CanReview:          user.CanReview,
	}
}

func (s *MemoryStore) teacherFromUser(user learning.User) learning.Teacher {
	bindStatus := "待绑定"
	if strings.TrimSpace(user.OpenID) != "" {
		bindStatus = "已绑定"
	}
	return learning.Teacher{
		ID:                user.ID,
		Name:              user.Name,
		Phone:             user.Phone,
		CampusID:          user.CampusID,
		LearningSpaceIDs:  append([]string(nil), user.LearningSpaceIDs...),
		LearningSpaces:    s.learningSpaceNames(user.LearningSpaceIDs),
		Grades:            s.learningSpaceGrades(user.LearningSpaceIDs),
		Subjects:          s.learningSpaceSubjects(user.LearningSpaceIDs),
		CanUploadHandout:  user.CanUploadHandout,
		CanUploadQuestion: user.CanUploadQuestion,
		CanReview:         user.CanReview,
		AccountStatus:     user.AccountStatus,
		BindStatus:        bindStatus,
		Remark:            user.Remark,
	}
}

func adminStaffFromUser(user learning.User) learning.AdminStaff {
	bindStatus := "待绑定"
	if strings.TrimSpace(user.OpenID) != "" {
		bindStatus = "已绑定"
	}
	return learning.AdminStaff{
		ID:            user.ID,
		Name:          user.Name,
		Phone:         user.Phone,
		Role:          primaryAdminRole(user.Roles),
		CampusID:      user.CampusID,
		AccountStatus: user.AccountStatus,
		BindStatus:    bindStatus,
		Remark:        user.Remark,
	}
}

func normalizeAdminStaffRequest(req learning.AdminStaffUpsertRequest, allowStatus bool) (learning.AdminStaffUpsertRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Role = learning.Role(strings.TrimSpace(string(req.Role)))
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)

	if req.Name == "" {
		return req, errors.New("请输入管理人员姓名")
	}
	if req.Phone == "" {
		return req, errors.New("请输入手机号")
	}
	if !isAdminRole(req.Role) {
		return req, errors.New("请选择正确的后台岗位")
	}
	if req.Role == learning.RoleCampusAdmin && req.CampusID == "" {
		return req, errors.New("校区管理员需要填写校区")
	}
	if req.Role == learning.RoleSuperAdmin {
		req.CampusID = ""
	}
	if !allowStatus || req.AccountStatus == "" {
		req.AccountStatus = "正常"
	}
	if req.AccountStatus != "正常" && req.AccountStatus != "停用" {
		return req, errors.New("账号状态不正确")
	}
	return req, nil
}

func (s *MemoryStore) normalizeTeacherRequest(principal learning.Principal, req learning.TeacherUpsertRequest, allowStatus bool) (learning.TeacherUpsertRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.Remark = strings.TrimSpace(req.Remark)
	req.LearningSpaceIDs = cleanPhrases(req.LearningSpaceIDs)

	if req.Name == "" {
		return req, errors.New("请输入教师姓名")
	}
	if req.Phone == "" {
		return req, errors.New("请输入手机号")
	}
	if len(req.LearningSpaceIDs) == 0 {
		return req, errors.New("请选择教师负责范围")
	}
	for _, spaceID := range req.LearningSpaceIDs {
		if !s.learningSpaceExists(spaceID) {
			return req, errors.New("教师负责范围不存在")
		}
	}
	if !req.CanUploadHandout && !req.CanUploadQuestion && !req.CanReview {
		return req, errors.New("请至少选择一项教师权限")
	}
	if !allowStatus || req.AccountStatus == "" {
		req.AccountStatus = "正常"
	}
	if req.AccountStatus != "正常" && req.AccountStatus != "停用" {
		return req, errors.New("账号状态不正确")
	}
	if hasRole(principal.Roles, learning.RoleCampusAdmin) && !hasRole(principal.Roles, learning.RoleSuperAdmin) {
		if principal.CampusID == "" {
			return req, errors.New("当前管理员未绑定校区")
		}
		if req.CampusID == "" {
			req.CampusID = principal.CampusID
		}
		if req.CampusID != principal.CampusID {
			return req, errors.New("不能管理其他校区教师")
		}
	}
	if req.CampusID == "" {
		req.CampusID = "campus-main"
	}
	return req, nil
}

func (s *MemoryStore) userPhoneExists(currentUserID, phone string) bool {
	for _, user := range s.users {
		if user.ID != currentUserID && user.Phone == phone {
			return true
		}
	}
	return false
}

func (s *MemoryStore) activeSuperAdminCount() int {
	count := 0
	for _, user := range s.users {
		if user.AccountStatus == "正常" && hasRole(user.Roles, learning.RoleSuperAdmin) {
			count++
		}
	}
	return count
}

func canBindByPhone(user learning.User) bool {
	return hasRole(user.Roles, learning.RoleTeacher) || hasRole(user.Roles, learning.RoleStudent) || isAdminStaffUser(user)
}

func canRebindByPhone(user learning.User, realWechatLogin bool) bool {
	if !canBindByPhone(user) {
		return false
	}
	openID := strings.TrimSpace(user.OpenID)
	return openID == "" || (realWechatLogin && strings.HasPrefix(openID, "demo-"))
}

func displayPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) == 11 {
		return phone[:3] + "****" + phone[7:]
	}
	return phone
}

func isAdminStaffUser(user learning.User) bool {
	for _, role := range user.Roles {
		if isAdminRole(role) {
			return true
		}
	}
	return false
}

func primaryAdminRole(roles []learning.Role) learning.Role {
	for _, role := range []learning.Role{learning.RoleSuperAdmin, learning.RoleCampusAdmin, learning.RoleOpsStaff} {
		if hasRole(roles, role) {
			return role
		}
	}
	return ""
}

func isAdminRole(role learning.Role) bool {
	return role == learning.RoleOpsStaff || role == learning.RoleCampusAdmin || role == learning.RoleSuperAdmin
}

func roleName(role learning.Role) string {
	switch role {
	case learning.RoleOpsStaff:
		return "运营教务"
	case learning.RoleCampusAdmin:
		return "校区管理员"
	case learning.RoleSuperAdmin:
		return "超级管理员"
	default:
		return string(role)
	}
}

func settingLabel(key string) string {
	switch key {
	case "academicYear":
		return "当前学年"
	case "grades":
		return "年级范围"
	case "semesters":
		return "学期设置"
	case "watermarkRule":
		return "水印规则"
	case "downloadPolicy":
		return "下载规则"
	default:
		return key
	}
}

func canManageTeacher(principal learning.Principal, user learning.User) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) {
		return true
	}
	if !hasRole(principal.Roles, learning.RoleCampusAdmin) {
		return false
	}
	return principal.CampusID != "" && user.CampusID == principal.CampusID
}

func canResetPassword(principal learning.Principal, user learning.User) bool {
	if principal.UserID == user.ID {
		return false
	}
	if hasRole(principal.Roles, learning.RoleSuperAdmin) {
		return hasRole(user.Roles, learning.RoleTeacher) || isAdminStaffUser(user)
	}
	if hasRole(principal.Roles, learning.RoleCampusAdmin) {
		return hasRole(user.Roles, learning.RoleTeacher) && principal.CampusID != "" && principal.CampusID == user.CampusID
	}
	return false
}

func canManageCommercial(principal learning.Principal) bool {
	return hasRole(principal.Roles, learning.RoleOpsStaff) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleSuperAdmin)
}

func (s *MemoryStore) canSeeCommercialStudent(principal learning.Principal, studentID string) bool {
	student, ok := s.findStudent(studentID)
	if !ok {
		return false
	}
	return canSeeStudent(principal, student, s.coursesForStudent(student.ID))
}

func (s *MemoryStore) commercialOrderForWrite(principal learning.Principal, orderID string) (int, learning.CommercialOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return -1, learning.CommercialOrder{}, errors.New("请选择订单")
	}
	if !canManageCommercial(principal) {
		return -1, learning.CommercialOrder{}, errors.New("没有权限维护订单")
	}
	for index, order := range s.commercialOrders {
		if order.ID != orderID {
			continue
		}
		if !s.canSeeCommercialStudent(principal, order.StudentID) {
			return -1, learning.CommercialOrder{}, errors.New("没有权限维护该订单")
		}
		return index, order, nil
	}
	return -1, learning.CommercialOrder{}, errors.New("订单不存在")
}

func canSeeStudent(principal learning.Principal, student learning.Student, studentCourses []learning.Course) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if hasRole(principal.Roles, learning.RoleStudent) {
		return principal.StudentID == student.ID
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		if len(principal.LearningSpaceIDs) == 0 {
			return false
		}
		for _, course := range studentCourses {
			if containsString(principal.LearningSpaceIDs, course.LearningSpaceID) {
				return true
			}
		}
	}
	return false
}

func canSeeCourse(principal learning.Principal, course learning.Course) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		return containsString(principal.LearningSpaceIDs, course.LearningSpaceID)
	}
	return false
}

func canSeeQuestionScope(principal learning.Principal, grade, semester, subject string, spaces []learningSpace) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if !hasRole(principal.Roles, learning.RoleTeacher) {
		return false
	}
	for _, space := range spaces {
		if containsString(principal.LearningSpaceIDs, space.ID) && space.Grade == grade && space.Semester == semester && space.Subject == subject && space.Status == learning.StatusEnabled {
			return true
		}
	}
	return false
}

func canEditQuestion(principal learning.Principal, item learning.QuestionBankItem) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && item.OwnerTeacherID == principal.UserID
}

func canUploadHandout(principal learning.Principal) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && principal.CanUploadHandout
}

func canUploadQuestion(principal learning.Principal) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && principal.CanUploadQuestion
}

func canReviewHomework(principal learning.Principal) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	return hasRole(principal.Roles, learning.RoleTeacher) && principal.CanReview
}

func canSeeSubject(principal learning.Principal, subjects []string, value string) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	for _, subject := range subjects {
		if strings.Contains(value, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) findUser(id string) (learning.User, bool) {
	for _, user := range s.users {
		if user.ID == id {
			return user, true
		}
	}
	return learning.User{}, false
}

func (s *MemoryStore) canManageAvailability(principal learning.Principal, ownerType, ownerID string) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if ownerType == "teacher" && hasRole(principal.Roles, learning.RoleTeacher) {
		return principal.UserID == ownerID
	}
	if ownerType == "student" && hasRole(principal.Roles, learning.RoleStudent) {
		return principal.StudentID == ownerID
	}
	return false
}

func (s *MemoryStore) canSendNoticeTo(principal learning.Principal, values ...string) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if !hasRole(principal.Roles, learning.RoleTeacher) {
		return false
	}
	joined := strings.Join(values, " ")
	if strings.Contains(joined, "全部") {
		return false
	}
	for _, subject := range s.learningSpaceSubjects(principal.LearningSpaceIDs) {
		if subject != "" && strings.Contains(joined, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) availabilityOwnerName(ownerType, ownerID string) (string, error) {
	if ownerType == "teacher" {
		user, ok := s.findUser(ownerID)
		if !ok || !hasRole(user.Roles, learning.RoleTeacher) {
			return "", errors.New("老师不存在")
		}
		return user.Name, nil
	}
	if ownerType == "student" {
		student, ok := s.findStudent(ownerID)
		if !ok {
			return "", errors.New("学生不存在")
		}
		return student.Name, nil
	}
	return "", errors.New("可用时间类型不正确")
}

func (s *MemoryStore) ownerAvailability(ownerType, ownerID string) []learning.AvailabilitySlot {
	out := make([]learning.AvailabilitySlot, 0)
	for _, slot := range s.availability {
		if slot.OwnerType == ownerType && slot.OwnerID == ownerID {
			out = append(out, slot)
		}
	}
	sortAvailability(out)
	return out
}

func (s *MemoryStore) studentAvailable(studentID string, dayOfWeek, startMin, endMin int) bool {
	for _, slot := range s.ownerAvailability("student", studentID) {
		slotStart, _ := parseClock(slot.StartTime)
		slotEnd, _ := parseClock(slot.EndTime)
		if slot.DayOfWeek == dayOfWeek && slotStart <= startMin && slotEnd >= endMin {
			return true
		}
	}
	return false
}

func (s *MemoryStore) hasScheduleConflict(ownerType, ownerID string, dayOfWeek, startMin, endMin int) bool {
	return s.hasScheduleConflictExcept(ownerType, ownerID, dayOfWeek, startMin, endMin, "")
}

func (s *MemoryStore) hasScheduleConflictExcept(ownerType, ownerID string, dayOfWeek, startMin, endMin int, exceptID string) bool {
	for _, item := range s.scheduleClasses {
		if item.ID == exceptID || item.DayOfWeek != dayOfWeek || item.Status == "已取消" {
			continue
		}
		itemStart, okStart := parseClock(item.StartTime)
		itemEnd, okEnd := parseClock(item.EndTime)
		if !okStart || !okEnd || endMin <= itemStart || startMin >= itemEnd {
			continue
		}
		if ownerType == "teacher" && item.TeacherID == ownerID {
			return true
		}
		if ownerType == "student" {
			for _, student := range item.Students {
				if student.ID == ownerID {
					return true
				}
			}
		}
	}
	return false
}

func (s *MemoryStore) canSeeScheduleClass(principal learning.Principal, item learning.ScheduleClass) bool {
	if hasRole(principal.Roles, learning.RoleSuperAdmin) || hasRole(principal.Roles, learning.RoleCampusAdmin) || hasRole(principal.Roles, learning.RoleOpsStaff) {
		return true
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		return item.TeacherID == principal.UserID
	}
	if hasRole(principal.Roles, learning.RoleStudent) {
		for _, student := range item.Students {
			if student.ID == principal.StudentID {
				return true
			}
		}
	}
	return false
}

func (s *MemoryStore) ensureSchedulingTables() error {
	if s.db == nil {
		return nil
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS availability_slots (
			id VARCHAR(64) PRIMARY KEY,
			owner_type VARCHAR(16) NOT NULL,
			owner_id VARCHAR(64) NOT NULL,
			owner_name VARCHAR(64) NOT NULL DEFAULT '',
			day_of_week TINYINT NOT NULL,
			start_time CHAR(5) NOT NULL,
			end_time CHAR(5) NOT NULL,
			start_date DATE NULL,
			end_date DATE NULL,
			remark VARCHAR(255) NOT NULL DEFAULT '',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_availability_owner (owner_type, owner_id),
			KEY idx_availability_day (day_of_week, start_time, end_time)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS schedule_classes (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			course_id VARCHAR(64) NOT NULL,
			course_name VARCHAR(128) NOT NULL DEFAULT '',
			teacher_id VARCHAR(64) NOT NULL,
			teacher_name VARCHAR(64) NOT NULL DEFAULT '',
			class_type VARCHAR(16) NOT NULL,
			capacity INT NOT NULL DEFAULT 1,
			duration_minutes INT NOT NULL DEFAULT 90,
			day_of_week TINYINT NOT NULL,
			start_time CHAR(5) NOT NULL,
			end_time CHAR(5) NOT NULL,
			start_date DATE NULL,
			end_date DATE NULL,
			status VARCHAR(32) NOT NULL DEFAULT '已确认',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			KEY idx_schedule_teacher_time (teacher_id, day_of_week, start_time, end_time),
			KEY idx_schedule_course (course_id, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS schedule_class_students (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			schedule_class_id VARCHAR(64) NOT NULL,
			student_id VARCHAR(64) NOT NULL,
			student_name VARCHAR(64) NOT NULL DEFAULT '',
			UNIQUE KEY uk_schedule_student (schedule_class_id, student_id),
			KEY idx_schedule_student_time (student_id, schedule_class_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryStore) bootstrapSchedulingData() error {
	if s.db == nil {
		return nil
	}
	var availabilityCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM availability_slots").Scan(&availabilityCount); err != nil {
		return err
	}
	if availabilityCount == 0 {
		for _, slot := range s.availability {
			if err := s.insertAvailabilitySlot(slot); err != nil {
				return err
			}
		}
	} else {
		slots, err := s.loadAvailabilitySlots()
		if err != nil {
			return err
		}
		s.availability = slots
	}
	classes, err := s.loadScheduleClasses()
	if err != nil {
		return err
	}
	s.scheduleClasses = classes
	return nil
}

func (s *MemoryStore) replaceAvailabilitySlots(ownerType, ownerID string, slots []learning.AvailabilitySlot) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM availability_slots WHERE owner_type = ? AND owner_id = ?", ownerType, ownerID); err != nil {
		tx.Rollback()
		return err
	}
	for _, slot := range slots {
		if _, err := tx.Exec(
			`INSERT INTO availability_slots (id, owner_type, owner_id, owner_name, day_of_week, start_time, end_time, start_date, end_date, remark)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			slot.ID, slot.OwnerType, slot.OwnerID, slot.OwnerName, slot.DayOfWeek, slot.StartTime, slot.EndTime, nullableDate(slot.StartDate), nullableDate(slot.EndDate), slot.Remark,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *MemoryStore) insertAvailabilitySlot(slot learning.AvailabilitySlot) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO availability_slots (id, owner_type, owner_id, owner_name, day_of_week, start_time, end_time, start_date, end_date, remark)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		slot.ID, slot.OwnerType, slot.OwnerID, slot.OwnerName, slot.DayOfWeek, slot.StartTime, slot.EndTime, nullableDate(slot.StartDate), nullableDate(slot.EndDate), slot.Remark,
	)
	return err
}

func (s *MemoryStore) loadAvailabilitySlots() ([]learning.AvailabilitySlot, error) {
	rows, err := s.db.Query(`SELECT id, owner_type, owner_id, owner_name, day_of_week, start_time, end_time, start_date, end_date, remark FROM availability_slots ORDER BY owner_type, owner_id, day_of_week, start_time`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]learning.AvailabilitySlot, 0)
	for rows.Next() {
		var slot learning.AvailabilitySlot
		var startDate, endDate sql.NullTime
		if err := rows.Scan(&slot.ID, &slot.OwnerType, &slot.OwnerID, &slot.OwnerName, &slot.DayOfWeek, &slot.StartTime, &slot.EndTime, &startDate, &endDate, &slot.Remark); err != nil {
			return nil, err
		}
		slot.StartDate = dateString(startDate)
		slot.EndDate = dateString(endDate)
		out = append(out, slot)
	}
	return out, rows.Err()
}

func (s *MemoryStore) insertScheduleClass(item learning.ScheduleClass) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO schedule_classes (id, name, course_id, course_name, teacher_id, teacher_name, class_type, capacity, duration_minutes, day_of_week, start_time, end_time, start_date, end_date, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Name, item.CourseID, item.CourseName, item.TeacherID, item.TeacherName, item.ClassType, item.Capacity, item.DurationMinutes, item.DayOfWeek, item.StartTime, item.EndTime, nullableDate(item.StartDate), nullableDate(item.EndDate), item.Status, item.CreatedAt,
	); err != nil {
		tx.Rollback()
		return err
	}
	for _, student := range item.Students {
		if _, err := tx.Exec(
			`INSERT INTO schedule_class_students (schedule_class_id, student_id, student_name) VALUES (?, ?, ?)`,
			item.ID, student.ID, student.Name,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *MemoryStore) updateScheduleClassStatus(id, status string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec("UPDATE schedule_classes SET status = ? WHERE id = ?", status, id)
	return err
}

func (s *MemoryStore) updateScheduleClass(item learning.ScheduleClass) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE schedule_classes
		 SET name = ?, course_id = ?, course_name = ?, teacher_id = ?, teacher_name = ?, class_type = ?, capacity = ?, duration_minutes = ?, day_of_week = ?, start_time = ?, end_time = ?, start_date = ?, end_date = ?, status = ?
		 WHERE id = ?`,
		item.Name, item.CourseID, item.CourseName, item.TeacherID, item.TeacherName, item.ClassType, item.Capacity, item.DurationMinutes, item.DayOfWeek, item.StartTime, item.EndTime, nullableDate(item.StartDate), nullableDate(item.EndDate), item.Status, item.ID,
	); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM schedule_class_students WHERE schedule_class_id = ?", item.ID); err != nil {
		tx.Rollback()
		return err
	}
	for _, student := range item.Students {
		if _, err := tx.Exec(
			`INSERT INTO schedule_class_students (schedule_class_id, student_id, student_name) VALUES (?, ?, ?)`,
			item.ID, student.ID, student.Name,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *MemoryStore) loadScheduleClasses() ([]learning.ScheduleClass, error) {
	rows, err := s.db.Query(`SELECT id, name, course_id, course_name, teacher_id, teacher_name, class_type, capacity, duration_minutes, day_of_week, start_time, end_time, start_date, end_date, status, created_at FROM schedule_classes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]learning.ScheduleClass, 0)
	for rows.Next() {
		var item learning.ScheduleClass
		var startDate, endDate, createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.CourseID, &item.CourseName, &item.TeacherID, &item.TeacherName, &item.ClassType, &item.Capacity, &item.DurationMinutes, &item.DayOfWeek, &item.StartTime, &item.EndTime, &startDate, &endDate, &item.Status, &createdAt); err != nil {
			return nil, err
		}
		item.StartDate = dateString(startDate)
		item.EndDate = dateString(endDate)
		item.CreatedAt = dateTimeString(createdAt)
		students, err := s.loadScheduleClassStudents(item.ID)
		if err != nil {
			return nil, err
		}
		item.Students = students
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *MemoryStore) loadScheduleClassStudents(classID string) ([]learning.CandidateStudent, error) {
	rows, err := s.db.Query(`SELECT student_id, student_name FROM schedule_class_students WHERE schedule_class_id = ? ORDER BY id`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]learning.CandidateStudent, 0)
	for rows.Next() {
		var student learning.CandidateStudent
		if err := rows.Scan(&student.ID, &student.Name); err != nil {
			return nil, err
		}
		if decorated, ok := s.findStudent(student.ID); ok {
			detail := s.decorateStudent(decorated)
			student.Name = detail.Name
			student.Grade = detail.Grade
			student.OpenedPackages = detail.OpenedPackages
		}
		out = append(out, student)
	}
	return out, rows.Err()
}

func (s *MemoryStore) courseForScheduling(principal learning.Principal, courseID string) (learning.Course, error) {
	for _, course := range s.courses {
		if course.ID != courseID {
			continue
		}
		if !canSeeCourse(principal, course) {
			return learning.Course{}, errors.New("不能给未负责的课程排课")
		}
		if course.Status != learning.StatusEnabled {
			return learning.Course{}, errors.New("课程已停用，不能排课")
		}
		return course, nil
	}
	return learning.Course{}, errors.New("请选择课程")
}

func sortAvailability(slots []learning.AvailabilitySlot) {
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].DayOfWeek == slots[j].DayOfWeek {
			return slots[i].StartTime < slots[j].StartTime
		}
		return slots[i].DayOfWeek < slots[j].DayOfWeek
	})
}

func parseClock(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func minutesToClock(value int) string {
	hour := value / 60
	minute := value % 60
	return twoDigit(hour) + ":" + twoDigit(minute)
}

func nullableDate(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func dateString(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func dateTimeString(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02 15:04:05")
}

func classCapacity(classType string) int {
	classType = strings.TrimSpace(strings.ToUpper(classType))
	if !strings.HasPrefix(classType, "1V") {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimPrefix(classType, "1V"))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func minClassStudents(capacity int) int {
	if capacity <= 1 {
		return 1
	}
	return 2
}

func candidateReason(studentCount, capacity int) string {
	if studentCount >= capacity {
		return "人数已满足满班，可直接确认"
	}
	if studentCount >= minClassStudents(capacity) {
		return "人数已达到成班线，可继续补充学生"
	}
	return "人数不足成班线，需协调更多学生时间"
}

// intersects 判断两个字符串集合是否存在交集。
func intersects(a, b []string) bool {
	for _, item := range a {
		if containsString(b, item) {
			return true
		}
	}
	return false
}

func hasRole(roles []learning.Role, role learning.Role) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendCourseUnique(values []learning.Course, addition learning.Course) []learning.Course {
	for _, value := range values {
		if value.ID == addition.ID {
			return values
		}
	}
	return append(values, addition)
}

func appendMaterialUnique(values []learning.Material, addition learning.Material) []learning.Material {
	for _, value := range values {
		if value.ID == addition.ID {
			return values
		}
	}
	return append(values, addition)
}

func appendHomeworkUnique(values []learning.Homework, addition learning.Homework) []learning.Homework {
	for _, value := range values {
		if value.ID == addition.ID {
			return values
		}
	}
	return append(values, addition)
}

func bankItemQuestion(item learning.QuestionBankItem) learning.Question {
	answer := item.Answer
	answers := cleanPhrases(item.Answers)
	if answer == "" && len(answers) > 0 {
		answer = answers[0]
	}
	return learning.Question{
		ID: item.ID, Type: item.Type, Stem: item.Stem, Options: append([]string(nil), item.Options...),
		Answer: answer, Answers: answers, Score: item.Score,
	}
}

func questionIDs(questions []learning.Question) []string {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}
	return ids
}

func normalizedQuestionAnswers(question learning.Question) []string {
	if len(question.Answers) > 0 {
		return cleanPhrases(question.Answers)
	}
	if strings.TrimSpace(question.Answer) != "" {
		return []string{strings.TrimSpace(question.Answer)}
	}
	return nil
}

func sameChoiceSet(left, right []string) bool {
	a := cleanPhrases(left)
	b := cleanPhrases(right)
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for index := range a {
		if !strings.EqualFold(a[index], b[index]) {
			return false
		}
	}
	return true
}

func grantEndsAt(grant packageGrant) string {
	if grant.EndsAt != "" {
		return grant.EndsAt
	}
	return grant.EffectiveUntil
}

func grantActive(grant packageGrant) bool {
	today := time.Now().Format("2006-01-02")
	status := grant.Status
	if status == "" {
		status = "active"
	}
	return status == "active" && grantEndsAt(grant) >= today
}

func contentTypeLabel(value string) string {
	switch value {
	case "course":
		return "课程"
	case "question":
		return "题"
	case "handout":
		return "讲义"
	default:
		return value
	}
}

func validContentType(value string) bool {
	return value == "course" || value == "question" || value == "handout"
}

func packageTypeLabel(values []string) string {
	labels := make([]string, 0, len(values))
	for _, value := range []string{"course", "question", "handout"} {
		if containsString(values, value) {
			labels = append(labels, contentTypeLabel(value))
		}
	}
	if len(labels) == 0 {
		return "自定义"
	}
	return strings.Join(labels, "+")
}

func courseNames(courses []learning.Course) []string {
	out := make([]string, 0, len(courses))
	for _, course := range courses {
		out = append(out, course.Name)
	}
	return out
}

func subjectsForCourses(courses []learning.Course) []string {
	out := make([]string, 0, len(courses))
	for _, course := range courses {
		out = appendUnique(out, course.Subject)
	}
	return out
}
