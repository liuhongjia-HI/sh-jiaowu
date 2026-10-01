package store

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/infrastructure/config"
	"starline/learning-api/internal/infrastructure/logger"
	"starline/learning-api/internal/interfaces/http/router"
)

// An opt-in loopback fixture for the real React -> API -> outbox workflow.
// Never connects a database or calls WeChat; normal Go runs skip this server.
func TestBusinessNoticeBrowserHarness(t *testing.T) {
	if os.Getenv("STARLINE_NOTICE_BROWSER_TEST") != "1" {
		t.Skip("opt-in local browser fixture")
	}
	s := NewMemoryStore()
	s.scheduleClasses = nil
	s.businessScheduleSnapshots = map[string]learning.ScheduleClass{}
	s.guardians = []learning.Guardian{{ID: "browser-parent", Name: "本地试点家长", Phone: "17900000001", UnionID: "browser-union", AccountStatus: "正常"}}
	s.guardianStudents = []learning.GuardianStudent{{GuardianID: "browser-parent", StudentID: "stu-001", Status: learning.GuardianStudentActive}}
	s.officialFollowers = []learning.OfficialFollower{{OpenID: "browser-openid", UnionID: "browser-union", Subscribed: true}}
	s.officialTemplates = nil
	titles := map[string]string{learning.NoticeScheduleConfirmed: "排课时间已确认通知", learning.NoticeScheduleChanged: "调课成功通知", learning.NoticeScheduleReminder: "课程预约结果通知"}
	for _, binding := range defaultBusinessNoticeBindings() {
		title, ok := titles[binding.Kind]
		if !ok {
			continue
		}
		content := ""
		for key, label := range binding.RequiredFields {
			content += label + "：{{" + key + ".DATA}}\n"
		}
		s.officialTemplates = append(s.officialTemplates, learning.OfficialTemplate{ID: "browser-" + binding.Kind, Title: title, Content: content, Fields: parseOfficialTemplateFields(content), Status: "启用"})
	}
	date := time.Now().In(businessNoticeLocation).AddDate(0, 0, 2)
	day := int(date.Weekday())
	if day == 0 {
		day = 7
	}
	s.availability = []learning.AvailabilitySlot{{ID: "browser-teacher", OwnerType: "teacher", OwnerID: "user-teacher", DayOfWeek: day, StartTime: "08:00", EndTime: "22:00"}, {ID: "browser-student", OwnerType: "student", OwnerID: "stu-001", DayOfWeek: day, StartTime: "08:00", EndTime: "22:00"}}
	var control sync.Mutex
	mode := "temporary"
	s.officialMessageSender = func(req learning.OfficialMessageRequest) (string, error) {
		control.Lock()
		defer control.Unlock()
		if mode == "temporary" {
			return "", &officialSendError{reason: "本地模拟微信限流", temporary: true}
		}
		return "browser-msg-" + req.ClientMessageID[:12], nil
	}
	cfg := config.MustLoad()
	cfg.App.Env = "test"
	cfg.Auth.TokenSecret = "isolated-browser-notices"
	cfg.Demo.AdminPasswordLogin = true
	cfg.Demo.StudentPasswordLogin = true
	engine := router.New(router.Dependencies{Config: cfg, Logger: logger.New("test"), Service: learningapp.NewService(s)})
	mux := http.NewServeMux()
	mux.Handle("/api/", engine)
	mux.HandleFunc("/__notice/meta", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"date": date.Format("2006-01-02")})
	})
	mux.HandleFunc("/__notice/process", func(w http.ResponseWriter, r *http.Request) {
		minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
		if minutes < 0 || minutes > 60 {
			http.Error(w, "invalid clock", 400)
			return
		}
		if err := s.ProcessBusinessNotices(time.Now().Add(time.Duration(minutes) * time.Minute)); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/__notice/success", func(w http.ResponseWriter, r *http.Request) {
		control.Lock()
		mode = "success"
		control.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:18992")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	fmt.Println("NOTICE_BROWSER_FIXTURE_READY")
	select {
	case err := <-serveErrors:
		if err != http.ErrServerClosed {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Minute):
	}
}
