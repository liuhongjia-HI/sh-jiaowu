package store

import (
	"sort"
	"strings"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// backfillGuardianLinksForPhone 把一个手机号能匹配到的所有学生（不管是已经有
// 登录账号的，还是只有学生档案、还没绑过微信的）都关联到同一个家长身份下。
// 调用方必须已经持有 s.mu（跟包里其它 xxxUnlocked 入口一样），自己按 s.db 是否
// 配置决定要不要包一层持久化事务，遵循这个包里现有的写入函数统一采用的写法。
func (s *MemoryStore) backfillGuardianLinksForPhone(phone string) error {
	if s.db == nil {
		return s.backfillGuardianLinksForPhoneUnlocked(phone)
	}
	return persistentMutationError(s, func(work *MemoryStore) error {
		return work.backfillGuardianLinksForPhoneUnlocked(phone)
	})
}

// backfillGuardianLinksForPhoneUnlocked 只在命中两个及以上学生时才动手——命中
// 0 个或 1 个的情况交给登录成功那一步的 ensureGuardianLink 处理就够了，不需要
// 在这里抢跑。命中多个的时候必须在这里就把关系建好并落库（而不是等家长选完
// 具体登录哪一个），因为"需要选择"那个分支会直接返回 error，外层事务一看到
// error 就把整个函数调用期间的写入全部丢弃——晚一步做，关系就建不起来。
func (s *MemoryStore) backfillGuardianLinksForPhoneUnlocked(phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return nil
	}
	found := map[string]bool{}
	studentIDs := make([]string, 0, 2)
	addStudent := func(id string) {
		if id != "" && !found[id] {
			found[id] = true
			studentIDs = append(studentIDs, id)
		}
	}
	for _, user := range s.users {
		if user.Phone == phone && hasRole(user.Roles, learning.RoleStudent) {
			addStudent(user.StudentID)
		}
	}
	for _, student := range s.students {
		if phoneSame(student.Phone, phone) {
			addStudent(student.ID)
		}
	}
	if len(studentIDs) < 2 {
		return nil
	}
	sort.Strings(studentIDs) // 让"谁是默认主关系人"这种细节不随 map/切片遍历顺序抖动。
	for _, studentID := range studentIDs {
		s.ensureGuardianLink(phone, "", studentID)
	}
	return nil
}

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

// primaryGuardianIDForStudent 反查"这个孩子属于哪个家长"。一个孩子可能同时
// 关联爸爸和妈妈两个家长身份，所以这里的选择必须是确定性的：优先 is_primary，
// 否则按 guardianID 字典序取第一个，避免同一个 token 在不同请求之间拿到不同的
// 家长身份、切换器时有时无。
//
// 这只是给"token 里没有 GuardianID"的存量会话兜底用的推断值；token 里带了
// GuardianID 的一律以 token 为准（见中间件）。
func (s *MemoryStore) primaryGuardianIDForStudent(studentID string) string {
	best := ""
	bestPrimary := false
	for _, relation := range s.guardianStudents {
		if relation.StudentID != studentID || relation.Status != learning.GuardianStudentActive {
			continue
		}
		if best == "" || (relation.IsPrimary && !bestPrimary) ||
			(relation.IsPrimary == bestPrimary && relation.GuardianID < best) {
			best = relation.GuardianID
			bestPrimary = relation.IsPrimary
		}
	}
	return best
}
