package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStudentDownloadPickupExpirySecretAndCapacity(t *testing.T) {
	h := &LearningHandler{pickups: map[string]downloadPickup{
		"active":  {JobID: "download-" + strings.Repeat("a", 32), BrowserKey: "private-browser-key", Expires: time.Now().Add(time.Minute)},
		"expired": {BrowserKey: "old-key", Expires: time.Now().Add(-time.Second)},
	}}
	engine := gin.New()
	engine.GET("/pickup/:challenge", h.DownloadPickupStatus)
	engine.POST("/pickup", h.CreateDownloadPickup)
	call := func(method, path, key, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res
	}
	if res := call(http.MethodGet, "/pickup/active", "active", ""); res.Code != http.StatusForbidden {
		t.Fatal("QR challenge authorizes browser")
	}
	if res := call(http.MethodGet, "/pickup/active", "private-browser-key", ""); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "waiting") {
		t.Fatal("browser secret not accepted")
	}
	if res := call(http.MethodGet, "/pickup/expired", "old-key", ""); res.Code != http.StatusBadRequest {
		t.Fatal("expired QR remains usable")
	}
	if res := call(http.MethodPost, "/pickup", "", `{"jobId":"download-../bad"}`); res.Code != http.StatusBadRequest {
		t.Fatal("invalid job accepted")
	}
	if res := call(http.MethodPost, "/pickup", "", `{"jobId":"download-`+strings.Repeat("a", 32)+`"}`); res.Code != http.StatusOK {
		t.Fatal("session creation failed")
	}
	if _, ok := h.pickups["expired"]; ok {
		t.Fatal("expired sessions accumulate")
	}
	for len(h.pickups) < 1000 {
		id := string(rune(10000 + len(h.pickups)))
		h.pickups[id] = downloadPickup{Expires: time.Now().Add(time.Minute)}
	}
	if res := call(http.MethodPost, "/pickup", "", `{"jobId":"download-`+strings.Repeat("a", 32)+`"}`); res.Code != http.StatusBadRequest {
		t.Fatal("unbounded session allocation")
	}
}
