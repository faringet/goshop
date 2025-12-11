package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"goshop/pkg/httpx"
	"goshop/pkg/jwtauth"
	"goshop/services/users/internal/adapters/repo/sessionpg"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestUsersHandlers_Refresh_InvalidJSON(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:     "test-secret",
		Issuer:     "test-issuer",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})

	h := &UsersHandlers{
		log:      log,
		jwtm:     jwtm,
		sessions: &sessionpg.Repo{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"refresh_token": `
	req, err := http.NewRequest(http.MethodPost, "/v1/users/refresh", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.Refresh(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "invalid json" {
		t.Fatalf("expected error=%q, got %v", "invalid json", resp["error"])
	}
}

func TestUsersHandlers_Refresh_InvalidToken(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:     "test-secret",
		Issuer:     "test-issuer",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})

	h := &UsersHandlers{
		log:      log,
		jwtm:     jwtm,
		sessions: &sessionpg.Repo{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"refresh_token":"not-a-valid-jwt"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/users/refresh", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.Refresh(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "invalid token" {
		t.Fatalf("expected error=%q, got %v", "invalid token", resp["error"])
	}
}

func TestUsersHandlers_Logout_InvalidJSON(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:     "test-secret",
		Issuer:     "test-issuer",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})

	h := &UsersHandlers{
		log:      log,
		jwtm:     jwtm,
		sessions: &sessionpg.Repo{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"foo":"bar"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/users/logout", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.Logout(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "invalid json" {
		t.Fatalf("expected error=%q, got %v", "invalid json", resp["error"])
	}
}

func TestUsersHandlers_Logout_InvalidToken(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	jwtm := jwtauth.New(jwtauth.Config{
		Secret:     "test-secret",
		Issuer:     "test-issuer",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})

	h := &UsersHandlers{
		log:      log,
		jwtm:     jwtm,
		sessions: &sessionpg.Repo{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	body := `{"refresh_token":"totally-bad-token"}`
	req, err := http.NewRequest(http.MethodPost, "/v1/users/logout", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.Logout(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "invalid token" {
		t.Fatalf("expected error=%q, got %v", "invalid token", resp["error"])
	}
}

func TestUsersHandlers_Me_Unauthorized_NoClaims(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := &UsersHandlers{
		log:      log,
		jwtm:     nil,
		sessions: &sessionpg.Repo{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.Me(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Fatalf("expected error=%q, got %v", "unauthorized", resp["error"])
	}
}

func TestUsersHandlers_Me_Success(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := &UsersHandlers{
		log:      log,
		jwtm:     nil,
		sessions: &sessionpg.Repo{},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	now := time.Now().UTC()
	claims := &jwtauth.Claims{
		UserID: "user-123",
		Email:  "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "subject-123",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	c.Set(httpx.CtxKeyJWTClaims, claims)

	h.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		UID       string           `json:"uid"`
		Email     string           `json:"email"`
		Issuer    string           `json:"issuer"`
		Subject   string           `json:"subject"`
		IssuedAt  *jwt.NumericDate `json:"issued_at"`
		ExpiresAt *jwt.NumericDate `json:"expires_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.UID != claims.UserID {
		t.Fatalf("uid: expected %q, got %q", claims.UserID, resp.UID)
	}
	if resp.Email != claims.Email {
		t.Fatalf("email: expected %q, got %q", claims.Email, resp.Email)
	}
	if resp.Issuer != claims.Issuer {
		t.Fatalf("issuer: expected %q, got %q", claims.Issuer, resp.Issuer)
	}
	if resp.Subject != claims.Subject {
		t.Fatalf("subject: expected %q, got %q", claims.Subject, resp.Subject)
	}
	if resp.IssuedAt == nil || resp.IssuedAt.Time.IsZero() {
		t.Fatalf("issued_at should not be nil/zero")
	}
	if resp.ExpiresAt == nil || resp.ExpiresAt.Time.IsZero() {
		t.Fatalf("expires_at should not be nil/zero")
	}
}
