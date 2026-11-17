package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/host/exp"
)

// exportDoneMsg 是 /export 命令的最终结果。
//
// 不像 /import 走事件流：导出是同步本地 IO，没有中间进度可言；
// 在 goroutine 里跑完后一次性回投这条消息。
type exportDoneMsg struct {
	result *exp.Result
	err    error
}

// startExport 解析参数并返回 tea.Cmd。
// 真正的导出在 tea.Cmd 里跑（避免阻塞 UI），完成后投递 exportDoneMsg。
func startExport(rt *host.Host, args []string) (tea.Cmd, error) {
	opts, err := parseExportArgs(args)
	if err != nil {
		return nil, err
	}
	return func() tea.Msg {
		res, err := rt.Export(context.Background(), opts)
		return exportDoneMsg{result: res, err: err}
	}, nil
}

// parseExportArgs phân tích `/export [path] [lang=vi|zh|bilingual] [from=N] [to=M] [--overwrite]`.
//
// Tham số vị trí: tối đa một đường dẫn đích; để trống sẽ xuất về thư mục truyện.
func parseExportArgs(args []string) (exp.Options, error) {
	var opts exp.Options
	for _, a := range args {
		if a == "--overwrite" {
			opts.Overwrite = true
			continue
		}
		if k, v, ok := strings.Cut(a, "="); ok {
			switch strings.ToLower(k) {
			case "from":
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 {
					return exp.Options{}, fmt.Errorf("from phải là số nguyên không âm: %q", v)
				}
				opts.From = n
			case "to":
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 {
					return exp.Options{}, fmt.Errorf("to phải là số nguyên không âm: %q", v)
				}
				opts.To = n
			case "lang":
				switch strings.ToLower(strings.TrimSpace(v)) {
				case "", "auto":
					opts.Lang = exp.LangAuto
				case "vi":
					opts.Lang = exp.LangVI
				case "zh":
					opts.Lang = exp.LangZH
				case "bilingual", "songngu":
					opts.Lang = exp.LangBilingual
				default:
					return exp.Options{}, fmt.Errorf("lang không hợp lệ %q (hỗ trợ: vi / zh / bilingual / auto)", v)
				}
			default:
				return exp.Options{}, fmt.Errorf("tham số không hợp lệ %q (hỗ trợ: lang / from / to)", k)
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			return exp.Options{}, fmt.Errorf("cờ không hợp lệ %q", a)
		}
		if opts.OutPath != "" {
			return exp.Options{}, fmt.Errorf("chỉ hỗ trợ một đường dẫn đầu ra: %q", a)
		}
		opts.OutPath = a
	}
	return opts, nil
}

// formatExportSuccess định dạng kết quả xuất thành tóm tắt sự kiện.
func formatExportSuccess(res *exp.Result) string {
	var b strings.Builder
	langLabel := ""
	switch res.Lang {
	case exp.LangVI:
		langLabel = " [Tiếng Việt]"
	case exp.LangZH:
		langLabel = " [Tiếng Trung]"
	case exp.LangBilingual:
		langLabel = " [Song ngữ Việt-Trung]"
	}
	fmt.Fprintf(&b, "✓ Đã xuất%s %d chương / %s → %s", langLabel, res.Chapters, humanBytes(res.Bytes), res.Path)
	if n := len(res.Untranslated); n > 0 {
		fmt.Fprintf(&b, " (còn %d chương chưa dịch, giữ bản Trung: %s)", n, briefIntList(res.Untranslated, 5))
	}
	if n := len(res.Skipped); n > 0 {
		fmt.Fprintf(&b, " (bỏ qua %d chương chưa xong: %s)", n, briefIntList(res.Skipped, 5))
	}
	return b.String()
}

func humanBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func briefIntList(xs []int, max int) string {
	if len(xs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(xs))
	for i, x := range xs {
		if i >= max {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, strconv.Itoa(x))
	}
	return strings.Join(parts, ",")
}
