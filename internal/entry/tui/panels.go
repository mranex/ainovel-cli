package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/voocel/ainovel-cli/internal/host"
)

// renderTopBar 渲染顶部状态栏。
// 左侧：provider/model，中间：书名，右侧：状态胶囊。
func renderTopBar(snap host.UISnapshot, width int, spinnerFrame, version string) string {
	bookTitle := snap.BookTitle
	if bookTitle == "" {
		bookTitle = "Chưa đặt tên truyện"
	}

	var infoParts []string
	if version != "" {
		infoParts = append(infoParts, "ainovel-cli "+version)
	}
	if snap.Provider != "" {
		infoParts = append(infoParts, snap.Provider)
	}
	if snap.ModelName != "" {
		if w := formatContextWindow(snap.ModelContextWindow); w != "" {
			infoParts = append(infoParts, snap.ModelName+"("+w+")")
		} else {
			infoParts = append(infoParts, snap.ModelName)
		}
	}
	if snap.Style != "" && snap.Style != "default" {
		infoParts = append(infoParts, snap.Style)
	}
	leftText := strings.Join(infoParts, " · ")

	label := snap.StatusLabel
	if label == "" {
		label = "READY"
	}
	color, ok := statusColors[label]
	if !ok {
		color = colorDim
	}
	disp, ok := statusDisplay[label]
	if !ok {
		disp = struct {
			icon  string
			label string
		}{"○", strings.ToLower(label)}
	}
	icon := disp.icon
	if snap.IsRunning && spinnerFrame != "" {
		icon = spinnerFrame
	}
	var status string
	if icon != "" {
		status = statusIconStyle.Foreground(color).Render(icon) + " " + statusLabelStyle.Render(disp.label)
	} else {
		status = statusLabelStyle.Render(disp.label)
	}

	innerW := max(12, width-2)
	titleText := truncate(bookTitle, max(8, innerW/3))
	centerW := max(16, lipgloss.Width(titleText)+6)
	if centerW > innerW-24 {
		centerW = max(8, innerW-24)
	}
	sideTotal := innerW - centerW
	if sideTotal < 0 {
		sideTotal = 0
		centerW = innerW
	}
	leftW := sideTotal / 2
	rightW := innerW - centerW - leftW

	leftCell := lipgloss.NewStyle().
		Width(leftW).
		AlignHorizontal(lipgloss.Left).
		Foreground(colorDim).
		Render(truncate(leftText, leftW))
	centerCell := lipgloss.NewStyle().
		Width(centerW).
		AlignHorizontal(lipgloss.Center).
		Bold(true).
		Foreground(bodyTextColor).
		Render(titleText)
	rightCell := lipgloss.NewStyle().
		Width(rightW).
		AlignHorizontal(lipgloss.Right).
		Render(status)

	content := leftCell + centerCell + rightCell
	return topBarStyle.Width(width).
		Border(baseBorder, false, false, true, false).
		BorderForeground(colorDim).
		Render(content)
}

// renderStatePanel 把状态侧栏内容(已在 stateVP 中)包进左侧带右边框的盒子。
// 与 renderDetailPanel 对称：内容由 renderStateContent 生成并喂进 viewport，这里只负责框。
// MaxHeight 钳高，防止窗口缩小时溢出比右栏高（见 panels_test.go 的高度契约）。
func renderStatePanel(vp viewport.Model, width, height int, focused bool) string {
	borderColor := colorDim
	if focused {
		borderColor = colorAccent
	}
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		Border(baseBorder, false, true, false, false).
		BorderForeground(borderColor).
		Padding(1, 1, 0, 1)
	return style.Render(vp.View())
}

// renderDetailPanel 渲染右侧可滚动详情面板。
func renderDetailPanel(vp viewport.Model, width, height int, focused bool) string {
	borderColor := colorDim
	if focused {
		borderColor = colorAccent
	}
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		Border(baseBorder, false, false, false, true).
		BorderForeground(borderColor).
		Padding(0, 1)

	return style.Render(vp.View())
}

// renderWelcome 渲染新建态首屏。
func renderWelcome(width, height int, errMsg string, mode startupMode, importHint, updateHint string) string {
	// 简洁标题
	title := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		Render("A I N O V E L")

	// 副标题
	subtitle := lipgloss.NewStyle().
		Foreground(colorMuted).
		Italic(true).
		Render("AI-Powered Novel Creation Engine")

	// 分隔线
	divW := 44
	if divW > width-8 {
		divW = width - 8
	}
	divider := lipgloss.NewStyle().Foreground(colorDim).
		Render(strings.Repeat("~", divW))

	// 功能亮点
	features := []struct{ icon, label, desc string }{
		{">>", "Đa model phối hợp", "Architect lập dàn ý / Writer viết / Editor thẩm định / Translator dịch Việt"},
		{"::", "Khôi phục tức thì", "Tự động nối lại đúng tiến độ sau sự cố hoặc tạm dừng"},
		{"<>", "Can thiệp trực tiếp", "Chỉnh sửa hướng truyện theo thời gian thực ngay bằng tiếng Việt"},
		{"##", "Trường thiên phân tầng", "Hỗ trợ cấu trúc Quyển - Arc - Chương cho tiểu thuyết dài kỳ"},
	}
	iconStyle := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true)
	featLabelStyle := lipgloss.NewStyle().Foreground(bodyTextColor)
	descStyle := lipgloss.NewStyle().Foreground(colorDim)
	var featLines []string
	for _, f := range features {
		line := iconStyle.Render(f.icon) + " " +
			featLabelStyle.Render(f.label) + "  " +
			descStyle.Render(f.desc)
		featLines = append(featLines, line)
	}
	feats := strings.Join(featLines, "\n")

	// 输入提示
	prompt := lipgloss.NewStyle().Foreground(bodyTextColor).Render("Nhập ý tưởng hoặc yêu cầu viết truyện bên dưới để bắt đầu")

	modeLine := lipgloss.NewStyle().
		Foreground(colorMuted).
		Render("Chế độ hiện tại: " + mode.label() + " · " + mode.subtitle())

	// 示例
	examples := []string{
		"Viết bộ tiên hiệp 50 chương, nhân vật chính xuất thân từ ngoại môn",
		"Sáng tác dark fantasy 18+ phong cách phương Tây, nhịp nhanh, chặt chém",
		"Viết bộ trinh thám đô thị hiện đại 12 chương, nhân vật chính là nữ pháp y",
	}
	exStyle := lipgloss.NewStyle().Foreground(colorAccent)
	dotStyle := lipgloss.NewStyle().Foreground(colorDim)
	var exLines []string
	for _, ex := range examples {
		exLines = append(exLines, dotStyle.Render("  . ")+exStyle.Render(ex))
	}
	exBlock := strings.Join(exLines, "\n")

	// 组装
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(subtitle)
	b.WriteString("\n\n")
	b.WriteString(divider)
	b.WriteString("\n\n")
	b.WriteString(feats)
	b.WriteString("\n\n")
	b.WriteString(divider)
	b.WriteString("\n\n")
	b.WriteString(modeLine)
	b.WriteString("\n\n")
	b.WriteString(prompt)
	b.WriteString("\n\n")
	b.WriteString(exBlock)
	b.WriteString("\n\n")
	if importHint != "" {
		// 这本书停在导入半路：显著提示恢复入口，替代常规导入提示。
		b.WriteString(lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).
			Render("! " + importHint))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDim).
			Render("Đã có dàn ý? /start <file> tạo truyện · Có bản thảo cũ? /import <file> · Bật dịch/18+: /trans"))
	}
	if updateHint != "" {
		// 启动版本检查命中新版本：与 importHint 同款强调样式追加一行。
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).
			Render("! " + updateHint))
	}
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Italic(true).
		Render("Tab đổi chế độ · Nhấn Enter để bắt đầu nhanh hoặc vào Đồng sáng tạo"))

	if errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(colorError).Bold(true).Render("! " + errMsg))
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		AlignHorizontal(lipgloss.Center).
		AlignVertical(lipgloss.Center).
		Render(b.String())
}
