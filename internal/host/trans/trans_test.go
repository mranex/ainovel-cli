package trans

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

type fakeTransModel struct {
	calls   int64
	outputs []string
	prompts []string
}

func (m *fakeTransModel) Generate(_ context.Context, msgs []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	i := int(atomic.AddInt64(&m.calls, 1) - 1)
	if len(msgs) > 0 {
		m.prompts = append(m.prompts, msgs[0].TextContent()+"\n---\n"+msgs[len(msgs)-1].TextContent())
	}
	out := m.outputs[len(m.outputs)-1]
	if i < len(m.outputs) {
		out = m.outputs[i]
	}
	return &agentcore.LLMResponse{
		Message: agentcore.Message{
			Role:    agentcore.RoleAssistant,
			Content: []agentcore.ContentBlock{agentcore.TextBlock(out)},
		},
	}, nil
}

func setupTestBookStore(t *testing.T) *store.Store {
	t.Helper()
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Book.Save(domain.BookMetadata{
		Title:    "剑影",
		Synopsis: "少年林尘踏入仙途。",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Characters.Save([]domain.Character{
		{Name: "林尘", Role: "主角", Description: "坚毅少年"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init(2); err != nil {
		t.Fatal(err)
	}
	_ = st.Progress.UpdatePhase(domain.PhaseWriting)
	for _, ch := range []int{1, 2} {
		content := fmt.Sprintf("# 第 %d 章 启程\n\n第%d章正文内容，林尘拔剑出鞘。", ch, ch)
		if err := st.Drafts.SaveFinalChapter(ch, content); err != nil {
			t.Fatal(err)
		}
		if _, err := st.ChapterRecords.Accept(ch, domain.ChapterOriginGenerated, content, domain.ChapterFacts{
			Title:   fmt.Sprintf("启程%d", ch),
			Summary: "摘要",
		}, domain.StyleDelta{}); err != nil {
			t.Fatal(err)
		}
		_ = st.Summaries.SaveSummary(domain.ChapterSummary{
			Chapter: ch,
			Title:   fmt.Sprintf("启程%d", ch),
			Summary: "摘要",
		})
		_ = st.Progress.StartChapter(ch)
		_ = st.Progress.MarkChapterComplete(ch, 20, "hook", "main")
	}
	return st
}

func TestRun_SeedTranslateSkipAndRewriteInvalidation(t *testing.T) {
	st := setupTestBookStore(t)
	model := &fakeTransModel{
		outputs: []string{
			`<resolved_genre>tien_hiep</resolved_genre>
<book_title>Kiếm Ảnh</book_title>
<book_synopsis>Thiếu niên Lâm Trần bước vào tiên lộ.</book_synopsis>
<volumes></volumes>
<pronoun_guide>- Lâm Trần: ta - ngươi với địch thủ</pronoun_guide>
<glossary>
林尘 => Lâm Trần | character | nhân vật chính
</glossary>`,
			`<title>Khởi Hành Một</title>
<body>Nội dung chương 1, Lâm Trần rút kiếm khỏi vỏ.</body>
<glossary_updates>
青云剑 => Thanh Vân Kiếm | item | bội kiếm
</glossary_updates>`,
			`<title>Khởi Hành Hai</title>
<body>Nội dung chương 2, Lâm Trần tiến vào sơn môn.</body>
<glossary_updates></glossary_updates>`,
		},
	}

	res, err := Run(t.Context(), Deps{
		Store:   st,
		Model:   model,
		Profile: domain.TranslationProfile{Genre: domain.TransGenreTienHiep, Tone: domain.TransToneNormal},
	}, Options{})
	if err != nil {
		t.Fatalf("Run 1 failed: %v", err)
	}
	if len(res.TranslatedChapters) != 2 || res.BookTitleVi != "Kiếm Ảnh" {
		t.Fatalf("unexpected result 1: %+v", res)
	}

	// Kiểm tra glossary đã lưu cả thuật ngữ seed lẫn mới phát hiện ở chương 1.
	g, err := LoadGlossary(st.Dir())
	if err != nil || g == nil || len(g.Entries) != 2 {
		t.Fatalf("expected 2 glossary entries, got %+v err=%v", g, err)
	}

	// Chạy lại lần 2 khi SHA không đổi -> skip cả 2 chương, không gọi model thêm.
	callsBefore := atomic.LoadInt64(&model.calls)
	res2, err := Run(t.Context(), Deps{
		Store:   st,
		Model:   model,
		Profile: domain.TranslationProfile{Genre: domain.TransGenreTienHiep, Tone: domain.TransToneNormal},
	}, Options{})
	if err != nil {
		t.Fatalf("Run 2 failed: %v", err)
	}
	if len(res2.SkippedChapters) != 2 || len(res2.TranslatedChapters) != 0 {
		t.Fatalf("expected both chapters skipped, got %+v", res2)
	}
	if atomic.LoadInt64(&model.calls) != callsBefore {
		t.Fatalf("expected 0 extra model calls on skip")
	}

	// Giả lập Editor bắt viết lại chương 1 -> nội dung & SHA256 đổi -> chương 1 tự động dịch lại, chương 2 vẫn skip!
	newCh1 := "# 第 1 章 改写\n\n林尘重新铸剑，气势如虹。"
	if err := st.Drafts.SaveFinalChapter(1, newCh1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChapterRecords.Accept(1, domain.ChapterOriginGenerated, newCh1, domain.ChapterFacts{Title: "改写"}, domain.StyleDelta{}); err != nil {
		t.Fatal(err)
	}
	_ = st.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "改写", Summary: "摘要"})

	status := Inspect(st, domain.TranslationProfile{Genre: domain.TransGenreTienHiep})
	if status.OutdatedCount != 1 || status.PendingCount != 1 || status.TranslatedCount != 1 {
		t.Fatalf("unexpected status after rewrite: %+v", status)
	}

	model.outputs = append(model.outputs, `<title>Viết Lại</title>
<body>Lâm Trần đúc lại kiếm, khí thế như cầu vồng.</body>
<glossary_updates></glossary_updates>`)

	res3, err := Run(t.Context(), Deps{
		Store:   st,
		Model:   model,
		Profile: domain.TranslationProfile{Genre: domain.TransGenreTienHiep, Tone: domain.TransToneNormal},
	}, Options{})
	if err != nil {
		t.Fatalf("Run 3 failed: %v", err)
	}
	if len(res3.TranslatedChapters) != 1 || res3.TranslatedChapters[0] != 1 || len(res3.SkippedChapters) != 1 {
		t.Fatalf("expected only ch 1 retranslated, got %+v", res3)
	}

	titleVi, bodyVi, ok, err := LoadChapterVi(st.Dir(), 1)
	if err != nil || !ok || titleVi != "Viết Lại" || !strings.Contains(bodyVi, "đúc lại kiếm") {
		t.Fatalf("unexpected ch1 vi: title=%q body=%q ok=%v err=%v", titleVi, bodyVi, ok, err)
	}

	// Force translate với Fantasy + Mature -> prompt chứa rule khử Hán-Việt & 18+.
	model.outputs = append(model.outputs, `<resolved_genre>fantasy</resolved_genre>
<book_title>Shadow Blade</book_title>
<book_synopsis>...</book_synopsis>
<volumes></volumes>
<pronoun_guide>- tôi / cô ấy</pronoun_guide>
<glossary>林尘 => Lin Chen | character</glossary>`,
		`<title>Chương Một</title><body>Bản dịch fantasy mature.</body><glossary_updates></glossary_updates>`,
		`<title>Chương Hai</title><body>Bản dịch fantasy mature 2.</body><glossary_updates></glossary_updates>`,
	)
	res4, err := Run(t.Context(), Deps{
		Store:   st,
		Model:   model,
		Profile: domain.TranslationProfile{Genre: domain.TransGenreFantasy, Tone: domain.TransToneMature},
	}, Options{Force: true})
	if err != nil || len(res4.TranslatedChapters) != 2 {
		t.Fatalf("force run failed: %+v err=%v", res4, err)
	}
	lastPrompt := model.prompts[len(model.prompts)-1]
	if !strings.Contains(lastPrompt, "KHỬ HÁN-VIỆT TÊN TÂY") || !strings.Contains(lastPrompt, "18+ Trưởng thành") {
		t.Fatalf("prompt missing fantasy/mature instructions:\n%s", lastPrompt)
	}
	if _, err := os.Stat(filepath.Join(st.Dir(), filepath.FromSlash(ChapterRelPath(1)))); err != nil {
		t.Fatalf("missing translated file: %v", err)
	}
}
