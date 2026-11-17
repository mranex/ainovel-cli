package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/voocel/ainovel-cli/internal/domain"
)

type transRuntime interface {
	TranslationProfile() domain.TranslationProfile
	SetTranslationProfile(profile domain.TranslationProfile, saveGlobal bool) error
}

type transFocusField int

const (
	transFocusGenre transFocusField = iota
	transFocusTone
	transFocusAuto
	transFocusScope
)

type transOption[T ~string] struct {
	Key   T
	Label string
}

var transGenreOptions = []transOption[domain.TranslationGenre]{
	{Key: domain.TransGenreAuto, Label: "Tự nhận diện (Auto)"},
	{Key: domain.TransGenreTienHiep, Label: "Tiên hiệp / Huyền huyễn"},
	{Key: domain.TransGenreCoTrang, Label: "Cổ trang / Kiếm hiệp"},
	{Key: domain.TransGenreFantasy, Label: "Fantasy phương Tây (giữ tên gốc)"},
	{Key: domain.TransGenreHienDai, Label: "Đô thị / Hiện đại"},
}

var transToneOptions = []transOption[domain.TranslationTone]{
	{Key: domain.TransToneNormal, Label: "Thông thường"},
	{Key: domain.TransToneMature, Label: "Trưởng thành 18+ (giữ nguyên)"},
}

var transAutoOptions = []transOption[domain.AutoTranslateMode]{
	{Key: domain.AutoTranslateAfterArc, Label: "Sau khi duyệt Arc/Quyển"},
	{Key: domain.AutoTranslateAfterCommit, Label: "Ngay sau mỗi chương"},
	{Key: domain.AutoTranslateManual, Label: "Thủ công (/translate)"},
}

type transSwitchState struct {
	focus      transFocusField
	genreIdx   int
	toneIdx    int
	autoIdx    int
	saveGlobal bool
	message    string
}

func newTransSwitchState(rt transRuntime) *transSwitchState {
	profile := domain.TranslationProfile{}.Normalized()
	if rt != nil {
		profile = rt.TranslationProfile()
	}
	st := &transSwitchState{}
	for i, opt := range transGenreOptions {
		if opt.Key == profile.Genre {
			st.genreIdx = i
			break
		}
	}
	for i, opt := range transToneOptions {
		if opt.Key == profile.Tone {
			st.toneIdx = i
			break
		}
	}
	for i, opt := range transAutoOptions {
		if opt.Key == profile.AutoTranslate {
			st.autoIdx = i
			break
		}
	}
	return st
}

func (s *transSwitchState) profile() domain.TranslationProfile {
	return domain.TranslationProfile{
		Genre:         transGenreOptions[s.genreIdx].Key,
		Tone:          transToneOptions[s.toneIdx].Key,
		AutoTranslate: transAutoOptions[s.autoIdx].Key,
	}
}

func (s *transSwitchState) moveFocus(delta int) {
	const total = 4
	s.focus = transFocusField((int(s.focus) + delta + total) % total)
}

func (s *transSwitchState) cycle(delta int) {
	switch s.focus {
	case transFocusGenre:
		n := len(transGenreOptions)
		s.genreIdx = (s.genreIdx + delta + n) % n
	case transFocusTone:
		n := len(transToneOptions)
		s.toneIdx = (s.toneIdx + delta + n) % n
	case transFocusAuto:
		n := len(transAutoOptions)
		s.autoIdx = (s.autoIdx + delta + n) % n
	case transFocusScope:
		s.saveGlobal = !s.saveGlobal
	}
}

func (s *transSwitchState) apply(rt transRuntime) error {
	if rt == nil {
		return fmt.Errorf("runtime chưa sẵn sàng")
	}
	return rt.SetTranslationProfile(s.profile(), s.saveGlobal)
}

func applyTransArgs(rt transRuntime, args []string) error {
	if rt == nil {
		return fmt.Errorf("runtime chưa sẵn sàng")
	}
	profile := rt.TranslationProfile()
	saveGlobal := false
	for _, a := range args {
		if a == "--global" {
			saveGlobal = true
			continue
		}
		k, v, ok := strings.Cut(a, "=")
		if !ok {
			return fmt.Errorf("cú pháp: /trans [genre=auto|tien_hiep|co_trang|fantasy|hien_dai] [tone=normal|mature] [auto=after_arc|after_commit|manual] [--global]")
		}
		v = strings.ToLower(strings.TrimSpace(v))
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "genre", "g":
			profile.Genre = domain.TranslationGenre(v)
		case "tone", "t":
			profile.Tone = domain.TranslationTone(v)
		case "auto", "auto_translate":
			profile.AutoTranslate = domain.AutoTranslateMode(v)
		default:
			return fmt.Errorf("tham số không hợp lệ %q (hỗ trợ: genre / tone / auto / --global)", k)
		}
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	return rt.SetTranslationProfile(profile, saveGlobal)
}

func (m Model) handleTransSwitchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.transSwitch == nil {
		return m, nil
	}
	st := m.transSwitch
	switch msg.Type {
	case tea.KeyEsc:
		m.transSwitch = nil
		return m, m.textarea.Focus()
	case tea.KeyTab, tea.KeyDown:
		st.moveFocus(1)
		return m, nil
	case tea.KeyShiftTab, tea.KeyUp:
		st.moveFocus(-1)
		return m, nil
	case tea.KeyLeft:
		st.cycle(-1)
		return m, nil
	case tea.KeyRight:
		st.cycle(1)
		return m, nil
	case tea.KeyEnter:
		if err := st.apply(m.runtime); err != nil {
			st.message = err.Error()
			return m, nil
		}
		m.transSwitch = nil
		return m, tea.Batch(m.textarea.Focus(), fetchSnapshot(m.runtime))
	default:
		return m, nil
	}
}

func renderTransSwitchBar(width int, state *transSwitchState) string {
	if state == nil || width <= 0 {
		return ""
	}
	title := lipgloss.NewStyle().
		Foreground(colorMuted).
		Bold(true).
		Render("/trans Công tắc dịch Việt")

	scopeLabel := "Chỉ truyện này"
	if state.saveGlobal {
		scopeLabel = "Truyện này + Mặc định"
	}
	row1 := renderModelField("Thể loại", transGenreOptions[state.genreIdx].Label, state.focus == transFocusGenre)
	row2 := renderModelField("Sắc thái", transToneOptions[state.toneIdx].Label, state.focus == transFocusTone)
	row3 := renderModelField("Tự động dịch", transAutoOptions[state.autoIdx].Label, state.focus == transFocusAuto)
	row4 := renderModelField("Phạm vi lưu", scopeLabel, state.focus == transFocusScope)
	hint := lipgloss.NewStyle().
		Foreground(colorDim).
		Italic(true).
		Render("Tab đổi dòng   ←→ đổi lựa chọn   Enter lưu   Esc hủy")

	lines := []string{row1, row2, row3, row4, hint}
	if state.message != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorError).Italic(true).Render(truncate(state.message, width-8)))
	}

	content := strings.Join(lines, "\n")
	boxW := lipgloss.Width(content) + 8
	maxW := width - 2
	if maxW > 76 {
		maxW = 76
	}
	if boxW > maxW {
		boxW = maxW
	}
	if boxW < 60 {
		boxW = 60
	}

	innerW := max(16, boxW-2)
	sepW := max(0, innerW-lipgloss.Width(title)-3)
	lineStyle := lipgloss.NewStyle().Foreground(colorDim)
	topBorder := lineStyle.Render("┌─ ") + title + lineStyle.Render(" "+strings.Repeat("─", sepW)+"┐")
	bottomBorder := lineStyle.Render("└" + strings.Repeat("─", innerW) + "┘")

	body := make([]string, 0, len(lines))
	for _, line := range lines {
		padding := max(0, innerW-lipgloss.Width(line))
		body = append(body, lineStyle.Render("│")+line+strings.Repeat(" ", padding)+lineStyle.Render("│"))
	}
	return strings.Join(append(append([]string{topBorder}, body...), bottomBorder), "\n")
}
