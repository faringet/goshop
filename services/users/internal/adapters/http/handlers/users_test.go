package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"goshop/services/users/internal/adapters/repo/userpg"
	"goshop/services/users/internal/app"
	domain "goshop/services/users/internal/domain/user"
)

type stubUserRepo struct {
	t            testing.TB
	createFn     func(ctx context.Context, email string, passwordHash []byte) (domain.User, error)
	getByEmailFn func(ctx context.Context, email string) (domain.User, error)
}

func (s *stubUserRepo) Create(ctx context.Context, email string, passwordHash []byte) (domain.User, error) {
	if s.createFn == nil {
		s.t.Fatalf("unexpected call to Create(%q)", email)
	}
	return s.createFn(ctx, email, passwordHash)
}

func (s *stubUserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	if s.getByEmailFn == nil {
		s.t.Fatalf("unexpected call to GetByEmail(%q)", email)
	}
	return s.getByEmailFn(ctx, email)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func performRequest(t *testing.T, handler gin.HandlerFunc, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, err := http.NewRequest(method, path, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.Request = req
	handler(c)

	return w
}

func newUsersHandlersWithRepo(t *testing.T, repo app.UserRepository) *UsersHandlers {
	t.Helper()

	svc := app.NewService(repo, 4)
	return &UsersHandlers{
		log:      newTestLogger(),
		svc:      svc,
		jwtm:     nil,
		sessions: nil,
	}
}

func TestUsersHandlers_Register(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		repo := &stubUserRepo{t: t}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": `) // поломанный JSON
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "invalid json" {
			t.Fatalf("error = %v, want %q", resp["error"], "invalid json")
		}
	})

	t.Run("missing email or password", func(t *testing.T) {
		repo := &stubUserRepo{t: t}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": "", "password": ""}`)
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "email and password are required" {
			t.Fatalf("error = %v, want %q", resp["error"], "email and password are required")
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		repo := &stubUserRepo{t: t} // Create не должен вызываться
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": "bad-email", "password": "StrongPass123!"}`)
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "invalid email" {
			t.Fatalf("error = %v, want %q", resp["error"], "invalid email")
		}
	})

	t.Run("weak password", func(t *testing.T) {
		repo := &stubUserRepo{t: t} // Create не должен вызываться
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": "user@example.com", "password": "1234567"}`) // < 8 символов
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "weak password" {
			t.Fatalf("error = %v, want %q", resp["error"], "weak password")
		}
	})

	t.Run("email already taken", func(t *testing.T) {
		repo := &stubUserRepo{
			t: t,
			createFn: func(ctx context.Context, email string, passwordHash []byte) (domain.User, error) {
				return domain.User{}, userpg.ErrEmailTaken
			},
		}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": "user@example.com", "password": "StrongPass123!"}`)
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "email already taken" {
			t.Fatalf("error = %v, want %q", resp["error"], "email already taken")
		}
	})

	t.Run("internal error", func(t *testing.T) {
		repo := &stubUserRepo{
			t: t,
			createFn: func(ctx context.Context, email string, passwordHash []byte) (domain.User, error) {
				return domain.User{}, errors.New("db down")
			},
		}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": "user@example.com", "password": "StrongPass123!"}`)
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "internal error" {
			t.Fatalf("error = %v, want %q", resp["error"], "internal error")
		}
	})

	t.Run("success", func(t *testing.T) {
		now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		uid := uuid.New()

		repo := &stubUserRepo{
			t: t,
			createFn: func(ctx context.Context, email string, passwordHash []byte) (domain.User, error) {
				return domain.User{
					ID:           uid,
					Email:        domain.NormalizeEmail(email),
					PasswordHash: passwordHash,
					CreatedAt:    now,
					UpdatedAt:    now,
				}, nil
			},
		}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email": "User@Example.Com", "password": "StrongPass123!"}`)
		w := performRequest(t, h.Register, http.MethodPost, "/v1/users/register", bytes.NewReader(body))

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
		}

		var resp registerResp
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal resp: %v", err)
		}

		if resp.ID != uid.String() {
			t.Fatalf("id = %s, want %s", resp.ID, uid.String())
		}
		if resp.Email != "user@example.com" { // NormalizeEmail должен опустить регистр
			t.Fatalf("email = %s, want %s", resp.Email, "user@example.com")
		}
		if resp.CreatedAt.IsZero() {
			t.Fatalf("created_at is zero, want non-zero")
		}
	})
}

func TestUsersHandlers_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		repo := &stubUserRepo{t: t}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email":`) // битый JSON
		w := performRequest(t, h.Login, http.MethodPost, "/v1/users/login", bytes.NewReader(body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "invalid json" {
			t.Fatalf("error = %v, want %q", resp["error"], "invalid json")
		}
	})

	t.Run("missing email or password", func(t *testing.T) {
		repo := &stubUserRepo{t: t}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email":"", "password":""}`)
		w := performRequest(t, h.Login, http.MethodPost, "/v1/users/login", bytes.NewReader(body))

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "email and password are required" {
			t.Fatalf("error = %v, want %q", resp["error"], "email and password are required")
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 4)
		if err != nil {
			t.Fatalf("generate hash: %v", err)
		}

		repo := &stubUserRepo{
			t: t,
			getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
				now := time.Now().UTC()
				return domain.User{
					ID:           uuid.New(),
					Email:        domain.NormalizeEmail(email),
					PasswordHash: hash,
					CreatedAt:    now,
					UpdatedAt:    now,
				}, nil
			},
		}
		h := newUsersHandlersWithRepo(t, repo)

		body := []byte(`{"email":"user@example.com", "password":"wrong-password"}`)
		w := performRequest(t, h.Login, http.MethodPost, "/v1/users/login", bytes.NewReader(body))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] != "invalid credentials" {
			t.Fatalf("error = %v, want %q", resp["error"], "invalid credentials")
		}
	})

}
