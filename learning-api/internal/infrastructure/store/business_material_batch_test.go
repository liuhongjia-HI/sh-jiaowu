package store

import (
	"fmt"
	"os"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"testing"
	"time"
)

func materialBusinessFixture(t *testing.T, enabled bool) (*MemoryStore, learning.Principal, learning.MaterialUploadRequest) {
	t.Helper()
	s := NewMemoryStore()
	p, _ := s.PrincipalByUserID("user-super")
	course, _ := s.findCourse("course-g05-english-s1-q1")
	s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "resource-template", Status: "启用", Content: "资料：{{thing4.DATA}}\n数量：{{number3.DATA}}\n时间：{{time7.DATA}}"})
	if _, err := s.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeMaterialsPublished, TemplateID: "resource-template", Enabled: enabled, FieldMappings: map[string]string{"thing4": "resource_title", "number3": "resource_count", "time7": "published_at"}}); err != nil {
		t.Fatal(err)
	}
	req := learning.MaterialUploadRequest{BatchID: "business-batch", Title: "资料一", CourseID: course.ID, LessonID: firstLessonID(course), TagCode: "HD", File: learning.FileAsset{ID: "business-file-one", FileName: "one.pdf"}}
	return s, p, req
}

func attachMaterialParent(s *MemoryStore, studentID string) {
	for i := range s.students {
		if s.students[i].ID != studentID {
			s.students[i].AccountStatus = "停用"
		}
	}
	s.guardians = []learning.Guardian{{ID: "batch-parent", UnionID: "batch-union", AccountStatus: "正常"}}
	s.guardianStudents = []learning.GuardianStudent{{GuardianID: "batch-parent", StudentID: studentID, Status: learning.GuardianStudentActive}}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "batch-open", UnionID: "batch-union", Subscribed: true}}
}

func TestBusinessMaterialBatchCompletionRetryAndCurrentAccess(t *testing.T) {
	s, p, req := materialBusinessFixture(t, true)
	first, err := s.CreateMaterial("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	studentID := s.notices[0].RecipientStudentID
	attachMaterialParent(s, studentID)
	if len(s.businessNoticeTasks) != 0 || len(s.PendingMaterialNoticeBatches(p)) != 1 {
		t.Fatal("upload sent before completion or lost pending batch")
	}
	teacher, _ := s.PrincipalByUserID("user-teacher")
	if _, err = s.CompleteMaterialNoticeBatch("test", teacher, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err == nil {
		t.Fatal("another uploader completed the batch")
	}
	req.File.ID = "business-file-two"
	_, err = s.CreateMaterial("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResourceCount != 2 || result.AlreadyCompleted || len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].Values["number3"] != "2" {
		t.Fatalf("wrong merged delivery: %+v %+v", result, s.businessNoticeTasks)
	}
	taskID, eventID := s.businessNoticeTasks[0].ID, s.businessNoticeTasks[0].EventID
	// A newer operation's station notice must not be marked read by this event.
	other := req
	other.BatchID = "another-batch"
	other.File.ID = "business-file-other"
	if _, err = s.CreateMaterial("test", p, other); err != nil {
		t.Fatal(err)
	}
	req.File.ID = "business-file-recovered"
	if _, err = s.CreateMaterial("test", p, req); err != nil {
		t.Fatal(err)
	}
	pending := s.PendingMaterialNoticeBatches(p)
	if len(pending) != 2 {
		t.Fatal("recovered file did not persist a pending completion")
	}
	result, err = s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID})
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyCompleted || len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != taskID || s.businessNoticeTasks[0].Values["number3"] != "3" {
		t.Fatal("completion retry duplicated delivery or lost recovered count")
	}
	detail, err := s.BusinessNoticeDetail(learning.Principal{GuardianID: "batch-parent"}, eventID)
	if err != nil {
		t.Fatal(err)
	}
	expected := "notice-upload-" + businessNoticeHash(p.UserID, req.BatchID, req.CourseID, studentID)
	if detail.NoticeID != expected || !detail.CanSwitch || len(detail.CurrentMaterials) != 3 {
		t.Fatalf("wrong material entry: %+v", detail)
	}
	if _, err = s.BusinessNoticeDetail(learning.Principal{StudentID: "unrelated-student"}, eventID); err == nil {
		t.Fatal("unrelated child accessed details")
	}
	for i := range s.materials {
		if s.materials[i].ID == first.ID {
			s.materials[i].PublishStatus = "草稿"
		}
	}
	detail, err = s.BusinessNoticeDetail(learning.Principal{StudentID: studentID}, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.CurrentMaterials) != 2 || len(detail.Event.ResourceIDs) != 2 || detail.Event.Values != nil {
		t.Fatal("withdrawn material metadata leaked")
	}
	if _, err = s.StudentMaterial(learning.Principal{StudentID: studentID}, first.ID); err == nil {
		t.Fatal("preview fallback exposed a draft material")
	}
	course, _ := s.findCourse(req.CourseID)
	for _, material := range s.publishedMaterialsForCourse(course) {
		if material.ID == first.ID {
			t.Fatal("course catalog exposed a draft material")
		}
	}
	sent := 0
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sent++; return "fake", nil }
	if err = s.ProcessBusinessNotices(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sent != 0 || !strings.Contains(s.businessNoticeTasks[0].FailureReason, "资料已撤回") {
		t.Fatal("withdrawn material sent")
	}
}

func TestBusinessMaterialBatchDisabledNeverReplaysAndMappingRevalidates(t *testing.T) {
	for _, mode := range []string{"disabled", "mapping", "delivered"} {
		t.Run(mode, func(t *testing.T) {
			s, p, req := materialBusinessFixture(t, mode != "disabled")
			if _, err := s.CreateMaterial("test", p, req); err != nil {
				t.Fatal(err)
			}
			attachMaterialParent(s, s.notices[0].RecipientStudentID)
			if _, err := s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
				t.Fatal(err)
			}
			binding := s.bindingForBusinessKind(learning.NoticeMaterialsPublished)
			if mode == "disabled" {
				binding.Enabled = true
			} else if mode == "mapping" {
				binding.FieldMappings["thing4"] = "course_name"
			}
			if _, err := s.UpdateBusinessNoticeBinding("test", binding); err != nil {
				t.Fatal(err)
			}
			sent := 0
			s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) { sent++; return fmt.Sprint(sent), nil }
			if err := s.ProcessBusinessNotices(time.Now()); err != nil {
				t.Fatal(err)
			}
			if _, err := s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
				t.Fatal(err)
			}
			if err := s.ProcessBusinessNotices(time.Now().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			expected := 0
			if mode == "delivered" {
				expected = 1
			}
			if sent != expected {
				t.Fatalf("unexpected send/replay %s: %d", mode, sent)
			}
			if mode == "mapping" && !strings.Contains(s.businessNoticeTasks[0].FailureReason, "映射已变更") {
				t.Fatal("mapping change not detected")
			}
		})
	}
}

func TestBusinessMaterialBatchMySQLReloadAndFailedCompletion(t *testing.T) {
	dsn := os.Getenv("STARLINE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated material batch database required")
	}
	if !strings.Contains(dsn, "/starline_material_batch_test?") {
		t.Fatal("requires dedicated starline_material_batch_test database")
	}
	s, p, req := materialBusinessFixture(t, true)
	for i := range s.grants {
		s.grants[i].OpenedAt = "2026-10-03 09:00:00"
	}
	if err := s.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMaterial("test", p, req); err != nil {
		t.Fatal(err)
	}
	mutateBusiness(t, s, func(work *MemoryStore) {
		attachMaterialParent(work, s.notices[0].RecipientStudentID)
		work.guardians[0].Phone = "17900000001"
	})
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
	reload()
	defer func() { s.db.Close() }()
	if len(s.PendingMaterialNoticeBatches(p)) != 1 {
		t.Fatal("reload lost pending registry")
	}
	if _, err := s.db.Exec("CREATE TRIGGER material_batch_test_fail BEFORE INSERT ON business_notice_tasks FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced batch completion failure'"); err != nil {
		t.Fatal(err)
	}
	_, err := s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID})
	if _, cleanupErr := s.db.Exec("DROP TRIGGER material_batch_test_fail"); cleanupErr != nil {
		t.Fatal(cleanupErr)
	}
	if err == nil || len(s.PendingMaterialNoticeBatches(p)) != 1 || len(s.businessNoticeTasks) != 0 {
		t.Fatal("failed completion published state")
	}
	reload()
	if len(s.PendingMaterialNoticeBatches(p)) != 1 || len(s.businessNoticeTasks) != 0 {
		t.Fatal("failed transaction persisted completion")
	}
	if _, err = s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 {
		t.Fatalf("wrong persisted task count %d", len(s.businessNoticeTasks))
	}
	id := s.businessNoticeTasks[0].ID
	reload()
	if len(s.PendingMaterialNoticeBatches(p)) != 0 || len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != id {
		t.Fatal("reload lost completion or task identity")
	}
	req.File.ID = "mysql-recovered-material"
	if _, err = s.CreateMaterial("test", p, req); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.PendingMaterialNoticeBatches(p)) != 1 {
		t.Fatal("recovered file pending state not persisted")
	}
	if _, err = s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
		t.Fatal(err)
	}
	reload()
	if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].ID != id || s.businessNoticeTasks[0].Values["number3"] != "2" || len(s.PendingMaterialNoticeBatches(p)) != 0 {
		t.Fatal("retry duplicated delivery or lost updated count")
	}
}

func TestBusinessMaterialBatchTrialGuardianDoesNotNotifyOtherParent(t *testing.T) {
	s, p, req := materialBusinessFixture(t, true)
	first, err := s.CreateMaterial("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	studentID := s.notices[0].RecipientStudentID
	attachMaterialParent(s, studentID)
	s.guardians = append(s.guardians, learning.Guardian{ID: "other-batch-parent", UnionID: "other-batch-union", AccountStatus: "正常"})
	s.guardianStudents = append(s.guardianStudents, learning.GuardianStudent{GuardianID: "other-batch-parent", StudentID: studentID, Status: learning.GuardianStudentActive})
	s.officialFollowers = append(s.officialFollowers, learning.OfficialFollower{OpenID: "other-batch-open", UnionID: "other-batch-union", Subscribed: true})
	binding := s.bindingForBusinessKind(learning.NoticeMaterialsPublished)
	binding.StudentIDs = []string{studentID}
	binding.GuardianIDs = []string{"batch-parent"}
	if _, err = s.UpdateBusinessNoticeBinding("test", binding); err != nil {
		t.Fatal(err)
	}
	req.Title = "资料二"
	req.File.ID = "business-file-two"
	second, err := s.CreateMaterial("test", p, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteMaterialNoticeBatch("test", p, req.BatchID, learning.MaterialNoticeBatchRequest{CourseID: req.CourseID}); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeTasks) != 1 || s.businessNoticeTasks[0].GuardianID != "batch-parent" || s.businessNoticeTasks[0].Values["number3"] != "2" {
		t.Fatal("batch target broadened or successful files not combined")
	}
	event, ok := s.businessEvent(s.businessNoticeTasks[0].EventID)
	if !ok || !containsString(event.ResourceIDs, first.ID) || !containsString(event.ResourceIDs, second.ID) {
		t.Fatal("batch lost successful resource identifiers")
	}
}
