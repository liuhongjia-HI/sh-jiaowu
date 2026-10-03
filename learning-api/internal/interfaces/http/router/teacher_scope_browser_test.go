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
	"testing"
	"time"
)

func TestTeacherScopeRealBrowserFormAndCurrentAPI(t *testing.T) {
	if os.Getenv("STARLINE_REAL_UI") != "1" {
		t.Skip("set STARLINE_REAL_UI=1 for real browser integration")
	}
	app := newTestAppWithStorageRoot(t, t.TempDir())
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, err := app.store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.server.transport.handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	defer cancel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Clean(filepath.Join(cwd, "../../../../../web"))
	parsed, _ := url.Parse(server.URL)
	userJSON, _ := json.Marshal(p)
	cmd := exec.CommandContext(ctx, "node", "node_modules/@playwright/test/cli.js", "test", "--config=playwright.real-teacher.config.ts")
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		capability, _ := exec.Command("/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
		if string(capability) == "1\n" {
			cmd = exec.CommandContext(ctx, "/usr/bin/arch", "-arm64", "node", "node_modules/@playwright/test/cli.js", "test", "--config=playwright.real-teacher.config.ts")
		}
	}
	cmd.Dir = webDir
	cmd.Env = append(os.Environ(), "HTTP_PORT="+parsed.Port(), "STARLINE_REAL_API="+server.URL, "STARLINE_REAL_TOKEN="+token, "STARLINE_REAL_USER="+string(userJSON))
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatalf("real teacher browser integration failed: %v", err)
	}
}
