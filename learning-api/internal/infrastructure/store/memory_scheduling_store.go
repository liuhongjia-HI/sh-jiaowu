package store

import (
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"

	_ "github.com/go-sql-driver/mysql"
)

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
		if subject != "" && subjectTextContains(joined, subject) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) officialAccountOpenIDForTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	for _, student := range s.students {
		if student.OfficialAccountOpenID == "" {
			continue
		}
		if target == student.Name || target == student.Phone {
			return student.OfficialAccountOpenID
		}
	}
	return ""
}

func (s *MemoryStore) officialAccountOpenIDForStudent(studentID string) string {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return ""
	}
	for _, student := range s.students {
		if student.ID == studentID {
			return strings.TrimSpace(student.OfficialAccountOpenID)
		}
	}
	return ""
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

func (s *MemoryStore) studentAvailable(studentID string, dayOfWeek, startMin, endMin int, startDate, endDate string) bool {
	for _, slot := range s.ownerAvailability("student", studentID) {
		slotStart, _ := parseClock(slot.StartTime)
		slotEnd, _ := parseClock(slot.EndTime)
		if slot.DayOfWeek == dayOfWeek && slotStart <= startMin && slotEnd >= endMin && dateRangeContains(slot.StartDate, slot.EndDate, startDate, endDate) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) teacherAvailable(teacherID string, dayOfWeek, startMin, endMin int, startDate, endDate string) bool {
	for _, slot := range s.ownerAvailability("teacher", teacherID) {
		slotStart, _ := parseClock(slot.StartTime)
		slotEnd, _ := parseClock(slot.EndTime)
		if slot.DayOfWeek == dayOfWeek && slotStart <= startMin && slotEnd >= endMin && dateRangeContains(slot.StartDate, slot.EndDate, startDate, endDate) {
			return true
		}
	}
	return false
}

func (s *MemoryStore) hasScheduleConflict(ownerType, ownerID string, dayOfWeek, startMin, endMin int, startDate, endDate string) bool {
	return s.hasScheduleConflictExcept(ownerType, ownerID, dayOfWeek, startMin, endMin, startDate, endDate, "")
}

func (s *MemoryStore) hasScheduleConflictExcept(ownerType, ownerID string, dayOfWeek, startMin, endMin int, startDate, endDate, exceptID string) bool {
	for _, item := range s.scheduleClasses {
		if item.ID == exceptID || item.DayOfWeek != dayOfWeek || item.Status == "已取消" {
			continue
		}
		if !dateRangesOverlap(startDate, endDate, item.StartDate, item.EndDate) {
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

func (s *MemoryStore) hasRoomScheduleConflictExcept(campusID, roomName string, dayOfWeek, startMin, endMin int, startDate, endDate, exceptID string) bool {
	campusID = strings.TrimSpace(campusID)
	roomName = strings.TrimSpace(roomName)
	if roomName == "" {
		return false
	}
	for _, item := range s.scheduleClasses {
		if item.ID == exceptID || item.DayOfWeek != dayOfWeek || item.Status == "已取消" {
			continue
		}
		if !dateRangesOverlap(startDate, endDate, item.StartDate, item.EndDate) {
			continue
		}
		if strings.TrimSpace(item.RoomName) == "" || strings.TrimSpace(item.RoomName) != roomName || strings.TrimSpace(item.CampusID) != campusID {
			continue
		}
		itemStart, okStart := parseClock(item.StartTime)
		itemEnd, okEnd := parseClock(item.EndTime)
		if !okStart || !okEnd || endMin <= itemStart || startMin >= itemEnd {
			continue
		}
		return true
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
			campus_id VARCHAR(64) NOT NULL DEFAULT '',
			room_name VARCHAR(64) NOT NULL DEFAULT '',
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
			KEY idx_schedule_room_time (campus_id, room_name, day_of_week, start_time, end_time),
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

func (s *MemoryStore) loadScheduleClasses() ([]learning.ScheduleClass, error) {
	rows, err := s.db.Query(`SELECT id, name, course_id, course_name, teacher_id, teacher_name, campus_id, room_name, class_type, capacity, duration_minutes, day_of_week, start_time, end_time, start_date, end_date, status, created_at FROM schedule_classes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]learning.ScheduleClass, 0)
	for rows.Next() {
		var item learning.ScheduleClass
		var startDate, endDate, createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.CourseID, &item.CourseName, &item.TeacherID, &item.TeacherName, &item.CampusID, &item.RoomName, &item.ClassType, &item.Capacity, &item.DurationMinutes, &item.DayOfWeek, &item.StartTime, &item.EndTime, &startDate, &endDate, &item.Status, &createdAt); err != nil {
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

func validateDateRange(startDate, endDate string) error {
	start, hasStart, ok := parseDateBound(startDate)
	if !ok {
		return errors.New("开始日期格式应为 YYYY-MM-DD")
	}
	end, hasEnd, ok := parseDateBound(endDate)
	if !ok {
		return errors.New("结束日期格式应为 YYYY-MM-DD")
	}
	if hasStart && hasEnd && end.Before(start) {
		return errors.New("结束日期不能早于开始日期")
	}
	return nil
}

func dateRangeContains(containerStart, containerEnd, targetStart, targetEnd string) bool {
	if _, _, ok := parseDateBound(containerStart); !ok {
		return true
	}
	if _, _, ok := parseDateBound(containerEnd); !ok {
		return true
	}
	start, hasStart, ok := parseDateBound(targetStart)
	if !ok {
		return true
	}
	end, hasEnd, ok := parseDateBound(targetEnd)
	if !ok {
		return true
	}
	containerStartDate, hasContainerStart, _ := parseDateBound(containerStart)
	containerEndDate, hasContainerEnd, _ := parseDateBound(containerEnd)
	if hasStart && hasContainerStart && start.Before(containerStartDate) {
		return false
	}
	if hasEnd && hasContainerEnd && end.After(containerEndDate) {
		return false
	}
	return true
}

func dateRangesOverlap(startA, endA, startB, endB string) bool {
	aStart, hasAStart, ok := parseDateBound(startA)
	if !ok {
		return true
	}
	aEnd, hasAEnd, ok := parseDateBound(endA)
	if !ok {
		return true
	}
	bStart, hasBStart, ok := parseDateBound(startB)
	if !ok {
		return true
	}
	bEnd, hasBEnd, ok := parseDateBound(endB)
	if !ok {
		return true
	}
	if hasAEnd && hasBStart && aEnd.Before(bStart) {
		return false
	}
	if hasBEnd && hasAStart && bEnd.Before(aStart) {
		return false
	}
	return true
}

func parseDateBound(value string) (time.Time, bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, true
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, false, false
	}
	return parsed, true, true
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

func scheduleNoticeSummary(item learning.ScheduleClass, action string) string {
	parts := []string{
		item.CourseName + action,
		weekLabelCN(item.DayOfWeek) + " " + item.StartTime + "-" + item.EndTime,
		"老师：" + item.TeacherName,
	}
	if strings.TrimSpace(item.RoomName) != "" {
		parts = append(parts, "教室："+strings.TrimSpace(item.RoomName))
	}
	return strings.Join(parts, "，") + "。"
}

func weekLabelCN(day int) string {
	switch day {
	case 1:
		return "周一"
	case 2:
		return "周二"
	case 3:
		return "周三"
	case 4:
		return "周四"
	case 5:
		return "周五"
	case 6:
		return "周六"
	case 7:
		return "周日"
	default:
		return "上课日"
	}
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
