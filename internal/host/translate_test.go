package host

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host/trans"
	"github.com/voocel/ainovel-cli/internal/store"
)

type fakeHostTranslateModel struct {
	replies []string
	idx     int
}

func (f *fakeHostTranslateModel) Generate(_ context.Context, _ []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	out := "<translation><title>Không có</title><body>Nội dung giả lập</body><new_terms>[]</new_terms></translation>"
	if f.idx < len(f.replies) {
		out = f.replies[f.idx]
		f.idx++
	}
	return &agentcore.LLMResponse{
		Message: agentcore.Message{
			Role:    agentcore.RoleAssistant,
			Content: []agentcore.ContentBlock{agentcore.TextBlock(out)},
		},
	}, nil
}

func (f *fakeHostTranslateModel) GenerateStream(ctx context.Context, msgs []agentcore.Message, tools []agentcore.ToolSpec, opts ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	resp, err := f.Generate(ctx, msgs, tools, opts...)
	if err != nil {
		return nil, err
	}
	ch := make(chan agentcore.StreamEvent, 2)
	ch <- agentcore.StreamEvent{Type: agentcore.StreamEventDone, Message: resp.Message}
	close(ch)
	return ch, nil
}

func (f *fakeHostTranslateModel) SupportsTools() bool { return false }

func TestApprovedTranslationUpTo_LayeredArcGate(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := st.Outline.SaveLayeredOutline([]domain.VolumeOutline{
		{
			Index: 1,
			Title: "第一卷",
			Arcs: []domain.ArcOutline{
				{
					Index: 1,
					Title: "第一弧",
					Chapters: []domain.OutlineEntry{
						{Chapter: 1, Title: "第一章"},
						{Chapter: 2, Title: "第二章"},
					},
				},
				{
					Index: 2,
					Title: "第二弧",
					Chapters: []domain.OutlineEntry{
						{Chapter: 3, Title: "第三章"},
						{Chapter: 4, Title: "第四章"},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("save outline: %v", err)
	}
	if err := st.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		CurrentChapter:    3,
		Layered:           true,
		CompletedChapters: []int{1, 2, 3},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	if got := approvedTranslationUpTo(st); got != 0 {
		t.Fatalf("before arc review: got %d, want 0", got)
	}

	if err := st.World.SaveReview(domain.ReviewEntry{
		Chapter: 2,
		Scope:   "arc",
		Verdict: "accept",
	}); err != nil {
		t.Fatalf("save arc review: %v", err)
	}
	if got := approvedTranslationUpTo(st); got != 2 {
		t.Fatalf("after arc 1 review: got %d, want 2", got)
	}

	// Khi có rewrite đang treo ở chương 2 -> chưa được phê duyệt.
	if err := st.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		CurrentChapter:    3,
		Layered:           true,
		CompletedChapters: []int{1, 2, 3},
		PendingRewrites:   []int{2},
	}); err != nil {
		t.Fatalf("save progress pending rewrite: %v", err)
	}
	if got := approvedTranslationUpTo(st); got != 0 {
		t.Fatalf("with pending rewrite ch 2: got %d, want 0", got)
	}

	// Xong hết khi PhaseComplete & không còn rewrite.
	if err := st.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseComplete,
		CurrentChapter:    4,
		Layered:           true,
		CompletedChapters: []int{1, 2, 3, 4},
	}); err != nil {
		t.Fatalf("save complete: %v", err)
	}
	if got := approvedTranslationUpTo(st); got != 4 {
		t.Fatalf("on complete: got %d, want 4", got)
	}
}

func TestHostStartTranslateAndReadChapter(t *testing.T) {
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	_ = st.RunMeta.Init("default", "fake", "fake-trans")
	_ = st.Book.Save(domain.BookMetadata{Title: "问剑长生", Synopsis: "少年入宗问道。"})
	_ = st.Characters.Save([]domain.Character{{Name: "林默", Role: "protagonist", Description: "主角"}})
	_ = st.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		CurrentChapter:    1,
		TotalChapters:     10,
		CompletedChapters: []int{1},
	})
	_ = st.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "初入宗门"})
	_ = st.Drafts.SaveFinalChapter(1, "# 第一章 初入宗门\n\n青石山道尽头，少年林默负剑而立。")

	fake := &fakeHostTranslateModel{
		replies: []string{
			`<resolved_genre>tien_hiep</resolved_genre>
<book_title>Vấn Kiếm Trường Sinh</book_title>
<book_synopsis>Thiếu niên bước vào tiên tông vấn đạo.</book_synopsis>
<volumes></volumes>
<pronoun_guide>Mặc định ta-ngươi cổ trang.</pronoun_guide>
<glossary>
林默 => Lâm Mặc | character | nhân vật chính
</glossary>`,
			`<title>Sơ Nhập Tông Môn</title>
<body>Cuối con đường đá xanh, thiếu niên Lâm Mặc vác kiếm đứng sừng sững.</body>
<glossary_updates></glossary_updates>`,
		},
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &Host{
		store:     st,
		models:    &bootstrap.ModelSet{Default: bootstrap.NewSwappableModel("fake", "fake-trans", fake, nil)},
		events:    make(chan Event, 32),
		runCtx:    runCtx,
		runCancel: cancel,
	}

	if err := h.SetTranslationProfile(domain.TranslationProfile{
		Genre:         domain.TransGenreTienHiep,
		Tone:          domain.TransToneMature,
		AutoTranslate: domain.AutoTranslateAfterCommit,
	}, false); err != nil {
		t.Fatalf("SetTranslationProfile: %v", err)
	}

	if err := h.StartTranslate(context.Background(), trans.Options{From: 1, To: 1}); err != nil {
		t.Fatalf("StartTranslate: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap := h.translationSnapshot()
		if !snap.Running && snap.TranslatedCount == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	snap := h.translationSnapshot()
	if snap.TranslatedCount != 1 {
		t.Fatalf("TranslatedCount = %d, want 1", snap.TranslatedCount)
	}
	if snap.BookTitleVi != "Vấn Kiếm Trường Sinh" {
		t.Fatalf("BookTitleVi = %q", snap.BookTitleVi)
	}

	view, err := h.ReadChapter(1, true)
	if err != nil {
		t.Fatalf("ReadChapter: %v", err)
	}
	if !view.HasVi || !view.HasZh || view.OutdatedVi {
		t.Fatalf("unexpected ReadChapter flags: %+v", view)
	}
	if !strings.Contains(view.BodyVi, "Lâm Mặc") || view.TitleVi != "Sơ Nhập Tông Môn" {
		t.Fatalf("unexpected Vi chapter view: %+v", view)
	}
}
