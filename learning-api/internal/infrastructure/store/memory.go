package store

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var (
	richTextScriptPattern     = regexp.MustCompile(`(?is)<script[\s\S]*?>[\s\S]*?</script>`)
	richTextEventAttrPattern  = regexp.MustCompile(`(?i)\s+on[a-z0-9_-]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	richTextJSURLAttrPattern  = regexp.MustCompile(`(?i)\s+(href|src)\s*=\s*("|')?\s*javascript:[^\s"'>]+("|')?`)
	richTextTagPattern        = regexp.MustCompile(`(?is)<[^>]+>`)
	richTextTagNamePattern    = regexp.MustCompile(`(?is)^</?\s*([a-z0-9]+)`)
	richTextColorPattern      = regexp.MustCompile(`(?i)color\s*:\s*(#[0-9a-f]{3,6}|[a-z]+)`)
	richTextHTTPSImagePattern = regexp.MustCompile(`(?i)\ssrc\s*=\s*["'](https?://[^"']+)["']`)
)

type MemoryStore struct {
	mu                              sync.Mutex
	users                           []learning.User
	packages                        []learning.Package
	students                        []learning.Student
	learningSpaces                  []learningSpace
	packageSpaces                   []packageSpace
	contentTypes                    []packageContentType
	spaceAccess                     []learningSpaceAccess
	courses                         []learning.Course
	questionBank                    []learning.QuestionBankItem
	materials                       []learning.Material
	homework                        []learning.Homework
	fileAssets                      map[string]learning.FileAsset
	reviews                         []learning.Review
	notices                         []learning.Notice
	logs                            []learning.OperationLog
	settings                        map[string]string
	grants                          []packageGrant
	availability                    []learning.AvailabilitySlot
	scheduleClasses                 []learning.ScheduleClass
	commercialOrders                []learning.CommercialOrder
	payments                        []learning.PaymentRecord
	refunds                         []learning.RefundRecord
	contracts                       []learning.ContractRecord
	invoices                        []learning.InvoiceRecord
	lessonConsumptions              []learning.LessonConsumption
	renewalReminders                []learning.RenewalReminder
	parentNotices                   []learning.ParentNotice
	submissions                     map[string]learning.Submission
	favorites                       map[string]learning.Favorite
	subscriptionPreferences         map[string]learning.StudentSubscriptionPreference
	scoreRecords                    []learning.StudentScoreRecord
	wechatResolver                  func(code string) (string, error)
	phoneResolver                   func(phoneCode string) (string, error)
	officialNoticeSender            func(learning.Notice) error
	pendingNoticeDeliveries         []learning.Notice
	officialAccountReady            bool
	miniProgramSubscribeTemplateIDs []string
	db                              *sql.DB
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
		fileAssets:              map[string]learning.FileAsset{},
		submissions:             map[string]learning.Submission{},
		favorites:               map[string]learning.Favorite{},
		subscriptionPreferences: map[string]learning.StudentSubscriptionPreference{},
		settings:                map[string]string{},
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
	s.settings = defaultSettings()
	s.seedBaseLearningSpaces()
}

func defaultSettings() map[string]string {
	return map[string]string{
		"academicYear":                 "2025.2026学年",
		"grades":                       "G1-G9",
		"semesters":                    "S1 / S2",
		"watermarkRule":                "姓名/昵称 + 手机尾号 + 时间 + 学生ID后缀",
		"downloadPolicy":               "学生端仅安全预览，不提供下载",
		"miniProgramDomainStatus":      "待确认",
		"officialAccountBindingStatus": "待确认",
		"templateMessageStatus":        "待确认",
		"miniProgramSubscribeStatus":   "待配置",
		"productionApiDomain":          "待配置",
	}
}

// academicYearForDate 与后台套餐默认学年保持一致：每年 7 月 1 日进入下一学年。
func academicYearForDate(value time.Time) string {
	startYear := value.Year()
	if value.Month() < time.July {
		startYear--
	}
	return fmt.Sprintf("%d.%d学年", startYear, startYear+1)
}

func currentAcademicYear() string {
	return academicYearForDate(time.Now())
}

func (s *MemoryStore) ensureDefaultSettings() {
	if s.settings == nil {
		s.settings = map[string]string{}
	}
	for key, value := range defaultSettings() {
		if strings.TrimSpace(s.settings[key]) == "" {
			s.settings[key] = value
		}
	}
}

func (s *MemoryStore) seedBaseLearningSpaces() {
	const academicYear = "2025.2026学年"
	exists := map[string]bool{}
	for _, space := range s.learningSpaces {
		exists[space.ID] = true
	}
	for _, space := range baseLearningSpaces(academicYear) {
		if exists[space.ID] {
			continue
		}
		s.learningSpaces = append(s.learningSpaces, space)
		exists[space.ID] = true
	}
}

func baseLearningSpaces(academicYear string) []learningSpace {
	spaces := make([]learningSpace, 0, 180)
	for gradeIndex, grade := range demoGrades {
		for _, subject := range demoSubjects {
			if !subjectAppliesToGrade(gradeIndex, subject) {
				continue
			}
			for semesterIndex, semester := range demoSemesters {
				for phaseIndex, phase := range demoPhases {
					spaces = append(spaces, learningSpace{
						ID: learningSpaceID(gradeIndex, subject, semesterIndex, phaseIndex), AcademicYear: academicYear,
						Grade: grade, Subject: subject, Semester: semester, Phase: phase,
						Name: grade + subject + semester + phase, Status: learning.StatusEnabled,
					})
				}
			}
		}
	}
	return spaces
}

func (s *MemoryStore) seedDemoUsers(adminPasswordHash string) {
	s.users = []learning.User{
		{ID: "user-super", Name: "超级管理员", Phone: "13800000001", OpenID: "demo-super", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleSuperAdmin}, CampusScopes: []string{"campus-main"}},
		{ID: "user-campus", Name: "校区管理员", Phone: "13800000002", OpenID: "demo-campus", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleCampusAdmin}, CampusID: "campus-main", CampusScopes: []string{"campus-main"}},
		{ID: "user-ops", Name: "运营教务", Phone: "13800000003", OpenID: "demo-ops", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleOpsStaff}, CampusID: "campus-main", CampusScopes: []string{"campus-main"}},
		{ID: "user-teacher", Name: "英语老师", Phone: "13800000004", OpenID: "demo-teacher", PasswordHash: adminPasswordHash, AccountStatus: "正常", Roles: []learning.Role{learning.RoleTeacher}, CampusID: "campus-main", LearningSpaceIDs: []string{"space-g05-english-s1-q1", "space-g05-english-s1-q2"}, CanUploadHandout: true, CanUploadQuestion: true, CanReview: true},
		{ID: "user-student-001", Name: "小明", Phone: "18500009069", OpenID: "", AccountStatus: "正常", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-001", CampusID: "campus-main"},
		{ID: "user-student-002", Name: "Lucy", Phone: "13600002201", OpenID: "", AccountStatus: "待提醒", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-002", CampusID: "campus-main"},
		{ID: "user-student-003", Name: "小航", Phone: "13700003303", OpenID: "", AccountStatus: "正常", Roles: []learning.Role{learning.RoleStudent}, StudentID: "stu-003", CampusID: "campus-main"},
	}
	s.students = []learning.Student{
		{ID: "stu-001", Name: "小明", EnrollmentAcademicYear: currentAcademicYear(), EnrollmentGrade: "五年级", Phone: "185****9069", LearningStatus: "连续7天", AccountStatus: "正常", StreakDays: 7, BadgeCount: 5, BindStatus: "待绑定", LastStudyAt: "2026-05-22 18:20:00", EffectiveUntil: "2027-05-22"},
		{ID: "stu-002", Name: "Lucy", EnrollmentAcademicYear: currentAcademicYear(), EnrollmentGrade: "五年级", Phone: "136****2201", LearningStatus: "今日未学", AccountStatus: "待提醒", StreakDays: 3, BadgeCount: 3, BindStatus: "待绑定", LastStudyAt: "2026-05-21 19:10:00", EffectiveUntil: "2027-05-22"},
		{ID: "stu-003", Name: "小航", EnrollmentAcademicYear: currentAcademicYear(), EnrollmentGrade: "五年级", Phone: "137****3303", LearningStatus: "刚开通", AccountStatus: "正常", StreakDays: 1, BadgeCount: 1, BindStatus: "待绑定", LastStudyAt: "2026-05-22 20:00:00", EffectiveUntil: "2027-05-22"},
	}
	s.reviews = []learning.Review{
		{ID: "rev-001", StudentID: "stu-001", HomeworkID: "hw-g05-english-s1-q1", StudentName: "小明", PackageName: "英语班", Homework: "阅读挑战", SystemScore: 86, TeacherComment: "阅读理解整体不错，注意把答案依据写完整。", Reward: "阅读小星星", Status: "待批改"},
		{ID: "rev-002", StudentID: "stu-002", HomeworkID: "hw-g05-math-s1-q1", StudentName: "Lucy", PackageName: "数学班", Homework: "图形挑战", SystemScore: 78, TeacherComment: "图形思路基本正确，错题建议再画一遍辅助线。", Reward: "图形探索徽章", Status: "待复核"},
	}
	s.notices = []learning.Notice{
		{ID: "notice-001", Type: "练", Title: "英语阅读挑战已发布", Target: "英语班 86 名学生", Summary: "今天的小挑战别忘啦", Status: "已发送"},
		{ID: "notice-002", Type: "评", Title: "批改完成提醒", Target: "小明", Summary: "老师反馈已经准备好了", Status: "自动发送"},
	}
	s.logs = []learning.OperationLog{
		{ID: "log-001", Operator: "本地开发", Action: "初始化权限演示", Target: "完整学习空间与三种套餐", Time: "2026-05-22 09:30:00"},
	}
}

func (s *MemoryStore) connectSchedulingDBUnlocked(dsn string) error {
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

var demoGrades = gradeSequence

var demoSubjects = []string{"数学", "英文", "语文", "综合科学", "科学", "地理", "历史", "物理", "化学"}

// demoGradeSubjects 是基础空间初始化使用的年级—学科矩阵。
// 年级使用现有业务数据格式（一年级至九年级），学科名称按课程配置原文保留。
var demoGradeSubjects = [][]string{
	{"数学", "英文", "语文", "综合科学"},
	{"数学", "英文", "语文", "综合科学"},
	{"数学", "英文", "语文", "综合科学"},
	{"数学", "英文", "语文", "科学", "地理"},
	{"数学", "英文", "语文", "科学", "地理"},
	{"数学", "英文", "语文", "科学", "历史"},
	{"数学", "英文", "语文", "科学", "历史"},
	{"数学", "英文", "语文", "科学", "地理", "物理"},
	{"数学", "英文", "语文", "科学", "历史", "物理", "化学"},
}

// subjectAppliesToGrade 判断某年级是否开设该学科，避免生成矩阵外的无效空间。
func subjectAppliesToGrade(gradeIndex int, subject string) bool {
	if gradeIndex < 0 || gradeIndex >= len(demoGradeSubjects) {
		return false
	}
	for _, allowed := range demoGradeSubjects[gradeIndex] {
		if allowed == subject {
			return true
		}
	}
	return false
}

var demoSemesters = []string{"S1", "S2"}

var demoPhases = []string{"Q1", "Q2"}

var demoPackageTypes = []packageTypeSpec{
	{Code: "question", Label: "题", Summary: "只开放题", ContentTypes: []string{"question"}},
	{Code: "question_handout", Label: "题+学习资料", Summary: "开放题和学习资料", ContentTypes: []string{"question", "handout"}},
	{Code: "full", Label: "课程+题+学习资料", Summary: "开放课程、题和学习资料", ContentTypes: []string{"course", "question", "handout"}},
}

func seedPermissionDemoData(s *MemoryStore) {
	const academicYear = "2025.2026学年"
	s.seedBaseLearningSpaces()
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
					s.courses = append(s.courses, learning.Course{
						ID: courseID(spaceID), Name: courseName, Subject: subject, Grade: grade, LearningSpaceID: spaceID,
						ChapterCount: 8, MaterialNum: 1, HomeworkNum: 1, Status: learning.StatusEnabled,
					})
					s.materials = append(s.materials, learning.Material{
						ID: materialID(spaceID), Title: spaceName + "核心学习资料", CourseID: courseID(spaceID), Course: courseName, LearningSpaceID: spaceID,
						Chapter: "基础巩固", Type: "学习资料", ViewCount: demoViewCount(gradeIndex, semesterIndex, phaseIndex),
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
		{ID: "grant-001", StudentID: "stu-001", PackageID: packageID(4, "英文", 0, "full"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		{ID: "grant-002", StudentID: "stu-002", PackageID: packageID(4, "数学", 0, "question_handout"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		{ID: "grant-003", StudentID: "stu-003", PackageID: packageID(4, "语文", 0, "question"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		// Lucy、小航 同时开通 五年级英文，用于演示「同年级同学科」成班与协调。
		{ID: "grant-004", StudentID: "stu-002", PackageID: packageID(4, "英文", 0, "full"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
		{ID: "grant-005", StudentID: "stu-003", PackageID: packageID(4, "英文", 0, "question_handout"), StartsAt: "2026-05-22", EndsAt: "2027-05-22", Status: "active", EffectiveUntil: "2027-05-22"},
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
	case "英文", "英语":
		return "english"
	case "综合科学":
		return "integrated-science"
	case "科学":
		return "science"
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

// subjectsMatch 保持旧数据中的“英语”和新课程字典中的“英文”兼容。
// 两者共享 english slug，历史套餐、课程和教师范围不应因字典名称调整而失效。
func subjectsMatch(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == right {
		return true
	}
	return (left == "英文" && right == "英语") || (left == "英语" && right == "英文")
}

func subjectTextContains(value, subject string) bool {
	if strings.Contains(value, subject) {
		return true
	}
	if subject == "英文" {
		return strings.Contains(value, "英语")
	}
	if subject == "英语" {
		return strings.Contains(value, "英文")
	}
	return false
}

func phaseSlug(phaseIndex int) string {
	if phaseIndex == 0 {
		return "q1"
	}
	return "q2"
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
	if value == "S2" || strings.Contains(value, "第二") {
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
