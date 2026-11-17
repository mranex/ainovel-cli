package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/voocel/ainovel-cli/internal/host"
)

type readRuntime interface {
	ReadChapter(chapter int, preferVi bool) (*host.ChapterReadView, error)
}

type chapterReaderState struct {
	rt       readRuntime
	chapter  int
	preferVi bool
	view     *host.ChapterReadView
	viewport viewport.Model
	notice   string
	renderW  int
}

func newChapterReaderState(rt readRuntime, width, height int, args []string) (*chapterReaderState, error) {
	if rt == nil {
		return nil, fmt.Errorf("chưa có bộ truyện đang mở")
	}
	chapter := 0
	if len(args) > 0 {
		ch, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil || ch <= 0 {
			return nil, fmt.Errorf("số chương không hợp lệ: %q (cú pháp: /read [chương])", args[0])
		}
		chapter = ch
	}
	view, err := rt.ReadChapter(chapter, true)
	if err != nil {
		return nil, err
	}

	boxW, boxH := reportModalSize(width, height)
	contentW := paddedModalContentWidth(boxW)
	vp := viewport.New(contentW, max(4, boxH-4))

	st := &chapterReaderState{
		rt:       rt,
		chapter:  view.Chapter,
		preferVi: true,
		view:     view,
		viewport: vp,
	}
	st.refreshNotice()
	st.setContent(contentW)
	return st, nil
}

func (s *chapterReaderState) showVi() bool {
	if s.view == nil {
		return s.preferVi
	}
	if s.preferVi && s.view.HasVi {
		return true
	}
	if !s.view.HasZh && s.view.HasVi {
		return true
	}
	return false
}

func (s *chapterReaderState) refreshNotice() {
	s.notice = ""
	if s.view == nil {
		return
	}
	if s.preferVi && !s.view.HasVi && s.view.HasZh {
		s.notice = "Chương này chưa dịch sang tiếng Việt (chạy /translate " + strconv.Itoa(s.view.Chapter) + "); đang hiển thị bản tiếng Trung."
	} else if s.showVi() && s.view.OutdatedVi {
		s.notice = "Bản gốc vừa được viết lại; bản dịch này có thể cũ (chạy /translate " + strconv.Itoa(s.view.Chapter) + " để cập nhật)."
	}
}

func (s *chapterReaderState) loadChapter(ch int) {
	if ch <= 0 || s.rt == nil {
		return
	}
	view, err := s.rt.ReadChapter(ch, s.preferVi)
	if err != nil {
		s.notice = err.Error()
		if s.renderW > 0 {
			s.setContent(s.renderW)
		}
		return
	}
	s.view = view
	s.chapter = view.Chapter
	s.refreshNotice()
	s.viewport.SetYOffset(0)
	if s.renderW > 0 {
		s.setContent(s.renderW)
	}
}

func (s *chapterReaderState) stepChapter(delta int) {
	if s.view == nil {
		return
	}
	if len(s.view.Completed) > 0 {
		idx := slices.Index(s.view.Completed, s.view.Chapter)
		if idx >= 0 {
			nextIdx := idx + delta
			if nextIdx < 0 {
				s.notice = "Đang ở chương đầu tiên."
				if s.renderW > 0 {
					s.setContent(s.renderW)
				}
				return
			}
			if nextIdx >= len(s.view.Completed) {
				s.notice = "Đang ở chương hoàn thành cuối cùng."
				if s.renderW > 0 {
					s.setContent(s.renderW)
				}
				return
			}
			s.loadChapter(s.view.Completed[nextIdx])
			return
		}
	}
	target := s.chapter + delta
	if target < 1 {
		s.notice = "Đang ở chương đầu tiên."
		if s.renderW > 0 {
			s.setContent(s.renderW)
		}
		return
	}
	s.loadChapter(target)
}

func (s *chapterReaderState) setContent(contentW int) {
	s.renderW = contentW
	s.viewport.SetContent(renderReaderBody(s.view, s.showVi(), s.notice, contentW))
}

func renderReaderBody(view *host.ChapterReadView, showVi bool, notice string, width int) string {
	if view == nil {
		return "Không có nội dung."
	}
	var b strings.Builder

	bookTitle := strings.TrimSpace(view.BookTitleVi)
	if !showVi || bookTitle == "" {
		bookTitle = strings.TrimSpace(view.BookTitleZh)
	}
	title := strings.TrimSpace(view.TitleVi)
	body := view.BodyVi
	langTag := "[Tiếng Việt]"
	if !showVi {
		langTag = "[Nguyên bản Trung]"
		title = strings.TrimSpace(view.TitleZh)
		body = view.BodyZh
	}

	titleStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(colorAccent2)
	warnStyle := lipgloss.NewStyle().Foreground(colorReview)
	bodyStyle := lipgloss.NewStyle().Foreground(bodyTextColor)

	header := fmt.Sprintf("Chương %d", view.Chapter)
	if view.TotalChapters > 0 {
		header = fmt.Sprintf("Chương %d / %d", view.Chapter, view.TotalChapters)
	}
	if title != "" {
		header += " — " + title
	}
	b.WriteString(titleStyle.Render(wrapText(header, width)))
	b.WriteString("\n")
	meta := langTag
	if bookTitle != "" {
		meta = bookTitle + " · " + langTag
	}
	b.WriteString(metaStyle.Render(wrapText(meta, width)))
	b.WriteString("\n")
	if notice != "" {
		b.WriteString(warnStyle.Render(wrapText("⚠ "+notice, width)))
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat("─", min(width, 48)))
	b.WriteString("\n\n")

	normalized := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	for _, para := range strings.Split(normalized, "\n") {
		p := strings.TrimSpace(para)
		if p == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(bodyStyle.Render(wrapText(p, width)))
		b.WriteString("\n")
	}
	return b.String()
}

func renderReaderModal(width, height int, state *chapterReaderState) string {
	if state == nil {
		return ""
	}
	boxW, boxH := reportModalSize(width, height)
	contentW := paddedModalContentWidth(boxW)

	if state.viewport.Width != contentW || state.renderW != contentW {
		state.viewport.Width = contentW
		state.setContent(contentW)
	}
	if state.viewport.Height != max(4, boxH-4) {
		state.viewport.Height = max(4, boxH-4)
	}

	title := "Trình đọc truyện"
	if state.view != nil {
		title = fmt.Sprintf("Trình đọc · Chương %d", state.view.Chapter)
	}
	modal := renderPaddedModalFrame(
		boxW,
		boxH,
		title,
		"  ↑↓ Cuộn · ←→ Chương trước/sau · v Việt/Trung · r Tải lại · Esc Đóng",
		strings.Split(state.viewport.View(), "\n"),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) handleReaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.reader == nil {
		return m, nil
	}
	st := m.reader
	switch msg.Type {
	case tea.KeyEsc:
		m.reader = nil
		return m, m.textarea.Focus()
	case tea.KeyUp:
		st.viewport.ScrollUp(1)
		return m, nil
	case tea.KeyDown:
		st.viewport.ScrollDown(1)
		return m, nil
	case tea.KeyPgUp:
		st.viewport.HalfPageUp()
		return m, nil
	case tea.KeyPgDown, tea.KeySpace:
		st.viewport.HalfPageDown()
		return m, nil
	case tea.KeyLeft:
		st.stepChapter(-1)
		return m, nil
	case tea.KeyRight:
		st.stepChapter(1)
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q':
				m.reader = nil
				return m, m.textarea.Focus()
			case 'j':
				st.viewport.ScrollDown(1)
			case 'k':
				st.viewport.ScrollUp(1)
			case '[':
				st.stepChapter(-1)
			case ']':
				st.stepChapter(1)
			case 'v', 'V':
				if st.view != nil {
					if !st.preferVi && !st.view.HasVi {
						st.notice = "Chưa có bản dịch tiếng Việt; hãy chạy /translate " + strconv.Itoa(st.view.Chapter)
					} else if st.preferVi && !st.view.HasZh {
						st.notice = "Không tìm thấy nguyên bản tiếng Trung của chương này."
					} else {
						st.preferVi = !st.preferVi
						st.refreshNotice()
					}
					st.viewport.SetYOffset(0)
					if st.renderW > 0 {
						st.setContent(st.renderW)
					}
				}
			case 'r', 'R':
				st.loadChapter(st.chapter)
			}
		}
		return m, nil
	default:
		return m, nil
	}
}
