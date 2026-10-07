package store

import (
	"os"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

func teacherBatchFixture(t *testing.T, enabled bool) (*MemoryStore, learning.Principal, learning.TeachingPlanUploadRequest) {
	t.Helper()
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	for i := range s.users {
		if s.users[i].ID == "user-teacher" {
			s.users[i].UnionID = "teacher-batch-union"
			s.users[i].AccountStatus = "正常"
			s.users[i].LearningSpaceIDs = nil
			s.users[i].TeacherLibrary = &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "English"}}}
		}
	}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "teacher-official-open", UnionID: "teacher-batch-union", Subscribed: true}}
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "teacher-batch-template", Status: "启用", Content: "教案：{{thing4.DATA}}\n教师：{{thing5.DATA}}\n数量：{{number3.DATA}}\n时间：{{time7.DATA}}"})
	_, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeTeachingPlansUploaded, Enabled: enabled, TemplateID: "teacher-batch-template", TeacherIDs: []string{"user-teacher"}, WebOrigin: "https://school.example", FieldMappings: map[string]string{"thing4": "resource_title", "thing5": "teacher_name", "number3": "resource_count", "time7": "published_at"}})
	if err != nil {
		t.Fatal(err)
	}
	return s, p, learning.TeachingPlanUploadRequest{BatchID: "teacher-batch", Title: "第一课教案", Grade: "五年级", Subject: "English", File: learning.FileAsset{ID: "teacher-batch-file-1", FileName: "first.pdf", OriginalPath: "/test/first.pdf"}}
}

func TestBusinessTeacherBatchMergeRetryAndOneDelivery(t *testing.T) {
	s, p, req := teacherBatchFixture(t, true)
	first, err := s.CreateTeachingPlan("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 0 || len(s.PendingTeachingPlanNoticeBatches(p)) != 1 {
		t.Fatal("upload sent before completion")
	}
	other, _ := s.PrincipalByUserID("user-teacher")
	if _, err = s.CompleteTeachingPlanNoticeBatch("test", other, req.BatchID); err == nil {
		t.Fatal("other uploader completed batch")
	}
	req.File.ID = "teacher-batch-file-2"
	req.Title = "第二课教案"
	if _, err = s.CreateTeachingPlan("test", p, req); err != nil {
		t.Fatal(err)
	}
	result, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID)
	if err != nil || result.ResourceCount != 2 || result.AlreadyCompleted || len(s.businessNoticeTasks) != 1 {
		t.Fatalf("wrong completion: %+v %v %+v", result, err, s.businessNoticeTasks)
	}
	task := s.businessNoticeTasks[0]
	if task.Values["number3"] != "2" || task.RecipientUserID != "user-teacher" || task.OpenID != "teacher-official-open" || task.URL != "https://school.example/teaching-plans?notice="+task.EventID+"&plan="+first.ID || task.PagePath != "" {
		t.Fatalf("wrong teacher delivery: %+v", task)
	}
	req.File.ID = "teacher-batch-file-3"
	if _, err = s.CreateTeachingPlan("test", p, req); err != nil {
		t.Fatal(err)
	}
	if len(s.PendingTeachingPlanNoticeBatches(p)) != 1 {
		t.Fatal("recovered file missing pending batch")
	}
	result, err = s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID)
	if err != nil || !result.AlreadyCompleted || len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != task.ID || s.businessNoticeTasks[0].Values["number3"] != "3" || s.businessNoticeTasks[0].CreatedAt != task.CreatedAt {
		t.Fatal("retry lost task identity or count")
	}
	sent := 0
	s.officialMessageSender = func(r learning.OfficialMessageRequest) (string, error) {
		sent++
		if r.URL != task.URL || r.ClientMessageID != task.ID {
			t.Fatal("lost controlled link or idempotency")
		}
		return "fake", nil
	}
	if err = s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	req.File.ID = "teacher-batch-file-4"
	if _, err = s.CreateTeachingPlan("test", p, req); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
		t.Fatal(err)
	}
	if err = s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sent != 1 || len(s.PendingTeachingPlanNoticeBatches(p)) != 0 {
		t.Fatal("accepted batch sent again or completion remained pending")
	}
}

func TestBusinessTeacherBatchRevalidatesBeforeSending(t *testing.T) {
	for _, mode := range []string{"scope", "disabled", "unfollowed", "identity", "mapping", "origin", "trial"} {
		t.Run(mode, func(t *testing.T) {
			s, p, req := teacherBatchFixture(t, true)
			if _, err := s.CreateTeachingPlan("test", p, req); err != nil {
				t.Fatal(err)
			}
			if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
				t.Fatal(err)
			}
			if len(s.businessNoticeTasks) != 1 {
				t.Fatal("no test task")
			}
			binding := s.bindingForBusinessKind(learning.NoticeTeachingPlansUploaded)
			switch mode {
			case "scope":
				for i := range s.users {
					if s.users[i].ID == "user-teacher" {
						s.users[i].TeacherLibrary = &learning.TeacherLibraryPolicy{}
					}
				}
			case "disabled":
				for i := range s.users {
					if s.users[i].ID == "user-teacher" {
						s.users[i].AccountStatus = "停用"
					}
				}
			case "unfollowed":
				s.officialFollowers[0].Subscribed = false
			case "identity":
				s.officialFollowers[0].OpenID = "other-official-open"
			case "mapping":
				binding.FieldMappings["thing4"] = "teaching_scope"
			case "origin":
				binding.WebOrigin = "https://changed.example"
			case "trial":
				binding.TeacherIDs = nil
				binding.Enabled = false
			}
			if mode == "mapping" || mode == "origin" || mode == "trial" {
				if _, err := s.UpdateBusinessNoticeBinding("test", binding); err != nil {
					t.Fatal(err)
				}
			}
			sent := 0
			s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sent++; return "fake", nil }
			if err := s.ProcessBusinessNotices(time.Now()); err != nil {
				t.Fatal(err)
			}
			if sent != 0 || s.businessNoticeTasks[0].FailureReason == "" {
				t.Fatalf("invalidated task sent: %s %+v", mode, s.businessNoticeTasks[0])
			}
		})
	}
}

func TestBusinessTeacherBatchDisabledNoReplayAndIdentityRecovery(t *testing.T) {
	for _, mode := range []string{"disabled", "identity", "failed"} {
		t.Run(mode, func(t *testing.T) {
			s, p, req := teacherBatchFixture(t, mode != "disabled")
			if mode == "identity" {
				s.officialFollowers = nil
			}
			if _, err := s.CreateTeachingPlan("test", p, req); err != nil {
				t.Fatal(err)
			}
			if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
				t.Fatal(err)
			}
			if mode == "disabled" {
				if len(s.businessNoticeTasks) != 0 {
					t.Fatal("disabled notification queued")
				}
				binding := s.bindingForBusinessKind(learning.NoticeTeachingPlansUploaded)
				binding.Enabled = true
				if _, err := s.UpdateBusinessNoticeBinding("test", binding); err != nil {
					t.Fatal(err)
				}
				if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
					t.Fatal(err)
				}
				if len(s.businessNoticeTasks) != 0 {
					t.Fatal("disabled history replayed")
				}
				return
			}
			id := s.businessNoticeTasks[0].ID
			if mode == "identity" {
				if s.businessNoticeTasks[0].Status != "不可触达" {
					t.Fatal("missing identity treated reachable")
				}
				s.officialFollowers = []learning.OfficialFollower{{OpenID: "teacher-official-open", UnionID: "teacher-batch-union", Subscribed: true}}
				if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
					t.Fatal(err)
				}
				if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != id || s.businessNoticeTasks[0].Status != "待发送" {
					t.Fatal("identity recovery duplicated or lost task")
				}
			}
			sent := 0
			s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) {
				sent++
				if sent == 1 && mode == "failed" {
					return "", &officialSendError{reason: "fake temporary definite failure", temporary: true}
				}
				return "fake", nil
			}
			if err := s.ProcessBusinessNotices(time.Now()); err != nil {
				t.Fatal(err)
			}
			if mode == "failed" {
				if !strings.Contains(s.businessNoticeTasks[0].FailureReason, "fake") {
					t.Fatal("failure not retained")
				}
				if _, err := s.RetryBusinessNotice("test", id); err != nil {
					t.Fatal(err)
				}
				if err := s.ProcessBusinessNotices(time.Now()); err != nil {
					t.Fatal(err)
				}
				if sent != 2 || s.businessNoticeTasks[0].ID != id {
					t.Fatal("retry lost stable identity")
				}
			}
		})
	}
}

func TestBusinessTeacherBatchMySQLReloadRollbackAndIdentityRepair(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated teacher batch database required")
	}
	if !strings.Contains(dsn, "/starline_teacher_batch_test?") {
		t.Fatal("requires dedicated starline_teacher_batch_test database")
	}
	s, p, req := teacherBatchFixture(t, true)
	s.officialFollowers = nil
	for i := range s.grants {
		s.grants[i].OpenedAt = "2026-10-03 09:00:00"
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	reload := func() {
		t.Helper()
		if err := s.db.Close(); err != nil {
			t.Fatal(err)
		}
		s = NewMemoryStore()
		if err := s.ConnectDatabase(dsn); err != nil {
			t.Fatal(err)
		}
	}
	defer func() { s.db.Close() }()
	if _, err := s.CreateTeachingPlan("test", p, req); err != nil {
		t.Fatal(err)
	}
	reload()
	binding := s.bindingForBusinessKind(learning.NoticeTeachingPlansUploaded)
	if !binding.Enabled || !binding.Ready || len(binding.TeacherIDs) != 1 || binding.WebOrigin != "https://school.example" {
		t.Fatal("teacher configuration did not reload")
	}
	if len(s.PendingTeachingPlanNoticeBatches(p)) != 1 {
		t.Fatal("registry lost after reload")
	}
	if _, err := s.db.Exec("CREATE TRIGGER teacher_batch_test_fail BEFORE INSERT ON business_notice_tasks FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced teacher completion failure'"); err != nil {
		t.Fatal(err)
	}
	_, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID)
	if _, dropErr := s.db.Exec("DROP TRIGGER teacher_batch_test_fail"); dropErr != nil {
		t.Fatal(dropErr)
	}
	if err == nil || len(s.businessNoticeTasks) != 0 || len(s.PendingTeachingPlanNoticeBatches(p)) != 1 {
		t.Fatal("completion failure committed partial state")
	}
	reload()
	if len(s.businessNoticeTasks) != 0 || len(s.PendingTeachingPlanNoticeBatches(p)) != 1 {
		t.Fatal("rollback not durable after reconnect")
	}
	if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].Status != "不可触达" || s.businessNoticeTasks[0].RecipientUserID != "user-teacher" || s.businessNoticeTasks[0].URL == "" {
		t.Fatal("teacher task identity or URL lost")
	}
	taskID := s.businessNoticeTasks[0].ID
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.officialFollowers = []learning.OfficialFollower{{OpenID: "teacher-official-open", UnionID: "teacher-batch-union", Subscribed: true}}
	})
	if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != taskID || s.businessNoticeTasks[0].OpenID != "teacher-official-open" || s.businessNoticeTasks[0].Status != "待发送" {
		t.Fatal("identity repair lost after reload")
	}
	var recipientKey string
	if err := s.db.QueryRow("SELECT recipient_key FROM business_notice_tasks WHERE id=?", taskID).Scan(&recipientKey); err != nil {
		t.Fatal(err)
	}
	if recipientKey != businessNoticeHash("teacher-official-open", "") {
		t.Fatal("SQL recipient key remained stale after repair")
	}
	req.File.ID = "teacher-batch-recovered-mysql"
	if _, err := s.CreateTeachingPlan("test", p, req); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.PendingTeachingPlanNoticeBatches(p)) != 1 {
		t.Fatal("recovered upload lost pending completion")
	}
	if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != taskID || s.businessNoticeTasks[0].Values["number3"] != "2" || len(s.PendingTeachingPlanNoticeBatches(p)) != 0 {
		t.Fatal("reloaded completion duplicated task or lost count")
	}
}

func TestBusinessTeacherBatchPerRecipientVisibleCountAndLink(t *testing.T) {
	s, p, req := teacherBatchFixture(t, true)
	first, err := s.CreateTeachingPlan("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Subject = "数学"
	req.Title = "数学内部教案"
	req.File.ID = "math-plan-file"
	second, err := s.CreateTeachingPlan("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	s.users = append(s.users, learning.User{ID: "math-teacher", Name: "数学老师", Roles: []learning.Role{learning.RoleTeacher}, AccountStatus: "正常", UnionID: "math-teacher-union", TeacherLibrary: &learning.TeacherLibraryPolicy{Scopes: []learning.TeacherLibraryScope{{Grade: "五年级", Subject: "数学"}}}})
	s.officialFollowers = append(s.officialFollowers, learning.OfficialFollower{OpenID: "math-official", UnionID: "math-teacher-union", Subscribed: true})
	binding := s.bindingForBusinessKind(learning.NoticeTeachingPlansUploaded)
	binding.TeacherIDs = []string{"user-teacher", "math-teacher"}
	if _, err := s.UpdateBusinessNoticeBinding("test", binding); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 2 {
		t.Fatalf("wrong per-teacher tasks: %+v", s.businessNoticeTasks)
	}
	for _, task := range s.businessNoticeTasks {
		plan := first
		if task.RecipientUserID == "math-teacher" {
			plan = second
		}
		event, ok := s.businessEvent(task.EventID)
		if !ok || len(event.ResourceIDs) != 1 || event.ResourceIDs[0] != plan.ID || task.Values["number3"] != "1" || task.Values["thing4"] != plan.Title || task.URL != "https://school.example/teaching-plans?notice="+task.EventID+"&plan="+plan.ID {
			t.Fatalf("teacher task exposes unrelated resources: %+v %+v", task, event)
		}
	}
}

func TestBusinessTeacherNoticeBatchCurrentPermissionsAndOwnership(t *testing.T) {
	s, p, req := teacherBatchFixture(t, true)
	first, err := s.CreateTeachingPlan("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	req.File.ID = "teacher-batch-file-2"
	req.Title = "第二课教案"
	second, err := s.CreateTeachingPlan("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteTeachingPlanNoticeBatch("test", p, req.BatchID); err != nil {
		t.Fatal(err)
	}
	task := s.businessNoticeTasks[0]
	teacher, _ := s.PrincipalByUserID("user-teacher")
	detail, err := s.TeachingPlanNotice(teacher, task.EventID)
	if err != nil || len(detail.Plans) != 2 || detail.OriginalCount != 2 {
		t.Fatalf("batch incomplete: %v", err)
	}
	if _, err = s.TeachingPlanNotice(p, task.EventID); err == nil {
		t.Fatal("uploader opened another recipient's notice")
	}
	for i, plan := range s.teachingPlans {
		if plan.ID == second.ID {
			s.teachingPlans = append(s.teachingPlans[:i], s.teachingPlans[i+1:]...)
			break
		}
	}
	detail, err = s.TeachingPlanNotice(teacher, task.EventID)
	if err != nil || len(detail.Plans) != 1 || detail.Plans[0].ID != first.ID || detail.OriginalCount != 2 {
		t.Fatal("partial revocation leaked missing resource or hid remaining one")
	}
	for i := range s.users {
		if s.users[i].ID == teacher.UserID {
			s.users[i].AccountStatus = "停用"
		}
	}
	if _, err = s.TeachingPlanNotice(teacher, task.EventID); err == nil {
		t.Fatal("disabled teacher opened batch")
	}
}
