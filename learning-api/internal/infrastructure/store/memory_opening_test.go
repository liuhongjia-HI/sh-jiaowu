package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func openingTestStore() *MemoryStore {
	s := NewMemoryStoreWithOptions(Options{SeedDemoData: false})
	s.students = []learning.Student{{ID: "opening-student", Name: "开通测试", Grade: "四年级", AccountStatus: "正常"}}
	return s
}

func addOpeningTestLegacy(t *testing.T, s *MemoryStore, future bool) (string, packageGrant) {
	t.Helper()
	s.subjects = append(s.subjects, learning.SubjectMetadata{ID: "politics", Name: "政治", Status: "启用"})
	s.ensureLearningSpaces(currentAcademicYear())
	id := "space-g04-politics-s1-q1"
	start := time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
	if future {
		start = time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	}
	_, err := s.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{StudentID: "opening-student", LearningSpaceIDs: []string{id}, ContentTypeCodes: []string{"question"}, StartsAt: start, EndsAt: time.Now().Add(72 * time.Hour).Format("2006-01-02 15:04:05")})
	if err != nil {
		t.Fatal(err)
	}
	grant, ok := s.directGrantPeriod("opening-student", directGrantPackageID("opening-student", id))
	if !ok {
		t.Fatal("missing direct grant")
	}
	s.subjects = s.subjects[:len(s.subjects)-1] // Simulate previously deleted metadata with retained spaces/grants.
	return id, grant
}

func TestOpeningMatrixUsesSubjectMetadataAndGradeRules(t *testing.T) {
	s := openingTestStore()
	for i := range s.subjects {
		if s.subjects[i].ID == "history" {
			s.subjects[i].Status = "启用"
			s.subjects[i].SortOrder = 13
		}
	}
	s.ensureLearningSpaces(currentAcademicYear())
	for _, item := range []learningSpace{
		{ID: "legacy-politics", Subject: "政治", Grade: "四年级", Level: "S", Status: learning.StatusEnabled},
		{ID: "legacy-biology", Subject: "生物", Grade: "四年级", Level: "S", Status: learning.StatusEnabled},
		{ID: "legacy-integrated", Subject: "综合科学", Grade: "四年级", Level: "S", Status: learning.StatusEnabled},
		{ID: "legacy-geography", Subject: "地理", Grade: "四年级", Level: "S", Status: learning.StatusEnabled},
		{ID: "legacy-math-h", Subject: "数学", Grade: "四年级", Level: "H", Status: learning.StatusEnabled},
	} {
		s.learningSpaces = append(s.learningSpaces, item)
	}
	// Normalize a legacy English alias to the metadata identity.
	for i := range s.learningSpaces {
		if s.learningSpaces[i].ID == "space-g04-english-s1-q1" {
			s.learningSpaces[i].Subject = "English"
		}
	}
	rows := s.openingMatrixForStudent(s.students[0])
	if len(rows) != 20 {
		t.Fatalf("got %d spaces, want 20", len(rows))
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.SubjectID]++
		if row.SubjectID == "english" && row.Subject != "英文" {
			t.Fatalf("alias not normalized: %#v", row)
		}
		if row.SubjectID == "history" && row.SubjectSortOrder != 13 {
			t.Fatal("sort did not come from metadata")
		}
	}
	if len(counts) != 5 || counts["history"] != 4 {
		t.Fatalf("unexpected subjects: %v", counts)
	}
	for _, id := range []string{"legacy-politics", "legacy-biology", "legacy-integrated", "legacy-geography", "legacy-math-h"} {
		if _, err := s.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{StudentID: "opening-student", LearningSpaceIDs: []string{id}, ContentTypeCodes: []string{"question"}}); err == nil {
			t.Fatalf("POST accepted %s", id)
		}
		if _, err := s.ReplaceDirectGrant("测试", learning.DirectGrantReplaceRequest{StudentID: "opening-student", Selections: []learning.DirectGrantSelection{{LearningSpaceID: id, ContentTypeCodes: []string{"question"}}}}); err == nil {
			t.Fatalf("PUT accepted %s", id)
		}
	}
	if _, err := s.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{StudentID: "opening-student", LearningSpaceIDs: []string{"space-g04-history-s1-q1"}, ContentTypeCodes: []string{"course"}}); err != nil {
		t.Fatalf("enabled history: %v", err)
	}
}

func TestOpeningPreservesLegacyGrantsAndAllowsExplicitRevocation(t *testing.T) {
	for _, future := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "future"}[future], func(t *testing.T) {
			s := openingTestStore()
			id, before := addOpeningTestLegacy(t, s, future)
			req := learning.DirectGrantReplaceRequest{StudentID: "opening-student", Selections: []learning.DirectGrantSelection{{LearningSpaceID: "space-g04-english-s1-q1", ContentTypeCodes: []string{"course"}}}, StartsAt: time.Now().Format("2006-01-02 15:04:05"), EndsAt: time.Now().Add(96 * time.Hour).Format("2006-01-02 15:04:05")}
			if _, err := s.ReplaceDirectGrant("测试", req); err != nil {
				t.Fatal(err)
			}
			after, _ := s.directGrantPeriod("opening-student", before.PackageID)
			if after != before {
				t.Fatalf("legacy grant changed: %#v -> %#v", before, after)
			}
			// Older clients round-trip original permissions alongside a new shared period.
			req.Selections = append(req.Selections, learning.DirectGrantSelection{LearningSpaceID: id, ContentTypeCodes: []string{"question"}})
			if _, err := s.ReplaceDirectGrant("测试", req); err != nil {
				t.Fatal(err)
			}
			after, _ = s.directGrantPeriod("opening-student", before.PackageID)
			if after != before {
				t.Fatal("legacy grant was renewed")
			}
			req.Selections = nil
			if _, err := s.ReplaceDirectGrant("测试", req); err != nil {
				t.Fatal(err)
			}
			after, _ = s.directGrantPeriod("opening-student", before.PackageID)
			if after != before {
				t.Fatal("empty selections removed legacy grant")
			}
			req.RevokeDirectLearningSpaceIDs = []string{id}
			if _, err := s.ReplaceDirectGrant("测试", req); err != nil {
				t.Fatal(err)
			}
			after, _ = s.directGrantPeriod("opening-student", before.PackageID)
			if after.Status != "revoked" {
				t.Fatal("explicit revoke failed")
			}
		})
	}
}

func TestOpeningRejectsLegacyExpansionAndExpiredRoundTripsAtomically(t *testing.T) {
	for _, scenario := range []string{"expanded", "expired", "revoked", "conflict", "invalid-revoke", "invalid-period"} {
		t.Run(scenario, func(t *testing.T) {
			s := openingTestStore()
			id, grant := addOpeningTestLegacy(t, s, false)
			req := learning.DirectGrantReplaceRequest{StudentID: "opening-student", Selections: []learning.DirectGrantSelection{{LearningSpaceID: id, ContentTypeCodes: []string{"question"}}}}
			switch scenario {
			case "expanded":
				req.Selections[0].ContentTypeCodes = []string{"course"}
			case "expired", "revoked":
				for i := range s.grants {
					if s.grants[i].PackageID == grant.PackageID {
						if scenario == "expired" {
							s.grants[i].EndsAt = time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
						} else {
							s.grants[i].Status = "revoked"
						}
					}
				}
			case "conflict":
				req.RevokeDirectLearningSpaceIDs = []string{id}
			case "invalid-revoke":
				req.RevokeDirectLearningSpaceIDs = []string{"someone-elses-space"}
			case "invalid-period":
				req.Selections = []learning.DirectGrantSelection{{LearningSpaceID: "space-g04-english-s1-q1", ContentTypeCodes: []string{"course"}}}
				req.StartsAt = "invalid"
				req.EndsAt = "invalid"
			}
			before := s.cloneForMutation()
			if _, err := s.ReplaceDirectGrant("测试", req); err == nil {
				t.Fatal("invalid request accepted")
			}
			if !reflect.DeepEqual(before.grants, s.grants) || !reflect.DeepEqual(before.packages, s.packages) || !reflect.DeepEqual(before.logs, s.logs) || !reflect.DeepEqual(before.spaceAccess, s.spaceAccess) {
				t.Fatal("failed request partially changed state")
			}
		})
	}
}

func TestOpeningPackageChecksPreserveMaintenanceAndExistingSources(t *testing.T) {
	s := openingTestStore()
	req := learning.PackageUpsertRequest{Name: "历史测试套餐", Grade: "四年级", Subject: "英文", Semester: "S1", LearningSpaceIDs: []string{"space-g04-english-s1-q1"}, ContentTypeCodes: []string{"question"}, Status: learning.StatusEnabled}
	pkg, err := s.CreatePackage("测试", req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateGrant("测试", learning.GrantCreateRequest{StudentID: "opening-student", PackageID: pkg.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateDirectGrant("测试", learning.DirectGrantCreateRequest{StudentID: "opening-student", LearningSpaceIDs: req.LearningSpaceIDs, ContentTypeCodes: []string{"question"}}); err != nil {
		t.Fatal(err)
	}
	for i := range s.subjects {
		if s.subjects[i].ID == "english" {
			s.subjects[i].Status = "停用"
		}
	}
	if _, err = s.GrantPreview("opening-student", pkg.ID); err == nil {
		t.Fatal("preview accepted disabled subject")
	}
	if _, err = s.CreateGrant("测试", learning.GrantCreateRequest{StudentID: "opening-student", PackageID: pkg.ID}); err == nil {
		t.Fatal("renewal accepted disabled subject")
	}
	req.Summary = "仍可维护说明"
	if _, err = s.UpdatePackage("测试", pkg.ID, req); err != nil {
		t.Fatalf("maintenance failed: %v", err)
	}
	req.ContentTypeCodes = []string{"course"}
	if _, err = s.UpdatePackage("测试", pkg.ID, req); err == nil {
		t.Fatal("expanded disabled package")
	}
	req.ContentTypeCodes = []string{"question"}
	req.LearningSpaceIDs = append(req.LearningSpaceIDs, "space-g04-english-s1-q2")
	if _, err = s.UpdatePackage("测试", pkg.ID, req); err == nil {
		t.Fatal("added disabled scope")
	}
	if _, err = s.ReplaceDirectGrant("测试", learning.DirectGrantReplaceRequest{StudentID: "opening-student", RevokeDirectLearningSpaceIDs: []string{"space-g04-english-s1-q1"}}); err != nil {
		t.Fatal(err)
	}
	for _, g := range s.grants {
		if g.PackageID == pkg.ID && g.Status == "revoked" {
			t.Fatal("direct revoke removed package source")
		}
	}
	rows, err := s.studentGrantsUnlocked(learning.Principal{Roles: []learning.Role{learning.RoleSuperAdmin}}, "opening-student")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !strings.Contains(rows[0].OpeningBlockedReason, "停用") {
		t.Fatalf("missing historical reason: %#v", rows)
	}
	if _, err = s.RevokePackageGrant("测试", "opening-student", pkg.ID); err != nil {
		t.Fatal(err)
	}
}
