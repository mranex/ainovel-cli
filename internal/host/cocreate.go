package host

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// 冷启动共创：从零澄清需求，产出整本书的创作指令。
const coCreateSystemPrompt = `Bạn là trợ lý đồng sáng tạo tiểu thuyết (Co-create Assistant) trong ainovel-cli. Nhiệm vụ của bạn KHÔNG phải viết truyện ngay lập tức, mà là trò chuyện ngắn gọn nhiều lượt bằng tiếng Việt để giúp người dùng làm rõ ý tưởng sáng tác, đồng thời liên tục tổng hợp thành một bản Chỉ thị Sáng tác hoàn chỉnh (sau đó hệ thống sẽ tự động cầu nối cho dàn AI tiếng Trung thực thi).

Mỗi lượt trả lời PHẢI tuân thủ nghiêm ngặt định dạng 4 thẻ XML viết thường sau, xuất hiện theo đúng thứ tự, đủ thẻ đóng mở:

<reply>
Lời hồi đáp tự nhiên bằng tiếng Việt cho người dùng đọc: trả lời trực tiếp ý vừa nhập, sau đó đặt tối đa 1 đến 2 câu hỏi mấu chốt nhất còn thiếu. Nếu thông tin đã đủ để bắt đầu viết truyện, hãy nhắc người dùng có thể nhấn Ctrl+S để bắt đầu.
</reply>

<draft>
Bản thảo Chỉ thị Sáng tác hoàn chỉnh viết bằng tiếng Việt (định dạng Markdown, bắt đầu ngay từ tiêu đề cấp 2 như "## Chủ đề & Thể loại", "## Nhân vật & Thiết lập", "## Cốt truyện & Quy mô", "## Điểm cần làm rõ"). Mỗi lượt phải **cập nhật tích lũy** dựa trên bản thảo trước đó để tiếp thu ý mới của người dùng; ngay cả khi lượt này không thêm gì vẫn phải viết lại đầy đủ toàn bộ bản thảo — tuyệt đối không bỏ sót hay viết kiểu "(giữ nguyên như cũ)".
</draft>
` + coCreateProtocolTail

// 阶段共创：小说已写了一部分，规划"后续阶段"的走向。调用方需把当前故事状态摘要
// 追加到本 prompt 之后（"## 当前故事状态" 段），让模型在已写内容的基础上规划。
const stageCoCreateSystemPrompt = `Bạn là trợ lý "Đồng sáng tạo Giai đoạn" cho một bộ tiểu thuyết đang viết dở (tiến độ và tóm tắt nguyên tác tiếng Trung nằm ở mục "当前故事状态" bên dưới). Người dùng tạm dừng để cùng bạn thảo luận hướng đi cho giai đoạn tiếp theo trước khi tiếp tục sáng tác.

Nhiệm vụ của bạn không phải viết tiếp chính văn, mà là trò chuyện ngắn gọn bằng tiếng Việt để cùng người dùng chốt hướng đi cho đoạn tiếp theo (vài chương tới / Arc kế tiếp / Quyển kế tiếp), và liên tục tổng hợp thành một bản "Định hướng giai đoạn tiếp theo" rõ ràng.

Thiết luật: Mọi đề xuất PHẢI nhất quán với diễn biến, nhân vật và phục bút đã xảy ra trong "当前故事状态", tuyệt đối không lật ngược hay bỏ qua nội dung đã viết; chỉ lên kế hoạch "đoạn tiếp theo đi đâu", không thiết kế lại cả cuốn sách.

Mỗi lượt trả lời PHẢI tuân thủ nghiêm ngặt định dạng 4 thẻ XML viết thường sau, xuất hiện theo đúng thứ tự, đủ thẻ đóng mở:

<reply>
Lời hồi đáp tự nhiên bằng tiếng Việt cho người dùng đọc: phản hồi ý kiến vừa nhập, sau đó đặt tối đa 1 đến 2 câu hỏi quan trọng nhất. Nếu hướng đi đã đủ rõ, hãy nhắc người dùng nhấn Ctrl+S để áp dụng và tiếp tục viết truyện.
</reply>

<draft>
Bản "Định hướng giai đoạn tiếp theo" đầy đủ bằng tiếng Việt (Markdown, bắt đầu từ tiêu đề cấp 2 như "## Hướng đi tiếp theo", "## Bước ngoặt chính", "## Phục bút cần thu hồi", "## Nhịp độ & Số chương"). Mỗi lượt phải **cập nhật tích lũy** trên bản cũ; ngay cả khi không đổi vẫn phải xuất lại toàn bộ brief hoàn chỉnh — tuyệt đối không lược bớt.
</draft>
` + coCreateProtocolTail

// coCreateProtocolTail 是两种共创模式共用的输出协议尾部（<ready> / <suggestions> + 输出规范）。
// 两模式只在开场语境与 <draft> 语义上不同，协议完全一致。
const coCreateProtocolTail = `
<ready>false</ready>

<suggestions>
1-3 câu gợi ý "người dùng có thể muốn nói tiếp theo" bằng tiếng Việt, mỗi dòng bắt đầu bằng "- ". Đây là gợi ý nhanh để người dùng bấm phím số điền vào ô nhập liệu.

Yêu cầu:
- Đứng ở ngôi thứ nhất của người dùng (như lời người dùng nói với bạn), không viết thành câu hỏi ngược của trợ lý.
- Mỗi dòng ngắn gọn (không quá 20 từ), đa dạng câu chữ.
- Đưa ra xu hướng / lựa chọn / bổ sung ý đồ cụ thể, không viết thay toàn bộ kịch bản trong 1 dòng.
</suggestions>

Quy chuẩn đầu ra:
- Bắt buộc dùng đúng 4 thẻ XML viết thường: <reply> / <draft> / <ready> / <suggestions>, đủ cả thẻ mở và thẻ đóng.
- Không đổi tên thẻ thành chữ hoa hay tiếng Việt.
- Ngoài 4 thẻ này không xuất thêm giải thích, suy nghĩ hay khối code markdown.
- Bên trong <draft> viết Markdown nhiều dòng bình thường, không cần escape.
- <ready> chỉ điền true hoặc false. Khi thông tin đã đủ rõ để bắt đầu/tiếp tục thì điền true.
- Khi <ready>true</ready>, <suggestions> có thể để trống (<suggestions></suggestions>).`

// CoCreateProgressKind 标识流式回调的内容类型。
const (
	CoCreateProgressThinking = "thinking"
	CoCreateProgressReply    = "reply"
)

// 四段式 XML 标签输出。XML 风格比方括号 marker 更鲁棒——Claude/GPT 训练数据里
// 大量 <thinking>...</thinking> 这类格式，模型几乎不会把 <reply> 改写成 <REWRITE>
// 或其他变体；闭合标签也让流式中段截断更精确（不依赖找下一个 marker 来断尾）。
const (
	tagReply       = "reply"
	tagDraft       = "draft"
	tagReady       = "ready"
	tagSuggestions = "suggestions"
)

func coCreateTonePrompt(sysPrompt string, tone domain.TranslationTone) string {
	if tone.Normalize() != domain.TransToneMature {
		return sysPrompt
	}
	return sysPrompt + "\n\n---\n## Chế độ nội dung 18+ (Mature)\nNgười dùng đang bật chế độ truyện trưởng thành (18+). Khi trao đổi và tổng hợp `<draft>`, hãy tôn trọng và giữ nguyên trọn vẹn 100% ý định tình cảm, cảnh nóng hoặc yếu tố trưởng thành/dark mà người dùng yêu cầu, không tự ý làm nhẹ, né tránh hay cắt bỏ."
}

func coCreateStream(ctx context.Context, models *bootstrap.ModelSet, sessions *store.SessionStore, sysPrompt string, history []CoCreateMessage, onProgress func(kind, text string)) (reply CoCreateReply, err error) {
	if len(history) == 0 {
		return CoCreateReply{}, fmt.Errorf("cocreate history is empty")
	}

	model := models.ForRole("thinking")

	msgs := []agentcore.Message{agentcore.SystemMsg(sysPrompt)}
	for _, item := range history {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.Role)) {
		case "assistant":
			msgs = append(msgs, assistantMsg(content))
		default:
			msgs = append(msgs, agentcore.UserMsg(content))
		}
	}

	var raw, thinking strings.Builder

	// 排查 "cocreate empty response" 等偶发问题需要看到模型实际返回什么。
	// 每轮全程落盘到 <output>/meta/sessions/cocreate.jsonl，与正式创作的 session 日志同位。
	start := time.Now()
	defer func() {
		if sessions == nil {
			return
		}
		if logErr := sessions.LogCoCreate(coCreateLogEntry{
			Time:         time.Now(),
			DurationMS:   time.Since(start).Milliseconds(),
			InputHistory: history,
			RawResponse:  raw.String(),
			RawLen:       len([]rune(raw.String())),
			Thinking:     thinking.String(),
			ParsedReply:  reply.Message,
			ParsedDraft:  reply.Prompt,
			ParsedReady:  reply.Ready,
			ParsedSugs:   reply.Suggestions,
			Error:        errString(err),
		}); logErr != nil {
			slog.Warn("共创会话日志落盘失败", "module", "cocreate", "err", logErr)
		}
	}()

	// 不另设输出上限：思考 token 与回复共享 max_tokens，收窄会让推理模型把预算耗在
	// 思考上、回复为空或被截断。
	streamCh, err := model.GenerateStream(ctx, msgs, nil)
	if err != nil {
		return CoCreateReply{}, fmt.Errorf("cocreate generate: %w", err)
	}

	var streamed bool
	for ev := range streamCh {
		switch ev.Type {
		case agentcore.StreamEventThinkingDelta:
			thinking.WriteString(ev.Delta)
			if onProgress != nil {
				onProgress(CoCreateProgressThinking, thinking.String())
			}
		case agentcore.StreamEventTextDelta:
			streamed = true
			raw.WriteString(ev.Delta)
			if onProgress != nil {
				onProgress(CoCreateProgressReply, extractReplyPreview(raw.String()))
			}
		case agentcore.StreamEventDone:
			if !streamed {
				raw.WriteString(ev.Message.TextContent())
			}
		case agentcore.StreamEventError:
			if ev.Err != nil {
				return CoCreateReply{}, fmt.Errorf("cocreate generate: %w", ev.Err)
			}
			return CoCreateReply{}, fmt.Errorf("cocreate generate failed")
		}
	}

	reply, err = parseCoCreateResponse(raw.String())
	return reply, err
}

// coCreateLogEntry 是写入 meta/sessions/cocreate.jsonl 的一行结构。
// 字段命名贴近 jsonl 直查习惯（snake_case），方便 jq 过滤。
type coCreateLogEntry struct {
	Time         time.Time         `json:"time"`
	DurationMS   int64             `json:"duration_ms"`
	InputHistory []CoCreateMessage `json:"input_history"`
	RawResponse  string            `json:"raw_response"`
	RawLen       int               `json:"raw_len"`
	Thinking     string            `json:"thinking,omitempty"`
	ParsedReply  string            `json:"parsed_reply"`
	ParsedDraft  string            `json:"parsed_draft"`
	ParsedReady  bool              `json:"parsed_ready"`
	ParsedSugs   []string          `json:"parsed_sugs,omitempty"`
	Error        string            `json:"error,omitempty"`
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func assistantMsg(text string) agentcore.Message {
	return agentcore.Message{
		Role:      agentcore.RoleAssistant,
		Content:   []agentcore.ContentBlock{agentcore.TextBlock(text)},
		Timestamp: time.Now(),
	}
}

// parseCoCreateResponse 解析 XML 标签输出。模型若没遵守协议（直接说自然语言），
// 整段作为 reply 显示，draft 留空让 session 保留上一轮。
func parseCoCreateResponse(raw string) (CoCreateReply, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return CoCreateReply{}, fmt.Errorf("cocreate empty response")
	}

	reply, draft, ready, suggestions := splitCoCreateMarkers(raw)
	if reply == "" {
		// 模型没遵守 XML 协议：整段作为 reply。
		return CoCreateReply{Message: raw, Prompt: "", Ready: false, Raw: raw}, nil
	}
	return CoCreateReply{
		Message:     reply,
		Prompt:      draft,
		Ready:       ready,
		Suggestions: suggestions,
		Raw:         raw,
	}, nil
}

// splitCoCreateMarkers 按四个 XML 标签切分文本。
// 标签可能缺失（流式中段或模型遗漏），缺失部分对应字段为空 / false / nil。
// 缺失闭标签时，extractTagContent 会取到字符串末尾，仍尽力解析。
func splitCoCreateMarkers(s string) (reply, draft string, ready bool, suggestions []string) {
	reply = extractTagContent(s, tagReply)
	draft = extractTagContent(s, tagDraft)
	readyStr := strings.ToLower(extractTagContent(s, tagReady))
	ready = readyStr == "true" || readyStr == "yes"
	suggestions = parseSuggestions(extractTagContent(s, tagSuggestions))
	return
}

// extractTagContent 从 s 中抠出 <tag>...</tag> 之间的文本。
// 三种偶发故障场景兜底，避免直接走降级丢字段：
//  1. 有开无闭（流式中段）→ 切到下一个已知开标签前
//  2. 无开有闭（模型 typo，如 <suggestions> 写成 <uggestions>）→ 从最近一个已知
//     完整闭合标签的结束位置开始，到 </tag> 之前
//  3. reply 完全无开标签（模型直接以自然语言开篇，末尾贴 </reply>）→ 从开头到 </reply>
func extractTagContent(s, tag string) string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	oIdx := strings.Index(s, open)
	if oIdx >= 0 {
		rest := s[oIdx+len(open):]
		if cIdx := strings.Index(rest, closeTag); cIdx >= 0 {
			return strings.TrimSpace(rest[:cIdx])
		}
		// 有开无闭 → 切到下一个已知开标签前
		for _, other := range []string{"<reply>", "<draft>", "<ready>", "<suggestions>"} {
			if other == open {
				continue
			}
			if idx := strings.Index(rest, other); idx >= 0 {
				rest = rest[:idx]
			}
		}
		return strings.TrimSpace(rest)
	}

	// 无开有闭 → 从最近一个已知完整闭合标签的结束位置开始，到 </tag>。
	if cIdx := strings.Index(s, closeTag); cIdx >= 0 {
		prefix := s[:cIdx]
		start := 0
		for _, t := range []string{"</reply>", "</draft>", "</ready>", "</suggestions>"} {
			if t == closeTag {
				continue
			}
			if i := strings.LastIndex(prefix, t); i >= 0 {
				if end := i + len(t); end > start {
					start = end
				}
			}
		}
		return strings.TrimSpace(prefix[start:])
	}
	return ""
}

// parseSuggestions 把 <suggestions> 段每行抠出来，去掉 "- " / "* " / "1. " 等列表前缀。
// 最多保留 3 条；空行、过短（<2 字）、整行像 XML 标签的（typo 开标签兜底残留，
// 例如 <uggestions>）忽略。
func parseSuggestions(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 整行像 XML 标签 → 跳过（防 typo 开标签污染）
		if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
			continue
		}
		// 剥列表前缀
		switch {
		case strings.HasPrefix(line, "- "):
			line = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "* "):
			line = strings.TrimSpace(line[2:])
		case isOrderedSuggestion(line):
			line = stripOrderedPrefix(line)
		}
		if len([]rune(line)) < 2 {
			continue
		}
		out = append(out, line)
		if len(out) >= 3 {
			break
		}
	}
	return out
}

// isOrderedSuggestion 判断行首是否形如 "1. " / "12. "（数字+点+空格）。
func isOrderedSuggestion(line string) bool {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(line) && line[i] == '.' && line[i+1] == ' '
}

func stripOrderedPrefix(line string) string {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(line) {
		return line
	}
	return strings.TrimSpace(line[i+2:])
}

// extractReplyPreview 流式预览：raw 还在生长时给 UI 一段可显示的文本。
// 找到 <reply> 之后的内容，切到 </reply> 或下一个开标签 <draft> 之前。
// 模型半遵守（漏 <reply> 开标签）时，开头到 </reply> 或 <draft> 都算 reply。
func extractReplyPreview(raw string) string {
	trimmed := strings.TrimSpace(raw)
	open := "<" + tagReply + ">"
	closeTag := "</" + tagReply + ">"
	draftOpen := "<" + tagDraft + ">"

	rest := trimmed
	if rIdx := strings.Index(trimmed, open); rIdx >= 0 {
		rest = trimmed[rIdx+len(open):]
	}
	if cIdx := strings.Index(rest, closeTag); cIdx >= 0 {
		return strings.TrimSpace(rest[:cIdx])
	}
	if dIdx := strings.Index(rest, draftOpen); dIdx >= 0 {
		rest = rest[:dIdx]
	}
	return strings.TrimSpace(rest)
}
