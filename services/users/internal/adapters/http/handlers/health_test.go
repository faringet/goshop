package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthHandlers_Live_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	h := NewHealthHandlers(log, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Live(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := strings.TrimSpace(w.Body.String())
	if body != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}

	cc := w.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", cc, "no-store")
	}
}

func TestHealthHandlers_Ready_DBPoolNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	// db = nil, чтобы попасть в ветку "db not ready"
	h := NewHealthHandlers(log, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Ready(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	body := strings.TrimSpace(w.Body.String())
	if body != "db not ready" {
		t.Fatalf("body = %q, want %q", body, "db not ready")
	}

	cc := w.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", cc, "no-store")
	}
}

func TestHealthHandlers_DBPing_DBPoolNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	h := NewHealthHandlers(log, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.DBPing(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	status, _ := resp["status"].(string)
	errStr, _ := resp["err"].(string)

	if status != "fail" {
		t.Fatalf(`resp["status"] = %q, want %q`, status, "fail")
	}
	if errStr != "db is nil" {
		t.Fatalf(`resp["err"] = %q, want %q`, errStr, "db is nil")
	}

	cc := w.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", cc, "no-store")
	}
}
