package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"starline/learning-api/internal/domain/learning"
)

func TestMemoryStoreConcurrentStudentCreateAndRead(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load admin: %v", err)
	}

	const writers = 16
	const readers = 8
	start := make(chan struct{})
	errs := make(chan error, writers)
	createdIDs := make(chan string, writers)
	var wg sync.WaitGroup

	for index := 0; index < writers; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			student, err := store.CreateStudent("并发测试", admin, learning.StudentUpsertRequest{
				Name:       fmt.Sprintf("并发学生%02d", index),
				Phone:      fmt.Sprintf("1779000%04d", index),
				Grade:      "五年级",
				SchoolName: "并发测试学校",
			})
			if err != nil {
				errs <- err
				return
			}
			createdIDs <- student.ID
		}()
	}
	for index := 0; index < readers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for attempt := 0; attempt < 20; attempt++ {
				_ = store.Students(admin, learning.StudentQuery{})
				_ = store.StudentPermissions()
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errs)
	close(createdIDs)
	for err := range errs {
		t.Errorf("concurrent student creation failed: %v", err)
	}
	ids := map[string]bool{}
	for id := range createdIDs {
		if ids[id] {
			t.Errorf("duplicate student id generated: %s", id)
		}
		ids[id] = true
	}
	if len(ids) != writers {
		t.Fatalf("created %d distinct students, want %d", len(ids), writers)
	}
}

func TestOfficialNoticeSenderRunsOutsideStoreLockAndMayReenter(t *testing.T) {
	store := NewMemoryStore()
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("load ops: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	if err := store.UseOfficialAccountAPI("app", "secret", "template"); err != nil {
		t.Fatalf("configure sender: %v", err)
	}
	store.mu.Lock()
	store.officialNoticeSender = func(learning.Notice) error {
		close(entered)
		_ = store.Settings() // callback reentry must not deadlock
		<-release
		return nil
	}
	store.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := store.CreateNotice("并发测试", ops, learning.NoticeCreateRequest{
			Type: "通知", Title: "锁外发送", Target: "测试学生", Summary: "测试",
			Channel: "公众号模板消息", RecipientOpenID: "openid-test",
		})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("sender was not invoked")
	}

	readDone := make(chan struct{})
	go func() {
		_ = store.Students(ops, learning.StudentQuery{})
		close(readDone)
	}()
	select {
	case <-readDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("slow sender held the Store lock")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("create notice: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reentrant sender deadlocked")
	}
}

func TestMemoryStoreConcurrentGrantNoticeAndReads(t *testing.T) {
	store := NewMemoryStore()
	admin, err := store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatalf("load admin: %v", err)
	}
	ops, err := store.PrincipalByUserID("user-ops")
	if err != nil {
		t.Fatalf("load ops: %v", err)
	}
	student, err := store.CreateStudent("并发测试", admin, learning.StudentUpsertRequest{
		Name: "并发授权学生", Phone: "17791112222", Grade: "五年级", SchoolName: "并发测试学校",
	})
	if err != nil {
		t.Fatalf("create grant target: %v", err)
	}
	packageID := enabledPackageIDForGrade(store, "五年级")
	if packageID == "" {
		t.Fatal("missing enabled grade-five package")
	}

	const workers = 12
	start := make(chan struct{})
	errs := make(chan error, workers*2)
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		index := index
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if _, err := store.CreateGrant("并发测试", learning.GrantCreateRequest{StudentID: student.ID, PackageID: packageID}); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if _, err := store.CreateNotice("并发测试", ops, learning.NoticeCreateRequest{
				Type: "通知", Title: fmt.Sprintf("并发通知%02d", index),
				Target: "五年级英语班", Summary: "并发通知内容",
			}); err != nil {
				errs <- err
			}
			_ = store.Notices(ops)
			_ = store.StudentPermissions()
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent grant/notice operation failed: %v", err)
	}
	grants, err := store.StudentGrants(admin, student.ID)
	if err != nil {
		t.Fatalf("read student grants: %v", err)
	}
	matches := 0
	for _, grant := range grants {
		if grant.PackageID == packageID {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("concurrent duplicate grant creation: got %d grants for package %s", matches, packageID)
	}
}

func TestMemoryStorePublicBoundaryCopiesInputsAndOutputs(t *testing.T) {
	store := NewMemoryStore()

	settings := store.Settings()
	settings["contentPreviewMode"] = "caller-mutated"
	if got := store.Settings()["contentPreviewMode"]; got == "caller-mutated" {
		t.Fatal("Settings returned an alias to the internal map")
	}

	req := learning.PackageUpsertRequest{
		Name: "边界复制测试套餐", AcademicYear: "2025.2026学年", Grade: "五年级",
		Semester: "S1", Subject: "英语", PhaseScope: "Q1", PackageType: "题",
		LearningSpaceIDs: []string{"space-g05-english-s1-q1"},
		ContentTypeCodes: []string{"question"}, Status: learning.StatusEnabled,
	}
	created, err := store.CreatePackage("边界复制测试", req)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	req.LearningSpaceIDs[0] = "caller-mutated-input"
	created.LearningSpaceIDs[0] = "caller-mutated-output"

	packageFound := false
	for _, item := range store.Packages() {
		if item.ID != created.ID {
			continue
		}
		packageFound = true
		if item.LearningSpaceIDs[0] != "space-g05-english-s1-q1" {
			t.Fatalf("package retained caller-owned slice alias: %#v", item.LearningSpaceIDs)
		}
		break
	}
	if !packageFound {
		t.Fatalf("created package %s not found", created.ID)
	}

	student, err := store.PrincipalByUserID("user-student-001")
	if err != nil {
		t.Fatalf("load student principal: %v", err)
	}
	submissionReq := learning.SubmissionRequest{
		HomeworkID: "hw-g05-english-s1-q1",
		Answers: []learning.SubmissionAnswer{{
			QuestionID: "q1",
			Choices:    []string{"A", "B"},
			Text:       "boundary copy",
		}},
	}
	submission, err := store.CreateSubmission("边界复制测试", student, submissionReq)
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}
	submissionReq.Answers[0].Choices[0] = "caller-mutated-input"
	submission.Answers[0].Choices[0] = "caller-mutated-output"
	stored, err := store.StudentSubmission(student, submission.ID)
	if err != nil {
		t.Fatalf("reload submission: %v", err)
	}
	if stored.Answers[0].Choices[0] != "A" {
		t.Fatalf("submission retained caller-owned nested slice alias: %#v", stored.Answers)
	}
}
