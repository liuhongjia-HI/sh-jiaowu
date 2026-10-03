package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/domain/learning"
	"starline/learning-api/internal/interfaces/http/handler"
)

// Opt-in because this integration runs the real Vite app and Chromium in
// addition to the HTTP router and conversion worker. No database or live account.
func TestTeachingPlanRealBrowserUploadPreviewAndRecovery(t *testing.T) {
	if os.Getenv("STARLINE_REAL_UI") != "1" {
		t.Skip("set STARLINE_REAL_UI=1 for real browser integration")
	}
	root := t.TempDir()
	app := newTestAppWithStorageRoot(t, root)
	defer app.close()
	token := app.loginAdmin(t, "13800000001")
	p, err := app.store.PrincipalByUserID("user-super")
	if err != nil {
		t.Fatal(err)
	}
	var course learning.Course
	app.doJSON(t, http.MethodPost, "/api/courses", token, learning.CourseUpsertRequest{Name: "真实联验教案目录", LearningSpaceID: "space-g05-english-s1-q1", Status: learning.StatusEnabled, Curriculum: []learning.CurriculumNode{{ID: "browser-u1", Type: learning.CurriculumUnit, Name: "第一课", SortOrder: 1}, {ID: "browser-u2", Type: learning.CurriculumUnit, Name: "第二课", SortOrder: 2}}}, http.StatusOK, &course)
	for i, text := range []string{"First browser teaching plan", "Second browser teaching plan"} {
		name := []string{"独立第一.pdf", "独立第二.pdf"}[i]
		if err := os.WriteFile(filepath.Join(root, name), teachingPlanPDF(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// This control exists only in the ephemeral test server, outside /api. It
	// deletes a generated preview to exercise real recovery without altering originals.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test/remove-preview" {
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusForbidden)
				return
			}
			asset, err := app.store.TeachingPlanFile(p, r.URL.Query().Get("id"))
			if err != nil || asset.PreviewPath == "" || asset.PreviewPath == asset.OriginalPath {
				http.Error(w, "invalid test preview", http.StatusBadRequest)
				return
			}
			if err := os.Remove(asset.PreviewPath); err != nil {
				http.Error(w, "remove failed", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		app.server.transport.handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
	done := make(chan struct{})
	go func() { handler.NewPreviewWorker(learningapp.NewService(app.store), root).Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	webDir := filepath.Clean(filepath.Join(cwd, "../../../../../web"))
	parsed, _ := url.Parse(server.URL)
	userJSON, _ := json.Marshal(p)
	cmd := exec.CommandContext(ctx, "node", "node_modules/@playwright/test/cli.js", "test", "--config=playwright.real-plan.config.ts")
	arm64Host := false
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		capability, _ := exec.Command("/usr/sbin/sysctl", "-n", "hw.optional.arm64").Output()
		arm64Host = string(capability) == "1\n"
	}
	if arm64Host {
		// The local Go toolchain uses Rosetta while the existing web dependencies
		// are arm64. Run the native Node slice rather than installing extra packages.
		cmd = exec.CommandContext(ctx, "/usr/bin/arch", "-arm64", "node", "node_modules/@playwright/test/cli.js", "test", "--config=playwright.real-plan.config.ts")
	}
	cmd.Dir = webDir
	cmd.Env = append(os.Environ(), "HTTP_PORT="+parsed.Port(), "STARLINE_REAL_API="+server.URL, "STARLINE_REAL_TOKEN="+token, "STARLINE_REAL_USER="+string(userJSON), "STARLINE_REAL_PLAN_FILES="+root)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatalf("real browser integration failed: %v", err)
	}
	if len(app.store.PendingTeachingPlanNoticeBatches(p)) != 0 || len(app.store.BusinessNoticeTasks()) != 0 {
		t.Fatal("real browser did not complete registered batch or enabled external delivery")
	}
	list := app.store.TeachingPlans(p)
	if len(list.Plans) != 2 || len(list.UnreadPlanIDs) != 0 {
		t.Fatalf("browser upload/read result not committed: %#v", list)
	}
	for _, plan := range list.Plans {
		want := map[string]string{"独立第一": "browser-u1", "独立第二": "browser-u2"}[plan.Title]
		if want == "" || plan.CourseID != course.ID || plan.LessonID != want || plan.PreviewStatus != "可预览" {
			t.Fatalf("browser chapter or recovered preview incorrect: %#v", plan)
		}
	}
}
