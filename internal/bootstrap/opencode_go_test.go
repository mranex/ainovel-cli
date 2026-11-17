package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/llm"
)

func TestOpenCodeSessionHeaderAndUserAgent(t *testing.T) {
	type captured struct {
		path      string
		sessionID string
		userAgent string
		auth      string
		body      map[string]any
	}
	var (
		mu   sync.Mutex
		reqs []captured
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)

		mu.Lock()
		reqs = append(reqs, captured{
			path:      r.URL.Path,
			sessionID: r.Header.Get("x-opencode-session"),
			userAgent: r.Header.Get("User-Agent"),
			auth:      r.Header.Get("Authorization"),
			body:      decoded,
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","model":"deepseek-v4-pro","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`))
	}))
	defer srv.Close()

	model, err := createOpenCodeModel("opencode-go", "deepseek-v4-pro", ProviderConfig{
		APIKey:    "oc_sk_test",
		BaseURL:   srv.URL + "/inference/go",
		ExtraBody: map[string]any{"temperature": 0.7},
	})
	if err != nil {
		t.Fatalf("createOpenCodeModel: %v", err)
	}

	ctx := context.Background()
	msg1 := []agentcore.Message{agentcore.UserMsg("Chào")}

	// Cùng PromptCacheKey → cùng x-opencode-session qua nhiều lượt
	if _, err := model.Generate(ctx, msg1, nil, agentcore.WithCallPromptCacheKey("book-writer#1")); err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if _, err := model.Generate(ctx, msg1, nil, agentcore.WithCallPromptCacheKey("book-writer#1")); err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	// Khác PromptCacheKey → khác x-opencode-session
	if _, err := model.Generate(ctx, msg1, nil, agentcore.WithCallPromptCacheKey("book-writer#2")); err != nil {
		t.Fatalf("turn 3: %v", err)
	}

	// Đa lượt không dùng PromptCacheKey (như Co-create/Translator) → ổn định theo tin nhắn gốc
	sys := agentcore.SystemMsg("Hệ thống dịch")
	u1 := agentcore.UserMsg("Chương 1")
	a1 := agentcore.Message{Role: agentcore.RoleAssistant, Content: []agentcore.ContentBlock{agentcore.TextBlock("Bản nháp")}}
	u2 := agentcore.UserMsg("Sửa đoạn cuối")
	if _, err := model.Generate(ctx, []agentcore.Message{sys, u1}, nil); err != nil {
		t.Fatalf("stateless turn 1: %v", err)
	}
	if _, err := model.Generate(ctx, []agentcore.Message{sys, u1, a1, u2}, nil); err != nil {
		t.Fatalf("stateless turn 2: %v", err)
	}
	if _, err := model.Generate(ctx, []agentcore.Message{sys, agentcore.UserMsg("Chương 2")}, nil); err != nil {
		t.Fatalf("stateless other conv: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reqs) != 6 {
		t.Fatalf("expected 6 requests, got %d", len(reqs))
	}
	for i, r := range reqs {
		if r.path != "/inference/go/openai/v1/chat/completions" {
			t.Errorf("req[%d] path = %q", i, r.path)
		}
		if !strings.HasPrefix(r.sessionID, "ses_") {
			t.Errorf("req[%d] x-opencode-session = %q, want ses_...", i, r.sessionID)
		}
		if r.userAgent != DefaultOpenCodeUserAgent {
			t.Errorf("req[%d] User-Agent = %q, want %q", i, r.userAgent, DefaultOpenCodeUserAgent)
		}
		if r.auth != "Bearer oc_sk_test" {
			t.Errorf("req[%d] Authorization = %q", i, r.auth)
		}
		if _, leaked := r.body["session_id"]; leaked {
			t.Errorf("req[%d] leaked internal session_id into body", i)
		}
		if _, leaked := r.body["prompt_cache_key"]; leaked {
			t.Errorf("req[%d] leaked internal prompt_cache_key into body", i)
		}
		if temp, _ := r.body["temperature"].(float64); temp != 0.7 {
			t.Errorf("req[%d] extra_body temperature = %v, want 0.7", i, r.body["temperature"])
		}
	}

	if reqs[0].sessionID != reqs[1].sessionID {
		t.Errorf("same PromptCacheKey got different sessions: %q vs %q", reqs[0].sessionID, reqs[1].sessionID)
	}
	if reqs[0].sessionID == reqs[2].sessionID {
		t.Errorf("different PromptCacheKey reused same session: %q", reqs[0].sessionID)
	}
	if reqs[3].sessionID != reqs[4].sessionID {
		t.Errorf("same stateless root conversation got different sessions: %q vs %q", reqs[3].sessionID, reqs[4].sessionID)
	}
	if reqs[3].sessionID == reqs[5].sessionID {
		t.Errorf("different stateless conversations shared session: %q", reqs[3].sessionID)
	}
}

func TestOpenCodeMultiProtocolRouting(t *testing.T) {
	var (
		mu    sync.Mutex
		paths []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("x-opencode-session"), "ses_") {
			http.Error(w, "missing x-opencode-session", http.StatusBadRequest)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/inference/go/openai/v1/chat/completions":
			_, _ = w.Write([]byte(`{"id":"c1","model":"deepseek-v4-pro","choices":[{"index":0,"message":{"role":"assistant","content":"chat-ok"},"finish_reason":"stop"}]}`))
		case "/inference/go/anthropic/v1/messages":
			_, _ = w.Write([]byte(`{"id":"m1","type":"message","role":"assistant","model":"qwen4-max","content":[{"type":"text","text":"anthropic-ok"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`))
		case "/inference/go/openai/v1/responses":
			_, _ = w.Write([]byte(`{"id":"r1","object":"response","status":"completed","model":"gpt-6-luna","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"responses-ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}`))
		case "/inference/go/google/v1beta/models/gemini-4-argon:generateContent":
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"gemini-ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := Config{
		Provider:  "opencode-go",
		ModelName: "deepseek-v4-pro",
		Providers: map[string]ProviderConfig{
			"opencode-go": {
				APIKey:  "oc_sk_multi",
				BaseURL: srv.URL + "/inference/go",
			},
		},
		Roles: map[string]RoleConfig{
			"architect":  {Provider: "opencode-go", Model: "qwen4-max"},
			"writer":     {Provider: "opencode-go", Model: "gpt-6-luna"},
			"translator": {Provider: "opencode-go", Model: "gemini-4-argon"},
		},
	}
	if err := cfg.ValidateBase(); err != nil {
		t.Fatalf("ValidateBase: %v", err)
	}
	if win, _ := cfg.ResolveContextWindow("opencode-go", "deepseek-v4-pro"); win != 1000000 {
		t.Fatalf("ResolveContextWindow(deepseek-v4-pro) = %d, want 1000000", win)
	}

	ms, err := NewModelSet(cfg)
	if err != nil {
		t.Fatalf("NewModelSet: %v", err)
	}

	cases := []struct {
		model        agentcore.ChatModel
		wantText     string
		wantStrict   llm.Support
		wantProvider string
	}{
		{ms.Default, "chat-ok", llm.SupportYes, "opencode-go"},
		{ms.ForRole("architect"), "anthropic-ok", llm.SupportYes, "opencode-go"},
		{ms.ForRole("writer"), "responses-ok", llm.SupportYes, "opencode-go"},
		{ms.ForRole("translator"), "gemini-ok", llm.SupportNo, "opencode-go"},
	}
	for i, tc := range cases {
		if got := ModelProvider(tc.model); got != tc.wantProvider {
			t.Errorf("case[%d] provider = %q, want %q", i, got, tc.wantProvider)
		}
		if cp, ok := tc.model.(llm.CapabilityProvider); ok {
			caps := cp.Capabilities()
			if caps.Structured.JSONSchema != llm.SupportYes || caps.Structured.Strict != tc.wantStrict {
				t.Errorf("case[%d] structured = %+v, want JSONSchema=Yes Strict=%v", i, caps.Structured, tc.wantStrict)
			}
		} else {
			t.Errorf("case[%d] missing CapabilityProvider", i)
		}
		resp, err := tc.model.Generate(context.Background(), []agentcore.Message{agentcore.UserMsg("ping")}, nil)
		if err != nil {
			t.Fatalf("case[%d] Generate: %v", i, err)
		}
		if gotText := messageText(resp.Message); strings.TrimSpace(gotText) != tc.wantText {
			t.Errorf("case[%d] text = %q, want %q", i, gotText, tc.wantText)
		}
	}

	wantPaths := []string{
		"/inference/go/openai/v1/chat/completions",
		"/inference/go/anthropic/v1/messages",
		"/inference/go/openai/v1/responses",
		"/inference/go/google/v1beta/models/gemini-4-argon:generateContent",
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != len(wantPaths) {
		t.Fatalf("paths = %v, want %v", paths, wantPaths)
	}
	for i := range wantPaths {
		if paths[i] != wantPaths[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], wantPaths[i])
		}
	}
}

func TestOpenCodeExtraHeaderAndUserAgentOverride(t *testing.T) {
	var gotSession, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSession = r.Header.Get("x-opencode-session")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","model":"glm-5.3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	model, err := createOpenCodeModel("opencode-go", "glm-5.3", ProviderConfig{
		APIKey:  "oc_sk_override",
		BaseURL: srv.URL + "/inference/go",
		Extra: map[string]any{
			"user_agent": "custom-agent/2.0",
			"headers":    map[string]any{"x-opencode-session": "ses_pinned_custom"},
		},
	})
	if err != nil {
		t.Fatalf("createOpenCodeModel: %v", err)
	}
	if _, err := model.Generate(context.Background(), []agentcore.Message{agentcore.UserMsg("hi")}, nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotSession != "ses_pinned_custom" {
		t.Errorf("x-opencode-session = %q, want ses_pinned_custom", gotSession)
	}
	if gotUA != "custom-agent/2.0" {
		t.Errorf("User-Agent = %q, want custom-agent/2.0", gotUA)
	}
}

func TestOpenCodeNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		providerKey string
		baseURL     string
		model       string
		wantBase    string
	}{
		{"opencode-go", "", "deepseek-v4-pro", "https://opencode.ai/inference/go/openai/v1"},
		{"opencode", "", "qwen4-max", "https://opencode.ai/inference/anthropic"},
		{"opencode-go", "https://opencode.ai/zen/go/v1", "gemini-4-argon", "https://opencode.ai/inference/go/google"},
		{"opencode-go", "https://opencode.ai/inference/go/openai/v1", "gpt-6-luna", "https://opencode.ai/inference/go/openai/v1"},
		{"opencode-go", "https://opencode.ai/zen/v1", "deepseek-v4-pro", "https://opencode.ai/inference/openai/v1"},
	}
	for _, tc := range cases {
		route := resolveOpenCodeRoute(tc.providerKey, tc.model, ProviderConfig{BaseURL: tc.baseURL})
		if route.baseURL != tc.wantBase {
			t.Errorf("resolveOpenCodeRoute(%q, %q, %q).baseURL = %q, want %q", tc.providerKey, tc.model, tc.baseURL, route.baseURL, tc.wantBase)
		}
	}
}

func messageText(msg agentcore.Message) string {
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == agentcore.ContentText {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}
