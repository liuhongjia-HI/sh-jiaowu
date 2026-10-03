package store

import (
	"errors"
	"starline/learning-api/internal/domain/learning"
	"time"
)

// MarkScheduleClassCompleted records scheduling history only. It never writes
// attendance, grants, consumption, orders or payment records.
func (s *MemoryStore) MarkScheduleClassCompleted(o string, p learning.Principal, id string) (learning.ScheduleClass, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return persistentMutation(s, func(work *MemoryStore) (learning.ScheduleClass, error) {
		for i, old := range work.scheduleClasses {
			if old.ID != id {
				continue
			}
			if !scheduleCanApprove(p) && !(hasRole(p.Roles, learning.RoleTeacher) && p.UserID == old.TeacherID && work.canSeeScheduleClass(p, old)) {
				return learning.ScheduleClass{}, errors.New("没有权限标记这节课程")
			}
			if old.Status == "已取消" || old.AuditStatus != learning.AuditApproved {
				return learning.ScheduleClass{}, errors.New("仅已通过且未取消课程可标记已上课")
			}
			end, err := time.ParseInLocation("2006-01-02 15:04", old.LessonDate+" "+old.EndTime, time.Local)
			if err != nil || time.Now().Before(end) {
				return learning.ScheduleClass{}, errors.New("课程结束后才可标记已上课")
			}
			if old.Status == "已上课" {
				return cloneScheduleClass(old), nil
			}
			item := cloneScheduleClass(old)
			item.Status = "已上课"
			work.scheduleClasses[i] = item
			work.prependLogDetail(o, "标记已上课", item.Name+" / "+item.LessonDate, "仅记录排课状态")
			return cloneScheduleClass(item), nil
		}
		return learning.ScheduleClass{}, errors.New("课程不存在")
	})
}
