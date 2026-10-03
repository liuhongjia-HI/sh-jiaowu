package store

import (
	"database/sql"
	"reflect"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
)

func TestTeacherStatusPreservesGrantsAndRevokesSessions(t *testing.T) {
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	index, _ := s.managedTeacherIndex(admin, "user-teacher")
	snapshot := s.cloneForMutation()
	before := snapshot.users[index]
	lessons := snapshot.scheduleClasses
	if _, err := s.SetTeacherStatus("管理员", admin, before.ID, "停用"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrincipalByUserID(before.ID); err == nil {
		t.Fatal("disabled principal resolved")
	}
	if _, err := s.SetTeacherStatus("管理员", admin, before.ID, "正常"); err != nil {
		t.Fatal(err)
	}
	after := s.cloneForMutation().users[index]
	if after.TokenVersion != before.TokenVersion+1 {
		t.Fatal("old session not revoked")
	}
	after.TokenVersion = before.TokenVersion
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(lessons, s.cloneForMutation().scheduleClasses) {
		t.Fatal("status changed account grants or lessons")
	}
	if _, err := s.SetTeacherStatus("管理员", admin, before.ID, "invalid"); err == nil {
		t.Fatal("invalid status allowed")
	}
}

func TestDeleteTeacherCleansUnusedAccountAndProtectsHistory(t *testing.T) {
	cases := []struct {
		name string
		seed func(*MemoryStore, string)
	}{
		{"历史排课", func(s *MemoryStore, id string) {
			s.scheduleClasses = append(s.scheduleClasses, learning.ScheduleClass{TeacherID: id, Status: "已取消"})
		}},
		{"辅导", func(s *MemoryStore, id string) {
			s.tutoringAssignments = append(s.tutoringAssignments, learning.TutoringAssignment{TeacherID: id})
		}},
		{"教案", func(s *MemoryStore, id string) {
			s.teachingPlans = append(s.teachingPlans, learning.TeachingPlan{UploaderID: id})
		}},
		{"学习资料", func(s *MemoryStore, id string) {
			s.materials = append(s.materials, learning.Material{OwnerTeacherID: id})
		}},
		{"练习", func(s *MemoryStore, id string) {
			s.homework = append(s.homework, learning.Homework{OwnerTeacherID: id})
		}},
		{"题库", func(s *MemoryStore, id string) {
			s.questionBank = append(s.questionBank, learning.QuestionBankItem{OwnerTeacherID: id})
		}},
		{"批改", func(s *MemoryStore, id string) { s.reviews = append(s.reviews, learning.Review{ReviewerTeacherID: id}) }},
		{"反馈", func(s *MemoryStore, id string) {
			s.lessonFeedbacks = append(s.lessonFeedbacks, learning.LessonFeedback{TeacherID: id})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewMemoryStore()
			admin, _ := s.PrincipalByUserID("user-super")
			s.users = append(s.users, learning.User{ID: "test-teacher", Roles: []learning.Role{learning.RoleTeacher}, AccountStatus: "正常"})
			tc.seed(s, "test-teacher")
			before := cloneUsers(s.users)
			logs := len(s.logs)
			if err := s.DeleteTeacher("管理员", admin, "test-teacher"); err == nil || !strings.Contains(err.Error(), "请停用") {
				t.Fatalf("history deletion allowed: %v", err)
			}
			if !reflect.DeepEqual(before, s.users) || logs != len(s.logs) {
				t.Fatal("rejected deletion mutated state")
			}
		})
	}
	s := NewMemoryStore()
	admin, _ := s.PrincipalByUserID("user-super")
	s.users = append(s.users, learning.User{ID: "test-teacher", Name: "测试", Phone: "13912340000", Roles: []learning.Role{learning.RoleTeacher}, AccountStatus: "正常", LearningSpaceIDs: []string{"space-g05-english-s1-q1"}})
	s.availability = append(s.availability, learning.AvailabilitySlot{OwnerType: "teacher", OwnerID: "test-teacher"})
	s.teacherMaterialReads = append(s.teacherMaterialReads, teacherMaterialRead{ID: "test-read", UserID: "test-teacher"})
	beforeStudents := cloneStudents(s.students)
	if err := s.DeleteTeacher("管理员", admin, "test-teacher"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrincipalByUserID("test-teacher"); err == nil {
		t.Fatal("deleted principal resolved")
	}
	for _, v := range s.availability {
		if v.OwnerID == "test-teacher" {
			t.Fatal("orphan availability")
		}
	}
	for _, v := range s.teacherMaterialReads {
		if v.UserID == "test-teacher" {
			t.Fatal("orphan reads")
		}
	}
	if !reflect.DeepEqual(beforeStudents, s.students) {
		t.Fatal("student data changed")
	}
	if s.logs[0].Action != "删除教师" {
		t.Fatal("missing audit log")
	}
}

func TestTeacherLifecycleEnforcesRoleAndCampus(t *testing.T) {
	s := NewMemoryStore()
	for _, p := range []learning.Principal{{UserID: "user-teacher", Roles: []learning.Role{learning.RoleTeacher}}, {Roles: []learning.Role{learning.RoleOpsStaff}}, {Roles: []learning.Role{learning.RoleCampusAdmin}, CampusID: "other"}} {
		if err := s.DeleteTeacher("test", p, "user-teacher"); err == nil {
			t.Fatal("unauthorized delete")
		}
		if _, err := s.SetTeacherStatus("test", p, "user-teacher", "停用"); err == nil {
			t.Fatal("unauthorized status change")
		}
	}
	admin, _ := s.PrincipalByUserID("user-super")
	if err := s.DeleteTeacher("test", admin, "user-super"); err == nil {
		t.Fatal("deleted non-teacher")
	}
}

func TestTeacherLifecyclePersistenceCommitAndRollback(t *testing.T) {
	for _, operation := range []string{"delete", "status"} {
		for _, fail := range []bool{true, false} {
			t.Run(operation+map[bool]string{true: "-rollback", false: "-commit"}[fail], func(t *testing.T) {
				mutationDriverState.reset(fail)
				db, err := sql.Open(mutationTestDriverName, "")
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				s := NewMemoryStore()
				s.db = db
				admin, _ := s.PrincipalByUserID("user-super")
				s.users = append(s.users, learning.User{ID: "lifecycle-test", Name: "测试教师", Roles: []learning.Role{learning.RoleTeacher}, LearningSpaceIDs: []string{"space-g05-english-s1-q1"}, AccountStatus: "正常"})
				s.availability = append(s.availability, learning.AvailabilitySlot{ID: "lifecycle-slot", OwnerType: "teacher", OwnerID: "lifecycle-test"})
				s.teacherMaterialReads = append(s.teacherMaterialReads, teacherMaterialRead{ID: "lifecycle-read", UserID: "lifecycle-test"})
				before := s.cloneForMutation()
				if operation == "delete" {
					err = s.DeleteTeacher("教务", admin, "lifecycle-test")
				} else {
					_, err = s.SetTeacherStatus("教务", admin, "lifecycle-test", "停用")
				}
				if fail {
					if err == nil {
						t.Fatal("database failure ignored")
					}
					after := s.cloneForMutation()
					if !reflect.DeepEqual(before.users, after.users) || !reflect.DeepEqual(before.availability, after.availability) || !reflect.DeepEqual(before.teacherMaterialReads, after.teacherMaterialReads) || !reflect.DeepEqual(before.logs, after.logs) {
						t.Fatal("failed transaction published account or audit changes")
					}
					if mutationDriverState.rollbacks != 1 || mutationDriverState.commits != 0 {
						t.Fatal("transaction did not roll back")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if mutationDriverState.commits != 1 {
					t.Fatal("transaction not committed")
				}
				statements := strings.Join(mutationDriverState.statements, "\n")
				if !strings.Contains(statements, "INSERT INTO operation_logs") {
					t.Fatal("missing persistent audit")
				}
				if operation == "delete" {
					for _, table := range []string{"users", "user_roles", "teacher_learning_space_access", "availability_slots", "teacher_material_reads"} {
						if !strings.Contains(statements, "DELETE FROM "+table+" WHERE") {
							t.Fatalf("missing cleanup for %s: %s", table, statements)
						}
					}
				} else if !strings.Contains(statements, "INSERT INTO users") {
					t.Fatal("status not persisted")
				}
			})
		}
	}
}
