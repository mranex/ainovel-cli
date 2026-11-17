package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/voocel/ainovel-cli/internal/host"
)

type fakeReadRuntime struct {
	views map[int]*host.ChapterReadView
}

func (f *fakeReadRuntime) ReadChapter(chapter int, _ bool) (*host.ChapterReadView, error) {
	if chapter <= 0 {
		chapter = 1
	}
	v, ok := f.views[chapter]
	if !ok {
		return nil, fmt.Errorf("không tìm thấy chương %d", chapter)
	}
	return v, nil
}

func TestReaderModalToggleAndNavigation(t *testing.T) {
	rt := &fakeReadRuntime{
		views: map[int]*host.ChapterReadView{
			1: {
				Chapter:       1,
				TotalChapters: 2,
				TitleVi:       "Sơ Nhập Tông Môn",
				BodyVi:        "Lâm Mặc rút kiếm.",
				HasVi:         true,
				OutdatedVi:    true,
				TitleZh:       "初入宗门",
				BodyZh:        "林默拔剑。",
				HasZh:         true,
				BookTitleVi:   "Vấn Kiếm",
				Completed:     []int{1, 2},
			},
			2: {
				Chapter:       2,
				TotalChapters: 2,
				TitleVi:       "Thanh Vân Thí Kiếm",
				BodyVi:        "Kiếm khí tung hoành.",
				HasVi:         true,
				TitleZh:       "青云试剑",
				BodyZh:        "剑气纵横。",
				HasZh:         true,
				BookTitleVi:   "Vấn Kiếm",
				Completed:     []int{1, 2},
			},
		},
	}

	st, err := newChapterReaderState(rt, 120, 40, []string{"1"})
	if err != nil {
		t.Fatalf("newChapterReaderState: %v", err)
	}
	m := Model{reader: st, width: 120, height: 40}

	outVi := ansi.Strip(renderReaderModal(120, 40, m.reader))
	if !strings.Contains(outVi, "Sơ Nhập Tông Môn") || !strings.Contains(outVi, "Lâm Mặc rút kiếm") || !strings.Contains(outVi, "có thể cũ") {
		t.Fatalf("vi view missing expected fields:\n%s", outVi)
	}

	// Bấm 'v' chuyển sang nguyên bản tiếng Trung.
	next, _ := m.handleReaderKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = next.(Model)
	outZh := ansi.Strip(renderReaderModal(120, 40, m.reader))
	if !strings.Contains(outZh, "Nguyên bản Trung") || !strings.Contains(outZh, "林默拔剑") {
		t.Fatalf("zh toggle failed:\n%s", outZh)
	}

	// Bấm ']' nhảy sang chương 2.
	next, _ = m.handleReaderKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = next.(Model)
	if m.reader.chapter != 2 {
		t.Fatalf("expected chapter 2, got %d", m.reader.chapter)
	}
}
