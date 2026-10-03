package store

import (
	"encoding/json"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
)

func TestBusinessTeacherAudienceUsesCurrentScopeAndUniqueOfficialIdentity(t *testing.T) {
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	plan, err := s.CreateTeachingPlan("test", p, learning.TeachingPlanUploadRequest{Title: "内部教案", Grade: "五年级", Subject: "English", File: learning.FileAsset{ID: "audience-file", OriginalPath: "/test/audience.pdf", FileName: "audience.pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	users := []learning.User{}
	for _, user := range s.users {
		if !hasRole(user.Roles, learning.RoleTeacher) {
			users = append(users, user)
		}
	}
	for _, row := range []struct{ id, union, grade, subject, status string }{
		{"matched", "u-matched", "五年级", "English", "正常"},
		{"missing", "", "五年级", "English", "正常"},
		{"unfollowed", "u-unfollowed", "五年级", "English", "正常"},
		{"duplicate-a", "u-duplicate", "五年级", "English", "正常"},
		{"duplicate-b", "u-duplicate", "五年级", "English", "正常"},
		{"multiple-followers", "u-multiple", "五年级", "English", "正常"},
		{"wrong-scope", "u-wrong", "四年级", "数学", "正常"},
		{"disabled", "u-disabled", "五年级", "English", "停用"},
	} {
		users = append(users, learning.User{ID: row.id, Name: row.id, Roles: []learning.Role{learning.RoleTeacher}, AccountStatus: row.status, UnionID: row.union, OpenID: "mini-open-never-use", TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Grade: row.grade, Subject: row.subject}}}})
	}
	s.users = users
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "official-matched", UnionID: "u-matched", Subscribed: true}, {OpenID: "official-old", UnionID: "u-unfollowed", Subscribed: false}, {OpenID: "official-duplicate", UnionID: "u-duplicate", Subscribed: true}, {OpenID: "official-multiple-a", UnionID: "u-multiple", Subscribed: true}, {OpenID: "official-multiple-b", UnionID: "u-multiple", Subscribed: true}}
	rows, err := s.TeachingPlanNotificationAudience(p, learning.TeachingPlanAudienceRequest{PlanIDs: []string{plan.ID, plan.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatalf("wrong related active teachers: %+v", rows)
	}
	for _, row := range rows {
		if len(row.PlanIDs) != 1 || row.PlanIDs[0] != plan.ID {
			t.Fatal("duplicated or unrelated plans")
		}
		if row.UserID == "matched" {
			if !row.Reachable || row.Reason != "" {
				t.Fatal("unique official identity not reachable")
			}
		} else if row.Reachable || row.Reason == "" {
			t.Fatalf("ambiguous/unmatched identity reachable: %+v", row)
		}
	}
	raw, _ := json.Marshal(rows)
	if strings.Contains(string(raw), "official-matched") || strings.Contains(string(raw), "mini-open") {
		t.Fatal("audience response leaked platform identities")
	}
	for i := range s.users {
		if s.users[i].ID == "matched" {
			s.users[i].TeacherLibrary = &learning.TeacherLibraryPolicy{}
		}
	}
	rows, err = s.TeachingPlanNotificationAudience(p, learning.TeachingPlanAudienceRequest{PlanIDs: []string{plan.ID}})
	if err != nil || len(rows) != 5 {
		t.Fatal("preview retained revoked teaching scope")
	}
	if len(s.businessNoticeTasks) != 0 {
		t.Fatal("audience preview queued external messages")
	}
}

func TestBusinessTeacherAudienceRejectsInvalidAndReadOnlyRequests(t *testing.T) {
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	plan, err := s.CreateTeachingPlan("test", p, learning.TeachingPlanUploadRequest{Grade: "五年级", Subject: "English", File: learning.FileAsset{ID: "audience-file", OriginalPath: "/test/audience.pdf", FileName: "audience.pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []learning.TeachingPlanAudienceRequest{{}, {PlanIDs: make([]string, 501)}, {PlanIDs: []string{plan.ID, "unknown"}}} {
		if _, err = s.TeachingPlanNotificationAudience(p, req); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	readonly := learning.Principal{UserID: "readonly", Roles: []learning.Role{learning.RoleTeacher}, TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "English"}}}}
	for _, account := range []learning.Principal{readonly, {StudentID: "stu-001", Roles: []learning.Role{learning.RoleStudent}}} {
		if _, err = s.TeachingPlanNotificationAudience(account, learning.TeachingPlanAudienceRequest{PlanIDs: []string{plan.ID}}); err == nil {
			t.Fatal("read-only/student user enumerated notification audience")
		}
	}
}
