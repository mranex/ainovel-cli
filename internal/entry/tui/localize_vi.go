package tui

import (
	"regexp"
	"strings"

	"github.com/voocel/ainovel-cli/internal/host"
)

var (
	reZhVolArc  = regexp.MustCompile(`第\s*(\d+)\s*卷\s*[·・.\-]?\s*第\s*(\d+)\s*弧`)
	reZhVol     = regexp.MustCompile(`第\s*(\d+)\s*卷`)
	reZhArc     = regexp.MustCompile(`第\s*(\d+)\s*弧`)
	reZhChapter = regexp.MustCompile(`第\s*(\d+)\s*章`)
)

var streamHeaderViMap = map[string]string{
	"✻ 规划":    "✻ Lập dàn ý chương",
	"✻ 打磨":    "✻ Trau chuốt",
	"✻ 章节提交":  "✻ Nộp chương",
	"✻ 审阅":    "✻ Thẩm định",
	"✻ 返工复核":  "✻ Phúc khảo sửa lại",
	"✻ 弧摘要":   "✻ Tóm tắt Arc",
	"✻ 卷摘要":   "✻ Tóm tắt Quyển",
	"✻ 设定":    "✻ Thiết lập",
	"✻ 修订大纲":  "✻ Sửa đại cương",
	"✻ 读章节":   "✻ Đọc chương",
	"✻ 一致性检查": "✻ Kiểm tra nhất quán",
	"✻ 查询上下文": "✻ Tra ngữ cảnh",
}

// localizeStreamHeaderVi dịch tiêu đề khối stream `✻ ...` từ stream_extract sang tiếng Việt,
// giữ nguyên tiền tố `✻ ` để pipeline tô màu TUI tiếp tục nhận diện chính xác.
func localizeStreamHeaderVi(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "✻ ") {
		return line
	}
	if mapped, ok := streamHeaderViMap[trimmed]; ok {
		return mapped
	}
	return line
}

var commonZhReplacements = []struct{ old, new string }{
	{"等待模型", "Đang chờ model"},
	{"思考中", "Đang suy luận"},
	{"模型响应 错误: ", "Lỗi phản hồi model: "},
	{"模型响应", "Phản hồi model"},
	{"启动裁定已补齐(规划师: ", "Đã bổ sung phân xử khởi tạo (kiến trúc sư: "},
	{"启动裁定", "Phân xử khởi tạo"},
	{"创作需求: ", "Yêu cầu sáng tác: "},
	{"[用户干预] ", "[Chỉ thị người dùng] "},
	{"模型已切换：", "Đã chuyển model: "},
	{"推理强度已切换：", "Đã đổi mức suy luận: "},
	{"Provider 配置已保存：", "Đã lưu cấu hình Provider: "},
	{"；使用 /model 切换", "; dùng /model để chuyển"},
	{"进入阶段共创", "Đã vào Đồng sáng tạo chặng"},
	{"已退出阶段共创，创作保持暂停（可在输入框继续）", "Đã thoát Đồng sáng tạo chặng, truyện vẫn tạm dừng (nhập tiếp ở ô bên dưới)"},
	{"已重开本书为创作状态（用户撤销完结裁定）", "Đã mở lại truyện sang trạng thái sáng tác"},
	{"续写方向: ", "Hướng viết tiếp: "},
	{"已取消一次性暂停", "Đã hủy điểm tạm dừng một lần"},
	{"已设置一次性暂停: ", "Đã đặt điểm tạm dừng một lần: "},
	{"已重开全书返工: ", "Đã mở lại chỉnh sửa truyện: "},
	{"干预动作执行失败,已保留;恢复/继续时自动重试", "Thực thi thao tác can thiệp thất bại, đã lưu lại để tự thử lại khi tiếp tục"},
	{"引擎已停,裁定派单未执行;干预已保留,继续创作时自动重新裁定", "Engine đã dừng, lệnh điều phối chưa chạy; can thiệp đã được giữ lại"},
	{"裁定派单已过时(事实推进),以最新事实重新裁定", "Lệnh điều phối đã cũ theo diễn biến mới, sẽ phân xử lại"},
	{"裁定: ", "Phân xử: "},
	{"只读回答用户问题", "Trả lời câu hỏi (chỉ đọc)"},
	{"上下文预算告警", "Cảnh báo ngân sách ngữ cảnh"},
	{"流空闲超时", "Quá thời gian chờ luồng phản hồi (stream idle)"},
	{"基础设定缺项未齐，照缺项续派同一规划师", "Thiết lập nền còn thiếu, tiếp tục bổ sung"},
	{"按已固化的启动裁定开始规划", "Bắt đầu lập kế hoạch theo phân xử khởi tạo"},
	{"写作目标章未展开,先展开再续写", "Chương đích chưa khai triển, mở rộng trước rồi viết tiếp"},
	{"用户干预裁定", "Theo chỉ thị can thiệp của người dùng"},
	{"队列剩余 ", "hàng đợi còn "},
	{"正文返工已排空，定向复核尚未完成", "Đã viết lại xong, chờ phúc khảo lại"},
	{"弧级审阅缺失", "Thiếu thẩm định cấp Arc"},
	{"弧级摘要缺失", "Thiếu tóm tắt cấp Arc"},
	{"卷摘要缺失", "Thiếu tóm tắt cấp Quyển"},
	{"全局审阅缺失", "Thiếu thẩm định tổng thể"},
	{"弧末评审未完成", "Chưa hoàn tất thẩm định cuối Arc"},
	{"弧摘要未完成", "Chưa hoàn tất tóm tắt Arc"},
	{"卷摘要未完成", "Chưa hoàn tất tóm tắt Quyển"},
	{"下一弧骨架待展开", "Cần mở rộng khung Arc tiếp theo"},
	{"卷末需决定追加新卷、收官卷或结束全书", "Cuối Quyển: quyết định mở Quyển mới, Quyển kết hay hoàn tất"},
	{"非分层大纲已耗尽，需决定完结或续接", "Đã hết đại cương, cần quyết định kết thúc hay viết tiếp"},
	{"续写下一章", "Viết chương tiếp theo"},
	{"条外部修订影响尚未传播到后续规划", "chỉnh sửa bên ngoài chưa đồng bộ sang kế hoạch sau"},
	{"未发现外部修改，所有已完成章节与记录一致", "Không có chỉnh sửa bên ngoài, các chương đã khớp hoàn toàn"},
	{"章节外部修改待同步", "Có chương được sửa từ bên ngoài cần đồng bộ (/sync)"},
	{"创作完成", "Hoàn tất sáng tác"},
	{" 错误: ", " Lỗi: "},
}

func translateCommonZh(s string) string {
	if s == "" {
		return ""
	}
	out := s
	for _, r := range commonZhReplacements {
		out = strings.ReplaceAll(out, r.old, r.new)
	}
	if strings.HasPrefix(out, "生成 ") {
		out = "Đang tạo " + strings.TrimPrefix(out, "生成 ")
	}
	if strings.HasPrefix(out, "重试 (") {
		out = strings.ReplaceAll(out, "重试 (", "Thử lại (")
		out = strings.ReplaceAll(out, "次，", " lần, sau ")
		out = strings.ReplaceAll(out, "次)", " lần)")
		out = strings.ReplaceAll(out, "后):", "):")
	}
	out = reZhVolArc.ReplaceAllString(out, "Quyển $1 · Arc $2")
	out = reZhVol.ReplaceAllString(out, "Quyển $1")
	out = reZhArc.ReplaceAllString(out, "Arc $1")
	out = reZhChapter.ReplaceAllString(out, "Chương $1")
	return out
}

// localizeEventVi chuyển đổi nhãn/thông báo động do tầng host/engine/flow phát ra sang tiếng Việt.
func localizeEventVi(ev host.Event) host.Event {
	if ev.Category == "ANSWER" {
		return ev
	}
	ev.Summary = translateCommonZh(ev.Summary)
	return ev
}

// localizeSnapshotVi chuẩn hóa các trường hiển thị động trong UISnapshot sang tiếng Việt.
func localizeSnapshotVi(snap host.UISnapshot) host.UISnapshot {
	snap.CurrentVolumeArc = translateCommonZh(snap.CurrentVolumeArc)
	snap.RecoveryLabel = translateCommonZh(snap.RecoveryLabel)
	snap.LastCommitSummary = translateCommonZh(snap.LastCommitSummary)
	snap.LastReviewSummary = translateCommonZh(snap.LastReviewSummary)
	for i := range snap.Agents {
		snap.Agents[i].Summary = translateCommonZh(snap.Agents[i].Summary)
		if snap.Agents[i].Tool != "" {
			snap.Agents[i].Tool = translateCommonZh(snap.Agents[i].Tool)
		}
	}
	return snap
}
