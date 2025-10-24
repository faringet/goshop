package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	model      string
	http       *http.Client
	reqTimeout time.Duration
}

type ChatReq struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  *Options      `json:"options,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

type Options struct {
	NumThread int     `json:"num_thread,omitempty"`
	Temp      float32 `json:"temperature,omitempty"`
	NumCtx    int     `json:"num_ctx,omitempty"`
}

type ChatResp struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

func New(baseURL, model string, clientTimeout, reqTimeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if model == "" {
		model = "qwen2.5:3b-instruct"
	}
	if reqTimeout <= 0 {
		reqTimeout = 60 * time.Second
	}

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 55 * time.Second,
	}

	client := &http.Client{
		Timeout:   clientTimeout,
		Transport: tr,
	}

	return &Client{
		baseURL:    baseURL,
		model:      model,
		http:       client,
		reqTimeout: reqTimeout,
	}
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.reqTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.reqTimeout)
		defer cancel()
	}

	body := ChatReq{
		Model:  c.model,
		Stream: false, // читаем весь ответ разом
		Messages: []ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Options: &Options{
			NumThread: 12,   // кол-во ядер на нашем железе
			Temp:      0.2,  // детерминированнее
			NumCtx:    4096, // контекст
		},
	}

	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama %d: %s", resp.StatusCode, data)
	}

	var cr ChatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	return cr.Message.Content, nil
}
