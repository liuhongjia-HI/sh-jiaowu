package store

import (
	"errors"

	"starline/learning-api/internal/domain/learning"
)

// 7 天套餐体验已由“年级 × 学科首章节永久开放”取代。保留此方法仅为兼容
// 旧服务调用；它不能再创建有时限的授权。
func (s *MemoryStore) startStudentTrialUnlocked(learning.Principal, string) (learning.StudentTrialStartResult, error) {
	return learning.StudentTrialStartResult{}, errors.New("7天体验期已取消，每门学科首章节可永久查看")
}

// 保留返回字段，避免旧版小程序解析首页/学习页响应失败；新版界面不会展示体验卡。
func (s *MemoryStore) studentTrialUnlocked(learning.Student) learning.StudentTrial {
	return learning.StudentTrial{State: "unavailable"}
}

func (s *MemoryStore) findTrialRecord(studentID, academicYear string) (studentTrialRecord, bool) {
	for _, record := range s.trials {
		if record.StudentID == studentID && record.AcademicYear == academicYear {
			return record, true
		}
	}
	return studentTrialRecord{}, false
}

// trialFirstLessonForGrant 仅用于体验授权；体验内容固定为课程目录中的首个 Lesson。
func (s *MemoryStore) trialFirstLessonForGrant(grant packageGrant, courseID string) (string, bool) {
	record, ok := s.findTrialRecord(grant.StudentID, s.configuredAcademicYear())
	if !ok || record.Status != "active" || record.PackageID != grant.PackageID || record.StartsAt != grant.StartsAt || record.EndsAt != grantEndsAt(grant) {
		return "", false
	}
	for _, course := range s.courses {
		if course.ID != courseID {
			continue
		}
		lessons := make([]learning.CurriculumNode, 0)
		for _, node := range course.Curriculum {
			if node.Type == learning.CurriculumLesson {
				lessons = append(lessons, node)
			}
		}
		if len(lessons) > 0 {
			return lessons[0].ID, true
		}
		return "", false
	}
	return "", false
}
