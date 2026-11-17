package tui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host"
)

// renderStateContent 生成状态侧栏的纯内容(不含边框/外框)，供 stateVP.SetContent 使用。
func renderStateContent(snap host.UISnapshot, contentW int) string {
	contentW = max(12, contentW)
	agents := sidebarAgents(snap.Agents)
	idleAgents := sidebarIdleAgents(snap.Agents)
	var sections []string

	if snap.RecoveryLabel != "" {
		sections = append(sections, lipgloss.NewStyle().Foreground(colorMuted).Italic(true).
			Render(truncate(snap.RecoveryLabel, contentW)))
	}

	var overview strings.Builder
	overview.WriteString(renderField("Trạng thái", snapshotRuntimeStateLabel(snap.RuntimeState)))
	overview.WriteString(renderField("Giai đoạn", snapshotPhaseLabel(snap.Phase)))
	overview.WriteString(renderField("Luồng", snapshotFlowLabel(snap.Flow)))
	if snap.AdvanceMode == "review" {
		advance := "Duyệt từng ch."
		if snap.AdvancePermitChapter > 0 {
			advance = fmt.Sprintf("Mở tới Ch.%d", snap.AdvancePermitChapter)
		}
		overview.WriteString(renderField("Tiến độ", advance))
	} else if snap.AdvanceMode == "auto" {
		overview.WriteString(renderField("Tiến độ", "Tự động"))
	}
	if snap.Layered {
		overview.WriteString(renderField("Đã xong", fmt.Sprintf("%d ch.", snap.CompletedCount)))
		// 分层动态规划：右栏只展示当前弧已展开的章节，"已规划"也用同一个口径，
		// 否则会把骨架弧 EstimatedChapters 的粗估算（如 92）混进来，与可见大纲对不上。
		// progress.TotalChapters 那个值仅用于内部 ContextProfile 决策，不要泄漏到 UI。
		if planned := len(snap.Outline); planned > 0 {
			overview.WriteString(renderField("Đã dàn ý", fmt.Sprintf("%d ch.", planned)))
		}
	} else {
		switch {
		case snap.TotalChapters > 0:
			overview.WriteString(renderField("Chương", fmt.Sprintf("%d / %d ch.", snap.CompletedCount, snap.TotalChapters)))
		default:
			overview.WriteString(renderField("Đã xong", fmt.Sprintf("%d ch.", snap.CompletedCount)))
		}
	}
	overview.WriteString(renderField("Số chữ", formatNumber(snap.TotalWordCount)))
	if label, ch := inProgressDisplay(snap); label != "" {
		overview.WriteString(renderField(label, fmt.Sprintf("Ch.%d", ch)))
	}
	if headline := snapshotHeadline(snap); headline != "" {
		label := "Hiện tại"
		if !snap.IsRunning {
			label = "Chờ nối lại"
		}
		overview.WriteString(renderHighlightField(label, truncate(headline, contentW-10)))
	}
	sections = append(sections, renderSidebarSection("Tổng quan", overview.String(), contentW))

	if len(agents) > 0 {
		var agentBody strings.Builder
		for _, agent := range agents {
			agentBody.WriteString(renderAgentLine(agent, contentW))
			agentBody.WriteString("\n")
		}
		if len(idleAgents) > 0 {
			agentBody.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render("Chờ lệnh: " + truncate(strings.Join(idleAgents, " · "), max(8, contentW-2))))
			agentBody.WriteString("\n")
		}
		sections = append(sections, renderSidebarSection("Vai trò AI", agentBody.String(), contentW))
	}

	if len(snap.PendingRewrites) > 0 {
		var rewrite strings.Builder
		rewrite.WriteString(renderHighlightField("Hàng đợi", fmt.Sprintf("%v", snap.PendingRewrites)))
		if snap.RewriteReason != "" {
			rewrite.WriteString(renderField("Lý do", truncate(snap.RewriteReason, contentW-10)))
		}
		sections = append(sections, renderSidebarSection("Viết lại", rewrite.String(), contentW))
	}

	if snap.PendingSteer != "" {
		sections = append(sections, renderSidebarSection("Can thiệp",
			renderHighlightField("Đang chờ", truncate(snap.PendingSteer, contentW-10)), contentW))
	}
	if snap.HasAdvanceHold {
		sections = append(sections, renderSidebarSection("Điểm dừng",
			renderHighlightField("Đang đợi", truncate(snap.AdvanceHoldReason, contentW-10)), contentW))
	}

	if body := renderTranslationSidebar(snap, contentW); body != "" {
		sections = append(sections, renderSidebarSection("Dịch Việt", body, contentW))
	}

	if body := renderUsageSidebar(snap, contentW); body != "" {
		sections = append(sections, renderSidebarSection("Mức dùng", body, contentW))
	}

	if body := renderCacheSidebar(snap, contentW); body != "" {
		sections = append(sections, renderSidebarSection("Cache", body, contentW))
	}

	return strings.Join(sections, "\n\n")
}

func renderAgentLine(agent host.AgentSnapshot, width int) string {
	stateColor := taskStatusColor(agent.State)
	icon := lipgloss.NewStyle().Foreground(stateColor).Render(agentStateIcon(agent.State))
	badge := lipgloss.NewStyle().Foreground(stateColor).Render(agentStateLabel(agent.State))
	name := lipgloss.NewStyle().Bold(true).Foreground(bodyTextColor).Render(agentDisplayName(agent.Name))
	line := icon + " " + name + " " + badge

	taskLine := agentTaskLine(agent)
	if taskLine != "" {
		line += "\n" + lipgloss.NewStyle().Foreground(colorDim).Render("  "+truncate(taskLine, max(8, width-2)))
	}

	detail := agent.Summary
	if agent.Tool != "" {
		detail = agent.Tool
	}
	if agent.State == "idle" && (detail == "待命" || detail == "Chờ lệnh") {
		detail = ""
	}
	if detail != "" && detail != taskLine {
		line += "\n" + lipgloss.NewStyle().Foreground(colorMuted).Render("  "+truncate(detail, max(8, width-2)))
	}
	if ctx := agentContextLine(agent); ctx != "" {
		line += "\n" + lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("  "+truncate(ctx, max(8, width-2)))
	}
	return line
}

func renderSidebarSection(title, body string, width int) string {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return ""
	}
	lineW := max(0, width-lipgloss.Width(title)-1)
	header := panelTitleStyle.Render(title) + " " +
		lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("─", lineW))
	card := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorDim).
		PaddingLeft(1).
		Render(body)
	return header + "\n" + card
}

func sidebarAgents(agents []host.AgentSnapshot) []host.AgentSnapshot {
	var out []host.AgentSnapshot
	for _, agent := range agents {
		if agent.State == "idle" {
			continue
		}
		out = append(out, agent)
	}
	if len(out) == 0 {
		out = append(out, agents...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := out[i], out[j]
		if agentStateRank(li.State) != agentStateRank(lj.State) {
			return agentStateRank(li.State) < agentStateRank(lj.State)
		}
		return agentOrder(li.Name) < agentOrder(lj.Name)
	})
	return out
}

func sidebarIdleAgents(agents []host.AgentSnapshot) []string {
	var names []string
	hasActive := false
	for _, agent := range agents {
		if agent.State != "idle" {
			hasActive = true
			continue
		}
		names = append(names, agentDisplayName(agent.Name))
	}
	if !hasActive {
		return nil
	}
	sort.Strings(names)
	return names
}

func renderTranslationSidebar(snap host.UISnapshot, width int) string {
	tr := snap.Translation
	p := tr.Profile.Normalized()
	if tr.TotalCompleted == 0 && !tr.Running && tr.Profile.Genre == "" && tr.Profile.Tone == "" && tr.Profile.AutoTranslate == "" {
		return ""
	}
	var b strings.Builder

	genreLabel := string(p.Genre)
	switch p.Genre {
	case domain.TransGenreAuto:
		genreLabel = "Tự nhận diện"
	case domain.TransGenreTienHiep:
		genreLabel = "Tiên hiệp"
	case domain.TransGenreCoTrang:
		genreLabel = "Cổ trang"
	case domain.TransGenreFantasy:
		genreLabel = "Fantasy"
	case domain.TransGenreHienDai:
		genreLabel = "Hiện đại"
	}
	b.WriteString(renderField("Thể loại", genreLabel))

	if p.Tone == domain.TransToneMature {
		toneBadge := lipgloss.NewStyle().Foreground(colorReview).Bold(true).Render("18+ (Giữ nguyên)")
		b.WriteString(renderField("Sắc thái", toneBadge))
	} else {
		b.WriteString(renderField("Sắc thái", "Thông thường"))
	}

	autoLabel := string(p.AutoTranslate)
	switch p.AutoTranslate {
	case domain.AutoTranslateAfterArc:
		autoLabel = "Sau duyệt Arc"
	case domain.AutoTranslateAfterCommit:
		autoLabel = "Sau mỗi chương"
	case domain.AutoTranslateManual:
		autoLabel = "Thủ công"
	}
	b.WriteString(renderField("Tự động", autoLabel))

	if tr.TotalCompleted > 0 {
		prog := fmt.Sprintf("%d/%d ch.", tr.TranslatedCount, tr.TotalCompleted)
		if tr.OutdatedCount > 0 {
			prog += fmt.Sprintf(" (cũ %d)", tr.OutdatedCount)
		}
		b.WriteString(renderField("Đã dịch", prog))
	}
	if tr.Running {
		status := "Khởi tạo..."
		if tr.CurrentChapter > 0 {
			status = fmt.Sprintf("Chương %d", tr.CurrentChapter)
		}
		b.WriteString(renderHighlightField("Đang dịch", status))
	}
	_ = width
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render("/trans đổi · /read đọc"))
	b.WriteString("\n")
	return b.String()
}

// inProgressDisplay 计算"进行中"字段的标签和章节号。
// 根据 flow 选择动词（打磨/重写/写作）；in_progress_chapter 与 flow 不匹配时视为 stale：
//   - polishing/rewriting 模式下章节不在 pending_rewrites 中 → 回退到队列首章
//   - 字段为 0 时不渲染
func inProgressDisplay(snap host.UISnapshot) (label string, chapter int) {
	ch := snap.InProgressChapter
	switch snap.Flow {
	case "polishing":
		if ch <= 0 || !slices.Contains(snap.PendingRewrites, ch) {
			if len(snap.PendingRewrites) == 0 {
				return "", 0
			}
			ch = snap.PendingRewrites[0]
		}
		return "Đang gọt giũa", ch
	case "rewriting":
		if ch <= 0 || !slices.Contains(snap.PendingRewrites, ch) {
			if len(snap.PendingRewrites) == 0 {
				return "", 0
			}
			ch = snap.PendingRewrites[0]
		}
		return "Đang viết lại", ch
	default:
		if ch <= 0 {
			return "", 0
		}
		return "Đang viết", ch
	}
}

func snapshotHeadline(snap host.UISnapshot) string {
	if snap.PendingSteer != "" {
		if !snap.IsRunning {
			return "Chờ nối lại: xử lý can thiệp"
		}
		return "Đang chờ xử lý can thiệp"
	}
	if len(snap.PendingRewrites) > 0 {
		if !snap.IsRunning {
			return "Chờ nối lại: sửa lại chương"
		}
		return "Đang chờ sửa lại chương"
	}
	if snap.AdvanceMode == "review" && !snap.IsRunning && snap.Phase == "writing" {
		return "Chờ duyệt từng chương: chạy /next để viết tiếp"
	}
	return ""
}

func snapshotPhaseLabel(phase string) string {
	switch phase {
	case "premise":
		return "Tiền đề"
	case "outline":
		return "Đại cương"
	case "writing":
		return "Đang viết"
	case "complete":
		return "Hoàn tất"
	case "init":
		return "Khởi tạo"
	default:
		if phase == "" {
			return "-"
		}
		return phase
	}
}

func snapshotRuntimeStateLabel(state string) string {
	switch state {
	case "starting":
		return "Đang khởi động"
	case "running":
		return "Đang chạy"
	case "pausing":
		return "Đang dừng"
	case "paused":
		return "Đã tạm dừng"
	case "completed":
		return "Đã hoàn tất"
	default:
		return "Rảnh"
	}
}

func snapshotFlowLabel(flow string) string {
	switch flow {
	case "":
		return "-"
	case "writing":
		return "Viết"
	case "reviewing":
		return "Thẩm định"
	case "rewriting":
		return "Viết lại"
	case "polishing":
		return "Trau chuốt"
	case "steering":
		return "Can thiệp"
	default:
		return flow
	}
}

func renderUsageSidebar(snap host.UISnapshot, width int) string {
	if snap.TotalInputTokens <= 0 && snap.TotalOutputTokens <= 0 && snap.TotalCostUSD <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(renderField("Đầu vào", formatTokensCompact(snap.TotalInputTokens)))
	b.WriteString(renderField("Đầu ra", formatTokensCompact(snap.TotalOutputTokens)))
	if cost := formatCostUSD(snap.TotalCostUSD); cost != "" {
		b.WriteString(renderField("Chi phí", cost))
	}
	if saved := formatCostUSD(snap.TotalSavedUSD); saved != "" {
		b.WriteString(renderField("Tiết kiệm", saved))
	}
	if snap.BudgetLimitUSD > 0 {
		pct := snap.TotalCostUSD / snap.BudgetLimitUSD * 100
		b.WriteString(renderField("Ngân sách", fmt.Sprintf("$%.2f/$%.2f (%.0f%%)", snap.TotalCostUSD, snap.BudgetLimitUSD, pct)))
	}

	agentStats := usageStatsByCost(snap.CachePerAgent)
	if len(agentStats) > 0 {
		b.WriteString(renderUsageGroupHeader("Vai trò", width))
		limit := min(len(agentStats), 4)
		for i := 0; i < limit; i++ {
			a := agentStats[i]
			b.WriteString(renderUsageLine(agentDisplayName(a.Role), eventAgentColor(a.Role), a.Input, a.Output, a.Cost, width))
			b.WriteString("\n")
		}
	}
	modelStats := usageStatsByCost(snap.CachePerModel)
	if len(modelStats) > 0 {
		b.WriteString(renderUsageGroupHeader("Model", width))
		limit := min(len(modelStats), 4)
		for i := 0; i < limit; i++ {
			a := modelStats[i]
			b.WriteString(renderUsageLine(modelDisplayName(a.Model), bodyTextColor, a.Input, a.Output, a.Cost, width))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func usageStatsByCost(in []host.AgentCacheStat) []host.AgentCacheStat {
	out := append([]host.AgentCacheStat(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Cost != out[j].Cost {
			return out[i].Cost > out[j].Cost
		}
		return out[i].Input+out[i].Output > out[j].Input+out[j].Output
	})
	return out
}

func renderUsageGroupHeader(label string, width int) string {
	line := lipgloss.NewStyle().Foreground(colorDim).
		Render(strings.Repeat("·", max(8, width-lipgloss.Width(label)-3)))
	return lipgloss.NewStyle().Foreground(colorMuted).Render(label+" ") + line + "\n"
}

func renderUsageLine(name string, color lipgloss.TerminalColor, input, output int, cost float64, width int) string {
	nameW := 11
	if width < 24 {
		nameW = 8
	}
	nameCell := lipgloss.NewStyle().Foreground(color).Width(nameW).
		Render(truncate(name, nameW))
	tokens := formatTokensCompact(input + output)
	right := tokens
	if costStr := formatCostUSD(cost); costStr != "" {
		right += " · " + costStr
	}
	// 名称恰好占满固定列宽时，padding 不会留下尾随空格；显式分隔，避免
	// "gpt-5.6-sol5.3k" 这类模型名与用量粘连。
	return fitInlineLine(nameCell+" "+lipgloss.NewStyle().Foreground(colorDim).Render(right), width)
}

func modelDisplayName(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "unknown"
	}
	parts := strings.Split(model, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[1:], "/")
	}
	if len(parts) == 2 {
		return parts[1]
	}
	return model
}

// renderCacheSidebar 渲染左栏"缓存"区块。
//
// 三种态：
//  1. 完全没消费 token：返回空，section 不渲染
//  2. 当前会话所有 role 都跑的是不支持 prompt cache 的模型：仅渲染一行"未启用"提示
//  3. 已启用：顶部"命中率累计/近10 · 节省 · 读/写"+ 分隔 + per-role 行
//
// per-role 行 capable 时显示"累计/近10%"双数字；不 capable 时显示"未启用"。
// 通过累计 vs 近 N 次的对比可以识别"前期拖累"vs"稳态低命中"。
func renderCacheSidebar(snap host.UISnapshot, width int) string {
	// 上游 streaming 没发 OpenAI 的 final usage chunk —— 累计数据全为 0，
	// 但这不是"没启用 cache"也不是"用量太低被门控藏起来"，必须显式提示，
	// 否则用户会一直以为左栏写了缓存代码却显示不出来。优先级最高。
	if snap.MissingAssistantUsage > 0 && snap.TotalInputTokens <= 0 {
		warn := lipgloss.NewStyle().Foreground(colorError).Bold(true).
			Render(fmt.Sprintf("⚠ Nguồn chưa trả usage (%d lần)", snap.MissingAssistantUsage))
		hint := lipgloss.NewStyle().Foreground(colorDim).Italic(true).
			Render(truncate("Kiểm tra provider stream_options.include_usage", max(8, width-2)))
		return warn + "\n" + hint + "\n"
	}

	if snap.TotalInputTokens <= 0 && snap.TotalCacheWriteTokens <= 0 {
		return ""
	}

	// 全程未启用 → 显示一行解释，避免用户误判为"0% 命中需要排查"
	if !snap.OverallCacheCapable && snap.TotalCacheReadTokens == 0 && snap.TotalCacheWriteTokens == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).
			Render(truncate("Model hiện tại chưa bật prompt cache", max(8, width-2))) + "\n"
	}

	var b strings.Builder

	// 顶部综合指标：累计 + 近 N 各占一行，标签明示，避免 "X% · 近N Y%" 这种
	// 三种分隔符（百分号 / 中点 / 文字）混杂导致语义不清。
	overallHit := cacheHitRate(snap.TotalCacheReadTokens, snap.TotalInputTokens)
	b.WriteString(renderField("Tỉ lệ trúng", colorPercent(overallHit)))
	if snap.OverallRecentSamples > 0 && snap.OverallRecentInput > 0 {
		recent := cacheHitRate(snap.OverallRecentCacheRead, snap.OverallRecentInput)
		b.WriteString(renderField(fmt.Sprintf("Gần %d lần", snap.OverallRecentSamples), colorPercent(recent)))
	}

	if savedStr := formatCostUSD(snap.TotalSavedUSD); savedStr != "" {
		b.WriteString(renderField("Tiết kiệm", savedStr))
	}

	// 读/写量分两行。写量为 0 在 OpenAI / Gemini 系协议是常态——
	// 这两家是自动透明 caching，cache 写入完全免费（首次未命中按正常输入价，
	// 建立 cache 不收任何溢价），所以协议本身不暴露 cache_creation 字段，没必要。
	// 只有 Anthropic / Bedrock 系才报写量，因为他们写要加价（5m +25%/1h +100%），
	// 必须把这个量给用户用于计费。
	b.WriteString(renderField("Đọc cache", formatTokensCompact(snap.TotalCacheReadTokens)))
	if snap.TotalCacheWriteTokens > 0 {
		b.WriteString(renderField("Ghi cache", formatTokensCompact(snap.TotalCacheWriteTokens)))
	} else if snap.TotalCacheReadTokens > 0 {
		hint := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("(cache tự động)")
		b.WriteString(renderField("Ghi cache", "0 "+hint))
	}

	// 断裂 = 前缀未缩短而命中骤降（合法下降如换章/压缩已豁免）。次数多通常
	// 指向服务端逐出或中转轮询上游，详情看 tui.log 的"缓存链断裂"warn。
	if snap.TotalCacheBreaks > 0 {
		v := lipgloss.NewStyle().Foreground(colorReview).Render(fmt.Sprintf("%d lần", snap.TotalCacheBreaks))
		b.WriteString(renderField("Đứt mạch", v))
	}

	// Arbiter 按设计不参与 prompt cache（KB 级一次性裁定，无稳定前缀可复用），
	// 常驻"未启用"或"0%"只会引人排查；用量面板仍完整记它的账。
	var roles []host.AgentCacheStat
	for _, a := range snap.CachePerAgent {
		if a.Role != "arbiter" {
			roles = append(roles, a)
		}
	}
	if len(roles) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDim).
			Render(strings.Repeat("·", max(8, width-12))))
		b.WriteString("\n")
		for _, a := range roles {
			b.WriteString(renderCacheAgentLine(a, width))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// colorPercent 把百分比按命中率分档着色后转字符串，仅用于值列。
func colorPercent(p float64) string {
	return lipgloss.NewStyle().Foreground(cacheHitColor(p)).Bold(true).
		Render(formatPercent(p))
}

// renderCacheAgentLine 渲染单个 role 行：role + 命中率 + 缓存读 / 总输入。
//
// 把分子分母都摆出来（cacheRead / input）让用户一眼就能验算命中率的来源，
// 也能识别"高百分比但小样本"的侥幸数据（比如 100% / 1k 的可信度低于 80% / 300k）。
//
// 百分比优先用滑动窗稳态值；窗内无样本时回落到累计。整个左栏只有这一处用 "/"，
// 语义专一（数学除号：cache 命中量 / 总输入量），不会与其它分隔符混淆。
//
// 三种态：
//
//	未启用     "WRITER        未启用"
//	已启用     "WRITER        85%  · 323k / 394k"
//	无 cache  显式"未启用"，不混进 0/0 干扰判读
func renderCacheAgentLine(a host.AgentCacheStat, width int) string {
	// role 名与"运行角色"区保持完全一致；Width 取 12 让最长的 ARCHITECT
	// 仍能保留 1 列尾随空格做分隔，其它 role 自动右侧填充。
	roleStyle := lipgloss.NewStyle().Foreground(eventAgentColor(a.Role)).Width(12)
	role := roleStyle.Render(agentDisplayName(a.Role))

	if !a.CacheCapable {
		dim := lipgloss.NewStyle().Foreground(colorDim).Italic(true)
		_ = width
		return role + dim.Render("Chưa bật")
	}

	// 稳态命中率优先；窗内无样本时回落到累计。
	hit := cacheHitRate(a.RecentCacheRead, a.RecentInput)
	if a.RecentSamples == 0 || a.RecentInput == 0 {
		hit = cacheHitRate(a.CacheRead, a.Input)
	}
	// 百分比固定 4 列宽（"100%"），避免读量列在 "5%" 与 "85%" 之间左右跳。
	pctCell := lipgloss.NewStyle().Width(4).
		Render(colorPercent(hit))

	// 累计读 / 累计输入 — 即便上方百分比是滑动窗值，分子分母都用累计，因为
	// "看出规模"才是这一列的主诉求；百分比单独提供稳态信号即可。
	tokens := lipgloss.NewStyle().Foreground(colorDim).Render(
		" · " + formatTokensCompact(a.CacheRead) + " / " + formatTokensCompact(a.Input))
	_ = width
	return role + pctCell + tokens
}

// cacheHitRate 在 input 已含 cacheRead 的语义下直接除得百分比。
// input == 0 时返回 0，避免出现假命中。
func cacheHitRate(cacheRead, input int) float64 {
	if input <= 0 {
		return 0
	}
	return float64(cacheRead) / float64(input) * 100
}

// cacheHitColor 命中率染色：≥50% 绿 / 20–50% 黄 / <20% 红。
// 用与上下文使用率相反的方向：缓存命中率越高越健康。
func cacheHitColor(percent float64) lipgloss.AdaptiveColor {
	switch {
	case percent >= 50:
		return colorSuccess
	case percent >= 20:
		return colorReview
	default:
		return colorError
	}
}

func formatPercent(p float64) string {
	if p <= 0 {
		return "0%"
	}
	if p < 10 {
		return fmt.Sprintf("%.1f%%", p)
	}
	return fmt.Sprintf("%.0f%%", p)
}

// formatTokensCompact 把 token 数渲染成 "8.2k" / "1.4M" 这种紧凑形式。
// 用于狭窄的 per-role 行，避免和 formatNumber 的逗号风格挤出去。
func formatTokensCompact(n int) string {
	if n <= 0 {
		return "0"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func contextScopeLabel(scope string) string {
	switch scope {
	case "baseline":
		return "Cơ sở"
	case "projected":
		return "Chiếu"
	case "recovered":
		return "Khôi phục"
	case "committed":
		return "Đã lưu"
	case "skipped":
		return "Bỏ qua"
	default:
		return scope
	}
}

func contextStrategyLabel(strategy string) string {
	switch strategy {
	case "":
		return ""
	case "tool_result_microcompact":
		return "Nén nhẹ tool"
	case "light_trim":
		return "Cắt nhẹ"
	case "full_summary":
		return "Tóm tắt đầy đủ"
	default:
		return strategy
	}
}

func agentDisplayName(name string) string {
	return strings.ToUpper(name)
}

func agentTaskLine(agent host.AgentSnapshot) string {
	if agent.TaskKind != "" {
		return taskKindLabel(agent.TaskKind)
	}
	if agent.Summary != "" {
		return agent.Summary
	}
	return ""
}

func agentContextLine(agent host.AgentSnapshot) string {
	ctx := agent.Context
	if ctx.ContextWindow <= 0 || ctx.Tokens <= 0 {
		return ""
	}
	percentColor := contextPercentColor(ctx.Percent)
	percentStr := lipgloss.NewStyle().Foreground(percentColor).Render(fmt.Sprintf("ctx %.0f%%", ctx.Percent))
	parts := []string{percentStr}
	if scope := contextScopeLabel(ctx.Scope); scope != "" {
		parts = append(parts, scope)
	}
	if strategy := contextStrategyLabel(ctx.Strategy); strategy != "" {
		parts = append(parts, strategy)
	}
	return strings.Join(parts, " · ")
}

func agentStateRank(state string) int {
	switch state {
	case "running":
		return 0
	case "failed":
		return 1
	default:
		return 2
	}
}

func agentOrder(name string) int {
	switch {
	case strings.HasPrefix(name, "architect"):
		return 0
	case name == "editor":
		return 2
	case name == "writer":
		return 3
	default:
		return 9
	}
}

func agentStateLabel(state string) string {
	switch state {
	case "running":
		return "Đang chạy"
	case "failed":
		return "Lỗi"
	case "idle":
		return "Rảnh"
	default:
		return state
	}
}

func agentStateIcon(state string) string {
	switch state {
	case "running":
		return "●"
	case "failed":
		return "×"
	default:
		return "·"
	}
}

func taskStatusColor(status string) lipgloss.AdaptiveColor {
	switch status {
	case "running":
		return colorSuccess
	case "queued":
		return colorMuted
	case "failed", "canceled":
		return colorError
	case "succeeded":
		return colorSuccess
	default:
		return colorDim
	}
}

func taskKindLabel(kind string) string {
	switch kind {
	case "foundation_plan":
		return "Lập nền tảng"
	case "chapter_write":
		return "Viết chương"
	case "chapter_review":
		return "Thẩm định chương"
	case "chapter_rewrite":
		return "Viết lại chương"
	case "chapter_polish":
		return "Trau chuốt chương"
	case "arc_expand":
		return "Mở rộng Arc"
	case "volume_append":
		return "Lập Quyển tiếp"
	case "steer_apply":
		return "Xử lý can thiệp"
	default:
		return kind
	}
}
