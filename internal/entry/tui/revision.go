package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/revision"
)

type revisionDoneMsg struct {
	checkOnly bool
	chapters  []int
	result    *revision.Result
	err       error
}

func startRevisionSync(rt *host.Host, args []string) (tea.Cmd, bool, error) {
	checkOnly := false
	for _, arg := range args {
		switch arg {
		case "--check":
			checkOnly = true
		default:
			return nil, false, fmt.Errorf("tham số không hợp lệ %q (hỗ trợ: --check)", arg)
		}
	}
	return func() tea.Msg {
		if checkOnly {
			chapters, err := rt.CheckChapterRevisions()
			return revisionDoneMsg{checkOnly: true, chapters: chapters, err: err}
		}
		result, err := rt.SyncChapterRevisions(context.Background())
		return revisionDoneMsg{result: result, err: err}
	}, checkOnly, nil
}

func formatRevisionResult(result *revision.Result) string {
	if result == nil || len(result.Applied) == 0 {
		return "Không phát hiện chỉnh sửa bên ngoài trên các chương"
	}
	parts := make([]string, 0, len(result.Analyses))
	for i, analysis := range result.Analyses {
		if i >= len(result.Applied) {
			break
		}
		part := fmt.Sprintf("Chương %d: %s", result.Applied[i], analysis.ChangeSummary)
		if analysis.StoryChanged {
			part += " (đã cập nhật tình tiết)"
		}
		if len(analysis.DownstreamIssues) > 0 {
			part += fmt.Sprintf(" (phát hiện %d xung đột về sau)", len(analysis.DownstreamIssues))
		}
		parts = append(parts, part)
	}
	summary := fmt.Sprintf("Đã tiếp nhận chỉnh sửa chương: %v", result.Applied)
	if len(parts) > 0 {
		summary += "; " + strings.Join(parts, "; ")
	}
	return summary
}
