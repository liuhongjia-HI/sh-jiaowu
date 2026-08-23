package store

import (
	"os"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

// 阶段1 只打通 guardians / guardian_students 的存取通路，登录逻辑还没有切过来
// （阶段2再改），所以这里不走任何 HTTP 或登录路径，直接对 MemoryStore 的私有
// mutation 机制做端到端验证：clone → 落库 → 发布到内存 → 重启后从库里加载回来，
// 跟其它表（比如 students/users）已经在用的那条路径完全一样。
func TestMySQLGuardianRelationsSurviveStoreRestart(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_TEST_MYSQL_DSN is not configured")
	}
	// 这套测试可能在一个没有每次清空的持久化 DB 上重复跑（比如本地反复
	// 手动执行），所以身份用纳秒级时间戳打散，断言只认自己写下的那几行，
	// 不能假设"跑完这段代码库里应该正好有 N 条"——那个假设只在全新库上成立。
	nonce := time.Now().Format("20060102150405.000000000")
	phone := "179" + nonce[15:24] // 9 位纳秒数字，够撑起手机号唯一性，不需要合法号段
	guardianID := "guardian-test-" + nonce

	first := NewMemoryStoreWithOptions(Options{
		SkipBaseData: true, BootstrapAdminName: "集成测试管理员",
		BootstrapAdminPhone: "13900000002", BootstrapAdminPassword: "Integration123!",
	})
	if err := first.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect first store: %v", err)
	}
	admin, err := first.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load bootstrap admin: %v", err)
	}
	elder, err := first.CreateStudent("MySQL集成测试", admin, learning.StudentUpsertRequest{
		Name: "关系测试哥哥", Phone: phone, Grade: "五年级", SchoolName: "集成测试学校",
	})
	if err != nil {
		t.Fatalf("create elder student: %v", err)
	}
	younger, err := first.CreateStudent("MySQL集成测试", admin, learning.StudentUpsertRequest{
		Name: "关系测试弟弟", Phone: phone, Grade: "三年级", SchoolName: "集成测试学校",
	})
	if err != nil {
		t.Fatalf("create younger sibling student: %v", err)
	}

	// 一个家长关联两个孩子（多子女），模拟阶段2要落地的真实数据形状。
	if err := persistentMutationError(first, func(work *MemoryStore) error {
		work.guardians = append(work.guardians, learning.Guardian{
			ID: guardianID, Phone: phone, Name: "关系测试家长", AccountStatus: "正常",
			LastStudentID: elder.ID,
		})
		work.guardianStudents = append(work.guardianStudents,
			learning.GuardianStudent{GuardianID: guardianID, StudentID: elder.ID, Relation: learning.GuardianRelationMother, IsPrimary: true, Status: learning.GuardianStudentActive},
			learning.GuardianStudent{GuardianID: guardianID, StudentID: younger.ID, Relation: learning.GuardianRelationMother, Status: learning.GuardianStudentActive},
		)
		return nil
	}); err != nil {
		t.Fatalf("persist guardian relations: %v", err)
	}

	if !containsGuardian(first.guardians, guardianID) || countRelationsFor(first.guardianStudents, guardianID) != 2 {
		t.Fatalf("expected in-memory state to reflect the write, guardians contains=%v relations=%d",
			containsGuardian(first.guardians, guardianID), countRelationsFor(first.guardianStudents, guardianID))
	}

	if err := first.db.Close(); err != nil {
		t.Fatalf("close first store db: %v", err)
	}

	second := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := second.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect restarted store: %v", err)
	}
	defer second.db.Close()

	var reloaded *learning.Guardian
	for i := range second.guardians {
		if second.guardians[i].ID == guardianID {
			reloaded = &second.guardians[i]
			break
		}
	}
	if reloaded == nil {
		t.Fatal("guardian missing after restart")
	}
	if reloaded.Phone != phone || reloaded.Name != "关系测试家长" || reloaded.LastStudentID != elder.ID {
		t.Fatalf("reloaded guardian mismatch: %#v", *reloaded)
	}

	relations := map[string]learning.GuardianStudent{}
	for _, relation := range second.guardianStudents {
		if relation.GuardianID == guardianID {
			relations[relation.StudentID] = relation
		}
	}
	if len(relations) != 2 {
		t.Fatalf("expected 2 reloaded relations, got %d: %#v", len(relations), relations)
	}
	elderRelation, ok := relations[elder.ID]
	if !ok || !elderRelation.IsPrimary || elderRelation.Relation != learning.GuardianRelationMother || elderRelation.Status != learning.GuardianStudentActive {
		t.Fatalf("reloaded elder relation mismatch: %#v", elderRelation)
	}
	youngerRelation, ok := relations[younger.ID]
	if !ok || youngerRelation.IsPrimary || youngerRelation.Relation != learning.GuardianRelationMother {
		t.Fatalf("reloaded younger relation mismatch: %#v", youngerRelation)
	}

	// 结课/解绑不物理删学生，但关系确实要能删——比如家长换绑、或者数据订正。
	// 用同一套 before/after key-diff 机制验证删除会同步到库里，而不是留下孤儿行。
	if err := persistentMutationError(second, func(work *MemoryStore) error {
		kept := make([]learning.GuardianStudent, 0, len(work.guardianStudents))
		for _, relation := range work.guardianStudents {
			if relation.GuardianID == guardianID && relation.StudentID == younger.ID {
				continue
			}
			kept = append(kept, relation)
		}
		work.guardianStudents = kept
		return nil
	}); err != nil {
		t.Fatalf("unlink relation: %v", err)
	}

	third := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := third.ConnectDatabase(dsn); err != nil {
		t.Fatalf("connect third store: %v", err)
	}
	defer third.db.Close()
	remaining := 0
	for _, relation := range third.guardianStudents {
		if relation.GuardianID == guardianID {
			remaining++
			if relation.StudentID != elder.ID {
				t.Fatalf("expected only the elder relation to remain, found %#v", relation)
			}
		}
	}
	if remaining != 1 {
		t.Fatalf("expected unlinked relation to be deleted from the database, remaining=%d", remaining)
	}
}

func containsGuardian(guardians []learning.Guardian, id string) bool {
	for _, guardian := range guardians {
		if guardian.ID == id {
			return true
		}
	}
	return false
}

func countRelationsFor(relations []learning.GuardianStudent, guardianID string) int {
	count := 0
	for _, relation := range relations {
		if relation.GuardianID == guardianID {
			count++
		}
	}
	return count
}
