//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutAllResponse struct {
	Revoked int `json:"revoked"`
}

// Logout по refresh_token инвалидирует сессию,
// а дальнейший refresh этим токеном даёт 401 invalid or expired session.
func TestUsers_Logout_RevokesSession(t *testing.T) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	email := fmt.Sprintf("logout_%d@example.com", time.Now().UnixNano())
	password := "StrongPass123!"

	// 1) Регистрация
	var regResp registerResponse
	doPostJSON(t, client, "/v1/users/register",
		registerRequest{
			Email:    email,
			Password: password,
		},
		http.StatusCreated,
		&regResp,
	)

	// 2) Логин -> получаем refresh_token
	var loginResp loginResponse
	doPostJSON(t, client, "/v1/users/login",
		loginRequest{
			Email:    email,
			Password: password,
		},
		http.StatusOK,
		&loginResp,
	)

	if loginResp.RefreshToken == "" {
		t.Fatalf("login: empty refresh_token: %#v", loginResp)
	}

	// 3) Logout по refresh_token
	doPostJSON(t, client, "/v1/users/logout",
		logoutRequest{
			RefreshToken: loginResp.RefreshToken,
		},
		http.StatusNoContent,
		nil,
	)

	// 4) Попытка refresh тем же токеном -> 401 invalid or expired session
	errBody := doPostJSONError(t, client, "/v1/users/refresh",
		refreshRequest{
			RefreshToken: loginResp.RefreshToken,
		},
		http.StatusUnauthorized,
	)

	if errBody["error"] != "invalid or expired session" {
		t.Fatalf("refresh after logout: expected error=%q, got %v",
			"invalid or expired session", errBody["error"])
	}
}

// LogoutAll по access_token отзывает все активные сессии пользователя.
// После этого все имеющиеся refresh_token'ы больше не работают.
func TestUsers_LogoutAll_RevokesAllSessions(t *testing.T) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	email := fmt.Sprintf("logoutall_%d@example.com", time.Now().UnixNano())
	password := "StrongPass123!"

	// 1) Регистрация
	var regResp registerResponse
	doPostJSON(t, client, "/v1/users/register",
		registerRequest{
			Email:    email,
			Password: password,
		},
		http.StatusCreated,
		&regResp,
	)

	// 2) Первый логин -> сессия 1
	var loginResp1 loginResponse
	doPostJSON(t, client, "/v1/users/login",
		loginRequest{
			Email:    email,
			Password: password,
		},
		http.StatusOK,
		&loginResp1,
	)

	// 3) Второй логин -> сессия 2 (тот же user, другой refresh)
	var loginResp2 loginResponse
	doPostJSON(t, client, "/v1/users/login",
		loginRequest{
			Email:    email,
			Password: password,
		},
		http.StatusOK,
		&loginResp2,
	)

	if loginResp1.RefreshToken == "" || loginResp2.RefreshToken == "" {
		t.Fatalf("login: empty refresh tokens: 1=%#v 2=%#v", loginResp1, loginResp2)
	}

	// 4) LogoutAll по access_token из первой сессии
	var laResp logoutAllResponse
	doPostJSONAuth(t, client, "/v1/users/logout_all",
		loginResp1.AccessToken,
		struct{}{}, // тело не используется хендлером
		http.StatusOK,
		&laResp,
	)

	if laResp.Revoked != 2 {
		t.Fatalf("logout_all: revoked=%d, want 2", laResp.Revoked)
	}

	// 5) Оба refresh-токена больше не должны работать
	errBody1 := doPostJSONError(t, client, "/v1/users/refresh",
		refreshRequest{
			RefreshToken: loginResp1.RefreshToken,
		},
		http.StatusUnauthorized,
	)
	if errBody1["error"] != "invalid or expired session" {
		t.Fatalf("refresh1 after logout_all: expected error=%q, got %v",
			"invalid or expired session", errBody1["error"])
	}

	errBody2 := doPostJSONError(t, client, "/v1/users/refresh",
		refreshRequest{
			RefreshToken: loginResp2.RefreshToken,
		},
		http.StatusUnauthorized,
	)
	if errBody2["error"] != "invalid or expired session" {
		t.Fatalf("refresh2 after logout_all: expected error=%q, got %v",
			"invalid or expired session", errBody2["error"])
	}
}

// doPostJSONAuth — POST с JSON-телом и Authorization: Bearer <token>
func doPostJSONAuth(t *testing.T, client *http.Client, path, accessToken string, body any, wantStatus int, out any) {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body for %s: %v", path, err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST %s: status=%d, want=%d, body=%s",
			path, resp.StatusCode, wantStatus, string(respBody))
	}

	if out == nil || len(respBody) == 0 {
		return
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		t.Fatalf("decode response from %s: %v (body=%s)", path, err, string(respBody))
	}
}

// doPostJSONError — POST, ожидаем ошибочный статус + JSON {"error": "..."}.
func doPostJSONError(t *testing.T, client *http.Client, path string, body any, wantStatus int) map[string]any {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body for %s: %v", path, err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST %s: status=%d, want=%d, body=%s",
			path, resp.StatusCode, wantStatus, string(respBody))
	}

	if len(respBody) == 0 {
		return map[string]any{}
	}

	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		t.Fatalf("decode error response from %s: %v (body=%s)", path, err, string(respBody))
	}
	return m
}
