package messages

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

func TestMessagesHandler_ClaudeDesktopConnectionTestNonStream(t *testing.T) {
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	router := newMessagesTestRouter(t, config.UpstreamConfig{
		Name:        "connection-test-upstream",
		BaseURL:     upstream.URL,
		APIKeys:     []string{"sk-test"},
		ServiceType: "claude",
		Status:      "active",
	})

	w := performMessagesHandlerRequest(t, router, `{"model":"haiku","max_tokens":1,"messages":[{"role":"user","content":"."}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if got := atomic.LoadInt32(&upstreamCalls); got != 0 {
		t.Fatalf("upstream calls = %d, want 0", got)
	}

	var resp struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if resp.Type != "message" || resp.Role != "assistant" || resp.Model != "haiku" {
		t.Fatalf("unexpected message metadata: %#v", resp)
	}
	if len(resp.Content) != 1 || resp.Content[0].Type != "text" || resp.Content[0].Text != claudeDesktopConnectionTestText {
		t.Fatalf("unexpected content: %#v", resp.Content)
	}
	if resp.StopReason != "end_turn" {
		t.Fatalf("stop_reason = %q, want end_turn", resp.StopReason)
	}
	if resp.Usage.InputTokens != 15 || resp.Usage.OutputTokens != 14 {
		t.Fatalf("usage = input:%d output:%d, want input:15 output:14", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}
}

func TestMessagesHandler_ClaudeDesktopConnectionTestStream(t *testing.T) {
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	router := newMessagesTestRouter(t, config.UpstreamConfig{
		Name:        "connection-test-stream-upstream",
		BaseURL:     upstream.URL,
		APIKeys:     []string{"sk-test"},
		ServiceType: "claude",
		Status:      "active",
	})

	w := performMessagesHandlerRequest(t, router, `{"model":"haiku","max_tokens":1,"stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"."}]}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if got := atomic.LoadInt32(&upstreamCalls); got != 0 {
		t.Fatalf("upstream calls = %d, want 0", got)
	}
	if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}

	body := w.Body.String()
	for _, want := range []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
		`"text":"I"`,
		`"text":" on?"`,
		`"stop_reason":"end_turn"`,
		`"output_tokens":14`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream body missing %q:\n%s", want, body)
		}
	}
}

func TestMessagesHandler_ClaudeDesktopConnectionTestDoesNotInterceptOtherPrompts(t *testing.T) {
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_upstream","type":"message","role":"assistant","content":[{"type":"text","text":"from upstream"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	router := newMessagesTestRouter(t, config.UpstreamConfig{
		Name:        "connection-test-pass-through",
		BaseURL:     upstream.URL,
		APIKeys:     []string{"sk-test"},
		ServiceType: "claude",
		Status:      "active",
	})

	w := performMessagesHandlerRequest(t, router, `{"model":"haiku","max_tokens":1,"messages":[{"role":"user","content":"not a connection test"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if got := atomic.LoadInt32(&upstreamCalls); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
	if !strings.Contains(w.Body.String(), "from upstream") {
		t.Fatalf("expected upstream response, got %s", w.Body.String())
	}
}
