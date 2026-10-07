package router_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"starline/learning-api/internal/domain/learning"
	"testing"
	"time"
)

// Real React -> loopback HTTP -> business state. No production database or WeChat.
func TestReviewExceptionRealBrowserMarkReopenAndResolve(t *testing.T) {
	if os.Getenv("STARLINE_REAL_UI") != "1" {
		t.Skip("set STARLINE_REAL_UI=1 for browser integration")
	}
	app := newTestApp(t)
	defer app.close()
	if _, err := app.store.UpdateBusinessNoticeBinding("test", learning.BusinessNoticeBinding{Kind: learning.NoticeReviewException, Enabled: false, ApprovedReasons: []string{"作业缺页"}}); err != nil {
		t.Fatal(err)
	}
	token := app.loginAdmin(t, "13800000004")
	p, err := app.store.PrincipalByUserID("user-teacher")
	if err != nil {
		t.Fatal(err)
	}
	user, _ := json.Marshal(p)
	server := httptest.NewServer(app.server.transport.handler)
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	args := []string{"node_modules/@playwright/test/cli.js", "test", "--config=playwright.review-exception.config.ts"}
	cmd := exec.CommandContext(ctx, "node", args...)
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		capability, _ := exec.Command("/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
		if string(capability) == "1\n" {
			cmd = exec.CommandContext(ctx, "/usr/bin/arch", append([]string{"-arm64", "node"}, args...)...)
		}
	}
	cmd.Dir = filepath.Clean(filepath.Join(cwd, "../../../../../web"))
	cmd.Env = append(os.Environ(), "HTTP_PORT="+parsed.Port(), "STARLINE_REAL_REVIEW_API="+server.URL, "STARLINE_REAL_REVIEW_TOKEN="+token, "STARLINE_REAL_REVIEW_USER="+string(user))
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatalf("real exception browser failed: %v", err)
	}
	for _, review := range app.store.Reviews(p) {
		if review.ID == "rev-001" && (review.Status != "待复核" || review.ExceptionEventID != "") {
			t.Fatal("browser state not committed")
		}
	}
	if len(app.store.BusinessNoticeTasks()) != 0 {
		t.Fatal("isolated disabled business emitted external tasks")
	}
}
