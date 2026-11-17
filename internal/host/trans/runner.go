package trans

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/llmretry"
	"github.com/voocel/ainovel-cli/internal/store"
)

const (
	seedMaxTokens    = 16384
	chapterMaxTokens = 65536
)

// Deps chứa các phụ thuộc để chạy dịch tiếng Việt.
type Deps struct {
	Store     *store.Store
	Model     llmretry.Generator
	Thinking  agentcore.ThinkingLevel
	ModelName string
	Profile   domain.TranslationProfile
	OnEvent   func(Event)
}

// Run dịch các chương hoàn thành trong khoảng yêu cầu (bỏ qua chương đã dịch và khớp SourceSHA256 trừ khi Force=true).
func Run(ctx context.Context, deps Deps, opts Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("trans: deps.Store is nil")
	}
	if deps.Model == nil {
		return nil, fmt.Errorf("trans: model translator chưa được cấu hình")
	}
	profile := deps.Profile.Normalized()
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	progress, err := deps.Store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("đọc progress: %w", err)
	}
	if progress == nil || len(progress.CompletedChapters) == 0 {
		return nil, fmt.Errorf("chưa có chương hoàn thành nào để dịch")
	}

	chapters := slices.Clone(progress.CompletedChapters)
	slices.Sort(chapters)
	from := opts.From
	if from <= 0 {
		from = chapters[0]
	}
	to := opts.To
	if to <= 0 {
		to = chapters[len(chapters)-1]
	}
	if from > to {
		return nil, fmt.Errorf("phạm vi chương không hợp lệ: from=%d > to=%d", from, to)
	}

	var targets []int
	for _, ch := range chapters {
		if ch >= from && ch <= to {
			targets = append(targets, ch)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("không có chương hoàn thành nào trong khoảng %d..%d", from, to)
	}

	manifest, glossary, err := ensureBookAndGlossary(ctx, deps, profile, opts)
	if err != nil {
		return nil, err
	}

	effectiveGenre := resolveEffectiveGenre(profile.Genre, glossary)

	result := &Result{BookTitleVi: manifest.BookTitleVi}
	var toTranslate []int
	srcSHAs := make(map[int]string, len(targets))
	for _, ch := range targets {
		sha, shaErr := ChapterSourceSHA256(deps.Store, ch)
		if shaErr != nil {
			return nil, fmt.Errorf("đọc chương %d: %w", ch, shaErr)
		}
		srcSHAs[ch] = sha
		if opts.Force || chapterNeedsTranslation(deps.Store.Dir(), manifest, ch, sha) {
			toTranslate = append(toTranslate, ch)
		} else {
			result.SkippedChapters = append(result.SkippedChapters, ch)
		}
	}

	if len(toTranslate) == 0 {
		emit(deps.OnEvent, Event{
			Stage:   "done",
			Total:   0,
			Summary: "Tất cả chương trong phạm vi đã được dịch và khớp bản mới nhất",
		})
		return result, nil
	}

	total := len(toTranslate)
	for idx, ch := range toTranslate {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		emit(deps.OnEvent, Event{
			Stage:   "chapter",
			Chapter: ch,
			Total:   total,
			Summary: fmt.Sprintf("Đang dịch chương %d (%d/%d)...", ch, idx+1, total),
		})

		rawZh, err := deps.Store.Drafts.LoadChapterText(ch)
		if err != nil {
			return result, fmt.Errorf("đọc nguyên tác chương %d: %w", ch, err)
		}
		if strings.TrimSpace(rawZh) == "" {
			return result, fmt.Errorf("chương %d trống, không thể dịch", ch)
		}

		titleZh := resolveChapterTitleZh(deps.Store, ch)
		bodyZh := domain.ChapterBody(rawZh, titleZh)
		prevTailVi := loadPrevChapterTailVi(deps.Store.Dir(), ch)

		titleVi, bodyVi, newTerms, err := translateSingleChapter(ctx, deps, effectiveGenre, profile.Tone, ch, titleZh, bodyZh, glossary, prevTailVi)
		if err != nil {
			emit(deps.OnEvent, Event{
				Stage:   "error",
				Chapter: ch,
				Total:   total,
				Summary: fmt.Sprintf("Dịch chương %d thất bại: %v", ch, err),
				Err:     err,
			})
			return result, fmt.Errorf("dịch chương %d: %w", ch, err)
		}

		formatted := FormatChapterVi(ch, titleVi, bodyVi)
		if err := writeFileAtomic(filepath.Join(deps.Store.Dir(), filepath.FromSlash(ChapterRelPath(ch))), []byte(formatted)); err != nil {
			return result, fmt.Errorf("lưu bản dịch chương %d: %w", ch, err)
		}

		added := mergeGlossaryEntries(glossary, newTerms)
		if added > 0 {
			result.UpdatedGlossary += added
			glossary.UpdatedAt = time.Now().Format(time.RFC3339)
			if err := SaveGlossary(deps.Store.Dir(), glossary); err != nil {
				return result, fmt.Errorf("cập nhật glossary sau chương %d: %w", ch, err)
			}
		}

		now := time.Now().Format(time.RFC3339)
		manifest.Chapters[ch] = ChapterManifestEntry{
			Chapter:      ch,
			SourceSHA256: srcSHAs[ch],
			TitleZh:      titleZh,
			TitleVi:      titleVi,
			WordCountVi:  countVietnameseWords(bodyVi),
			Genre:        string(effectiveGenre),
			Tone:         string(profile.Tone),
			Model:        deps.ModelName,
			TranslatedAt: now,
		}
		manifest.UpdatedAt = now
		if err := SaveManifest(deps.Store.Dir(), manifest); err != nil {
			return result, fmt.Errorf("lưu manifest sau chương %d: %w", ch, err)
		}
		result.TranslatedChapters = append(result.TranslatedChapters, ch)
	}

	emit(deps.OnEvent, Event{
		Stage:   "done",
		Total:   total,
		Summary: fmt.Sprintf("Hoàn tất dịch %d chương sang tiếng Việt", len(result.TranslatedChapters)),
	})
	return result, nil
}

func ensureBookAndGlossary(ctx context.Context, deps Deps, profile domain.TranslationProfile, opts Options) (*Manifest, *Glossary, error) {
	dir := deps.Store.Dir()
	manifest, err := LoadManifest(dir)
	if err != nil {
		return nil, nil, err
	}
	glossary, err := LoadGlossary(dir)
	if err != nil {
		return nil, nil, err
	}

	book, _ := deps.Store.Book.Load()
	volumes, _ := deps.Store.Outline.LoadLayeredOutline()
	bookSHA := bookMetadataSHA256(book, volumes)

	needSeed := glossary == nil || manifest.BookTitleVi == "" || manifest.BookSHA256 != bookSHA ||
		(opts.Force && opts.From == 0 && opts.To == 0) ||
		(glossary != nil && glossary.Genre != string(profile.Genre))

	if !needSeed {
		return manifest, glossary, nil
	}

	emit(deps.OnEvent, Event{
		Stage:   "seed",
		Summary: "Đang khởi tạo tiêu đề, văn án và từ điển nhất quán (Glossary) tiếng Việt...",
	})

	premise, _ := deps.Store.Outline.LoadPremise()
	chars, _ := deps.Store.Characters.Load()
	worldRules, _ := deps.Store.World.LoadWorldRules()

	var titleZh, synopsisZh string
	if book != nil {
		titleZh = book.Title
		synopsisZh = book.Synopsis
	}

	payload := formatSeedUserPayload(titleZh, synopsisZh, premise, volumes, chars, worldRules, profile.Genre)
	callOpts := callOptions(seedMaxTokens, deps.Thinking)
	resp, err := llmretry.Generate(ctx, deps.Model, llmretry.Config{Agent: "translator"}, []agentcore.Message{
		agentcore.SystemMsg(SeedSystemPrompt(profile.Genre, profile.Tone)),
		agentcore.UserMsg(payload),
	}, callOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("khởi tạo glossary/thông tin truyện: %w", err)
	}
	if resp == nil {
		return nil, nil, fmt.Errorf("model trả về rỗng khi khởi tạo glossary")
	}

	raw := resp.Message.TextContent()
	resolvedGenre := domain.TranslationGenre(strings.ToLower(strings.TrimSpace(extractXMLTag(raw, "resolved_genre"))))
	if profile.Genre != domain.TransGenreAuto && profile.Genre.Valid() {
		resolvedGenre = profile.Genre
	} else if !resolvedGenre.Valid() || resolvedGenre == domain.TransGenreAuto {
		resolvedGenre = domain.TransGenreTienHiep
	}

	bookTitleVi := strings.TrimSpace(extractXMLTag(raw, "book_title"))
	if bookTitleVi == "" {
		bookTitleVi = strings.TrimSpace(titleZh)
	}
	synopsisVi := strings.TrimSpace(extractXMLTag(raw, "book_synopsis"))
	volumesVi := parseVolumeLines(extractXMLTag(raw, "volumes"))
	pronounGuide := strings.TrimSpace(extractXMLTag(raw, "pronoun_guide"))
	seedEntries := parseGlossaryLines(extractXMLTag(raw, "glossary"))

	now := time.Now().Format(time.RFC3339)
	if glossary == nil || (opts.Force && opts.From == 0 && opts.To == 0) || glossary.Genre != string(profile.Genre) {
		glossary = &Glossary{
			Version:       glossaryVersion,
			Genre:         string(profile.Genre),
			ResolvedGenre: string(resolvedGenre),
			PronounGuide:  pronounGuide,
			Entries:       seedEntries,
			UpdatedAt:     now,
		}
	} else {
		glossary.Genre = string(profile.Genre)
		glossary.ResolvedGenre = string(resolvedGenre)
		if pronounGuide != "" {
			glossary.PronounGuide = pronounGuide
		}
		mergeGlossaryEntries(glossary, seedEntries)
		glossary.UpdatedAt = now
	}
	if err := SaveGlossary(dir, glossary); err != nil {
		return nil, nil, err
	}

	manifest.BookTitleZh = titleZh
	manifest.BookTitleVi = bookTitleVi
	manifest.SynopsisZh = synopsisZh
	manifest.SynopsisVi = synopsisVi
	manifest.BookSHA256 = bookSHA
	for idx, vt := range volumesVi {
		manifest.VolumesVi[idx] = vt
	}
	manifest.UpdatedAt = now
	if err := SaveManifest(dir, manifest); err != nil {
		return nil, nil, err
	}
	_ = SaveBookMarkdown(dir, bookTitleVi, synopsisVi)
	return manifest, glossary, nil
}

func translateSingleChapter(
	ctx context.Context,
	deps Deps,
	genre domain.TranslationGenre,
	tone domain.TranslationTone,
	chapter int,
	titleZh, bodyZh string,
	glossary *Glossary,
	prevTailVi string,
) (titleVi, bodyVi string, terms []GlossaryEntry, err error) {
	sysPrompt := ChapterSystemPrompt(genre, tone)
	userPayload := formatChapterUserPayload(chapter, titleZh, bodyZh, glossary, prevTailVi)
	messages := []agentcore.Message{
		agentcore.SystemMsg(sysPrompt),
		agentcore.UserMsg(userPayload),
	}
	callOpts := callOptions(chapterMaxTokens, deps.Thinking)

	srcRunes := utf8.RuneCountInString(strings.TrimSpace(bodyZh))
	for attempt := 0; attempt < 2; attempt++ {
		resp, genErr := llmretry.Generate(ctx, deps.Model, llmretry.Config{Agent: "translator"}, messages, callOpts...)
		if genErr != nil {
			return "", "", nil, genErr
		}
		if resp == nil {
			return "", "", nil, fmt.Errorf("model trả về rỗng")
		}
		if resp.Message.StopReason == agentcore.StopReasonSafety {
			return "", "", nil, fmt.Errorf("provider từ chối dịch do bộ lọc an toàn (stop_reason=safety) — hãy dùng /model translator đổi sang model/proxy ít kiểm duyệt hơn")
		}
		raw := strings.TrimSpace(resp.Message.TextContent())
		titleVi, bodyVi, terms = parseChapterTranslation(raw)
		if bodyVi == "" && attempt == 0 {
			messages = append(messages,
				agentcore.Message{Role: agentcore.RoleAssistant, Content: []agentcore.ContentBlock{agentcore.TextBlock(raw)}},
				agentcore.UserMsg("Đầu ra vừa rồi thiếu thẻ <body>...</body> hoặc nội dung bị rỗng. Hãy trả về đầy đủ đúng 3 thẻ <title>, <body>, <glossary_updates>."),
			)
			continue
		}
		// Nếu bản dịch quá ngắn bất thường so với bản gốc dài, nhắc model dịch lại đầy đủ (tránh bị model tóm tắt).
		if srcRunes >= 400 && utf8.RuneCountInString(bodyVi) < srcRunes/5 && attempt == 0 {
			messages = append(messages,
				agentcore.Message{Role: agentcore.RoleAssistant, Content: []agentcore.ContentBlock{agentcore.TextBlock(raw)}},
				agentcore.UserMsg("Bản dịch trong thẻ <body> quá ngắn so với nguyên tác (có dấu hiệu bị tóm tắt hoặc cắt cụt). Hãy dịch đầy đủ trọn vẹn 100% nội dung từng đoạn của chương trong thẻ <body>...</body>."),
			)
			continue
		}
		if bodyVi == "" {
			return "", "", nil, fmt.Errorf("không trích xuất được nội dung bản dịch từ phản hồi model")
		}
		return titleVi, bodyVi, terms, nil
	}
	return "", "", nil, fmt.Errorf("không thể dịch hoàn chỉnh chương %d", chapter)
}

func resolveEffectiveGenre(selected domain.TranslationGenre, glossary *Glossary) domain.TranslationGenre {
	if selected.Normalize() != domain.TransGenreAuto {
		return selected.Normalize()
	}
	if glossary != nil {
		rg := domain.TranslationGenre(strings.TrimSpace(glossary.ResolvedGenre))
		if rg.Valid() && rg != domain.TransGenreAuto {
			return rg
		}
	}
	return domain.TransGenreTienHiep
}

func resolveChapterTitleZh(st *store.Store, chapter int) string {
	if title, err := st.Summaries.LoadSummaryTitle(chapter); err == nil && strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	if entry, err := st.Outline.GetChapterOutline(chapter); err == nil && entry != nil && strings.TrimSpace(entry.Title) != "" {
		return strings.TrimSpace(entry.Title)
	}
	if entry, err := st.Outline.GetChapterFromLayered(chapter); err == nil && entry != nil && strings.TrimSpace(entry.Title) != "" {
		return strings.TrimSpace(entry.Title)
	}
	return ""
}

func loadPrevChapterTailVi(novelDir string, chapter int) string {
	if chapter <= 1 {
		return ""
	}
	_, body, exists, err := LoadChapterVi(novelDir, chapter-1)
	if err != nil || !exists || strings.TrimSpace(body) == "" {
		return ""
	}
	r := []rune(strings.TrimSpace(body))
	const maxTail = 650
	if len(r) <= maxTail {
		return string(r)
	}
	return "..." + string(r[len(r)-maxTail:])
}

func callOptions(maxTokens int, thinking agentcore.ThinkingLevel) []agentcore.CallOption {
	opts := []agentcore.CallOption{agentcore.WithMaxTokens(maxTokens)}
	if thinking != "" {
		opts = append(opts, agentcore.WithThinking(thinking))
	}
	return opts
}

func parseChapterTranslation(raw string) (titleVi, bodyVi string, terms []GlossaryEntry) {
	titleVi = strings.TrimSpace(extractXMLTag(raw, "title"))
	bodyVi = strings.TrimSpace(extractXMLTag(raw, "body"))
	terms = parseGlossaryLines(extractXMLTag(raw, "glossary_updates"))

	// Fallback nếu model quên bọc <body> nhưng trả về văn bản thuần không thẻ XML.
	if bodyVi == "" && !strings.Contains(raw, "<body") && !strings.Contains(raw, "<title") && raw != "" {
		bodyVi = strings.TrimSpace(raw)
	}
	titleVi = strings.TrimPrefix(titleVi, "# ")
	titleVi = strings.TrimSpace(titleVi)
	return titleVi, bodyVi, terms
}

func extractXMLTag(raw, tag string) string {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.Index(raw, open)
	if start < 0 {
		return ""
	}
	rest := raw[start+len(open):]
	if end := strings.Index(rest, closeTag); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	// Nếu thiếu thẻ đóng, cắt tới thẻ mở đã biết tiếp theo.
	cut := len(rest)
	for _, nextTag := range []string{"<resolved_genre>", "<book_title>", "<book_synopsis>", "<volumes>", "<pronoun_guide>", "<glossary>", "<title>", "<body>", "<glossary_updates>"} {
		if nextTag == open {
			continue
		}
		if idx := strings.Index(rest, nextTag); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return strings.TrimSpace(rest[:cut])
}

func parseVolumeLines(text string) map[int]string {
	out := make(map[int]string)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		if line == "" {
			continue
		}
		left, right, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		idx, err := strconv.Atoi(left)
		if err != nil || idx <= 0 || right == "" {
			continue
		}
		out[idx] = right
	}
	return out
}

func parseGlossaryLines(text string) []GlossaryEntry {
	var entries []GlossaryEntry
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		if line == "" {
			continue
		}
		src, rest, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		parts := strings.Split(rest, "|")
		target := strings.TrimSpace(parts[0])
		if target == "" {
			continue
		}
		entry := GlossaryEntry{Source: src, Target: target}
		if len(parts) > 1 {
			entry.Kind = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			entry.Note = strings.TrimSpace(parts[2])
		}
		entries = append(entries, entry)
	}
	return entries
}

func mergeGlossaryEntries(glossary *Glossary, incoming []GlossaryEntry) int {
	if glossary == nil || len(incoming) == 0 {
		return 0
	}
	existing := make(map[string]bool, len(glossary.Entries))
	for _, e := range glossary.Entries {
		existing[e.Source] = true
	}
	added := 0
	for _, e := range incoming {
		if e.Source == "" || e.Target == "" || existing[e.Source] {
			continue
		}
		existing[e.Source] = true
		glossary.Entries = append(glossary.Entries, e)
		added++
	}
	return added
}

func countVietnameseWords(text string) int {
	return len(strings.Fields(text))
}

func emit(fn func(Event), ev Event) {
	if fn != nil {
		fn(ev)
	}
}
