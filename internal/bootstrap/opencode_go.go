package bootstrap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/llm"
	"github.com/voocel/ainovel-cli/internal/errs"
	"github.com/voocel/ainovel-cli/internal/models"
	"github.com/voocel/litellm"
	"github.com/voocel/litellm/provider/anthropic"
	"github.com/voocel/litellm/provider/compat"
	"github.com/voocel/litellm/provider/gemini"
	"github.com/voocel/litellm/provider/openai"
)

const (
	DefaultOpenCodeGoBaseURL = "https://opencode.ai/inference/go"
	DefaultOpenCodeBaseURL   = "https://opencode.ai/inference"
	DefaultOpenCodeUserAgent = "ainovel-cli/1.0"
	openCodeSessionHeader    = "x-opencode-session"
)

type openCodeSessionCtxKey struct{}

var openCodeProcessSalt = randomOpenCodeHex(8)

func randomOpenCodeHex(nBytes int) string {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf)
}

func withOpenCodeSession(ctx context.Context, sessionID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, openCodeSessionCtxKey{}, sessionID)
}

func openCodeSessionFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(openCodeSessionCtxKey{}).(string)
	return v
}

// IsOpenCodeProvider báo true nếu tên provider hoặc type là opencode-go / opencode.
func IsOpenCodeProvider(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "opencode-go", "opencode":
		return true
	default:
		return false
	}
}

func isOpenCodeEndpoint(providerKey string, pc ProviderConfig) bool {
	if IsOpenCodeProvider(providerKey) || IsOpenCodeProvider(pc.Type) {
		return true
	}
	raw := strings.TrimSpace(pc.BaseURL)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "opencode.ai")
}

func resolveOpenCodeAPIKey(providerKey string, pc ProviderConfig) string {
	if key := strings.TrimSpace(pc.APIKey); key != "" {
		return key
	}
	if IsOpenCodeProvider(providerKey) || IsOpenCodeProvider(pc.Type) {
		return strings.TrimSpace(os.Getenv("OPENCODE_API_KEY"))
	}
	return ""
}

// deriveOpenCodeSessionID tạo ID session ổn định cho mỗi hội thoại (`ses_...`) để đáp ứng
// bắt buộc `x-opencode-session` và tối ưu routing/prompt cache trên OpenCode Go.
func deriveOpenCodeSessionID(messages []agentcore.Message, opts []agentcore.CallOption) string {
	callCfg := agentcore.ResolveCallConfig(opts)
	seed := strings.TrimSpace(callCfg.SessionID)
	if seed == "" {
		seed = strings.TrimSpace(callCfg.PromptCacheKey)
	}
	if seed == "" {
		var root strings.Builder
		const maxRootBytes = 4096
		tookFirstNonSystem := false
		for _, msg := range messages {
			if msg.Role != agentcore.RoleSystem {
				if tookFirstNonSystem {
					continue
				}
				tookFirstNonSystem = true
			}
			for _, block := range msg.Content {
				if block.Type != agentcore.ContentText || block.Text == "" {
					continue
				}
				root.WriteString(string(msg.Role))
				root.WriteByte(':')
				remain := maxRootBytes - root.Len()
				if remain <= 0 {
					break
				}
				if len(block.Text) > remain {
					root.WriteString(block.Text[:remain])
				} else {
					root.WriteString(block.Text)
				}
				root.WriteByte('\n')
			}
			if root.Len() >= maxRootBytes {
				break
			}
		}
		if root.Len() > 0 {
			seed = "conv:" + root.String()
		}
	}
	if seed == "" {
		return "ses_" + randomOpenCodeHex(12)
	}
	sum := sha256.Sum256([]byte(openCodeProcessSalt + "|" + seed))
	return "ses_" + hex.EncodeToString(sum[:12])
}

type openCodeTransport struct {
	base      http.RoundTripper
	userAgent string
	apiKey    string
	headers   map[string]string
}

func (t *openCodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	ua := strings.TrimSpace(t.userAgent)
	if ua == "" {
		ua = DefaultOpenCodeUserAgent
	}
	cloned.Header.Set("User-Agent", ua)
	if t.apiKey != "" &&
		cloned.Header.Get("Authorization") == "" &&
		cloned.Header.Get("x-api-key") == "" &&
		cloned.Header.Get("x-goog-api-key") == "" {
		cloned.Header.Set("Authorization", "Bearer "+t.apiKey)
	}
	for k, v := range t.headers {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		cloned.Header.Set(k, v)
	}
	if cloned.Header.Get(openCodeSessionHeader) == "" {
		sid := openCodeSessionFromContext(cloned.Context())
		if sid == "" {
			sid = "ses_" + randomOpenCodeHex(12)
		}
		cloned.Header.Set(openCodeSessionHeader, sid)
	}
	return base.RoundTrip(cloned)
}

type openCodeRoute struct {
	wireModel string
	spec      models.OpenCodeModelSpec
	protocol  models.OpenCodeProtocol
	baseURL   string
}

func resolveOpenCodeRoute(providerKey, model string, pc ProviderConfig) openCodeRoute {
	wireModel := models.StripOpenCodeModelPrefix(model)
	spec := models.ResolveOpenCodeModelSpec(wireModel)

	protocol := spec.Protocol
	switch strings.ToLower(strings.TrimSpace(pc.Type)) {
	case "anthropic":
		protocol = models.OpenCodeProtoMessages
	case "gemini":
		protocol = models.OpenCodeProtoGemini
	case "openai":
		if strings.EqualFold(strings.TrimSpace(pc.API), openai.APIResponses) {
			protocol = models.OpenCodeProtoResponses
		} else {
			protocol = models.OpenCodeProtoChat
		}
	default:
		if strings.EqualFold(strings.TrimSpace(pc.API), openai.APIResponses) {
			protocol = models.OpenCodeProtoResponses
		} else if strings.EqualFold(strings.TrimSpace(pc.API), openai.APIChat) {
			protocol = models.OpenCodeProtoChat
		}
	}

	root := normalizeOpenCodeRoot(providerKey, pc)
	var baseURL string
	switch protocol {
	case models.OpenCodeProtoMessages:
		baseURL = root + "/anthropic"
	case models.OpenCodeProtoGemini:
		baseURL = root + "/google"
	default:
		baseURL = root + "/openai/v1"
	}
	return openCodeRoute{
		wireModel: wireModel,
		spec:      spec,
		protocol:  protocol,
		baseURL:   baseURL,
	}
}

func normalizeOpenCodeRoot(providerKey string, pc ProviderConfig) string {
	raw := strings.TrimSpace(pc.BaseURL)
	if raw == "" {
		if strings.EqualFold(strings.TrimSpace(providerKey), "opencode") ||
			strings.EqualFold(strings.TrimSpace(pc.Type), "opencode") {
			return DefaultOpenCodeBaseURL
		}
		return DefaultOpenCodeGoBaseURL
	}
	trimmed := strings.TrimRight(raw, "/")
	for _, suffix := range []string{
		"/openai/v1",
		"/openai",
		"/anthropic/v1",
		"/anthropic",
		"/google/v1beta",
		"/google",
		"/v1beta",
		"/v1",
	} {
		if strings.HasSuffix(strings.ToLower(trimmed), suffix) {
			trimmed = strings.TrimRight(trimmed[:len(trimmed)-len(suffix)], "/")
			break
		}
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && strings.EqualFold(parsed.Hostname(), "opencode.ai") {
		p := strings.TrimRight(parsed.Path, "/")
		switch strings.ToLower(p) {
		case "/zen/go":
			parsed.Path = "/inference/go"
		case "/zen":
			parsed.Path = "/inference"
		}
		return strings.TrimRight(parsed.String(), "/")
	}
	return trimmed
}

func parseOpenCodeExtraHeaders(extra map[string]any) (map[string]string, error) {
	if len(extra) == 0 {
		return nil, nil
	}
	raw, ok := extra["headers"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch h := raw.(type) {
	case map[string]string:
		return maps.Clone(h), nil
	case map[string]any:
		out := make(map[string]string, len(h))
		for k, v := range h {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("opencode-go: header %q phải là chuỗi", k)
			}
			out[k] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("opencode-go: extra.headers phải là map[string]string")
	}
}

func createOpenCodeModel(providerKey, model string, pc ProviderConfig) (agentcore.ChatModel, error) {
	apiKey := resolveOpenCodeAPIKey(providerKey, pc)
	if apiKey == "" {
		return nil, fmt.Errorf("provider %s: chưa cấu hình API Key (hoặc biến môi trường OPENCODE_API_KEY): %w", providerKey, errs.ErrConfig)
	}
	streamIdle, err := pc.StreamIdleTimeoutValue()
	if err != nil {
		return nil, fmt.Errorf("provider %s stream_idle_timeout: %w: %w", providerKey, errs.ErrConfig, err)
	}
	extraHeaders, err := parseOpenCodeExtraHeaders(pc.Extra)
	if err != nil {
		return nil, fmt.Errorf("provider %s: %w: %w", providerKey, errs.ErrConfig, err)
	}
	ua, _ := pc.Extra["user_agent"].(string)
	ua = strings.TrimSpace(ua)
	if ua == "" {
		ua = DefaultOpenCodeUserAgent
	}

	route := resolveOpenCodeRoute(providerKey, model, pc)
	transport := &openCodeTransport{
		base:      http.DefaultTransport,
		userAgent: ua,
		apiKey:    apiKey,
		headers:   extraHeaders,
	}

	var providerImpl litellm.Provider
	switch route.protocol {
	case models.OpenCodeProtoMessages:
		beta, _ := pc.Extra["anthropic_beta"].(string)
		providerImpl, err = anthropic.New(anthropic.Config{
			APIKey:    apiKey,
			BaseURL:   route.baseURL,
			Transport: transport,
			Beta:      beta,
			UserAgent: ua,
			Headers:   extraHeaders,
		})
	case models.OpenCodeProtoGemini:
		providerImpl, err = gemini.New(gemini.Config{
			APIKey:    apiKey,
			BaseURL:   route.baseURL,
			Transport: transport,
		})
	case models.OpenCodeProtoResponses:
		providerImpl, err = openai.New(openai.Config{
			API:               openai.APIResponses,
			APIKey:            apiKey,
			BaseURL:           route.baseURL,
			Transport:         transport,
			UserAgent:         ua,
			Headers:           extraHeaders,
			PromptCacheParams: boolFromExtra(pc.Extra, "prompt_cache_params"),
		})
	default:
		providerImpl, err = compat.New(compat.Config{
			APIKey:                      apiKey,
			BaseURL:                     route.baseURL,
			Transport:                   transport,
			UserAgent:                   ua,
			Headers:                     extraHeaders,
			AllowUnknownProviderOptions: true,
		}, buildOpenCodeChatSpec(route.baseURL, route.spec, cloneMap(pc.ExtraBody)))
	}
	if err != nil {
		return nil, fmt.Errorf("provider %s (opencode %s): %w: %w", providerKey, route.protocol, errs.ErrProvider, err)
	}

	client, err := litellm.New(
		providerImpl,
		litellm.WithStreamIdleTimeout(streamIdle),
		litellm.WithMessageRepair(litellm.RepairNormalizeToolUseIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("provider %s client: %w: %w", providerKey, errs.ErrProvider, err)
	}

	adapter := llm.NewLiteLLMAdapter(route.wireModel, client)
	return &openCodeModel{
		LiteLLMAdapter:    adapter,
		providerKey:       providerKey,
		modelName:         model,
		protocol:          route.protocol,
		spec:              route.spec,
		stripCacheControl: route.protocol != models.OpenCodeProtoMessages,
	}, nil
}

func buildOpenCodeChatSpec(baseURL string, spec models.OpenCodeModelSpec, extraBody map[string]any) compat.Spec {
	return compat.Spec{
		Name: "opencode-go",
		Endpoint: compat.EndpointSpec{
			BaseURL: baseURL,
		},
		Auth: compat.AuthSpec{APIKeyRequired: true},
		Request: compat.RequestSpec{
			SupportsJSONSchema:          spec.StructuredOutput,
			AllowUnknownProviderOptions: true,
			Thinking: func(thinking *litellm.Thinking, _ string) (map[string]any, error) {
				if !spec.ReasoningSupported || thinking == nil || thinking.Mode == litellm.ThinkingUnspecified {
					return nil, nil
				}
				if thinking.Mode == litellm.ThinkingDisabled {
					if slices.Contains(spec.ReasoningEfforts, "none") {
						return map[string]any{"reasoning_effort": "none"}, nil
					}
					return nil, nil
				}
				effort := clampOpenCodeEffort(thinking.Effort, spec.ReasoningEfforts)
				if effort == "" {
					return nil, nil
				}
				return map[string]any{"reasoning_effort": effort}, nil
			},
			ProviderOptions: func(options litellm.ProviderOptions, body map[string]any, _ *litellm.Request) error {
				for k, v := range extraBody {
					if _, exists := body[k]; !exists {
						body[k] = v
					}
				}
				for k, v := range options {
					switch k {
					case "session_id", "prompt_cache_key":
						// Hai key nội bộ của agentcore dùng để tính header x-opencode-session, không ghi vào body.
						continue
					}
					if _, exists := body[k]; !exists {
						body[k] = v
					}
				}
				return nil
			},
		},
		Response: compat.ResponseSpec{
			ModelFromResponse:         true,
			ContentAsInterface:        true,
			ReasoningFields:           []string{"reasoning_content", "reasoning_details", "reasoning"},
			HasCompletionTokenDetails: true,
			HasCacheTokens:            true,
		},
		Stream: compat.StreamSpec{
			ReasoningFields: []string{"reasoning_content", "reasoning_details", "reasoning"},
		},
		Features: compat.FeatureSpec{
			StrictTools: compat.StrictToolsForward,
		},
		Capabilities: func(_ string, caps litellm.Capabilities) litellm.Capabilities {
			if spec.StructuredOutput {
				caps.Structured.JSONSchema = litellm.SupportYes
				caps.Structured.Strict = litellm.SupportYes
			} else {
				caps.Structured.JSONSchema = litellm.SupportNo
				caps.Structured.Strict = litellm.SupportNo
			}
			if spec.ReasoningSupported {
				caps.Thinking.Supported = litellm.SupportYes
				caps.Thinking.Efforts = openCodeThinkingEfforts(spec.ReasoningEfforts)
				if slices.Contains(spec.ReasoningEfforts, "none") {
					caps.Thinking.Disable = litellm.SupportYes
				} else {
					caps.Thinking.Disable = litellm.SupportNo
				}
			} else {
				caps.Thinking.Supported = litellm.SupportNo
				caps.Thinking.Disable = litellm.SupportNo
				caps.Thinking.Efforts = nil
			}
			return caps
		},
	}
}

func clampOpenCodeEffort(effort string, allowed []string) string {
	if len(allowed) == 0 {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(effort))
	if v == "" {
		return ""
	}
	if slices.Contains(allowed, v) {
		return v
	}
	order := []string{"minimal", "low", "medium", "high", "xhigh", "max"}
	idx := slices.Index(order, v)
	if idx < 0 {
		return ""
	}
	// Tìm mức gần nhất ưu tiên hạ xuống nhẹ hơn, nếu không có thì nâng lên mức gần nhất.
	for i := idx - 1; i >= 0; i-- {
		if slices.Contains(allowed, order[i]) {
			return order[i]
		}
	}
	for i := idx + 1; i < len(order); i++ {
		if slices.Contains(allowed, order[i]) {
			return order[i]
		}
	}
	return ""
}

func openCodeThinkingEfforts(efforts []string) []string {
	out := make([]string, 0, len(efforts))
	for _, e := range efforts {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" || e == "none" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func boolFromExtra(extra map[string]any, key string) bool {
	if len(extra) == 0 {
		return false
	}
	switch v := extra[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

type openCodeModel struct {
	*llm.LiteLLMAdapter
	providerKey       string
	modelName         string
	protocol          models.OpenCodeProtocol
	spec              models.OpenCodeModelSpec
	stripCacheControl bool
}

func (m *openCodeModel) ProviderName() string {
	return m.providerKey
}

func (m *openCodeModel) Info() llm.ModelInfo {
	info := m.LiteLLMAdapter.Info()
	info.Provider = m.providerKey
	info.Name = m.modelName
	return info
}

func (m *openCodeModel) Capabilities() llm.Capabilities {
	caps := m.LiteLLMAdapter.Capabilities()
	caps.Provider = m.providerKey
	caps.Model = m.modelName
	caps.ProviderBaseline = false

	if m.spec.StructuredOutput {
		caps.Structured.JSONSchema = llm.SupportYes
		if m.protocol == models.OpenCodeProtoGemini {
			caps.Structured.Strict = llm.SupportNo
		} else {
			caps.Structured.Strict = llm.SupportYes
		}
	} else {
		caps.Structured.JSONSchema = llm.SupportNo
		caps.Structured.Strict = llm.SupportNo
	}

	if m.spec.ReasoningSupported {
		caps.Thinking.Supported = llm.SupportYes
		efforts := openCodeThinkingEfforts(m.spec.ReasoningEfforts)
		levels := make([]agentcore.ThinkingLevel, 0, len(efforts))
		for _, e := range efforts {
			levels = append(levels, agentcore.ThinkingLevel(e))
		}
		caps.Thinking.Efforts = levels
		if slices.Contains(m.spec.ReasoningEfforts, "none") || m.protocol == models.OpenCodeProtoMessages {
			caps.Thinking.Disable = llm.SupportYes
		} else {
			caps.Thinking.Disable = llm.SupportNo
		}
	} else {
		caps.Thinking.Supported = llm.SupportNo
		caps.Thinking.Disable = llm.SupportNo
		caps.Thinking.Efforts = nil
	}
	return caps
}

func (m *openCodeModel) Generate(ctx context.Context, messages []agentcore.Message, tools []agentcore.ToolSpec, opts ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	sid := deriveOpenCodeSessionID(messages, opts)
	ctx = withOpenCodeSession(ctx, sid)
	if m.stripCacheControl {
		messages = withoutCacheControl(messages)
	}
	return m.LiteLLMAdapter.Generate(ctx, messages, tools, opts...)
}

func (m *openCodeModel) GenerateStream(ctx context.Context, messages []agentcore.Message, tools []agentcore.ToolSpec, opts ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	sid := deriveOpenCodeSessionID(messages, opts)
	ctx = withOpenCodeSession(ctx, sid)
	if m.stripCacheControl {
		messages = withoutCacheControl(messages)
	}
	return m.LiteLLMAdapter.GenerateStream(ctx, messages, tools, opts...)
}
