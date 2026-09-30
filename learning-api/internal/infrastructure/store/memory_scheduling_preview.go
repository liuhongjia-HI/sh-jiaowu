package store

import (
	"errors"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// seriesTargets is shared by preview and cancellation. Detached/cancelled and
// historical lessons never participate in a batch operation.
func (s *MemoryStore) seriesTargets(p learning.Principal, anchor learning.ScheduleClass, scope string) ([]learning.ScheduleClass, error) {
	if !s.canSeeScheduleClass(p, anchor) {
		return nil, errors.New("没有权限调整该课程")
	}
	if err := scheduleEditPermission(p, anchor); err != nil {
		return nil, err
	}
	if scope == learning.EditScopeThis {
		return []learning.ScheduleClass{anchor}, nil
	}
	boundary := time.Now().Format("2006-01-02")
	if scope == learning.EditScopeThisAndFuture && anchor.LessonDate > boundary {
		boundary = anchor.LessonDate
	}
	targets := []learning.ScheduleClass{}
	for _, item := range s.scheduleClasses {
		if item.SeriesID != anchor.SeriesID || item.Detached || item.Status == "已取消" || item.LessonDate < boundary {
			continue
		}
		if err := scheduleEditPermission(p, item); err != nil {
			return nil, errors.New("系列中包含无权调整的课程：" + err.Error())
		}
		targets = append(targets, item)
	}
	if len(targets) == 0 {
		return nil, errors.New("这个系列没有可调整的未来课次")
	}
	return targets, nil
}

func (s *MemoryStore) PreviewScheduleClass(p learning.Principal, request learning.SchedulePreviewRequest) (learning.SchedulePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	work := s.cloneForMutation()
	req := request.ScheduleClassCreateRequest
	dates := []string{}
	if request.ID == "" {
		repeat, err := normalizeRepeat(req.Repeat)
		if err != nil {
			return learning.SchedulePreview{}, err
		}
		var errExpand error
		dates, errExpand = expandRepeatDates(repeat, strings.TrimSpace(req.StartDate))
		if errExpand != nil {
			return learning.SchedulePreview{}, errExpand
		}
	} else {
		var anchor learning.ScheduleClass
		for _, item := range work.scheduleClasses {
			if item.ID == request.ID {
				anchor = item
				break
			}
		}
		if anchor.ID == "" {
			return learning.SchedulePreview{}, errors.New("课程不存在")
		}
		if anchor.Status == "已取消" {
			return learning.SchedulePreview{}, errors.New("已取消课程不能调课")
		}
		scope, err := resolveEditScope(anchor, req.EditScope)
		if err != nil {
			return learning.SchedulePreview{}, err
		}
		targets, err := work.seriesTargets(p, anchor, scope)
		if err != nil {
			return learning.SchedulePreview{}, err
		}
		date := req.StartDate
		if date == "" {
			date = anchor.LessonDate
		}
		offset, err := dayOffset(anchor.LessonDate, date)
		if err != nil {
			return learning.SchedulePreview{}, err
		}
		ids := map[string]bool{}
		for _, target := range targets {
			shifted, err := shiftDate(target.LessonDate, offset)
			if err != nil {
				return learning.SchedulePreview{}, err
			}
			dates = append(dates, shifted)
			ids[target.ID] = true
		}
		remaining := []learning.ScheduleClass{}
		for _, item := range work.scheduleClasses {
			if !ids[item.ID] {
				remaining = append(remaining, item)
			}
		}
		work.scheduleClasses = remaining
	}
	result := learning.SchedulePreview{CanSave: true, Lessons: []learning.SchedulePreviewLesson{}}
	req.IgnoreWarnings = true
	for _, date := range dates {
		lesson := learning.SchedulePreviewLesson{Date: date, Errors: []string{}, Warnings: []string{}}
		item, err := work.buildScheduleClass(p, "", date, req)
		if err != nil {
			details := []string{}
			if strings.Contains(err.Error(), "该时间已有课程") {
				for _, occupied := range work.scheduleClasses {
					if occupied.Status == "已取消" || occupied.LessonDate != date || occupied.StartTime >= req.EndTime || occupied.EndTime <= req.StartTime {
						continue
					}
					participants := []string{}
					if occupied.TeacherID == req.TeacherID {
						participants = append(participants, occupied.TeacherName)
					}
					for _, student := range occupied.Students {
						for _, id := range req.StudentIDs {
							if student.ID == id {
								participants = append(participants, student.Name)
							}
						}
					}
					if len(participants) == 0 {
						continue
					}
					title := "已有课程"
					if work.canSeeScheduleClass(p, occupied) {
						title = occupied.Name
					}
					details = append(details, strings.Join(participants, "、")+" 与「"+title+"」冲突（"+occupied.StartTime+"–"+occupied.EndTime+"）")
				}
			}
			if len(details) == 0 {
				details = append(details, err.Error())
			}
			lesson.Errors = details
			result.CanSave = false
		} else {
			if item.OverrideNote != "" {
				start, _ := parseClock(req.StartTime)
				end, _ := parseClock(req.EndTime)
				owners := []struct{ typ, id, name string }{{"teacher", item.TeacherID, item.TeacherName}}
				for _, student := range item.Students {
					owners = append(owners, struct{ typ, id, name string }{"student", student.ID, student.Name})
				}
				for _, owner := range owners {
					registered := false
					for _, slot := range work.ownerAvailability(owner.typ, owner.id) {
						if !slot.Unavailable {
							registered = true
						}
					}
					if work.ownerUnavailable(owner.typ, owner.id, item.DayOfWeek, start, end, date, date) {
						lesson.Warnings = append(lesson.Warnings, owner.name+" 已登记临时不可上课")
					} else if !registered {
						lesson.Warnings = append(lesson.Warnings, owner.name+" 未登记可上课时间")
					} else if !work.teacherOrStudentAvailable(owner.typ, owner.id, item.DayOfWeek, start, end, date) {
						lesson.Warnings = append(lesson.Warnings, owner.name+" 超出已登记可上课时间")
					}
				}
			}
			// Include rebuilt lessons to detect collisions within a moved series.
			work.scheduleClasses = append(work.scheduleClasses, item)
		}
		result.Lessons = append(result.Lessons, lesson)
	}
	return result, nil
}

func (s *MemoryStore) CancelScheduleClassScope(o string, p learning.Principal, id, rawScope string) (learning.ScheduleClass, error) {
	return noticeMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		return work.cancelScheduleClassScopeUnlocked(o, p, id, rawScope)
	}, nil)
}

func (s *MemoryStore) cancelScheduleClassScopeUnlocked(o string, p learning.Principal, id, rawScope string) (learning.ScheduleClass, error) {
	if s.db != nil {
		return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
			return work.cancelScheduleClassScopeUnlocked(o, p, id, rawScope)
		})
	}
	var anchor learning.ScheduleClass
	for _, item := range s.scheduleClasses {
		if item.ID == id {
			anchor = item
			break
		}
	}
	if anchor.ID == "" {
		return learning.ScheduleClass{}, errors.New("课程不存在")
	}
	scope, err := resolveEditScope(anchor, rawScope)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	targets, err := s.seriesTargets(p, anchor, scope)
	if err != nil {
		return learning.ScheduleClass{}, err
	}
	// All permission checks precede any mutation or notification.
	var result learning.ScheduleClass
	for _, target := range targets {
		item, err := s.cancelScheduleClassUnlocked(o, p, target.ID)
		if err != nil {
			return learning.ScheduleClass{}, err
		}
		if result.ID == "" || item.ID == id {
			result = item
		}
	}
	return result, nil
}

func (s *MemoryStore) teacherOrStudentAvailable(typ, id string, day, start, end int, date string) bool {
	if typ == "teacher" {
		return s.teacherAvailable(id, day, start, end, date, date)
	}
	return s.studentAvailable(id, day, start, end, date, date)
}
