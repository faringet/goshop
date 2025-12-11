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

const baseURL = "http://localhost:5081"

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type meResponse struct {
	UID       string `json:"uid"`
	Email     string `json:"email"`
	Issuer    string `json:"issuer"`
	Subject   string `json:"subject"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// register -> login -> me -> refresh -> me (с новым access)
func TestUsers_AuthFlow(t *testing.T) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	email := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())
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

	if regResp.Email == "" || regResp.ID == "" {
		t.Fatalf("register: empty email or id in response: %#v", regResp)
	}

	// 2) Логин
	var loginResp1 loginResponse
	doPostJSON(t, client, "/v1/users/login",
		loginRequest{
			Email:    email,
			Password: password,
		},
		http.StatusOK,
		&loginResp1,
	)

	if loginResp1.AccessToken == "" || loginResp1.RefreshToken == "" {
		t.Fatalf("login: tokens are empty: %#v", loginResp1)
	}

	// 3) Me по access_token
	var meResp1 meResponse
	doGetJSONAuth(t, client, "/v1/users/me", loginResp1.AccessToken, http.StatusOK, &meResp1)

	if meResp1.Email != regResp.Email {
		t.Fatalf("me: email = %q, want %q", meResp1.Email, regResp.Email)
	}
	if meResp1.UID == "" {
		t.Fatalf("me: empty uid in response: %#v", meResp1)
	}

	// 4) Refresh по refresh_token
	var refResp refreshResponse
	doPostJSON(t, client, "/v1/users/refresh",
		refreshRequest{
			RefreshToken: loginResp1.RefreshToken,
		},
		http.StatusOK,
		&refResp,
	)

	if refResp.AccessToken == "" || refResp.RefreshToken == "" {
		t.Fatalf("refresh: tokens are empty: %#v", refResp)
	}

	//if refResp.AccessToken == loginResp1.AccessToken {
	//	t.Errorf("refresh: access token was not rotated")
	//}
	if refResp.RefreshToken == loginResp1.RefreshToken {
		t.Errorf("refresh: refresh token was not rotated")
	}

	// 5) Me по НОВОМУ access_token
	var meResp2 meResponse
	doGetJSONAuth(t, client, "/v1/users/me", refResp.AccessToken, http.StatusOK, &meResp2)

	if meResp2.Email != regResp.Email {
		t.Fatalf("me (after refresh): email = %q, want %q", meResp2.Email, regResp.Email)
	}
	if meResp2.UID != meResp1.UID {
		t.Fatalf("me (after refresh): uid changed: %q -> %q", meResp1.UID, meResp2.UID)
	}
}

// doPostJSON шлёт POST с json-телом и декодит ответ, проверяя статус
func doPostJSON(t *testing.T, client *http.Client, path string, body any, wantStatus int, out any) {
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

	// если out == nil — не парсим тело
	if out == nil || len(respBody) == 0 {
		return
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		t.Fatalf("decode response from %s: %v (body=%s)", path, err, string(respBody))
	}
}

// doGetJSONAuth шлёт GET с Authorization: Bearer ... и декодит JSON
func doGetJSONAuth(t *testing.T, client *http.Client, path, accessToken string, wantStatus int, out any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: status=%d, want=%d, body=%s",
			path, resp.StatusCode, wantStatus, string(respBody))
	}

	if out == nil || len(respBody) == 0 {
		return
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		t.Fatalf("decode response from %s: %v (body=%s)", path, err, string(respBody))
	}
}
