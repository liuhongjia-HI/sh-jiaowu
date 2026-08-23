package store

import (
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// ensureGuardianLink 维护"手机号 -> 家长 -> 学生"的关系，登录成功、确定了具体
// 是哪个学生之后调用。优先按手机号找家长（同一个手机号只应该对应一个家长），
// 找不到再按 openID 找（比如家长换了手机号但还是用同一个微信登录），都找不到
// 才新建一个。
//
// 这也是老数据的懒迁移路径：阶段2上线之前的账号本来没有 guardian 记录，第一次
// 登录时这里会把它补上，不需要单独跑一次性回填脚本，也不需要提前假定所有存量
// 数据都已经有关系表条目。
func (s *MemoryStore) ensureGuardianLink(phone, openID, studentID string) string {
	phone = strings.TrimSpace(phone)
	openID = strings.TrimSpace(openID)
	idx := -1
	if phone != "" {
		for i, guardian := range s.guardians {
			if phoneSame(guardian.Phone, phone) {
				idx = i
				break
			}
		}
	}
	if idx == -1 && openID != "" {
		for i, guardian := range s.guardians {
			if guardian.OpenID == openID {
				idx = i
				break
			}
		}
	}
	if idx == -1 {
		s.guardians = append(s.guardians, learning.Guardian{
			ID:            "guardian-" + time.Now().Format("20060102150405.000000000"),
			Phone:         phone,
			OpenID:        openID,
			AccountStatus: "正常",
		})
		idx = len(s.guardians) - 1
	}
	if phone != "" {
		s.guardians[idx].Phone = phone
	}
	if openID != "" {
		s.guardians[idx].OpenID = openID
	}
	s.guardians[idx].LastStudentID = studentID
	guardianID := s.guardians[idx].ID

	linked := false
	hasOtherActive := false
	for i, relation := range s.guardianStudents {
		if relation.GuardianID != guardianID {
			continue
		}
		if relation.StudentID == studentID {
			if relation.Status != learning.GuardianStudentActive {
				s.guardianStudents[i].Status = learning.GuardianStudentActive
			}
			linked = true
			continue
		}
		if relation.Status == learning.GuardianStudentActive {
			hasOtherActive = true
		}
	}
	if !linked {
		s.guardianStudents = append(s.guardianStudents, learning.GuardianStudent{
			GuardianID: guardianID, StudentID: studentID,
			Relation:  learning.GuardianRelationGuardian,
			IsPrimary: !hasOtherActive,
			Status:    learning.GuardianStudentActive,
		})
	}
	return guardianID
}

// findGuardianByOpenIDIndex 只在 s.guardians 里查，不加锁——调用方必须已经持有
// s.mu（跟这个包里其它 xxxUnlocked 辅助函数的约定一致）。
func (s *MemoryStore) findGuardianByOpenIDIndex(openID string) (int, bool) {
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return 0, false
	}
	for i, guardian := range s.guardians {
		if guardian.OpenID == openID {
			return i, true
		}
	}
	return 0, false
}

// guardianStudentActive 判断某个家长和某个学生之间是否存在"在读"状态的关联，
// 用在两个地方：切换账号时校验目标学生是否真的属于这个家长，以及中间件里对
// 每一次请求重新校验——关系被后台解除之后，旧 token 不能继续读那个孩子的数据。
func (s *MemoryStore) guardianStudentActive(guardianID, studentID string) bool {
	if guardianID == "" || studentID == "" {
		return false
	}
	for _, relation := range s.guardianStudents {
		if relation.GuardianID == guardianID && relation.StudentID == studentID {
			return relation.Status == learning.GuardianStudentActive
		}
	}
	return false
}

// GuardianStudentActive 是上面那个校验的加锁导出版本，供中间件在每个请求里调用
// （通过 middleware.PrincipalResolver 接口）。
func (s *MemoryStore) GuardianStudentActive(guardianID, studentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.guardianStudentActive(guardianID, studentID)
}
