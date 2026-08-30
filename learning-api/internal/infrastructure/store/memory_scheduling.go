package store

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

func (s *MemoryStore) availabilityUnlocked(principal learning.Principal, ownerType, ownerID string) ([]learning.AvailabilitySlot, error) {
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

func (s *MemoryStore) availabilityOverviewUnlocked(principal learning.Principal) []learning.AvailabilitySlot {
	out := make([]learning.AvailabilitySlot, 0)
	for _, slot := range s.availability {
		if slot.OwnerType == "teacher" {
			teacher, ok := s.findUser(slot.OwnerID)
			if !ok || !isActiveTeacher(teacher) {
				continue
			}
		}
		if s.canManageAvailability(principal, slot.OwnerType, slot.OwnerID) {
			out = append(out, slot)
		}
	}
	sortAvailability(out)
	return out
}

func (s *MemoryStore) saveAvailabilityUnlocked(operator string, principal learning.Principal, req learning.AvailabilityUpsertRequest) ([]learning.AvailabilitySlot, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) ([]learning.AvailabilitySlot, error) {
			return work.saveAvailabilityUnlocked(operator, principal, req)
		})
	}
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
		if err := validateDateRange(slot.StartDate, slot.EndDate); err != nil {
			return nil, err
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
	s.availability = next
	sortAvailability(slots)
	s.prependLog(operator, "维护可上课时间", ownerName)
	return slots, nil
}

func (s *MemoryStore) scheduleCandidatesUnlocked(principal learning.Principal, req learning.ScheduleCandidateRequest) ([]learning.ScheduleCandidate, error) {
	req.Subject = strings.TrimSpace(req.Subject)
	req.Grade = strings.TrimSpace(req.Grade)
	req.Level = strings.TrimSpace(req.Level)
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.ClassType = strings.TrimSpace(req.ClassType)
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 90
	}
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return nil, err
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
		req.Level = s.learningSpaceLevel(course.LearningSpaceID)
	} else {
		if req.Subject == "" || req.Grade == "" {
			return nil, errors.New("请选择学科和年级")
		}
		if req.Level == "" {
			req.Level = "S"
		}
		if !validLearningLevel(req.Level) {
			return nil, errors.New("请选择正确的课程等级")
		}
		for _, course := range s.courses {
			if course.Status == learning.StatusEnabled && subjectsMatch(course.Subject, req.Subject) && course.Grade == req.Grade && s.learningSpaceLevel(course.LearningSpaceID) == req.Level && canSeeCourse(principal, course) {
				targetCourses = append(targetCourses, course)
			}
		}
		if len(targetCourses) == 0 {
			return nil, errors.New("没有该学科 + 年级 + 等级的可排课程")
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
		if !isActiveTeacher(user) {
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
		if !s.canSeeStudent(principal, decorated, s.coursesForStudent(student.ID)) {
			continue
		}
		if decorated.Grade != req.Grade || !s.studentHasSubjectGradeLevel(student.ID, req.Subject, req.Grade, req.Level) {
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
			if !dateRangeContains(teacherSlot.StartDate, teacherSlot.EndDate, req.StartDate, req.EndDate) {
				continue
			}
			for candidateStart := startMin; candidateStart+req.DurationMinutes <= endMin; candidateStart += 30 {
				candidateEnd := candidateStart + req.DurationMinutes
				if s.hasScheduleConflict("teacher", teacher.ID, teacherSlot.DayOfWeek, candidateStart, candidateEnd, req.StartDate, req.EndDate) {
					continue
				}
				available := make([]learning.CandidateStudent, 0)
				missing := make([]learning.CandidateStudent, 0)
				for _, student := range eligible {
					if !s.hasScheduleConflict("student", student.ID, teacherSlot.DayOfWeek, candidateStart, candidateEnd, req.StartDate, req.EndDate) &&
						s.studentAvailable(student.ID, teacherSlot.DayOfWeek, candidateStart, candidateEnd, req.StartDate, req.EndDate) {
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
					Level:             req.Level,
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

func (s *MemoryStore) scheduleClassesUnlocked(principal learning.Principal) []learning.ScheduleClass {
	out := make([]learning.ScheduleClass, 0)
	for _, item := range s.scheduleClasses {
		if s.canSeeScheduleClass(principal, item) {
			out = append(out, cloneScheduleClass(item))
		}
	}
	return out
}

func (s *MemoryStore) lessonFeedbacksUnlocked(principal learning.Principal, classID string) ([]learning.LessonFeedback, error) {
	var class learning.ScheduleClass
	for _, item := range s.scheduleClasses {
		if item.ID == strings.TrimSpace(classID) {
			class = item
			break
		}
	}
	if class.ID == "" || !s.canSeeScheduleClass(principal, class) {
		return nil, errors.New("没有权限查看该课次反馈")
	}
	out := make([]learning.LessonFeedback, 0)
	for _, item := range s.lessonFeedbacks {
		if item.ScheduleClassID == class.ID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *MemoryStore) upsertLessonFeedbackUnlocked(operator string, principal learning.Principal, classID string, req learning.LessonFeedbackUpsertRequest) (learning.LessonFeedback, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.LessonFeedback, error) {
			return work.upsertLessonFeedbackUnlocked(operator, principal, classID, req)
		})
	}
	req.StudentID, req.Summary, req.Homework, req.NextStep = strings.TrimSpace(req.StudentID), sanitizeRichText(req.Summary), sanitizeRichText(req.Homework), sanitizeRichText(req.NextStep)
	if req.StudentID == "" || req.Summary == "" {
		return learning.LessonFeedback{}, errors.New("请选择学生并填写本节课反馈")
	}
	for _, class := range s.scheduleClasses {
		if class.ID != strings.TrimSpace(classID) {
			continue
		}
		if class.TeacherID != principal.UserID && !scheduleCanApprove(principal) {
			return learning.LessonFeedback{}, errors.New("只能填写自己授课的课后反馈")
		}
		if class.Status == "已取消" || class.AuditStatus != learning.AuditApproved {
			return learning.LessonFeedback{}, errors.New("课程未生效，不能填写课后反馈")
		}
		var student learning.CandidateStudent
		for _, item := range class.Students {
			if item.ID == req.StudentID {
				student = item
				break
			}
		}
		if student.ID == "" {
			return learning.LessonFeedback{}, errors.New("该学生不在本节课名单中")
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		for index := range s.lessonFeedbacks {
			item := &s.lessonFeedbacks[index]
			if item.ScheduleClassID == class.ID && item.StudentID == student.ID {
				item.Summary, item.Homework, item.NextStep, item.UpdatedAt = req.Summary, req.Homework, req.NextStep, now
				return *item, nil
			}
		}
		item := learning.LessonFeedback{ID: "feedback-" + class.ID + "-" + student.ID, ScheduleClassID: class.ID, StudentID: student.ID, StudentName: student.Name, TeacherID: class.TeacherID, TeacherName: class.TeacherName, CourseName: class.CourseName, LessonDate: class.LessonDate, Summary: req.Summary, Homework: req.Homework, NextStep: req.NextStep, CreatedAt: now, UpdatedAt: now}
		s.lessonFeedbacks = append(s.lessonFeedbacks, item)
		s.prependLogDetail(operator, "填写课后反馈", student.Name, class.CourseName)
		return item, nil
	}
	return learning.LessonFeedback{}, errors.New("课程不存在")
}

func (s *MemoryStore) createScheduleClassUnlocked(operator string, principal learning.Principal, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
			return work.createScheduleClassUnlocked(operator, principal, req)
		})
	}
	repeat, err := normalizeRepeat(req.Repeat)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	dates, err := expandRepeatDates(repeat, strings.TrimSpace(req.StartDate))
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	stamp := time.Now().Format("20060102150405.000000000")
	operatorName := parseAuditOperator(operator).Name
	seriesID := ""
	if repeat.Freq != "" {
		seriesID = "series-" + stamp
	}
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 先把每一节都构造并校验一遍，任何一节不合法就整批拒绝。
	// 不能边建边校验：那样会在课表里留下半截课程，用户既看不出排到哪断了，
	// 也没法一键回退。
	built := make([]learning.ScheduleClass, 0, len(dates))
	for index, date := range dates {
		item, err := s.buildScheduleClass(principal, "", date, req)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		item.SeriesID = seriesID
		item.ID = "schedule-" + stamp + "-" + itoa(index)
		item.CreatedAt = createdAt
		item.CreatedBy = operatorName
		item.CreatedByRole = schedulePrincipalRole(principal)
		item.AuditStatus = initialAuditStatus(principal)
		if item.AuditStatus == learning.AuditApproved {
			item.AuditedBy = operatorName
			item.AuditedAt = createdAt
		}
		built = append(built, item)
	}

	prepended := make([]learning.ScheduleClass, 0, len(built))
	for _, item := range built {
		prepended = append(prepended, cloneScheduleClass(item))
	}
	s.scheduleClasses = append(prepended, s.scheduleClasses...)

	first := built[0]
	// 通知按课次发，但一次重复排课只给学生发一条汇总，避免一口气排 40 节
	// 就往家长那边推 40 条模板消息。
	// 待审核的课不发通知：还没被管理员认可的安排不能先惊动家长。
	if first.AuditStatus == learning.AuditApproved && first.Status == "已确认" {
		s.notifyScheduleClass(first, "课程已安排", "已安排")
	}
	s.prependLog(operator, scheduleCreateLogAction(first), scheduleBatchLogTarget(first, len(built)))
	return cloneScheduleClass(first), nil
}

// 老师排的课要经管理员确认；管理员自己排的课直接生效，不用再找老师确认。
func initialAuditStatus(principal learning.Principal) string {
	if scheduleCanApprove(principal) {
		return learning.AuditApproved
	}
	return learning.AuditPending
}

func scheduleCanApprove(principal learning.Principal) bool {
	return hasRole(principal.Roles, learning.RoleSuperAdmin) ||
		hasRole(principal.Roles, learning.RoleCampusAdmin) ||
		hasRole(principal.Roles, learning.RoleOpsStaff)
}

func schedulePrincipalRole(principal learning.Principal) string {
	if scheduleCanApprove(principal) {
		return "admin"
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		return string(learning.RoleTeacher)
	}
	return ""
}

func scheduleCreateLogAction(item learning.ScheduleClass) string {
	if item.AuditStatus == learning.AuditPending {
		return "提交待审核排课"
	}
	if item.Status == "已确认" {
		return "确认排课"
	}
	return "创建待成班排课"
}

func scheduleBatchLogTarget(item learning.ScheduleClass, count int) string {
	target := item.Name + " / " + item.TeacherName
	if count > 1 {
		target += "（共 " + itoa(count) + " 节）"
	}
	return target
}

// buildScheduleClass 构造「一节课」。lessonDate 是这节课的具体日期，
// StartDate/EndDate 对课次而言恒等于它——这样既有的冲突判定与日期区间
// helper（hasScheduleConflictExcept / dateRangesOverlap 等）不用改就继续成立。
func (s *MemoryStore) buildScheduleClass(principal learning.Principal, exceptID, lessonDate string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	lessonDate = strings.TrimSpace(lessonDate)
	if lessonDate == "" {
		return learning.ScheduleClass{}, errors.New("请选择上课日期")
	}
	parsedDate, hasDate, ok := parseDateBound(lessonDate)
	if !ok || !hasDate {
		return learning.ScheduleClass{}, errors.New("上课日期格式应为 YYYY-MM-DD")
	}
	// 课次自己就是一天，星期由日期推导，不再由调用方传 DayOfWeek 决定，
	// 从根上杜绝「拖动一节课却改掉整个学期星期」这类问题。
	req.DayOfWeek = weekdayOf(parsedDate)
	req.StartDate = lessonDate
	req.EndDate = lessonDate
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.RoomName = strings.TrimSpace(req.RoomName)
	req.ClassType = strings.TrimSpace(req.ClassType)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)
	req.ReservationNote = strings.TrimSpace(req.ReservationNote)
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 90
	}
	if req.CampusID == "" {
		req.CampusID = principal.CampusID
	}
	if req.CampusID == "" {
		req.CampusID = "campus-main"
	}
	// teacherId 是外部请求字段，不能依赖前端只展示当前教师来保证权限。
	// 教师只能给自己排课；管理员仍可为任意可管理教师排课。
	if !scheduleCanApprove(principal) && hasRole(principal.Roles, learning.RoleTeacher) && req.TeacherID != principal.UserID {
		return learning.ScheduleClass{}, errors.New("教师只能给自己排课")
	}
	course, err := s.courseForScheduling(principal, req.CourseID)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	teacher, ok := s.findUser(req.TeacherID)
	if !ok || !hasRole(teacher.Roles, learning.RoleTeacher) {
		return learning.ScheduleClass{}, errors.New("请选择老师")
	}
	if teacher.AccountStatus != "正常" {
		return learning.ScheduleClass{}, errors.New("该教师账号已停用，不能排课")
	}
	capacity := classCapacity(req.ClassType)
	if capacity <= 0 {
		return learning.ScheduleClass{}, errors.New("请选择正确班型")
	}
	if len(req.StudentIDs) > capacity {
		return learning.ScheduleClass{}, errors.New("学生人数超过班型容量")
	}
	courseLevel := s.learningSpaceLevel(course.LearningSpaceID)
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
			// 学生列表与详情只能看有效辅导关系；排课提交则是受课程空间
			// 约束的窄入口，允许老师为自己负责的课程录入实际到课学生。
			// 这样既不会把全量学生重新暴露给老师，也兼容存量排课数据。
			if hasRole(principal.Roles, learning.RoleTeacher) && principal.UserID == teacher.ID && containsString(teacher.LearningSpaceIDs, course.LearningSpaceID) {
				var found bool
				student, found = s.findStudent(studentID)
				if !found {
					return learning.ScheduleClass{}, errors.New("学生不存在")
				}
			} else {
				return learning.ScheduleClass{}, err
			}
		}
		if student.Grade != course.Grade {
			return learning.ScheduleClass{}, errors.New(student.Name + " 与班级年级不一致，只有同年级才能排一起")
		}
		if !s.studentHasSubjectGradeLevel(student.ID, course.Subject, course.Grade, courseLevel) {
			return learning.ScheduleClass{}, errors.New(student.Name + " 未开通该学科等级，只有同学科同等级才能排一起")
		}
		students = append(students, learning.CandidateStudent{ID: student.ID, Name: student.Name, Grade: student.Grade, OpenedPackages: student.OpenedPackages})
	}
	if len([]rune(req.ReservationNote)) > 255 {
		return learning.ScheduleClass{}, errors.New("预约备注最多255个字")
	}
	if req.ExpectedStudentCount <= 0 {
		req.ExpectedStudentCount = minClassStudents(capacity)
	}
	if req.ExpectedStudentCount < len(students) {
		req.ExpectedStudentCount = len(students)
	}
	if req.ExpectedStudentCount > capacity {
		return learning.ScheduleClass{}, errors.New("预计人数不能超过班型容量")
	}
	startMin, ok := parseClock(req.StartTime)
	if !ok {
		return learning.ScheduleClass{}, errors.New("开始时间格式应为 HH:mm")
	}
	endMin, ok := parseClock(req.EndTime)
	if !ok || endMin <= startMin {
		return learning.ScheduleClass{}, errors.New("结束时间必须晚于开始时间")
	}
	// 硬拦截：物理上不可能的事。撞课、超容量、学科年级不符一律当场拒绝。
	// 教室不在硬拦截之列——现有产品决策是教室只作为登记信息、不阻塞排课，
	// 见 TestScheduleClassKeepsRoomMetadataWithoutBlocking。
	if s.hasScheduleConflictExcept("teacher", teacher.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate, exceptID) {
		return learning.ScheduleClass{}, errors.New(lessonDate + " 老师该时间已有课程")
	}
	for _, student := range students {
		if s.hasScheduleConflictExcept("student", student.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate, exceptID) {
			return learning.ScheduleClass{}, errors.New(lessonDate + " " + student.Name + " 该时间已有课程")
		}
	}
	// 软提醒：越出可上课时间。客户明确管理员可能线下已跟师生约好时间，
	// 可上课时间是匹配与监管的参考范围，不是排课的硬约束，所以这里不再直接
	// 拒绝，而是交由调用方确认（IgnoreWarnings），越界内容写进 OverrideNote 留痕。
	warnings := make([]string, 0, 2)
	if !s.teacherAvailable(teacher.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate) {
		warnings = append(warnings, lessonDate+" 超出 "+teacher.Name+" 的可上课时间")
	}
	for _, student := range students {
		if !s.studentAvailable(student.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate) {
			warnings = append(warnings, lessonDate+" 超出 "+student.Name+" 的可上课时间")
		}
	}
	if len(warnings) > 0 && !req.IgnoreWarnings {
		return learning.ScheduleClass{}, errors.New(strings.Join(warnings, "；") + "。确认要继续排这节课，请再次提交确认。")
	}
	status := "待确认"
	if len(students) >= minClassStudents(capacity) {
		status = "已确认"
	}
	// 学年、学期按开课日期落校历判定一次，写入排课记录后不再变化，
	// 见 resolveScheduleTerm；fallbackSemester 兜底取自课程所属学习空间。
	fallbackSemester := ""
	if space, ok := s.findLearningSpace(course.LearningSpaceID); ok {
		fallbackSemester = space.Semester
	}
	academicYear, semester := s.resolveScheduleTerm(req.StartDate, fallbackSemester)
	return learning.ScheduleClass{
		LessonDate:   lessonDate,
		OverrideNote: strings.Join(warnings, "；"),
		// 标题按客户在 Outlook 里的约定拼：教师 年级 科目 学生，
		// 见 scheduleClassName。原来是「英文 1V1 小班」，把班型放在最前面——
		// 而班型在课程块上本来就有独立标签，不需要再占标题的开头。
		Name:                 scheduleClassName(teacher.Name, course.Grade, s.subjectShortLabel(course.Subject), students),
		CourseID:             course.ID,
		CourseName:           course.Name,
		TeacherID:            teacher.ID,
		TeacherName:          teacher.Name,
		CampusID:             req.CampusID,
		RoomName:             req.RoomName,
		ClassType:            req.ClassType,
		Capacity:             capacity,
		DurationMinutes:      req.DurationMinutes,
		DayOfWeek:            req.DayOfWeek,
		StartTime:            req.StartTime,
		EndTime:              req.EndTime,
		StartDate:            req.StartDate,
		EndDate:              req.EndDate,
		AcademicYear:         academicYear,
		Semester:             semester,
		Students:             students,
		ExpectedStudentCount: req.ExpectedStudentCount,
		ReservationNote:      req.ReservationNote,
		Status:               status,
	}, nil
}

func (s *MemoryStore) updateScheduleClassUnlocked(operator string, principal learning.Principal, id string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
			return work.updateScheduleClassUnlocked(operator, principal, id, req)
		})
	}
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
		if err := scheduleEditPermission(principal, existing); err != nil {
			return learning.ScheduleClass{}, err
		}
		if existing.Status == "已取消" {
			return learning.ScheduleClass{}, errors.New("已取消课程不能调课")
		}
		scope, err := resolveEditScope(existing, req.EditScope)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		if scope != learning.EditScopeThis {
			return s.updateScheduleSeriesUnlocked(operator, principal, existing, scope, req)
		}
		lessonDate := strings.TrimSpace(req.StartDate)
		if lessonDate == "" {
			lessonDate = existing.LessonDate
		}
		item, err := s.buildScheduleClass(principal, id, lessonDate, req)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
		item.SeriesID = existing.SeriesID
		carryScheduleAuditFieldsAfterEdit(principal, existing, &item)
		// 单独改过的课次就此脱离系列，之后对系列的批量改动一律绕开它。
		// 这是 split-series 的落点：不必再维护一张例外表和它的一致性。
		item.Detached = existing.Detached || existing.SeriesID != ""
		// 开课日期和课程都没变时，学年/学期沿用原判定，不因为校历后来被
		// 改过、或者只是调了教室/人数这类无关字段而跟着漂移；
		// 真正改了开课日期或课程才重新按 resolveScheduleTerm 落一次。
		if existing.StartDate == item.StartDate && existing.CourseID == item.CourseID {
			item.AcademicYear = existing.AcademicYear
			item.Semester = existing.Semester
		}
		s.scheduleClasses[index] = cloneScheduleClass(item)
		if item.AuditStatus == learning.AuditApproved && item.Status == "已确认" {
			s.notifyScheduleClass(item, "课程调整提醒", "已调整")
		}
		s.prependLogDetail(operator, "调整排课", item.Name+" / "+item.TeacherName, auditChangeDetail(scheduleClassAuditSnapshot(existing), scheduleClassAuditSnapshot(item)))
		return cloneScheduleClass(item), nil
	}
	return learning.ScheduleClass{}, errors.New("课程不存在")
}

func (s *MemoryStore) cancelScheduleClassUnlocked(operator string, principal learning.Principal, id string) (learning.ScheduleClass, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
			return work.cancelScheduleClassUnlocked(operator, principal, id)
		})
	}
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
		if err := scheduleEditPermission(principal, item); err != nil {
			return learning.ScheduleClass{}, err
		}
		if item.Status == "已取消" {
			return cloneScheduleClass(item), nil
		}
		before := item
		item.Status = "已取消"
		s.scheduleClasses[index] = cloneScheduleClass(item)
		if before.AuditStatus == learning.AuditApproved && before.Status == "已确认" {
			s.notifyScheduleClass(item, "课程取消提醒", "已取消")
		}
		s.prependLogDetail(operator, "取消排课", item.Name+" / "+item.TeacherName, auditChangeDetail(scheduleClassAuditSnapshot(before), scheduleClassAuditSnapshot(item)))
		return cloneScheduleClass(item), nil
	}
	return learning.ScheduleClass{}, errors.New("课程不存在")
}

func (s *MemoryStore) notifyScheduleClass(item learning.ScheduleClass, title, action string) {
	for _, candidate := range item.Students {
		target := candidate.Name
		openID := ""
		if student, ok := s.findStudent(candidate.ID); ok {
			target = student.Name
			openID = student.OfficialAccountOpenID
		}
		notice := learning.Notice{
			ID:              "notice-schedule-" + action + "-" + item.ID + "-" + candidate.ID + "-" + time.Now().Format("20060102150405.000000000"),
			Type:            "课",
			Title:           title,
			Target:          target,
			Summary:         scheduleNoticeSummary(item, action),
			Channel:         "公众号模板消息",
			RecipientOpenID: openID,
			RelatedType:     "schedule",
			RelatedID:       item.ID,
		}
		notice = s.deliverNotice(notice)
		s.prependNoticeRecord(notice)
	}
}

// updateScheduleSeriesUnlocked 处理「此课次及后续」和「整个系列」两种范围。
//
// 两条硬规则：
//  1. 已上过的课次一律不动。历史课次背后挂着考勤和课时消耗，改它等于篡改已发生的事实。
//     所以「整个系列」的实际含义是「未来所有未脱离的课次」。
//  2. 改期按整体平移处理，不是把所有课次改到同一天——后者会把一学期的课压在一天上。
func (s *MemoryStore) updateScheduleSeriesUnlocked(operator string, principal learning.Principal, anchor learning.ScheduleClass, scope string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	newDate := strings.TrimSpace(req.StartDate)
	if newDate == "" {
		newDate = anchor.LessonDate
	}
	offset, err := dayOffset(anchor.LessonDate, newDate)
	if err != nil {
		return learning.ScheduleClass{}, err
	}

	today := time.Now().Format("2006-01-02")
	// 「整个系列」也从今天算起：历史课次不参与重排。
	boundary := today
	if scope == learning.EditScopeThisAndFuture && anchor.LessonDate > boundary {
		boundary = anchor.LessonDate
	}

	original := s.scheduleClasses
	targets := make([]learning.ScheduleClass, 0, 8)
	targetIDs := map[string]bool{}
	for _, item := range original {
		if item.SeriesID != anchor.SeriesID || item.Detached || item.Status == "已取消" {
			continue
		}
		if item.LessonDate < boundary {
			continue
		}
		// 批量调整必须逐节校验权限，不能只信任作为入口的锚点课次。
		// 同一系列可能部分已通过、部分仍待审核，教师不能借后者修改前者。
		if err := scheduleEditPermission(principal, item); err != nil {
			return learning.ScheduleClass{}, errors.New("系列中包含无权调整的课程：" + err.Error())
		}
		targets = append(targets, item)
		targetIDs[item.ID] = true
	}
	if len(targets) == 0 {
		return learning.ScheduleClass{}, errors.New("这个系列没有可调整的未来课次")
	}

	// 先把待重排的课次从课表里摘掉再逐节重建，否则系列整体平移时，
	// 还没挪的兄弟课次会跟已挪的课次互相判为撞课。
	// 重建过程中逐节放回，这样批内真撞上了仍然查得出来。
	remaining := make([]learning.ScheduleClass, 0, len(original))
	for _, item := range original {
		if !targetIDs[item.ID] {
			remaining = append(remaining, item)
		}
	}
	s.scheduleClasses = remaining

	rebuilt := make(map[string]learning.ScheduleClass, len(targets))
	for _, target := range targets {
		date, err := shiftDate(target.LessonDate, offset)
		if err != nil {
			s.scheduleClasses = original
			return learning.ScheduleClass{}, err
		}
		item, err := s.buildScheduleClass(principal, target.ID, date, req)
		if err != nil {
			s.scheduleClasses = original
			return learning.ScheduleClass{}, err
		}
		item.ID = target.ID
		item.CreatedAt = target.CreatedAt
		item.SeriesID = target.SeriesID
		carryScheduleAuditFieldsAfterEdit(principal, target, &item)
		if target.LessonDate == item.LessonDate && target.CourseID == item.CourseID {
			item.AcademicYear = target.AcademicYear
			item.Semester = target.Semester
		}
		rebuilt[target.ID] = item
		s.scheduleClasses = append(s.scheduleClasses, cloneScheduleClass(item))
	}

	// 按原顺序写回，避免整批课次因为重排而跑到列表末尾。
	final := make([]learning.ScheduleClass, 0, len(original))
	for _, item := range original {
		if replacement, ok := rebuilt[item.ID]; ok {
			final = append(final, cloneScheduleClass(replacement))
			continue
		}
		final = append(final, item)
	}
	s.scheduleClasses = final

	result := rebuilt[anchor.ID]
	if result.ID == "" {
		result = rebuilt[targets[0].ID]
	}
	if result.AuditStatus == learning.AuditApproved && result.Status == "已确认" {
		s.notifyScheduleClass(result, "课程调整提醒", "已调整")
	}
	s.prependLogDetail(operator, scheduleScopeLogAction(scope), scheduleBatchLogTarget(result, len(targets)), auditChangeDetail(scheduleClassAuditSnapshot(anchor), scheduleClassAuditSnapshot(result)))
	return cloneScheduleClass(result), nil
}

func scheduleScopeLogAction(scope string) string {
	if scope == learning.EditScopeAll {
		return "调整整个系列排课"
	}
	return "调整本次及后续排课"
}

// scheduleEditPermission 决定谁能改一节已经排好的课。
//
// 管理员随时可以改。老师只能改自己提交、且还没被审核通过的课：
// 一旦通过，学生和家长已经看到并按这个时间安排了，再让老师单方面改
// 等于绕过审核——那种情况走教务。
func scheduleEditPermission(principal learning.Principal, item learning.ScheduleClass) error {
	if scheduleCanApprove(principal) {
		return nil
	}
	if hasRole(principal.Roles, learning.RoleTeacher) {
		if item.TeacherID != principal.UserID {
			return errors.New("只能调整自己的课程")
		}
		if item.AuditStatus != learning.AuditPending && item.AuditStatus != learning.AuditRejected {
			return errors.New("该课程已通过审核，请联系教务调整")
		}
		return nil
	}
	return errors.New("请联系教务调整课程")
}

// reviewScheduleClassUnlocked 是管理员对老师提交的排课作出裁决的唯一入口。
// approve=false 时必须给理由，否则老师不知道要改什么。
func (s *MemoryStore) reviewScheduleClassUnlocked(operator string, principal learning.Principal, id string, approve bool, reason string) (learning.ScheduleClass, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
			return work.reviewScheduleClassUnlocked(operator, principal, id, approve, reason)
		})
	}
	if !scheduleCanApprove(principal) {
		return learning.ScheduleClass{}, errors.New("没有权限审核排课")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return learning.ScheduleClass{}, errors.New("请选择要审核的课程")
	}
	reason = strings.TrimSpace(reason)
	if !approve && reason == "" {
		return learning.ScheduleClass{}, errors.New("驳回时请填写理由")
	}
	if len([]rune(reason)) > 255 {
		return learning.ScheduleClass{}, errors.New("审核理由最多255个字")
	}
	for index, existing := range s.scheduleClasses {
		if existing.ID != id {
			continue
		}
		if existing.Status == "已取消" {
			return learning.ScheduleClass{}, errors.New("已取消课程不需要审核")
		}
		if existing.AuditStatus != learning.AuditPending {
			return learning.ScheduleClass{}, errors.New("该课程已经审核过了")
		}
		item := existing
		item.AuditReason = reason
		item.AuditedBy = parseAuditOperator(operator).Name
		item.AuditedAt = time.Now().Format("2006-01-02 15:04:05")
		if approve {
			item.AuditStatus = learning.AuditApproved
		} else {
			item.AuditStatus = learning.AuditRejected
		}
		s.scheduleClasses[index] = cloneScheduleClass(item)
		// 通知只在通过之后才发：驳回的课学生本来就没见过，不该收到任何提醒。
		if item.AuditStatus == learning.AuditApproved && item.Status == "已确认" {
			s.notifyScheduleClass(item, "课程已安排", "已安排")
		}
		action := "驳回排课"
		if approve {
			action = "通过排课审核"
		}
		s.prependLogDetail(operator, action, item.Name+" / "+item.TeacherName+" / "+item.LessonDate, reason)
		return cloneScheduleClass(item), nil
	}
	return learning.ScheduleClass{}, errors.New("课程不存在")
}

// pendingScheduleClassesUnlocked 返回待审核队列。
func (s *MemoryStore) pendingScheduleClassesUnlocked(principal learning.Principal) []learning.ScheduleClass {
	out := make([]learning.ScheduleClass, 0)
	for _, item := range s.scheduleClasses {
		if item.AuditStatus != learning.AuditPending || item.Status == "已取消" {
			continue
		}
		if !s.canSeeScheduleClass(principal, item) {
			continue
		}
		out = append(out, cloneScheduleClass(item))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LessonDate == out[j].LessonDate {
			return out[i].StartTime < out[j].StartTime
		}
		return out[i].LessonDate < out[j].LessonDate
	})
	return out
}

// carryScheduleAuditFieldsAfterEdit 把审核轨迹从旧课次搬到重建出来的课次上。
//
// buildScheduleClass 是从请求重新构造一节课的，不带审核信息。少了这一步，
// 管理员改一节已通过的课会让它退回空审核状态，
// 然后被 scheduleVisibleToStudent 判为不可见——课在学生端直接消失。
//
// 管理员改课时保留原审核状态；老师改待审核课程时仍然待审核；
// 老师修改被驳回课程则视为重新提交，清掉旧结论并回到待审核。
func carryScheduleAuditFieldsAfterEdit(principal learning.Principal, existing learning.ScheduleClass, item *learning.ScheduleClass) {
	item.AuditStatus = existing.AuditStatus
	item.AuditReason = existing.AuditReason
	item.AuditedBy = existing.AuditedBy
	item.AuditedAt = existing.AuditedAt
	item.CreatedBy = existing.CreatedBy
	item.CreatedByRole = existing.CreatedByRole
	if !scheduleCanApprove(principal) && hasRole(principal.Roles, learning.RoleTeacher) && existing.AuditStatus == learning.AuditRejected {
		item.AuditStatus = learning.AuditPending
		item.AuditReason = ""
		item.AuditedBy = ""
		item.AuditedAt = ""
	}
}
