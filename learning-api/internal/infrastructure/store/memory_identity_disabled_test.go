package store

import (
	"errors"
	"strings"
	"testing"

	"starline/learning-api/internal/domain/learning"
)

func TestWechatBindingIgnoresUnavailableSibling(t *testing.T) {
	for _, mode := range []string{"accounts", "profiles", "masked", "active_without_account"} {
		for _, disabled := range []string{"profile", "account", "both"} {
			if mode == "profiles" && disabled == "account" {
				continue
			}
			t.Run(mode+"/"+disabled, func(t *testing.T) {
				for _, scenario := range []string{"one_active", "twins", "all_disabled", "selected_disabled", "selected_unknown", "wrong_name"} {
					t.Run(scenario, func(t *testing.T) {
						s := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
						const phone = "13700002222"
						storedPhone := phone
						if mode == "masked" {
							storedPhone = "137****2222"
						}
						s.students = []learning.Student{
							{ID: "old", Name: "旧档案", Phone: storedPhone, Grade: "四年级", AccountStatus: "正常"},
							{ID: "active", Name: "孩子一", Phone: storedPhone, Grade: "四年级", AccountStatus: "正常"},
						}
						if scenario == "twins" {
							s.students = append(s.students, learning.Student{ID: "twin", Name: "孩子二", Phone: storedPhone, Grade: "四年级", AccountStatus: "正常"})
						}
						if disabled != "account" {
							s.students[0].AccountStatus = "停用"
						}
						if scenario == "all_disabled" {
							s.students[1].AccountStatus = "停用"
						}
						if mode != "profiles" {
							for _, student := range s.students {
								if mode == "active_without_account" && student.ID != "old" {
									continue
								}
								accountStatus := "正常"
								if student.ID == "old" && disabled != "profile" {
									accountStatus = "停用"
								}
								s.users = append(s.users, learning.User{ID: "user-" + student.ID, Name: student.Name, Phone: storedPhone, StudentID: student.ID, AccountStatus: accountStatus, Roles: []learning.Role{learning.RoleStudent}})
							}
						}
						beforeStudents, beforeUsers := len(s.students), len(s.users)
						req := learning.WechatLoginRequest{Code: "disabled-sibling", Phone: phone, StudentName: "孩子一", Grade: "四年级"}
						if scenario == "selected_disabled" {
							req.SelectedStudentID = "old"
						}
						if scenario == "selected_unknown" {
							req.SelectedStudentID = "unknown"
						}
						if scenario == "wrong_name" {
							req.StudentName = "不匹配"
						}
						principal, err := s.LoginWithWechatCode(req)
						switch scenario {
						case "twins":
							var selection *learning.StudentSelectionRequiredError
							if !errors.As(err, &selection) || len(selection.Candidates) != 2 {
								t.Fatalf("expected two active twins, got %v", err)
							}
							for _, candidate := range selection.Candidates {
								if candidate.StudentID == "old" {
									t.Fatal("disabled sibling was offered")
								}
							}
							req.SelectedStudentID = "twin"
							req.StudentName = "孩子二"
							principal, err = s.LoginWithWechatCode(req)
							if err != nil || principal.StudentID != "twin" {
								t.Fatalf("select twin: principal=%+v err=%v", principal, err)
							}
						case "one_active":
							if err != nil || principal.StudentID != "active" {
								t.Fatalf("expected active child login, principal=%+v err=%v", principal, err)
							}
						default:
							want := "选择的学生账号不存在"
							if scenario == "all_disabled" {
								want = "账号已停用"
							}
							if scenario == "wrong_name" {
								want = "学生姓名与后台档案不一致"
							}
							if err == nil || !strings.Contains(err.Error(), want) {
								t.Fatalf("expected %s, got %v", want, err)
							}
							if len(s.users) != beforeUsers {
								t.Fatal("failed binding created an account")
							}
						}
						if len(s.students) != beforeStudents {
							t.Fatal("binding created a duplicate student")
						}
						if disabled != "account" && s.students[0].AccountStatus != "停用" {
							t.Fatal("disabled profile was reactivated")
						}
						for _, user := range s.users {
							if user.StudentID == "old" && user.OpenID != "" {
								t.Fatal("disabled sibling was bound")
							}
						}
					})
				}
			})
		}
	}
}
