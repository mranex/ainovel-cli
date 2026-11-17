package exp

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host/trans"
	"github.com/voocel/ainovel-cli/internal/store"
)

func seedViTestStore(t *testing.T) *store.Store {
	t.Helper()
	st := store.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	_ = st.Book.Save(domain.BookMetadata{Title: "太虚剑行", Synopsis: "少年御剑破境。"})
	_ = st.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		CurrentChapter:    2,
		CompletedChapters: []int{1, 2},
	})
	_ = st.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 1, Title: "问剑"})
	_ = st.Summaries.SaveSummary(domain.ChapterSummary{Chapter: 2, Title: "入谷"})
	_ = st.Drafts.SaveFinalChapter(1, "# 第一章 问剑\n\n风雪夜，少年拔剑出鞘。")
	_ = st.Drafts.SaveFinalChapter(2, "# 第二章 入谷\n\n幽谷深处剑鸣不止。")

	// Chỉ dịch chương 1, chương 2 để trống nhằm kiểm tra fallback & Untranslated.
	if err := os.MkdirAll(filepath.Join(st.Dir(), "translations", "vi", "chapters"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(st.Dir(), trans.ChapterRelPath(1)),
		[]byte("# Chương 1: Vấn Kiếm\n\nĐêm gió tuyết, thiếu niên rút kiếm khỏi vỏ.\n"), 0o644)
	_ = trans.SaveManifest(st.Dir(), &trans.Manifest{
		BookTitleZh: "太虚剑行",
		BookTitleVi: "Thái Hư Kiếm Hành",
		SynopsisVi:  "Thiếu niên ngự kiếm phá cảnh.",
		Chapters: map[int]trans.ChapterManifestEntry{
			1: {Chapter: 1, TitleVi: "Vấn Kiếm", SourceSHA256: "abc"},
		},
	})
	return st
}

func TestExport_LangModes(t *testing.T) {
	st := seedViTestStore(t)

	// 1. Auto phải chọn LangVI khi đã có bản dịch chương 1.
	resAuto, err := Run(context.Background(), Deps{Store: st}, Options{
		Format:    FormatTXT,
		Overwrite: true,
	})
	if err != nil {
		t.Fatalf("auto txt: %v", err)
	}
	if resAuto.Lang != LangVI {
		t.Fatalf("auto lang = %q, want vi", resAuto.Lang)
	}
	if len(resAuto.Untranslated) != 1 || resAuto.Untranslated[0] != 2 {
		t.Fatalf("Untranslated = %v, want [2]", resAuto.Untranslated)
	}
	txtData, _ := os.ReadFile(resAuto.Path)
	txtStr := string(txtData)
	if !strings.Contains(txtStr, "Thái Hư Kiếm Hành") || !strings.Contains(txtStr, "Chương 1: Vấn Kiếm") || !strings.Contains(txtStr, "thiếu niên rút kiếm khỏi vỏ") {
		t.Fatalf("vi txt missing expected content:\n%s", txtStr)
	}
	if !strings.Contains(txtStr, "[Chưa dịch tiếng Việt — nguyên bản tiếng Trung]") {
		t.Fatalf("missing fallback marker for ch2:\n%s", txtStr)
	}

	// 2. LangZH giữ nguyên tiếng Trung.
	zhPath := filepath.Join(t.TempDir(), "orig.txt")
	resZH, err := Run(context.Background(), Deps{Store: st}, Options{
		Format:  FormatTXT,
		Lang:    LangZH,
		OutPath: zhPath,
	})
	if err != nil {
		t.Fatalf("zh txt: %v", err)
	}
	if resZH.Lang != LangZH {
		t.Fatalf("zh lang = %q, want zh", resZH.Lang)
	}
	zhData, _ := os.ReadFile(zhPath)
	if !strings.Contains(string(zhData), "《太虚剑行》") || strings.Contains(string(zhData), "Vấn Kiếm") {
		t.Fatalf("zh txt contaminated:\n%s", string(zhData))
	}

	// 3. LangBilingual có cả bản Việt lẫn bản Trung.
	resBi, err := Run(context.Background(), Deps{Store: st}, Options{
		Format: FormatTXT,
		Lang:   LangBilingual,
	})
	if err != nil {
		t.Fatalf("bilingual txt: %v", err)
	}
	biData, _ := os.ReadFile(resBi.Path)
	biStr := string(biData)
	if !strings.Contains(biStr, "thiếu niên rút kiếm khỏi vỏ") || !strings.Contains(biStr, "少年拔剑出鞘") {
		t.Fatalf("bilingual missing vi or zh body:\n%s", biStr)
	}

	// 4. EPUB LangVI ghi metadata <dc:language>vi</dc:language>.
	resEPUB, err := Run(context.Background(), Deps{Store: st}, Options{
		Format: FormatEPUB,
		Lang:   LangVI,
	})
	if err != nil {
		t.Fatalf("vi epub: %v", err)
	}
	epubBytes, _ := os.ReadFile(resEPUB.Path)
	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var opfText string
	for _, f := range zr.File {
		if f.Name == "OEBPS/content.opf" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			opfText = string(b)
		}
	}
	if !strings.Contains(opfText, "<dc:language>vi</dc:language>") || !strings.Contains(opfText, "Thái Hư Kiếm Hành") {
		t.Fatalf("unexpected vi OPF:\n%s", opfText)
	}
}
