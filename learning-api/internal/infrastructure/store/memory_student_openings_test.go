package store

import (
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestStudentActiveOpenings(t *testing.T) {
	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	future := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	store := &MemoryStore{
		learningSpaces: []learningSpace{
			{ID: "s", Grade: "四年级", Subject: "英语", Level: "S", Status: learning.StatusEnabled},
			{ID: "alias", Grade: "四年级", Subject: "English", Level: "S", Status: learning.StatusEnabled},
			{ID: "a", Grade: "四年级", Subject: "英语", Level: "A", Status: learning.StatusEnabled},
			{ID: "missing", Grade: "五年级", Subject: "数学", Status: learning.StatusEnabled},
			{ID: "disabled", Grade: "六年级", Subject: "数学", Level: "S", Status: learning.StatusDisabled},
		},
	}
	add := func(id, student, space, status, start, end string) {
		store.grants = append(store.grants, packageGrant{ID: id, StudentID: student, PackageID: id, Status: status, StartsAt: start, EndsAt: end})
		store.spaceAccess = append(store.spaceAccess, learningSpaceAccess{PackageGrantID: id, StudentID: student, LearningSpaceID: space, Status: status, StartsAt: start, EndsAt: end})
	}
	add("package", "student", "s", "active", past, future)
	add("direct-student-s", "student", "alias", "active", past, future)
	add("a", "student", "a", "active", past, future)
	add("missing", "student", "missing", "active", past, future)
	add("disabled", "student", "disabled", "active", past, future)
	add("expired", "expired-student", "s", "active", "", past)
	add("pending", "expired-student", "a", "active", future, future)
	add("revoked", "expired-student", "missing", "revoked", past, future)
	add("other", "other-student", "s", "active", past, future)
	scopes := store.studentActiveOpenings("student")
	if len(scopes) != 3 {
		t.Fatalf("expected 3 distinct effective scopes, got %#v", scopes)
	}
	found := map[string]bool{}
	for _, scope := range scopes {
		found[scope.Grade+"/"+scope.Subject+"/"+scope.Level] = true
	}
	for _, key := range []string{"四年级/English/S", "四年级/English/A", "五年级/Mathematics/"} {
		if !found[key] {
			t.Errorf("missing scope %s in %#v", key, scopes)
		}
	}
	if got := store.studentActiveOpenings("expired-student"); len(got) != 0 {
		t.Fatalf("inactive grants leaked: %#v", got)
	}
	if got := store.studentActiveOpenings("unknown"); len(got) != 0 {
		t.Fatalf("other students leaked: %#v", got)
	}
	active := learning.Student{ActiveOpenings: scopes}
	expired := learning.Student{OpenedPackages: []string{"historical"}}
	if !matchesStudentQuery(active, learning.StudentQuery{PackageState: "已开通"}) || matchesStudentQuery(expired, learning.StudentQuery{PackageState: "已开通"}) {
		t.Fatal("opened filter must use effective scopes")
	}
	if matchesStudentQuery(active, learning.StudentQuery{PackageState: "未开通"}) || !matchesStudentQuery(expired, learning.StudentQuery{PackageState: "未开通"}) {
		t.Fatal("unopened filter must use effective scopes")
	}
}
