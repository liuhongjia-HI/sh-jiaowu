package learning

type AvailabilitySlot struct {
	ID        string `json:"id"`
	OwnerType string `json:"ownerType"`
	OwnerID   string `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	DayOfWeek int    `json:"dayOfWeek"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Remark    string `json:"remark,omitempty"`
}

type AvailabilityUpsertRequest struct {
	OwnerType string             `json:"ownerType"`
	OwnerID   string             `json:"ownerId"`
	Slots     []AvailabilitySlot `json:"slots"`
}

type ScheduleCandidateRequest struct {
	Subject         string `json:"subject"`
	Grade           string `json:"grade"`
	Level           string `json:"level"`
	CourseID        string `json:"courseId"`
	TeacherID       string `json:"teacherId"`
	ClassType       string `json:"classType"`
	DurationMinutes int    `json:"durationMinutes"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
}

type CandidateStudent struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Grade          string   `json:"grade"`
	OpenedPackages []string `json:"openedPackages"`
}

type ScheduleCandidate struct {
	ID                string             `json:"id"`
	DayOfWeek         int                `json:"dayOfWeek"`
	StartTime         string             `json:"startTime"`
	EndTime           string             `json:"endTime"`
	TeacherID         string             `json:"teacherId"`
	TeacherName       string             `json:"teacherName"`
	CourseID          string             `json:"courseId"`
	CourseName        string             `json:"courseName"`
	Subject           string             `json:"subject"`
	Grade             string             `json:"grade"`
	Level             string             `json:"level"`
	ClassType         string             `json:"classType"`
	Capacity          int                `json:"capacity"`
	AvailableStudents []CandidateStudent `json:"availableStudents"`
	MissingStudents   []CandidateStudent `json:"missingStudents"`
	StudentCount      int                `json:"studentCount"`
	Score             int                `json:"score"`
	Reason            string             `json:"reason"`
}

type ScheduleClassCreateRequest struct {
	CourseID             string   `json:"courseId"`
	TeacherID            string   `json:"teacherId"`
	CampusID             string   `json:"campusId"`
	RoomName             string   `json:"roomName"`
	ClassType            string   `json:"classType"`
	DurationMinutes      int      `json:"durationMinutes"`
	DayOfWeek            int      `json:"dayOfWeek"`
	StartTime            string   `json:"startTime"`
	EndTime              string   `json:"endTime"`
	StartDate            string   `json:"startDate"`
	EndDate              string   `json:"endDate"`
	StudentIDs           []string `json:"studentIds"`
	ExpectedStudentCount int      `json:"expectedStudentCount"`
	ReservationNote      string   `json:"reservationNote"`
	// Repeat 为空表示只排一节课。
	Repeat *ScheduleRepeat `json:"repeat,omitempty"`
	// EditScope 只在修改重复课次时有意义，取 EditScope* 常量之一。
	EditScope string `json:"editScope,omitempty"`
	// IgnoreWarnings=true 表示调用方已在前端确认过软提醒（越出可上课时间），
	// 越界原因会写进课次的 OverrideNote 并留操作日志。
	IgnoreWarnings bool `json:"ignoreWarnings,omitempty"`
}

type ScheduleClass struct {
	ID string `json:"id"`
	// SeriesID 为空表示单次课。课次是排课结果的唯一载体，
	// 系列只负责生成它们，改系列不会反过来动已脱离的课次。
	SeriesID string `json:"seriesId,omitempty"`
	// LessonDate 是这节课的具体日期。StartDate/EndDate 对课次而言恒等于它，
	// 保留那两个字段是为了让既有的冲突判定与日期区间 helper 继续成立。
	LessonDate string `json:"lessonDate"`
	// Detached=true 表示这节课已被单独改过（编辑范围选了「仅此课次」），
	// 此后系列的批量改动一律跳过它。这是 split-series 做法的落点，
	// 好处是不必再维护一张例外表及其一致性。
	Detached        bool   `json:"detached,omitempty"`
	Name            string `json:"name"`
	CourseID        string `json:"courseId"`
	CourseName      string `json:"courseName"`
	TeacherID       string `json:"teacherId"`
	TeacherName     string `json:"teacherName"`
	CampusID        string `json:"campusId"`
	RoomName        string `json:"roomName"`
	ClassType       string `json:"classType"`
	Capacity        int    `json:"capacity"`
	DurationMinutes int    `json:"durationMinutes"`
	DayOfWeek       int    `json:"dayOfWeek"`
	StartTime       string `json:"startTime"`
	EndTime         string `json:"endTime"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	// AcademicYear 与 Semester 在建班时按开课日期落校历判定一次后固定下来，
	// 不随日后校历调整或学年切换而改变，避免历史排课的学年归属跟着漂移。
	AcademicYear         string             `json:"academicYear,omitempty"`
	Semester             string             `json:"semester,omitempty"`
	Students             []CandidateStudent `json:"students"`
	ExpectedStudentCount int                `json:"expectedStudentCount"`
	ReservationNote      string             `json:"reservationNote,omitempty"`
	// 审核维度，与 Status 的「成班维度」是两件事，不要混用：
	//   Status      = 待确认 / 已确认 / 已取消 —— 人数够不够、有没有被取消
	//   AuditStatus = 待审核 / 已通过 / 已驳回 —— 管理员认不认这节课
	// 两者共用「确认」二字会让谁都说不清，所以审核这边换一套词。
	// 只有 AuditStatus == AuditApproved 的课次才对学生可见、才发通知。
	AuditStatus string `json:"auditStatus"`
	AuditReason string `json:"auditReason,omitempty"`
	AuditedBy   string `json:"auditedBy,omitempty"`
	AuditedAt   string `json:"auditedAt,omitempty"`
	// CreatedBy/CreatedByRole 记录是谁排的这节课：老师排的要审，管理员排的直接生效。
	CreatedBy     string `json:"createdBy,omitempty"`
	CreatedByRole string `json:"createdByRole,omitempty"`
	// OverrideNote 记录这节课越过了哪些软校验（谁的可上课时间）。
	// 把可上课时间从硬拦截降级为软提醒的前提就是越界必须可追溯。
	OverrideNote string `json:"overrideNote,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
}

// ScheduleRepeat 是重复规则，对应 RRULE 的一个子集。
// 一期只实现 daily / weekly，monthly 与特殊日期按客户要求后续迭代，
// 但字段先切出来，届时不用改表结构：Interval 不写死成 1 就是为了这个。
type ScheduleRepeat struct {
	Freq     string `json:"freq"`            // daily | weekly；空 = 不重复，只排一节
	Interval int    `json:"interval"`        // 每 N 天 / 每 N 周，默认 1
	ByDay    []int  `json:"byDay,omitempty"` // weekly 时的星期集合，1=周一 ... 7=周日
	Until    string `json:"until,omitempty"` // 按日期结束
	Count    int    `json:"count,omitempty"` // 按次数结束；与 Until 二选一
}

// 系列身份直接落在课次的 SeriesID 上，没有单独的系列表：
// 判断「哪些课次属于同一次提交」用 SeriesID 就够了。独立的系列记录只有在
// 需要「修改重复规则本身并重新展开」时才有价值，那是后续迭代的事；
// 现阶段「整个系列」的语义是批量改属性（时间、老师、教室），
// 改规则本身走「删了重排」——Outlook 实际也是这么做的。

// 审核状态。管理员排课直接落 AuditApproved；老师排课落 AuditPending，
// 经管理员通过后才对学生可见。
const (
	AuditPending  = "待审核"
	AuditApproved = "已通过"
	AuditRejected = "已驳回"
)

// 审核请求。驳回必须给理由，否则老师不知道要改什么。
type ScheduleAuditRequest struct {
	Reason string `json:"reason"`
}

// 编辑范围。拖动或修改重复课次时必须由调用方明确指定，
// 与 Outlook 的「仅此课次 / 此课次及后续 / 整个系列」一一对应。
const (
	EditScopeThis          = "this"
	EditScopeThisAndFuture = "thisAndFuture"
	EditScopeAll           = "all"
)
