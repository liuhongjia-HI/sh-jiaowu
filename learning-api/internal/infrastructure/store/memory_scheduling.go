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
	} else {
		if req.Subject == "" || req.Grade == "" {
			return nil, errors.New("请选择学科和年级")
		}
		for _, course := range s.courses {
			if course.Status == learning.StatusEnabled && subjectsMatch(course.Subject, req.Subject) && course.Grade == req.Grade && canSeeCourse(principal, course) {
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

func (s *MemoryStore) createScheduleClassUnlocked(operator string, principal learning.Principal, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
			return work.createScheduleClassUnlocked(operator, principal, req)
		})
	}
	item, err := s.buildScheduleClass(principal, "", req)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	item.ID = "schedule-" + time.Now().Format("20060102150405.000000000")
	item.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	s.scheduleClasses = append([]learning.ScheduleClass{cloneScheduleClass(item)}, s.scheduleClasses...)
	if item.Status == "已确认" {
		s.notifyScheduleClass(item, "课程已安排", "已安排")
		s.prependLog(operator, "确认排课", item.Name+" / "+item.TeacherName)
	} else {
		s.prependLog(operator, "创建待确认排课", item.Name+" / "+item.TeacherName)
	}
	return cloneScheduleClass(item), nil
}

func (s *MemoryStore) buildScheduleClass(principal learning.Principal, exceptID string, req learning.ScheduleClassCreateRequest) (learning.ScheduleClass, error) {
	req.CourseID = strings.TrimSpace(req.CourseID)
	req.TeacherID = strings.TrimSpace(req.TeacherID)
	req.CampusID = strings.TrimSpace(req.CampusID)
	req.RoomName = strings.TrimSpace(req.RoomName)
	req.ClassType = strings.TrimSpace(req.ClassType)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
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
	if err := validateDateRange(req.StartDate, req.EndDate); err != nil {
		return learning.ScheduleClass{}, err
	}
	if req.DayOfWeek < 1 || req.DayOfWeek > 7 {
		return learning.ScheduleClass{}, errors.New("请选择星期")
	}
	if s.hasScheduleConflictExcept("teacher", teacher.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate, exceptID) {
		return learning.ScheduleClass{}, errors.New("老师该时间已有课程")
	}
	for _, student := range students {
		if s.hasScheduleConflictExcept("student", student.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate, exceptID) {
			return learning.ScheduleClass{}, errors.New(student.Name + " 该时间已有课程")
		}
	}
	if !s.teacherAvailable(teacher.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate) {
		return learning.ScheduleClass{}, errors.New("老师该时间不可授课")
	}
	for _, student := range students {
		if !s.studentAvailable(student.ID, req.DayOfWeek, startMin, endMin, req.StartDate, req.EndDate) {
			return learning.ScheduleClass{}, errors.New(student.Name + " 该时间不可上课")
		}
	}
	status := "待确认"
	if len(students) >= minClassStudents(capacity) {
		status = "已确认"
	}
	return learning.ScheduleClass{
		Name:                 course.Subject + " " + req.ClassType + " 小班",
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
		s.scheduleClasses[index] = cloneScheduleClass(item)
		if item.Status == "已确认" {
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
		if hasRole(principal.Roles, learning.RoleTeacher) || hasRole(principal.Roles, learning.RoleStudent) {
			return learning.ScheduleClass{}, errors.New("请联系教务调整课程")
		}
		if item.Status == "已取消" {
			return cloneScheduleClass(item), nil
		}
		before := item
		item.Status = "已取消"
		s.scheduleClasses[index] = cloneScheduleClass(item)
		if before.Status == "已确认" {
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
