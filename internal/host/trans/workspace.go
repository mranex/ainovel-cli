package trans

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

const (
	viSubdir        = "translations/vi"
	manifestRelPath = "translations/vi/manifest.json"
	glossaryRelPath = "translations/vi/glossary.json"
	bookMDRelPath   = "translations/vi/book.md"
	manifestVersion = 1
	glossaryVersion = 1
)

var viChapterHeaderRe = regexp.MustCompile(`^(#{1,6}\s*)?(?i:chương|hồi|tiết)\s+[0-9]+`)

// ChapterRelPath trả về đường dẫn tương đối của chương tiếng Việt trong thư mục truyện.
func ChapterRelPath(chapter int) string {
	return fmt.Sprintf("%s/chapters/%02d.md", viSubdir, chapter)
}

// LoadManifest đọc translations/vi/manifest.json (trả về struct rỗng đã khởi tạo map nếu chưa có file).
func LoadManifest(novelDir string) (*Manifest, error) {
	path := filepath.Join(novelDir, manifestRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{
				Version:   manifestVersion,
				VolumesVi: make(map[int]string),
				Chapters:  make(map[int]ChapterManifestEntry),
			}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("đọc manifest bản dịch: %w", err)
	}
	if m.Version == 0 {
		m.Version = manifestVersion
	}
	if m.VolumesVi == nil {
		m.VolumesVi = make(map[int]string)
	}
	if m.Chapters == nil {
		m.Chapters = make(map[int]ChapterManifestEntry)
	}
	return &m, nil
}

// SaveManifest ghi nguyên tử manifest.json.
func SaveManifest(novelDir string, m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if m.Version == 0 {
		m.Version = manifestVersion
	}
	if m.VolumesVi == nil {
		m.VolumesVi = make(map[int]string)
	}
	if m.Chapters == nil {
		m.Chapters = make(map[int]ChapterManifestEntry)
	}
	return writeJSONAtomic(filepath.Join(novelDir, manifestRelPath), m)
}

// LoadGlossary đọc translations/vi/glossary.json (trả về nil nếu chưa tồn tại).
func LoadGlossary(novelDir string) (*Glossary, error) {
	path := filepath.Join(novelDir, glossaryRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var g Glossary
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("đọc glossary bản dịch: %w", err)
	}
	if g.Version == 0 {
		g.Version = glossaryVersion
	}
	return &g, nil
}

// SaveGlossary ghi nguyên tử glossary.json.
func SaveGlossary(novelDir string, g *Glossary) error {
	if g == nil {
		return fmt.Errorf("glossary is nil")
	}
	if g.Version == 0 {
		g.Version = glossaryVersion
	}
	return writeJSONAtomic(filepath.Join(novelDir, glossaryRelPath), g)
}

// SaveBookMarkdown ghi bản tóm tắt/tiêu đề tiếng Việt dễ đọc tại translations/vi/book.md.
func SaveBookMarkdown(novelDir, titleVi, synopsisVi string) error {
	var b strings.Builder
	if strings.TrimSpace(titleVi) != "" {
		b.WriteString("# ")
		b.WriteString(strings.TrimSpace(titleVi))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(synopsisVi) != "" {
		b.WriteString(strings.TrimSpace(synopsisVi))
		b.WriteString("\n")
	}
	return writeFileAtomic(filepath.Join(novelDir, bookMDRelPath), []byte(b.String()))
}

// FormatChapterVi chuẩn hóa nội dung markdown lưu đĩa của chương tiếng Việt.
func FormatChapterVi(chapter int, titleVi, bodyVi string) string {
	titleVi = strings.TrimSpace(titleVi)
	bodyVi = StripViChapterHeader(bodyVi, titleVi)
	if titleVi != "" {
		return fmt.Sprintf("# Chương %d: %s\n\n%s\n", chapter, titleVi, strings.TrimSpace(bodyVi))
	}
	return fmt.Sprintf("# Chương %d\n\n%s\n", chapter, strings.TrimSpace(bodyVi))
}

// StripViChapterHeader bỏ dòng tiêu đề "# Chương N: ..." ở đầu file (nếu có) để trả về phần thân sạch.
func StripViChapterHeader(content, title string) string {
	content = strings.TrimLeft(strings.ReplaceAll(strings.TrimPrefix(content, "\ufeff"), "\r\n", "\n"), "\n")
	first, rest, hasRest := strings.Cut(content, "\n")
	firstTrim := strings.TrimSpace(first)
	bare := strings.TrimSpace(strings.TrimLeft(firstTrim, "#"))
	if viChapterHeaderRe.MatchString(firstTrim) || (title != "" && strings.EqualFold(bare, strings.TrimSpace(title))) {
		if !hasRest {
			return ""
		}
		return strings.TrimLeft(rest, "\n")
	}
	return content
}

// LoadChapterVi đọc một chương đã dịch từ translations/vi/chapters/{ch}.md và manifest.
func LoadChapterVi(novelDir string, chapter int) (titleVi, bodyVi string, exists bool, err error) {
	m, mErr := LoadManifest(novelDir)
	if mErr != nil {
		return "", "", false, mErr
	}
	if entry, ok := m.Chapters[chapter]; ok {
		titleVi = strings.TrimSpace(entry.TitleVi)
	}
	data, rErr := os.ReadFile(filepath.Join(novelDir, ChapterRelPath(chapter)))
	if rErr != nil {
		if os.IsNotExist(rErr) {
			return titleVi, "", false, nil
		}
		return "", "", false, rErr
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return titleVi, "", false, nil
	}
	if titleVi == "" {
		first, _, _ := strings.Cut(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
		first = strings.TrimSpace(strings.TrimLeft(first, "#"))
		if _, after, ok := strings.Cut(first, ":"); ok {
			titleVi = strings.TrimSpace(after)
		}
	}
	return titleVi, StripViChapterHeader(raw, titleVi), true, nil
}

// ChapterSourceSHA256 lấy SHA256 chuẩn hóa của bản gốc tiếng Trung để phát hiện chương vừa bị rewrite.
func ChapterSourceSHA256(st *store.Store, chapter int) (string, error) {
	if st == nil {
		return "", fmt.Errorf("store is nil")
	}
	if rec, err := st.ChapterRecords.Load(chapter); err == nil && rec != nil && rec.ContentSHA256 != "" {
		return rec.ContentSHA256, nil
	}
	raw, err := st.Drafts.LoadChapterText(chapter)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return domain.ChapterContentSHA256(raw), nil
}

// chapterNeedsTranslation kiểm tra xem chương ch có cần dịch/dịch lại hay không.
func chapterNeedsTranslation(novelDir string, m *Manifest, ch int, srcSHA string) bool {
	if m == nil {
		return true
	}
	entry, ok := m.Chapters[ch]
	if !ok {
		return true
	}
	if srcSHA != "" && entry.SourceSHA256 != srcSHA {
		return true
	}
	info, err := os.Stat(filepath.Join(novelDir, ChapterRelPath(ch)))
	return err != nil || info.Size() == 0
}

// Inspect kiểm tra chỉ đọc tình trạng dịch của cuốn truyện hiện tại cho UI/Snapshot.
func Inspect(st *store.Store, profile domain.TranslationProfile) StatusSummary {
	summary := StatusSummary{Profile: profile.Normalized()}
	if st == nil {
		return summary
	}
	m, err := LoadManifest(st.Dir())
	if err == nil && m != nil {
		summary.BookTitleVi = m.BookTitleVi
		summary.SynopsisVi = m.SynopsisVi
	}
	progress, err := st.Progress.Load()
	if err != nil || progress == nil || len(progress.CompletedChapters) == 0 {
		return summary
	}
	completed := slices.Clone(progress.CompletedChapters)
	slices.Sort(completed)
	summary.TotalCompleted = len(completed)
	for _, ch := range completed {
		entry, hasEntry := m.Chapters[ch]
		info, statErr := os.Stat(filepath.Join(st.Dir(), ChapterRelPath(ch)))
		hasFile := statErr == nil && info.Size() > 0
		if hasEntry && hasFile {
			srcSHA, _ := ChapterSourceSHA256(st, ch)
			if srcSHA != "" && entry.SourceSHA256 != srcSHA {
				summary.OutdatedCount++
				summary.PendingCount++
			} else {
				summary.TranslatedCount++
			}
			continue
		}
		summary.PendingCount++
	}
	return summary
}

func bookMetadataSHA256(book *domain.BookMetadata, volumes []domain.VolumeOutline) string {
	var b strings.Builder
	if book != nil {
		b.WriteString(strings.TrimSpace(book.Title))
		b.WriteByte('\n')
		b.WriteString(strings.TrimSpace(book.Synopsis))
	}
	for _, v := range volumes {
		fmt.Fprintf(&b, "\nV%d:%s", v.Index, strings.TrimSpace(v.Title))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}
