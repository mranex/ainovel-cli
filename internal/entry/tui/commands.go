package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/entry/startup"
	"github.com/voocel/ainovel-cli/internal/host"
)

type slashCommandSpec struct {
	Name        string
	Aliases     []string
	Group       string
	Usage       string
	Description string
	AutoExecute bool
	Hidden      bool
	NeedsIdle   bool
	Run         func(m Model, args []string) (tea.Model, tea.Cmd)
}

type slashCommand struct {
	name string
	args []string
}

func parseSlashCommand(text string) (slashCommand, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return slashCommand{}, false
	}
	fields := strings.Fields(strings.TrimPrefix(text, "/"))
	if len(fields) == 0 {
		return slashCommand{}, false
	}
	return slashCommand{name: strings.ToLower(fields[0]), args: fields[1:]}, true
}

func (s slashCommandSpec) matches(name string) bool {
	if s.Name == name {
		return true
	}
	for _, alias := range s.Aliases {
		if strings.EqualFold(alias, name) {
			return true
		}
	}
	return false
}

func commandRegistryInstance() commandRegistry {
	return newCommandRegistry([]slashCommandSpec{
		{
			Name:        "help",
			Group:       "system",
			Usage:       "/help",
			Description: "Xem danh sách lệnh và phím tắt",
			AutoExecute: true,
			Run: func(m Model, _ []string) (tea.Model, tea.Cmd) {
				m.help = newHelpState(m.width, m.height)
				m.textarea.Blur()
				return m, nil
			},
		},
		{
			Name:        "model",
			Group:       "system",
			Usage:       "/model [role]",
			Description: "Đổi model & mức suy luận cho từng vai trò (architect / writer / editor / translator)",
			AutoExecute: true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				roleHint := ""
				if len(args) > 0 {
					roleHint = args[0]
					if normalizeRoleKey(roleHint) == "" {
						m.applyEvent(host.Event{
							Time: time.Now(), Category: "ERROR", Summary: "Vai trò không hợp lệ: " + roleHint, Level: "error",
						})
						m.refreshEventViewport()
						return m, nil
					}
				}
				m.modelSwitch = newModelSwitchState(m.runtime, roleHint)
				m.textarea.Blur()
				return m, nil
			},
		},
		{
			Name:        "config",
			Group:       "system",
			Usage:       "/config",
			Description: "Thêm hoặc sửa Provider, model và cửa sổ ngữ cảnh",
			AutoExecute: true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				if len(args) != 0 {
					m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cú pháp: /config", Level: "error"})
					m.refreshEventViewport()
					return m, nil
				}
				m.modelConfig = newModelConfigState(m.runtime)
				m.textarea.Blur()
				return m, nil
			},
		},
		{
			Name:        "trans",
			Aliases:     []string{"mode"},
			Group:       "writing",
			Usage:       "/trans [genre=auto|tien_hiep|co_trang|fantasy|hien_dai] [tone=normal|mature] [auto=after_arc|after_commit|manual] [--global]",
			Description: "Mở công tắc thể loại dịch, chế độ 18+ (normal/mature) và tự động dịch Việt",
			AutoExecute: true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				if len(args) > 0 {
					if err := applyTransArgs(m.runtime, args); err != nil {
						m.applyEvent(host.Event{
							Time: time.Now(), Category: "ERROR", Summary: "Cập nhật công tắc dịch thất bại: " + err.Error(), Level: "error",
						})
						m.refreshEventViewport()
						return m, nil
					}
					return m, fetchSnapshot(m.runtime)
				}
				m.transSwitch = newTransSwitchState(m.runtime)
				m.textarea.Blur()
				return m, nil
			},
		},
		{
			Name:        "translate",
			Aliases:     []string{"dich"},
			Group:       "writing",
			Usage:       "/translate [chương | N-M] [from=N] [to=M] [--force]",
			Description: "Dịch các chương đã hoàn thành sang tiếng Việt (bỏ qua chương đã khớp bản mới nhất trừ khi --force)",
			AutoExecute: true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				opts, err := startTranslate(m.runtime, args)
				if err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Không thể bắt đầu dịch: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				scope := "toàn bộ chương chưa dịch"
				if opts.From > 0 && opts.To > 0 {
					if opts.From == opts.To {
						scope = fmt.Sprintf("chương %d", opts.From)
					} else {
						scope = fmt.Sprintf("chương %d–%d", opts.From, opts.To)
					}
				} else if opts.To > 0 {
					scope = fmt.Sprintf("tới chương %d", opts.To)
				} else if opts.From > 0 {
					scope = fmt.Sprintf("từ chương %d", opts.From)
				}
				if opts.Force {
					scope += " (dịch lại cưỡng bức)"
				}
				m.applyEvent(host.Event{
					Time: time.Now(), Category: "SYSTEM", Summary: "Đã khởi chạy dịch tiếng Việt ở nền: " + scope, Level: "info",
				})
				m.refreshEventViewport()
				return m, fetchSnapshot(m.runtime)
			},
		},
		{
			Name:        "read",
			Aliases:     []string{"doc"},
			Group:       "writing",
			Usage:       "/read [chương]",
			Description: "Mở trình đọc truyện ngay trong TUI (bấm v để chuyển Việt/Trung, ←→ đổi chương)",
			AutoExecute: true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				reader, err := newChapterReaderState(m.runtime, m.width, m.height, args)
				if err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Không mở được trình đọc: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				m.reader = reader
				m.textarea.Blur()
				return m, nil
			},
		},
		{
			Name:        "diag",
			Group:       "analysis",
			Usage:       "/diag",
			Description: "Chẩn đoán sức khỏe bộ truyện và mức cache prompt",
			AutoExecute: true,
			Run: func(m Model, _ []string) (tea.Model, tea.Cmd) {
				m.reportSeq++
				m.report = newReportState(m.width, m.height, m.reportSeq, time.Now())
				m.textarea.Blur()
				return m, loadReport(m.runtime.Dir(), m.reportSeq)
			},
		},
		{
			Name:        "review",
			Group:       "writing",
			Usage:       "/review on|off",
			Description: "Bật/tắt chế độ dừng nghiệm thu sau từng chương",
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
					m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cú pháp: /review on|off", Level: "error"})
					m.refreshEventViewport()
					return m, nil
				}
				mode := domain.ChapterAdvanceReview
				if args[0] == "off" {
					mode = domain.ChapterAdvanceAuto
				}
				if err := m.runtime.SetAdvanceMode(mode); err != nil {
					m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Đổi chế độ duyệt thất bại: " + err.Error(), Level: "error"})
					m.refreshEventViewport()
					return m, nil
				}
				return m, fetchSnapshot(m.runtime)
			},
		},
		{
			Name:        "next",
			Group:       "writing",
			Usage:       "/next",
			Description: "Mở khóa viết tiếp một chương sau khi nghiệm thu",
			AutoExecute: true,
			NeedsIdle:   true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				if len(args) != 0 {
					m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Cú pháp: /next", Level: "error"})
					m.refreshEventViewport()
					return m, nil
				}
				if err := m.runtime.AdvanceOneChapter(); err != nil {
					m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Mở khóa chương tiếp thất bại: " + err.Error(), Level: "error"})
					m.refreshEventViewport()
					return m, nil
				}
				return m, tea.Batch(fetchSnapshot(m.runtime), listenDone(m.runtime), m.textarea.Focus())
			},
		},
		{
			Name:        "start",
			Group:       "writing",
			Usage:       "/start <path>",
			Description: "Tạo truyện mới từ file thiết lập hoặc đại cương",
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				if m.mode != modeNew {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "/start chỉ dùng được ở màn hình chào mừng để tạo truyện mới", Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				prompt, err := prepareFileStart(args)
				if err != nil {
					m.err = err
					return m, nil
				}
				cmd := m.enterStarting(prompt)
				return m, tea.Batch(startRuntime(m.runtime, prompt), cmd)
			},
		},
		{
			Name:        "import",
			Group:       "writing",
			Usage:       "/import <path> [--yes] [--story=open|closed] [--continue] [--guide=<hướng dẫn>]",
			Description: "Nhập tiểu thuyết có sẵn từ bên ngoài (không truyền tham số để chạy tiếp bản dở)",
			NeedsIdle:   true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				m.importSeq++
				state, listenCmd, err := startImport(m.runtime, m.importSeq, args, m.width, m.height)
				if err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Khởi chạy nhập truyện thất bại: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				m.importer = state
				m.importHint = "" // 已进入导入流程，欢迎屏的恢复提示完成使命
				m.textarea.Blur()
				return m, listenCmd
			},
		},
		{
			Name:        "reopen",
			Group:       "writing",
			Usage:       "/reopen [hướng viết tiếp]",
			Description: "Mở lại bộ truyện đã hoàn kết để viết tiếp hoặc chỉnh sửa",
			NeedsIdle:   true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				if err := m.runtime.Reopen(strings.Join(args, " ")); err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Mở lại truyện thất bại: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				return m, tea.Batch(m.textarea.Focus(), resumeBook(m.runtime))
			},
		},
		{
			Name:        "cocreate",
			Aliases:     []string{"plan"},
			Group:       "writing",
			Usage:       "/cocreate",
			Description: "Tạm dừng viết để đồng sáng tạo và chốt hướng đi cho chặng tiếp theo",
			AutoExecute: true,
			Run: func(m Model, _ []string) (tea.Model, tea.Cmd) {
				if m.mode != modeRunning {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Đồng sáng tạo chặng chỉ dùng khi bộ truyện đang chạy", Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				if !m.runtime.PauseForCoCreate() {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Không thể vào đồng sáng tạo chặng: truyện đã xong hoặc đang đồng sáng tạo", Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				m.cocreate = newStageCoCreateState()
				m.resizeTextarea()
				m.textarea.Blur()
				return m, m.sendCoCreate()
			},
		},
		{
			Name:        "simulate",
			Group:       "writing",
			Usage:       "/simulate",
			Description: "Đọc thư mục ./simulate để tạo hoặc cập nhật hồ sơ văn phong mô phỏng",
			NeedsIdle:   true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				m.simSeq++
				state, listenCmd, err := startSimulate(m.runtime, m.simSeq, args, m.width, m.height)
				if err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Khởi chạy mô phỏng văn phong thất bại: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				m.simulator = state
				m.textarea.Blur()
				return m, listenCmd
			},
		},
		{
			Name:        "importsim",
			Group:       "writing",
			Usage:       "/importsim <profile.json>",
			Description: "Nhập hồ sơ văn phong mô phỏng có sẵn và gộp theo ngữ liệu",
			NeedsIdle:   true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				m.simSeq++
				state, listenCmd, err := startImportSimulation(m.runtime, m.simSeq, args, m.width, m.height)
				if err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Nhập hồ sơ văn phong thất bại: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				m.simulator = state
				m.textarea.Blur()
				return m, listenCmd
			},
		},
		{
			Name:        "sync",
			Group:       "writing",
			Usage:       "/sync [--check]",
			Description: "Kiểm tra hoặc tiếp nhận các chỉnh sửa thủ công ngoài luồng trên chương đã viết",
			AutoExecute: true,
			NeedsIdle:   true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				cmd, checkOnly, err := startRevisionSync(m.runtime, args)
				if err != nil {
					m.applyEvent(host.Event{Time: time.Now(), Category: "ERROR", Summary: "Khởi chạy đồng bộ chương thất bại: " + err.Error(), Level: "error"})
					m.refreshEventViewport()
					return m, nil
				}
				summary := "Đang phân tích và tiếp nhận chỉnh sửa chương..."
				if checkOnly {
					summary = "Đang kiểm tra các chỉnh sửa bên ngoài trên chương..."
				}
				m.applyEvent(host.Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: "info"})
				m.refreshEventViewport()
				return m, cmd
			},
		},
		{
			Name:        "export",
			Group:       "writing",
			Usage:       "/export [path] [lang=vi|zh|bilingual] [from=N] [to=M] [--overwrite]",
			Description: "Xuất các chương đã hoàn thành ra file TXT hoặc EPUB (Việt / Trung / Song ngữ)",
			AutoExecute: true,
			Run: func(m Model, args []string) (tea.Model, tea.Cmd) {
				cmd, err := startExport(m.runtime, args)
				if err != nil {
					m.applyEvent(host.Event{
						Time: time.Now(), Category: "ERROR", Summary: "Khởi chạy xuất file thất bại: " + err.Error(), Level: "error",
					})
					m.refreshEventViewport()
					return m, nil
				}
				m.applyEvent(host.Event{
					Time: time.Now(), Category: "SYSTEM", Summary: "Đang xuất file...", Level: "info",
				})
				m.refreshEventViewport()
				return m, cmd
			},
		},
	})
}

func commandSpecs() []slashCommandSpec {
	return commandRegistryInstance().Visible()
}

func prepareFileStart(args []string) (string, error) {
	path := strings.TrimSpace(strings.Join(args, " "))
	if len(path) >= 2 && ((path[0] == '"' && path[len(path)-1] == '"') ||
		(path[0] == '\'' && path[len(path)-1] == '\'')) {
		path = path[1 : len(path)-1]
	}
	if path == "" {
		return "", fmt.Errorf("cú pháp: /start <đường dẫn file thiết lập hoặc đại cương>")
	}
	prompt, err := startup.LoadPromptFile(path)
	if err != nil {
		return "", err
	}
	return startup.PrepareQuick(prompt)
}

func (m Model) handleSlashCommand(cmd slashCommand) (tea.Model, tea.Cmd) {
	spec, ok := commandRegistryInstance().Find(cmd.name)
	if !ok {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Lệnh không tồn tại: /" + cmd.name, Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	if unavailable := m.commandUnavailable(spec.NeedsIdle); unavailable != "" {
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "ERROR", Summary: "Lệnh tạm chưa dùng được: /" + spec.Name + "; " + unavailable, Level: "error",
		})
		m.refreshEventViewport()
		return m, nil
	}
	return spec.Run(m, cmd.args)
}
