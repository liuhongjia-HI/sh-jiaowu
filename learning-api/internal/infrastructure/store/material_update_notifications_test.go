package store

import (
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

func TestMaterialNoticesRequireFormalAccess(t *testing.T) {
	for _, mode := range []string{"formal", "preview", "expired", "revoked", "trial", "disabled-account"} {
		t.Run(mode, func(t *testing.T) {
			s, p, req := materialBusinessFixture(t, true)
			course, _ := s.findCourse(req.CourseID)
			eligible := s.expectedStudentsForCourseHandout(course)
			if len(eligible) == 0 {
				t.Fatal("missing formal fixture")
			}
			id := eligible[0].ID
			attachMaterialParent(s, id)
			grants := []packageGrant{}
			for _, g := range s.grants {
				if g.StudentID == id {
					grants = append(grants, g)
				}
			}
			s.grants = grants
			switch mode {
			case "preview":
				s.grants = nil
			case "expired":
				for i := range s.grants {
					s.grants[i].EndsAt = "2000-01-01"
					s.grants[i].EffectiveUntil = "2000-01-01"
				}
			case "revoked":
				for i := range s.grants {
					s.grants[i].Status = "revoked"
				}
			case "trial":
				for _, g := range s.grants {
					s.trials = append(s.trials, studentTrialRecord{StudentID: id, AcademicYear: s.configuredAcademicYear(), PackageID: g.PackageID, StartsAt: g.StartsAt, EndsAt: grantEndsAt(g), Status: "active"})
				}
			case "disabled-account":
				for i := range s.students {
					if s.students[i].ID == id {
						s.students[i].AccountStatus = "停用"
					}
				}
			}
			s.notices = nil
			s.businessNoticeEvents = nil
			s.businessNoticeTasks = nil
			material, err := s.CreateMaterial("test", p, req)
			if err != nil {
				t.Fatal(err)
			}
			result, err := s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID})
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "formal" {
				want = 1
			}
			if result.RecipientCount != want || len(s.notices) != want || len(s.businessNoticeTasks) != want {
				t.Fatalf("%s: result=%+v station=%d external=%d material=%s", mode, result, len(s.notices), len(s.businessNoticeTasks), material.ID)
			}
		})
	}
}

func TestMaterialVersionNotificationAndStationWithoutWechat(t *testing.T) {
	for _, mode := range []string{"disabled", "unreachable", "reachable"} {
		t.Run(mode, func(t *testing.T) {
			s, p, req := materialBusinessFixture(t, mode != "disabled")
			req.BatchID = ""
			course, _ := s.findCourse(req.CourseID)
			id := s.expectedStudentsForCourseHandout(course)[0].ID
			attachMaterialParent(s, id)
			if mode == "unreachable" {
				s.officialFollowers = nil
			}
			s.notices = nil
			s.businessNoticeEvents = nil
			s.businessNoticeTasks = nil
			first, err := s.CreateMaterial("test", p, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.notices) != 1 || s.notices[0].RelatedType != "business" {
				t.Fatalf("missing linked station: %+v", s.notices)
			}
			originalNotice := s.notices[0].ID
			update := learning.MaterialUpdateRequest{Title: first.Title, CourseID: first.CourseID, LessonID: first.LessonID, TagCode: first.TagCode, Status: learning.StatusEnabled}
			update.Title = "讲义名称调整"
			if _, err = s.UpdateMaterial("test", p, first.ID, update); err != nil {
				t.Fatal(err)
			}
			if len(s.notices) != 1 {
				t.Fatal("rename sent notification")
			}
			update.File = learning.FileAsset{ID: "version-file-two", FileName: "新版.pdf", FileType: "PDF"}
			updated, err := s.UpdateMaterial("test", p, first.ID, update)
			if err != nil {
				t.Fatal(err)
			}
			if updated.FileID != update.File.ID || len(s.notices) != 2 {
				t.Fatalf("replacement missing: %+v notices=%d", updated, len(s.notices))
			}
			versionNotice := s.notices[0]
			detail, err := s.BusinessNoticeDetail(learning.Principal{StudentID: id}, versionNotice.RelatedID)
			if err != nil {
				t.Fatal(err)
			}
			if detail.NoticeID != versionNotice.ID || len(detail.CurrentMaterials) != 1 || detail.CurrentMaterials[0].FileID != updated.FileID || detail.Event.CreatedAt == "" || versionNotice.IsRead {
				t.Fatalf("invalid version detail: %+v", detail)
			}
			if _, err = s.UpdateMaterial("test", p, first.ID, update); err != nil {
				t.Fatal(err)
			}
			if len(s.notices) != 2 {
				t.Fatal("same version resent")
			}
			update.File = learning.FileAsset{}
			update.Status = learning.StatusDisabled
			if _, err = s.UpdateMaterial("test", p, first.ID, update); err != nil {
				t.Fatal(err)
			}
			update.Status = learning.StatusEnabled
			if _, err = s.UpdateMaterial("test", p, first.ID, update); err != nil {
				t.Fatal(err)
			}
			if len(s.notices) != 3 || s.notices[2].ID != originalNotice {
				t.Fatal("republish missing or old notice lost")
			}
			if mode == "disabled" && len(s.businessNoticeTasks) != 0 {
				t.Fatal("disabled external created tasks")
			}
			if mode == "unreachable" {
				for _, task := range s.businessNoticeTasks {
					if task.Status != "不可触达" {
						t.Fatalf("bad unreachable task: %+v", task)
					}
				}
			}
			for i := range s.grants {
				if s.grants[i].StudentID == id {
					s.grants[i].Status = "revoked"
				}
			}
			if _, err = s.BusinessNoticeDetail(learning.Principal{StudentID: id}, versionNotice.RelatedID); err == nil {
				t.Fatal("expired access exposed details")
			}
			sent := 0
			s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sent++; return "mock", nil }
			if err = s.ProcessBusinessNotices(time.Now()); err != nil {
				t.Fatal(err)
			}
			if sent != 0 {
				t.Fatal("revoked access sent external notification")
			}
		})
	}
}
