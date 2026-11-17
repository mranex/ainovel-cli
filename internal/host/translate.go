package host

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/flow"
	"github.com/voocel/ainovel-cli/internal/host/trans"
	storepkg "github.com/voocel/ainovel-cli/internal/store"
)

// ChapterReadView cung cấp dữ liệu song ngữ cho trình đọc chương trong TUI (/read).
type ChapterReadView struct {
	Chapter       int
	TotalChapters int
	TitleVi       string
	BodyVi        string
	HasVi         bool
	OutdatedVi    bool
	TitleZh       string
	BodyZh        string
	HasZh         bool
	BookTitleVi   string
	BookTitleZh   string
	Completed     []int
}

// TranslationProfile trả về cấu hình dịch Việt hiệu lực của cuốn truyện hiện tại
// (lấy riêng trong meta/run.json, trường nào bỏ trống sẽ fallback về config toàn cục).
func (h *Host) TranslationProfile() domain.TranslationProfile {
	if h == nil {
		return (domain.TranslationProfile{}).Normalized()
	}
	h.mu.Lock()
	fallback := h.cfg.DefaultTranslationProfile()
	st := h.store
	h.mu.Unlock()
	if st == nil || st.RunMeta == nil {
		return fallback.Normalized()
	}
	profile, err := st.RunMeta.LoadTranslationProfile(fallback)
	if err != nil {
		return fallback.Normalized()
	}
	return profile
}

// SetTranslationProfile cập nhật bộ công tắc dịch Việt cho bộ truyện hiện tại
// (và tùy chọn lưu làm mặc định toàn cục trong config.json).
func (h *Host) SetTranslationProfile(profile domain.TranslationProfile, saveGlobal bool) error {
	if h == nil {
		return fmt.Errorf("host is nil")
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	norm := profile.Normalized()

	h.mu.Lock()
	st := h.store
	h.mu.Unlock()

	if st != nil && st.RunMeta != nil {
		if err := st.RunMeta.SetTranslationProfile(norm); err != nil {
			return fmt.Errorf("lưu công tắc dịch truyện: %w", err)
		}
	}

	if saveGlobal {
		h.mu.Lock()
		h.cfg.Translate = bootstrap.TranslateConfig{
			Genre:         string(norm.Genre),
			Tone:          string(norm.Tone),
			AutoTranslate: string(norm.AutoTranslate),
		}
		cfgPath := h.configPath
		cfgCopy := h.cfg
		h.mu.Unlock()
		if cfgPath != "" {
			if err := bootstrap.SaveConfig(cfgPath, cfgCopy); err != nil {
				return fmt.Errorf("lưu cấu hình dịch mặc định: %w", err)
			}
		}
	}

	suffix := "chỉ truyện này"
	if saveGlobal {
		suffix = "truyện này + mặc định toàn cục"
	}
	h.emitEvent(Event{
		Time:     time.Now(),
		Category: "SYSTEM",
		Summary:  fmt.Sprintf("Chế độ dịch (%s): thể loại=%s | sắc thái=%s | tự động=%s", suffix, norm.Genre, norm.Tone, norm.AutoTranslate),
		Level:    "info",
	})
	return nil
}

// translationSnapshot dựng ảnh chụp tiến độ dịch hiện tại cho UISnapshot.
func (h *Host) translationSnapshot() trans.StatusSummary {
	profile := h.TranslationProfile()
	if h == nil || h.store == nil {
		return trans.StatusSummary{Profile: profile}
	}
	summary := trans.Inspect(h.store, profile)
	h.transMu.Lock()
	summary.Running = h.transRunning
	summary.CurrentChapter = h.transCurrentCh
	h.transMu.Unlock()
	return summary
}

// StartTranslate khởi chạy tiến trình dịch sang tiếng Việt ở nền (không chặn vòng viết chính).
func (h *Host) StartTranslate(ctx context.Context, opts trans.Options) error {
	if h == nil || h.store == nil {
		return fmt.Errorf("chưa có bộ truyện đang mở để dịch")
	}
	if _, _, _, err := h.translatorModelRuntime(); err != nil {
		return err
	}
	return h.launchTranslate(ctx, opts, false)
}

// enqueueAutoTranslateUpTo xếp hàng tự động dịch tối đa tới chương upTo (nếu đang chạy thì gộp mốc cao hơn).
func (h *Host) enqueueAutoTranslateUpTo(upTo int) {
	if upTo <= 0 || h == nil || h.store == nil {
		return
	}
	if _, _, _, err := h.translatorModelRuntime(); err != nil {
		return
	}
	_ = h.launchTranslate(h.runCtx, trans.Options{To: upTo}, true)
}

func (h *Host) launchTranslate(ctx context.Context, opts trans.Options, autoMode bool) error {
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return fmt.Errorf("phiên làm việc đang đóng")
	}
	fallbackCtx := h.runCtx
	h.mu.Unlock()

	if ctx == nil {
		ctx = fallbackCtx
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h.transMu.Lock()
	if h.transRunning {
		if autoMode && opts.From == 0 && opts.To > 0 && !opts.Force {
			if opts.To > h.transPendingUpTo {
				h.transPendingUpTo = opts.To
			}
			h.transMu.Unlock()
			return nil
		}
		cur := h.transCurrentCh
		h.transMu.Unlock()
		if cur > 0 {
			return fmt.Errorf("đang dịch chương %d, vui lòng chờ xong", cur)
		}
		return fmt.Errorf("đang có tiến trình dịch Việt chạy ở nền")
	}
	runCtx, cancel := context.WithCancel(ctx)
	h.transRunning = true
	h.transCurrentCh = 0
	h.transPendingUpTo = 0
	h.transCancel = cancel
	h.transMu.Unlock()

	h.asyncWG.Go(func() {
		defer cancel()
		h.runTranslateLoop(runCtx, opts, autoMode)
	})
	return nil
}

func (h *Host) runTranslateLoop(ctx context.Context, initialOpts trans.Options, autoMode bool) {
	defer func() {
		h.transMu.Lock()
		h.transRunning = false
		h.transCurrentCh = 0
		h.transPendingUpTo = 0
		h.transCancel = nil
		h.transMu.Unlock()
	}()

	opts := initialOpts
	isAuto := autoMode

	for {
		model, thinking, modelLabel, err := h.translatorModelRuntime()
		if err != nil {
			if !isAuto {
				h.emitEvent(Event{
					Time:     time.Now(),
					Category: "ERROR",
					Summary:  "Không khởi tạo được model dịch: " + err.Error(),
					Level:    "error",
				})
			}
			return
		}

		profile := h.TranslationProfile()
		deps := trans.Deps{
			Store:     h.store,
			Model:     model,
			Thinking:  thinking,
			ModelName: modelLabel,
			Profile:   profile,
			OnEvent: func(ev trans.Event) {
				if ev.Chapter > 0 {
					h.transMu.Lock()
					h.transCurrentCh = ev.Chapter
					h.transMu.Unlock()
				}
				switch ev.Stage {
				case "chapter", "seed":
					h.emitEvent(Event{
						Time:     time.Now(),
						Category: "SYSTEM",
						Summary:  ev.Summary,
						Level:    "info",
					})
				case "error":
					h.emitEvent(Event{
						Time:     time.Now(),
						Category: "ERROR",
						Summary:  ev.Summary,
						Level:    "error",
					})
				}
			},
		}

		res, runErr := trans.Run(ctx, deps, opts)
		if runErr != nil {
			if ctx.Err() == nil && !isAuto {
				h.emitEvent(Event{
					Time:     time.Now(),
					Category: "ERROR",
					Summary:  "Dịch tiếng Việt thất bại: " + runErr.Error(),
					Level:    "error",
				})
			} else if runErr != nil && ctx.Err() == nil && isAuto {
				h.emitEvent(Event{
					Time:     time.Now(),
					Category: "ERROR",
					Summary:  "Tự động dịch tiếng Việt gặp lỗi: " + runErr.Error(),
					Level:    "error",
				})
			}
		} else if res != nil {
			if len(res.TranslatedChapters) > 0 {
				h.emitEvent(Event{
					Time:     time.Now(),
					Category: "SYSTEM",
					Summary:  fmt.Sprintf("Đã dịch xong %d chương sang tiếng Việt (%v) [thuật ngữ +%d]", len(res.TranslatedChapters), res.TranslatedChapters, res.UpdatedGlossary),
					Level:    "success",
				})
			} else if !isAuto {
				h.emitEvent(Event{
					Time:     time.Now(),
					Category: "SYSTEM",
					Summary:  fmt.Sprintf("Các chương được chọn đều đã có bản dịch mới nhất (%d chương giữ nguyên)", len(res.SkippedChapters)),
					Level:    "info",
				})
			}
		}

		h.mu.Lock()
		closing := h.closing
		h.mu.Unlock()
		h.transMu.Lock()
		pendingUpTo := h.transPendingUpTo
		h.transPendingUpTo = 0
		h.transCurrentCh = 0
		h.transMu.Unlock()

		if closing || ctx.Err() != nil || pendingUpTo <= 0 {
			return
		}
		opts = trans.Options{To: pendingUpTo}
		isAuto = true
	}
}

func (h *Host) translatorModelRuntime() (agentcore.ChatModel, agentcore.ThinkingLevel, string, error) {
	if h == nil {
		return nil, "", "", fmt.Errorf("host is nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.models == nil {
		return nil, "", "", fmt.Errorf("chưa cấu hình model để dịch")
	}
	role := "translator"
	provider, modelName, explicit := h.models.CurrentSelection(role)
	if !explicit {
		provider, modelName, _ = h.models.CurrentSelection("default")
	}
	model := h.models.ForRoleWithFailover(role, func(ev bootstrap.FailoverEvent) {
		slog.Warn("Chuyển provider dự phòng cho dịch giả", "module", "trans", "role", ev.Role,
			"reason", ev.Reason, "from", fmt.Sprintf("%s/%s", ev.FromProvider, ev.FromModel),
			"to", fmt.Sprintf("%s/%s", ev.ToProvider, ev.ToModel), "err", ev.Err)
	})
	if model == nil {
		return nil, "", "", fmt.Errorf("không tìm thấy model khả dụng cho translator")
	}
	recorder := func(string, string, agentcore.AgentMessage) {}
	if h.usage != nil {
		recorder = h.usage.Record
	}
	model = newUsageTrackedModel(model, role, recorder)
	thinking := h.resolveThinkingForRoleLocked(role)
	label := modelName
	if provider != "" && modelName != "" {
		label = provider + "/" + modelName
	}
	return model, thinking, label, nil
}

// handleStepAutoTranslate kích hoạt dịch tự động khi một bước Worker vừa hoàn tất không lỗi.
func (h *Host) handleStepAutoTranslate(inst *flow.Instruction) {
	if h == nil || h.store == nil || inst == nil {
		return
	}
	profile := h.TranslationProfile()
	switch profile.AutoTranslate {
	case domain.AutoTranslateManual:
		return
	case domain.AutoTranslateAfterCommit:
		if inst.Agent != "writer" {
			return
		}
		progress, err := h.store.Progress.Load()
		if err != nil || progress == nil || len(progress.CompletedChapters) == 0 {
			return
		}
		maxCh := slices.Max(progress.CompletedChapters)
		if maxCh > 0 {
			h.enqueueAutoTranslateUpTo(maxCh)
		}
	case domain.AutoTranslateAfterArc:
		if inst.Agent != "editor" && inst.Agent != "writer" {
			return
		}
		if upTo := approvedTranslationUpTo(h.store); upTo > 0 {
			h.enqueueAutoTranslateUpTo(upTo)
		}
	}
}

// maybeAutoTranslateOnComplete kiểm tra khi Engine kết thúc để dịch nốt phần đã được duyệt.
func (h *Host) maybeAutoTranslateOnComplete() {
	if h == nil || h.store == nil {
		return
	}
	profile := h.TranslationProfile()
	switch profile.AutoTranslate {
	case domain.AutoTranslateManual:
		return
	case domain.AutoTranslateAfterCommit:
		progress, err := h.store.Progress.Load()
		if err != nil || progress == nil || len(progress.CompletedChapters) == 0 {
			return
		}
		if maxCh := slices.Max(progress.CompletedChapters); maxCh > 0 {
			h.enqueueAutoTranslateUpTo(maxCh)
		}
	case domain.AutoTranslateAfterArc:
		if upTo := approvedTranslationUpTo(h.store); upTo > 0 {
			h.enqueueAutoTranslateUpTo(upTo)
		}
	}
}

// approvedTranslationUpTo trả về mốc chương cao nhất đã qua thẩm định (arc review / global review / hoàn tất)
// và không vướng PendingRewrites hoặc PendingReviewRecheck. Tránh dịch nháp trước khi Editor duyệt/sửa lại.
func approvedTranslationUpTo(st *storepkg.Store) int {
	if st == nil {
		return 0
	}
	progress, err := st.Progress.Load()
	if err != nil || progress == nil || len(progress.CompletedChapters) == 0 {
		return 0
	}
	completed := slices.Clone(progress.CompletedChapters)
	slices.Sort(completed)
	maxCh := completed[len(completed)-1]

	minPending := 0
	for _, ch := range progress.PendingRewrites {
		if ch > 0 && (minPending == 0 || ch < minPending) {
			minPending = ch
		}
	}
	recheck, _ := st.World.LoadPendingReviewRecheck()

	if progress.Phase == domain.PhaseComplete && minPending == 0 && recheck == nil {
		return maxCh
	}
	if minPending == 0 && recheck == nil {
		if globalOK, gErr := st.World.HasGlobalReview(maxCh); gErr == nil && globalOK {
			return maxCh
		}
	}

	approved := 0
	boundaries, bErr := st.Outline.CompletedArcBoundaries(maxCh)
	if bErr == nil {
		for _, b := range boundaries {
			if b.EndChapter <= 0 {
				continue
			}
			if minPending > 0 && minPending <= b.EndChapter {
				break
			}
			if recheck != nil && recheck.Chapter <= b.EndChapter {
				break
			}
			hasArcReview, rErr := st.World.HasArcReview(b.EndChapter)
			if rErr != nil || !hasArcReview {
				break
			}
			if b.EndChapter > approved {
				approved = b.EndChapter
			}
		}
	}

	// Nếu có review phạm vi global trước đó (truyện vừa/ngắn) và không bị pending rewrite chặn, mở tới anchor.
	if reviews, rErr := st.World.LoadReviews(); rErr == nil {
		for _, r := range reviews {
			if r.Scope != "global" && r.Scope != "arc" {
				continue
			}
			if r.Chapter <= 0 || r.Chapter > maxCh {
				continue
			}
			if minPending > 0 && minPending <= r.Chapter {
				continue
			}
			if recheck != nil && recheck.Chapter <= r.Chapter {
				continue
			}
			if r.Verdict == "accept" || (r.Recheck != nil && r.Recheck.Result != nil && len(r.Recheck.Result.UnresolvedIssues) == 0) {
				if r.Chapter > approved {
					approved = r.Chapter
				}
			}
		}
	}

	return approved
}

// ReadChapter nạp dữ liệu đọc chương song ngữ Việt / Trung cho modal /read.
func (h *Host) ReadChapter(chapter int, preferVi bool) (*ChapterReadView, error) {
	if h == nil || h.store == nil {
		return nil, fmt.Errorf("chưa có bộ truyện đang mở")
	}
	progress, err := h.store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("đọc tiến độ truyện: %w", err)
	}
	var completed []int
	if progress != nil && len(progress.CompletedChapters) > 0 {
		completed = slices.Clone(progress.CompletedChapters)
		slices.Sort(completed)
	}

	manifest, _ := trans.LoadManifest(h.store.Dir())

	if chapter <= 0 {
		if preferVi && manifest != nil && len(manifest.Chapters) > 0 {
			for i := len(completed) - 1; i >= 0; i-- {
				ch := completed[i]
				if _, ok := manifest.Chapters[ch]; ok {
					chapter = ch
					break
				}
			}
		}
		if chapter <= 0 && len(completed) > 0 {
			chapter = completed[len(completed)-1]
		}
		if chapter <= 0 {
			chapter = 1
		}
	}

	view := &ChapterReadView{
		Chapter:   chapter,
		Completed: completed,
	}
	if progress != nil {
		view.TotalChapters = max(progress.TotalChapters, len(completed))
	} else {
		view.TotalChapters = len(completed)
	}
	if book, bErr := h.store.Book.Load(); bErr == nil && book != nil {
		view.BookTitleZh = book.Title
	}
	if manifest != nil {
		view.BookTitleVi = manifest.BookTitleVi
	}

	if textZh, cErr := h.store.Drafts.LoadChapterText(chapter); cErr == nil && strings.TrimSpace(textZh) != "" {
		view.HasZh = true
		if t, tErr := h.store.Summaries.LoadSummaryTitle(chapter); tErr == nil && strings.TrimSpace(t) != "" {
			view.TitleZh = t
		} else if entry, oErr := h.store.Outline.GetChapterOutline(chapter); oErr == nil && entry != nil {
			view.TitleZh = entry.Title
		}
		view.BodyZh = domain.ChapterBody(textZh, view.TitleZh)
	}

	if titleVi, bodyVi, ok, vErr := trans.LoadChapterVi(h.store.Dir(), chapter); vErr == nil && ok {
		view.HasVi = true
		view.TitleVi = titleVi
		view.BodyVi = bodyVi
		if manifest != nil {
			if entry, exists := manifest.Chapters[chapter]; exists {
				if srcSHA, sErr := trans.ChapterSourceSHA256(h.store, chapter); sErr == nil && srcSHA != "" && entry.SourceSHA256 != srcSHA {
					view.OutdatedVi = true
				}
			}
		}
	}

	if !view.HasVi && !view.HasZh {
		return nil, fmt.Errorf("không tìm thấy nội dung chương %d", chapter)
	}
	return view, nil
}
