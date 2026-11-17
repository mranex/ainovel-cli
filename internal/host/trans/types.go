package trans

import "github.com/voocel/ainovel-cli/internal/domain"

// Options kiểm soát phạm vi dịch sang tiếng Việt.
type Options struct {
	From  int
	To    int
	Force bool
}

// GlossaryEntry lưu một cặp thuật ngữ / tên riêng / danh xưng đã chốt cho bộ truyện.
type GlossaryEntry struct {
	Source string `json:"source"`         // Từ gốc tiếng Trung
	Target string `json:"target"`         // Bản dịch tiếng Việt đã thống nhất
	Kind   string `json:"kind,omitempty"` // character / place / term / title / skill
	Note   string `json:"note,omitempty"` // Ghi chú xưng hô / ngữ cảnh
}

// Glossary lưu từ điển nhất quán xuyên suốt bộ truyện tại translations/vi/glossary.json.
type Glossary struct {
	Version           int             `json:"version"`
	Genre             string          `json:"genre"`               // Giá trị switch người dùng chọn (auto / tien_hiep / ...)
	ResolvedGenre     string          `json:"resolved_genre"`      // Thể loại thực tế sau khi phân tích (tien_hiep / co_trang / fantasy / hien_dai)
	FoundationSeedSHA string          `json:"foundation_seed_sha"` // Dấu vân tay foundation khi seed glossary
	PronounGuide      string          `json:"pronoun_guide"`       // Quy tắc xưng hô cố định giữa các nhân vật chính
	Entries           []GlossaryEntry `json:"entries,omitempty"`
	UpdatedAt         string          `json:"updated_at,omitempty"`
}

// ChapterManifestEntry ghi lại trạng thái dịch của từng chương để phát hiện chương bị sửa/viết lại.
type ChapterManifestEntry struct {
	Chapter      int    `json:"chapter"`
	SourceSHA256 string `json:"source_sha256"`
	TitleZh      string `json:"title_zh,omitempty"`
	TitleVi      string `json:"title_vi,omitempty"`
	WordCountVi  int    `json:"word_count_vi,omitempty"`
	Genre        string `json:"genre,omitempty"`
	Tone         string `json:"tone,omitempty"`
	Model        string `json:"model,omitempty"`
	TranslatedAt string `json:"translated_at,omitempty"`
}

// Manifest là chỉ mục bản dịch tiếng Việt tại translations/vi/manifest.json.
type Manifest struct {
	Version     int                          `json:"version"`
	BookTitleZh string                       `json:"book_title_zh,omitempty"`
	BookTitleVi string                       `json:"book_title_vi,omitempty"`
	SynopsisZh  string                       `json:"synopsis_zh,omitempty"`
	SynopsisVi  string                       `json:"synopsis_vi,omitempty"`
	BookSHA256  string                       `json:"book_sha256,omitempty"`
	VolumesVi   map[int]string               `json:"volumes_vi,omitempty"`
	Chapters    map[int]ChapterManifestEntry `json:"chapters,omitempty"`
	UpdatedAt   string                       `json:"updated_at,omitempty"`
}

// StatusSummary cung cấp ảnh chụp gọn cho TUI / Snapshot / Exporter.
type StatusSummary struct {
	Profile         domain.TranslationProfile
	TranslatedCount int
	OutdatedCount   int
	PendingCount    int
	TotalCompleted  int
	Running         bool
	CurrentChapter  int
	BookTitleVi     string
	SynopsisVi      string
}

// Result tóm tắt kết quả một lượt chạy dịch.
type Result struct {
	BookTitleVi        string
	TranslatedChapters []int
	SkippedChapters    []int
	UpdatedGlossary    int
}

// Event thông báo tiến trình từng bước khi dịch.
type Event struct {
	Stage   string // seed / chapter / done / error
	Chapter int
	Total   int
	Summary string
	Err     error
}
