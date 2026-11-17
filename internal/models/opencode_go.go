package models

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	openCodeGoModelsURL   = "https://opencode.ai/inference/go/v1/models"
	openCodeCacheFileName = "opencode-go-models-cache.json"
)

// OpenCodeProtocol xác định giao thức HTTP mà OpenCode Go khuyến nghị cho từng model.
type OpenCodeProtocol string

const (
	OpenCodeProtoChat      OpenCodeProtocol = "chat-completions"
	OpenCodeProtoResponses OpenCodeProtocol = "responses"
	OpenCodeProtoMessages  OpenCodeProtocol = "messages"
	OpenCodeProtoGemini    OpenCodeProtocol = "generate-content"
)

// OpenCodeModelSpec lưu thông số định tuyến, khả năng reasoning/JSON schema và chi phí cho OpenCode Go.
type OpenCodeModelSpec struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	ContextWindow      int              `json:"context_window"`
	MaxTokens          int              `json:"max_tokens"`
	InputCostPer1M     float64          `json:"input_cost_per_1m,omitempty"`
	OutputCostPer1M    float64          `json:"output_cost_per_1m,omitempty"`
	CacheReadCostPer1M float64          `json:"cache_read_cost_per_1m,omitempty"`
	Protocol           OpenCodeProtocol `json:"protocol"`
	StructuredOutput   bool             `json:"structured_output"`
	ReasoningSupported bool             `json:"reasoning_supported"`
	ReasoningEfforts   []string         `json:"reasoning_efforts,omitempty"`
}

func (s OpenCodeModelSpec) toModelEntry() ModelEntry {
	return ModelEntry{
		Provider:           "opencode-go",
		ID:                 s.ID,
		Name:               s.Name,
		ContextWindow:      s.ContextWindow,
		MaxTokens:          s.MaxTokens,
		InputCostPer1M:     s.InputCostPer1M,
		OutputCostPer1M:    s.OutputCostPer1M,
		CacheReadCostPer1M: s.CacheReadCostPer1M,
	}
}

var defaultOpenCodeGoSpecs = []OpenCodeModelSpec{
	// OpenAI Chat Completions (/openai/v1/chat/completions)
	{ID: "deepseek-v4.1-flash", Name: "DeepSeek V4.1 Flash", ContextWindow: 1000000, MaxTokens: 384000, InputCostPer1M: 0.15, OutputCostPer1M: 0.6, CacheReadCostPer1M: 0.003, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "high", "max"}},
	{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", ContextWindow: 1000000, MaxTokens: 384000, InputCostPer1M: 0.66, OutputCostPer1M: 1.98, CacheReadCostPer1M: 0.022, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"high", "max"}},
	{ID: "glm-5.3", Name: "GLM-5.3", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 1.4, OutputCostPer1M: 4.4, CacheReadCostPer1M: 0.26, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "high", "max"}},
	{ID: "glm-5.2", Name: "GLM-5.2", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 1.4, OutputCostPer1M: 4.4, CacheReadCostPer1M: 0.26, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"high", "max"}},
	{ID: "glm-5.3-flash", Name: "GLM-5.3-Flash", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 0.15, OutputCostPer1M: 0.5, CacheReadCostPer1M: 0.03, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "high", "max"}},
	{ID: "kimi-k3.1-pro", Name: "Kimi K3.1 Pro", ContextWindow: 1048576, MaxTokens: 1048576, InputCostPer1M: 2, OutputCostPer1M: 10, CacheReadCostPer1M: 0.2, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "high", "max"}},
	{ID: "kimi-k3", Name: "Kimi K3", ContextWindow: 1048576, MaxTokens: 1048576, InputCostPer1M: 3, OutputCostPer1M: 15, CacheReadCostPer1M: 0.3, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"max"}},
	{ID: "kimi-k2.7-code", Name: "Kimi K2.7 Code", ContextWindow: 262144, MaxTokens: 262144, InputCostPer1M: 0.95, OutputCostPer1M: 4, CacheReadCostPer1M: 0.19, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true},
	{ID: "mimo-v2.6-pro", Name: "MiMo-V2.6-Pro", ContextWindow: 1048576, MaxTokens: 131072, InputCostPer1M: 0.435, OutputCostPer1M: 0.87, CacheReadCostPer1M: 0.003625, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true},
	{ID: "mimo-v2.6-flash", Name: "MiMo-V2.6-Flash", ContextWindow: 1048576, MaxTokens: 131072, InputCostPer1M: 0.14, OutputCostPer1M: 0.28, CacheReadCostPer1M: 0.0028, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true},
	{ID: "minimax-m3.1-flash", Name: "MiniMax M3.1 Flash", ContextWindow: 1000000, MaxTokens: 512000, InputCostPer1M: 0.15, OutputCostPer1M: 0.6, CacheReadCostPer1M: 0.015, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{ID: "hy4", Name: "Hy4", ContextWindow: 1048576, MaxTokens: 65536, InputCostPer1M: 0.834, OutputCostPer1M: 2.501, CacheReadCostPer1M: 0.042, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true, ReasoningEfforts: []string{"none", "high"}},
	{ID: "hy4-preview", Name: "Hy4 preview", ContextWindow: 1024000, MaxTokens: 64000, InputCostPer1M: 0.834, OutputCostPer1M: 2.501, CacheReadCostPer1M: 0.042, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true, ReasoningEfforts: []string{"none", "high"}},
	{ID: "hy4-flash-free", Name: "Hy4 flash Free", ContextWindow: 1048576, MaxTokens: 65536, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true, ReasoningEfforts: []string{"none", "high"}},
	{ID: "hy3", Name: "Hy3", ContextWindow: 256000, MaxTokens: 128000, InputCostPer1M: 0.14, OutputCostPer1M: 0.58, CacheReadCostPer1M: 0.035, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true, ReasoningEfforts: []string{"none", "low", "high"}},
	{ID: "longcat-2.0", Name: "LongCat-2.0", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 0.3, OutputCostPer1M: 1.2, CacheReadCostPer1M: 0.006, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true},
	{ID: "longcat-2.5-free", Name: "LongCat 2.5 Free", ContextWindow: 1000000, MaxTokens: 262144, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true},
	{ID: "step-5-preview", Name: "Step 5 Preview", ContextWindow: 1000000, MaxTokens: 65536, InputCostPer1M: 1, OutputCostPer1M: 2.7, CacheReadCostPer1M: 0.1, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "high"}},
	{ID: "beam-free", Name: "Beam Free", ContextWindow: 1048576, MaxTokens: 131072, Protocol: OpenCodeProtoChat, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max"}},
	{ID: "iceman-contributor-free", Name: "Iceman Contributor Free", ContextWindow: 262144, MaxTokens: 131072, Protocol: OpenCodeProtoChat, StructuredOutput: false, ReasoningSupported: true},

	// Anthropic Messages (/anthropic/v1/messages)
	{ID: "qwen4-max", Name: "Qwen4 Max", ContextWindow: 1000000, MaxTokens: 262144, InputCostPer1M: 1.5, OutputCostPer1M: 4.5, CacheReadCostPer1M: 0.075, Protocol: OpenCodeProtoMessages, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "high", "xhigh"}},
	{ID: "qwen3.8-max", Name: "Qwen3.8 Max", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 2, OutputCostPer1M: 6, CacheReadCostPer1M: 0.25, Protocol: OpenCodeProtoMessages, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "xhigh"}},
	{ID: "qwen3.8-flash", Name: "Qwen3.8 Flash", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 0.15, OutputCostPer1M: 0.47, CacheReadCostPer1M: 0.016, Protocol: OpenCodeProtoMessages, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "xhigh"}},
	{ID: "qwen3.7-plus", Name: "Qwen3.7 Plus", ContextWindow: 1000000, MaxTokens: 65536, InputCostPer1M: 0.4, OutputCostPer1M: 1.6, CacheReadCostPer1M: 0.04, Protocol: OpenCodeProtoMessages, StructuredOutput: false, ReasoningSupported: true},
	{ID: "minimax-m3", Name: "MiniMax-M3", ContextWindow: 1000000, MaxTokens: 131072, InputCostPer1M: 0.3, OutputCostPer1M: 1.2, CacheReadCostPer1M: 0.06, Protocol: OpenCodeProtoMessages, StructuredOutput: false, ReasoningSupported: true},
	{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", ContextWindow: 1000000, MaxTokens: 128000, InputCostPer1M: 0.1, OutputCostPer1M: 0.5, CacheReadCostPer1M: 0.01, Protocol: OpenCodeProtoMessages, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max"}},

	// OpenAI Responses (/openai/v1/responses)
	{ID: "gpt-6-luna", Name: "GPT-6 Luna", ContextWindow: 1050000, MaxTokens: 128000, InputCostPer1M: 0.1, OutputCostPer1M: 0.5, CacheReadCostPer1M: 0.01, Protocol: OpenCodeProtoResponses, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	{ID: "gpt-5.6-luna", Name: "GPT-5.6 Luna", ContextWindow: 1050000, MaxTokens: 128000, InputCostPer1M: 0.2, OutputCostPer1M: 1.2, CacheReadCostPer1M: 0.02, Protocol: OpenCodeProtoResponses, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"none", "low", "medium", "high", "xhigh", "max"}},
	{ID: "grok-4.7", Name: "Grok 4.7", ContextWindow: 500000, MaxTokens: 500000, InputCostPer1M: 2, OutputCostPer1M: 6, CacheReadCostPer1M: 0.5, Protocol: OpenCodeProtoResponses, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "high", "xhigh"}},
	{ID: "muse-spark-1.3-contributor", Name: "Muse Spark 1.3 Contributor", ContextWindow: 1048576, MaxTokens: 131072, InputCostPer1M: 0.1, OutputCostPer1M: 0.2, CacheReadCostPer1M: 0.002, Protocol: OpenCodeProtoResponses, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"minimal", "low", "medium", "high", "xhigh"}},
	{ID: "muse-spark-1.2-contributor", Name: "Muse Spark 1.2 Contributor", ContextWindow: 1048576, MaxTokens: 131072, InputCostPer1M: 0.1, OutputCostPer1M: 0.2, CacheReadCostPer1M: 0.002, Protocol: OpenCodeProtoResponses, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"minimal", "low", "medium", "high", "xhigh"}},

	// Google Gemini (/google/v1beta/models/<model>:generateContent)
	{ID: "gemini-4-argon", Name: "Gemini 4 Argon", ContextWindow: 1048576, MaxTokens: 65536, InputCostPer1M: 2, OutputCostPer1M: 10, CacheReadCostPer1M: 0.1, Protocol: OpenCodeProtoGemini, StructuredOutput: true, ReasoningSupported: true, ReasoningEfforts: []string{"low", "medium", "high"}},
}

var (
	openCodeSpecMu  sync.RWMutex
	openCodeSpecMap = initOpenCodeSpecMap()
)

func initOpenCodeSpecMap() map[string]OpenCodeModelSpec {
	m := make(map[string]OpenCodeModelSpec, len(defaultOpenCodeGoSpecs))
	for _, spec := range defaultOpenCodeGoSpecs {
		m[strings.ToLower(spec.ID)] = spec
	}
	return m
}

func mergeOpenCodeSpecs(specs []OpenCodeModelSpec) {
	if len(specs) == 0 {
		return
	}
	openCodeSpecMu.Lock()
	defer openCodeSpecMu.Unlock()
	for _, spec := range specs {
		id := strings.ToLower(strings.TrimSpace(spec.ID))
		if id == "" {
			continue
		}
		if spec.Protocol == "" {
			spec.Protocol = OpenCodeProtoChat
		}
		openCodeSpecMap[id] = spec
	}
}

// StripOpenCodeModelPrefix loại bỏ tiền tố "opencode-go/" hoặc "opencode/" nếu có.
func StripOpenCodeModelPrefix(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"opencode-go/", "opencode/"} {
		if strings.HasPrefix(lower, prefix) {
			return trimmed[len(prefix):]
		}
	}
	return trimmed
}

// ResolveOpenCodeModelSpec trả về cấu hình giao thức và năng lực cho một model của OpenCode Go.
func ResolveOpenCodeModelSpec(modelID string) OpenCodeModelSpec {
	bare := StripOpenCodeModelPrefix(modelID)
	key := strings.ToLower(bare)

	openCodeSpecMu.RLock()
	spec, ok := openCodeSpecMap[key]
	openCodeSpecMu.RUnlock()
	if ok {
		return spec
	}

	// Dự phòng theo tiền tố cho các model tương lai chưa có trong danh sách tĩnh.
	out := OpenCodeModelSpec{
		ID:                 bare,
		Name:               bare,
		ContextWindow:      200000,
		ReasoningSupported: true,
	}
	switch {
	case strings.HasPrefix(key, "claude-"):
		out.Protocol = OpenCodeProtoMessages
		out.StructuredOutput = true
		out.ReasoningEfforts = []string{"low", "medium", "high"}
	case strings.HasPrefix(key, "qwen"):
		out.Protocol = OpenCodeProtoMessages
		out.StructuredOutput = false
		out.ReasoningEfforts = []string{"low", "medium", "high"}
	case strings.HasPrefix(key, "gemini-"):
		out.Protocol = OpenCodeProtoGemini
		out.StructuredOutput = true
		out.ReasoningEfforts = []string{"low", "medium", "high"}
	case strings.HasPrefix(key, "gpt-"), strings.HasPrefix(key, "grok-"), strings.HasPrefix(key, "muse-spark-"):
		out.Protocol = OpenCodeProtoResponses
		out.StructuredOutput = true
		out.ReasoningEfforts = []string{"low", "medium", "high", "xhigh", "max"}
	default:
		out.Protocol = OpenCodeProtoChat
		out.StructuredOutput = false
		out.ReasoningEfforts = []string{"low", "medium", "high", "max"}
	}
	return out
}

// OpenCodeBaselineEntries chuyển danh mục baseline của OpenCode Go sang []ModelEntry.
func OpenCodeBaselineEntries() []ModelEntry {
	out := make([]ModelEntry, 0, len(defaultOpenCodeGoSpecs))
	for _, spec := range defaultOpenCodeGoSpecs {
		out = append(out, spec.toModelEntry())
	}
	return out
}

type openCodeSpecCache struct {
	FetchedAt time.Time           `json:"fetched_at"`
	Specs     []OpenCodeModelSpec `json:"specs"`
}

type openCodeAPIResponse struct {
	Data []openCodeAPIModel `json:"data"`
}

type openCodeAPIModel struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Capabilities *struct {
		StructuredOutput bool `json:"structured_output"`
		Reasoning        *struct {
			Supported bool     `json:"supported"`
			Effort    []string `json:"effort"`
		} `json:"reasoning"`
	} `json:"capabilities"`
	Limit *struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost *struct {
		Text *struct {
			Input *struct {
				Price float64 `json:"price"`
				Cache *struct {
					Read float64 `json:"read"`
				} `json:"cache"`
			} `json:"input"`
			Output *struct {
				Price float64 `json:"price"`
			} `json:"output"`
		} `json:"text"`
	} `json:"cost"`
	API map[string]any `json:"api"`
}

// RefreshOpenCodeGoModels đọc cache đĩa hoặc tải mới danh mục từ OpenCode Go và cập nhật spec nội bộ.
func RefreshOpenCodeGoModels(cacheDir string) ([]ModelEntry, error) {
	specs := loadOpenCodeCache(cacheDir)
	if specs == nil {
		fetched, err := fetchOpenCodeGoSpecs()
		if err != nil {
			return nil, err
		}
		specs = fetched
		saveOpenCodeCache(specs, cacheDir)
	}
	mergeOpenCodeSpecs(specs)
	out := make([]ModelEntry, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.toModelEntry())
	}
	return out, nil
}

func loadOpenCodeCache(cacheDir string) []OpenCodeModelSpec {
	if cacheDir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(cacheDir, openCodeCacheFileName))
	if err != nil {
		return nil
	}
	var c openCodeSpecCache
	if err := json.Unmarshal(data, &c); err != nil || time.Since(c.FetchedAt) > cacheTTL {
		return nil
	}
	return c.Specs
}

func saveOpenCodeCache(specs []OpenCodeModelSpec, cacheDir string) {
	if cacheDir == "" || len(specs) == 0 {
		return
	}
	p := filepath.Join(cacheDir, openCodeCacheFileName)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(openCodeSpecCache{FetchedAt: time.Now(), Specs: specs})
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(cacheDir, ".opencode-go-cache-*.tmp")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return
	}
	tmp.Close()
	if err := os.Rename(tmpPath, p); err != nil {
		os.Remove(tmpPath)
	}
}

func fetchOpenCodeGoSpecs() ([]OpenCodeModelSpec, error) {
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Get(openCodeGoModelsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode-go API returned %d", resp.StatusCode)
	}
	var payload openCodeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]OpenCodeModelSpec, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || (item.Type != "" && item.Type != "chat") {
			continue
		}
		spec := OpenCodeModelSpec{
			ID:       id,
			Name:     item.Name,
			Protocol: detectOpenCodeProtocol(item.API),
		}
		if spec.Name == "" {
			spec.Name = id
		}
		if item.Limit != nil {
			spec.ContextWindow = item.Limit.Context
			spec.MaxTokens = item.Limit.Output
		}
		if item.Capabilities != nil {
			spec.StructuredOutput = item.Capabilities.StructuredOutput
			if item.Capabilities.Reasoning != nil {
				spec.ReasoningSupported = item.Capabilities.Reasoning.Supported
				spec.ReasoningEfforts = append([]string(nil), item.Capabilities.Reasoning.Effort...)
			}
		}
		if item.Cost != nil && item.Cost.Text != nil {
			if item.Cost.Text.Input != nil {
				spec.InputCostPer1M = item.Cost.Text.Input.Price
				if item.Cost.Text.Input.Cache != nil {
					spec.CacheReadCostPer1M = item.Cost.Text.Input.Cache.Read
				}
			}
			if item.Cost.Text.Output != nil {
				spec.OutputCostPer1M = item.Cost.Text.Output.Price
			}
		}
		out = append(out, spec)
	}
	return out, nil
}

func detectOpenCodeProtocol(apiMap map[string]any) OpenCodeProtocol {
	for _, candidate := range []OpenCodeProtocol{
		OpenCodeProtoMessages,
		OpenCodeProtoResponses,
		OpenCodeProtoGemini,
		OpenCodeProtoChat,
	} {
		if _, ok := apiMap[string(candidate)]; ok {
			return candidate
		}
	}
	return OpenCodeProtoChat
}
