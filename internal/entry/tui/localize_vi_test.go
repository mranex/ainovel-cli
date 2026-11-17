package tui

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/host"
)

func TestLocalizeEventVi(t *testing.T) {
	ev := localizeEventVi(host.Event{
		Category: "MODEL",
		Summary:  "等待模型",
	})
	if ev.Summary != "Đang chờ model" {
		t.Fatalf("MODEL event summary = %q", ev.Summary)
	}

	disp := localizeEventVi(host.Event{
		Category: "DISPATCH",
		Summary:  "writer（续写下一章）",
	})
	if disp.Summary != "writer（Viết chương tiếp theo）" {
		t.Fatalf("DISPATCH summary = %q", disp.Summary)
	}

	chErr := localizeEventVi(host.Event{
		Category: "ERROR",
		Summary:  "draft_chapter(第12章) 错误: 超时",
	})
	if !strings.Contains(chErr.Summary, "Chương 12") || !strings.Contains(chErr.Summary, "Lỗi:") {
		t.Fatalf("error event summary = %q", chErr.Summary)
	}
}

func TestLocalizeStreamHeaderViPreservesPrefix(t *testing.T) {
	got := localizeStreamHeaderVi("✻ 章节提交")
	if !strings.HasPrefix(got, "✻ ") || !strings.Contains(got, "Nộp chương") {
		t.Fatalf("stream header = %q", got)
	}
}

func TestLocalizeSnapshotVi(t *testing.T) {
	snap := localizeSnapshotVi(host.UISnapshot{
		CurrentVolumeArc: "第2卷·第3弧",
		Agents: []host.AgentSnapshot{
			{Name: "writer", Summary: "思考中"},
		},
	})
	if snap.CurrentVolumeArc != "Quyển 2 · Arc 3" {
		t.Fatalf("CurrentVolumeArc = %q", snap.CurrentVolumeArc)
	}
	if snap.Agents[0].Summary != "Đang suy luận" {
		t.Fatalf("Agent summary = %q", snap.Agents[0].Summary)
	}
}
