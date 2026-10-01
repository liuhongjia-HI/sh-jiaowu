package store

import (
	"os"
	"starline/learning-api/internal/domain/learning"
	"strings"
	"sync"
	"testing"
	"time"
)

// Run only against a dedicated disposable database, never the production DSN.
func TestBusinessNoticeMySQLAtomicReloadAndConcurrentClaim(t *testing.T) {
	dsn := os.Getenv("STARLINE_BUSINESS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("STARLINE_BUSINESS_TEST_MYSQL_DSN is not configured")
	}
	if !strings.Contains(dsn, "/starline_business_test?") {
		t.Fatal("test requires dedicated starline_business_test database")
	}
	baseline := NewMemoryStoreWithOptions(Options{SkipBaseData: true})
	if err := baseline.ConnectDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	defer baseline.db.Close()
	for _, table := range []string{"business_notice_tasks", "business_notice_events", "business_notice_receipts", "business_schedule_snapshots"} {
		if _, err := baseline.db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := baseline.db.Exec("DROP TRIGGER IF EXISTS business_test_fail"); err != nil {
		t.Fatal(err)
	}
	s := businessFixture(t)
	s.guardians[0].Phone = "17900000001"
	s.guardians[1].Phone = "17900000002"
	s.db = baseline.db
	if err := s.bootstrapPersistAll(); err != nil {
		t.Fatal(err)
	}
	mutateBusiness(t, s, func(work *MemoryStore) {
		work.scheduleClasses = []learning.ScheduleClass{futureBusinessClass("mysql-business", "series")}
	})
	var events, tasks, notices int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM business_notice_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM business_notice_tasks").Scan(&tasks); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM notices WHERE related_type='business'").Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if events != 2 || tasks != 3 || notices != 2 {
		t.Fatalf("atomic outbox counts %d/%d/%d", events, tasks, notices)
	}
	if _, err := s.db.Exec("CREATE TRIGGER business_test_fail BEFORE INSERT ON business_notice_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced transaction failure'"); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	err := persistentMutationError(s, func(work *MemoryStore) error { work.scheduleClasses[0].StartTime = "17:00"; return nil })
	s.mu.Unlock()
	if _, cleanupErr := s.db.Exec("DROP TRIGGER business_test_fail"); cleanupErr != nil {
		t.Fatal(cleanupErr)
	}
	if err == nil || s.scheduleClasses[0].StartTime != "16:00" {
		t.Fatal("failed outbox transaction published scheduling state")
	}
	if err := s.loadBusinessNoticesFromDB(); err != nil {
		t.Fatal(err)
	}
	if len(s.businessNoticeEvents) != 2 || len(s.businessNoticeTasks) != 3 || s.businessNoticeTasks[0].OpenID == "" {
		t.Fatal("outbox or identity lost after reload")
	}
	copyStore := businessFixture(t)
	copyStore.db = s.db
	if err := copyStore.loadBusinessNoticesFromDB(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errors := make(chan error, 2)
	for _, worker := range []*MemoryStore{s, copyStore} {
		wg.Add(1)
		go func(worker *MemoryStore) {
			defer wg.Done()
			_, ok, err := worker.claimBusinessTask(0, time.Now())
			results <- ok
			errors <- err
		}(worker)
	}
	wg.Wait()
	close(results)
	close(errors)
	winners := 0
	for ok := range results {
		if ok {
			winners++
		}
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent claim winners=%d", winners)
	}
	if err := s.loadBusinessNoticesFromDB(); err != nil {
		t.Fatal(err)
	}
	if s.businessNoticeTasks[0].Status != "发送中" || s.businessNoticeTasks[0].Attempts != 1 {
		t.Fatal("lease was not persistent")
	}

	mutateBusiness(t, s, func(work *MemoryStore) {
		work.students[0].Grade = "G5"
		work.students[1].Grade = "G5"
		work.officialCampaigns = []learning.OfficialCampaign{{ID: "mysql-campaign", TemplateID: "template", TemplateTitle: "课程通知", Grades: []string{"G5"}, Values: map[string]string{"thing1": "课程"}, TargetCount: 1, CreatedBy: "test", CreatedAt: time.Now().Format("2006-01-02 15:04:05")}}
		work.officialCampaignRecipients = []learning.OfficialCampaignRecipient{{ID: "mysql-recipient", CampaignID: "mysql-campaign", GuardianID: "g1", OpenID: "o1", Status: "待发送"}}
	})
	s.officialMessageSender = func(learning.OfficialMessageRequest) (string, error) {
		if err := s.HandleOfficialDeliveryReceipt("o1", "mysql-manual-msg", "success"); err != nil {
			t.Fatal(err)
		}
		return "mysql-manual-msg", nil
	}
	s.deliverOfficialCampaign("mysql-campaign", false)
	if err := s.loadOfficialMessagingFromDB(); err != nil {
		t.Fatal(err)
	}
	if len(s.officialCampaignRecipients) != 1 || s.officialCampaignRecipients[0].MessageID != "mysql-manual-msg" || s.officialCampaignRecipients[0].DeliveredAt == "" || s.officialCampaignRecipients[0].Status != "已送达" {
		t.Fatal("manual message receipt lost after MySQL reload")
	}
}
