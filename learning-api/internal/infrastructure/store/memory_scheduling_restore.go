package store

import (
	"errors"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"time"
)

func (s *MemoryStore) restoreScheduleCandidate(p learning.Principal, id string, ignoreWarnings bool) (learning.ScheduleClass, error) {
	var original learning.ScheduleClass
	for _, item := range s.scheduleClasses {
		if item.ID == id {
			original = item
			break
		}
	}
	if original.ID == "" || !s.canSeeScheduleClass(p, original) {
		return original, errors.New("课程不存在或无权恢复")
	}
	if err := scheduleEditPermission(p, original); err != nil {
		return original, err
	}
	if original.Status != "已取消" {
		return original, errors.New("只有已取消课次可以恢复")
	}
	now := time.Now()
	if original.LessonDate == "" || original.LessonDate < now.Format("2006-01-02") || original.LessonDate == now.Format("2006-01-02") && original.EndTime <= now.Format("15:04") {
		return original, errors.New("已结束的历史课次不能恢复")
	}
	course, exists := s.findCourse(original.CourseID)
	teacher, teacherExists := s.findUser(original.TeacherID)
	if !exists || !teacherExists || !containsString(teacher.LearningSpaceIDs, course.LearningSpaceID) {
		return original, errors.New("课程已不在老师授课范围，请先调整授课范围")
	}
	students := []string{}
	for _, student := range original.Students {
		students = append(students, student.ID)
	}
	req := learning.ScheduleClassCreateRequest{CourseID: original.CourseID, TeacherID: original.TeacherID, CampusID: original.CampusID, RoomName: original.RoomName, ClassType: original.ClassType, DurationMinutes: original.DurationMinutes, StartDate: original.LessonDate, StartTime: original.StartTime, EndTime: original.EndTime, StudentIDs: students, ExpectedStudentCount: original.ExpectedStudentCount, ReservationNote: original.ReservationNote, IgnoreWarnings: ignoreWarnings}
	// No previous assignment exemption: restoring reserves the slot again and
	// must use the teacher's current active teaching scope.
	item, err := s.buildScheduleClass(p, original.ID, original.LessonDate, req)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	item.ID, item.CreatedAt, item.SeriesID, item.Detached = original.ID, original.CreatedAt, original.SeriesID, original.Detached
	item.AcademicYear, item.Semester = original.AcademicYear, original.Semester
	carryScheduleAuditFieldsAfterEdit(p, original, &item)
	return item, nil
}

func (s *MemoryStore) PreviewRestoreScheduleClass(p learning.Principal, id string) (learning.SchedulePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.cloneForMutation().restoreScheduleCandidate(p, id, true)
	preview := learning.SchedulePreview{CanSave: err == nil, Lessons: []learning.SchedulePreviewLesson{}}
	lesson := learning.SchedulePreviewLesson{Date: item.LessonDate, StartTime: item.StartTime, EndTime: item.EndTime, Errors: []string{}, Warnings: []string{}}
	if err != nil {
		lesson.Errors = append(lesson.Errors, err.Error())
	} else if item.OverrideNote != "" {
		lesson.Warnings = strings.Split(item.OverrideNote, "；")
	}
	preview.Lessons = append(preview.Lessons, lesson)
	return preview, nil
}

func (s *MemoryStore) RestoreScheduleClass(o string, p learning.Principal, id string, ignoreWarnings bool) (learning.ScheduleClass, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		item, err := work.restoreScheduleCandidate(p, id, ignoreWarnings)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		for i, old := range work.scheduleClasses {
			if old.ID == id {
				work.scheduleClasses[i] = cloneScheduleClass(item)
				work.prependLogDetail(o, "恢复排课", item.Name+" / "+item.TeacherName, auditChangeDetail(scheduleClassAuditSnapshot(old), scheduleClassAuditSnapshot(item)))
				break
			}
		}
		return cloneScheduleClass(item), nil
	}, nil)
}
