package exp

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host/trans"
)

// renderEPUB 把章节集合打包成 EPUB 3 字节流。
//
// 包结构（OEBPS 是 OPS package 容器）：
//
//	mimetype                    （必须 zip 第一项 + Method=Store 不压缩）
//	META-INF/container.xml      （指向 OEBPS/content.opf）
//	OEBPS/content.opf           （metadata + manifest + spine）
//	OEBPS/nav.xhtml             （EPUB 3 navigation）
//	OEBPS/style.css             （极简排版）
//	OEBPS/cover.xhtml           （书名，可选）
//	OEBPS/chapterNNN.xhtml      （每章一文件）
func renderEPUB(
	book domain.BookMetadata,
	chapters []int,
	titleIdx chapterTitleIndex,
	locations map[int]chapterLocation,
	bodies map[int]string,
) ([]byte, error) {
	return renderEPUBWithLang(book, chapters, titleIdx, locations, bodies, LangZH, nil, nil, nil)
}

func renderEPUBWithLang(
	book domain.BookMetadata,
	chapters []int,
	titleIdx chapterTitleIndex,
	locations map[int]chapterLocation,
	bodies map[int]string,
	lang Lang,
	hasVi map[int]bool,
	zhTitleIdx chapterTitleIndex,
	zhBodies map[int]string,
) ([]byte, error) {
	isVi := lang == LangVI || lang == LangBilingual
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// 1. mimetype 必须是 zip 第一项 + Store（不压缩）+ 内容精确无 BOM
	mt, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return nil, fmt.Errorf("create mimetype: %w", err)
	}
	if _, err := mt.Write([]byte("application/epub+zip")); err != nil {
		return nil, err
	}

	if err := zipDeflate(zw, "META-INF/container.xml", containerXML); err != nil {
		return nil, err
	}
	if err := zipDeflate(zw, "OEBPS/style.css", styleCSS); err != nil {
		return nil, err
	}

	hasCover := strings.TrimSpace(book.Title) != ""
	if hasCover {
		if err := zipDeflate(zw, "OEBPS/cover.xhtml", renderCoverXHTML(book.Title, isVi)); err != nil {
			return nil, err
		}
	}

	for _, ch := range chapters {
		loc, hasLoc := locations[ch]
		title := strings.TrimSpace(titleIdx[ch])
		zhTitle := strings.TrimSpace(zhTitleIdx[ch])
		zhRaw := strings.TrimSpace(zhBodies[ch])
		if zhRaw == "" {
			zhRaw = strings.TrimSpace(bodies[ch])
		}
		translated := !isVi || hasVi == nil || hasVi[ch]
		var body string
		if isVi && translated {
			body = trans.StripViChapterHeader(domain.ChapterBody(strings.TrimSpace(bodies[ch]), title), title)
		} else {
			body = domain.ChapterBody(strings.TrimSpace(bodies[ch]), zhTitle)
			if !isVi {
				body = domain.ChapterBody(strings.TrimSpace(bodies[ch]), title)
			}
		}
		zhBody := domain.ChapterBody(zhRaw, zhTitle)
		xhtml := renderChapterXHTML(ch, title, loc, hasLoc, body, lang, translated, zhTitle, zhBody)
		if err := zipDeflate(zw, "OEBPS/"+chapterFileName(ch), xhtml); err != nil {
			return nil, err
		}
	}

	if err := zipDeflate(zw, "OEBPS/nav.xhtml", renderNavXHTML(hasCover, chapters, titleIdx, isVi)); err != nil {
		return nil, err
	}

	if err := zipDeflate(zw, "OEBPS/content.opf", renderOPF(book, hasCover, chapters, isVi)); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}
	return buf.Bytes(), nil
}

// zipDeflate 写入一个普通（压缩）条目。
func zipDeflate(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	_, err = w.Write([]byte(content))
	return err
}

func chapterFileName(ch int) string {
	return fmt.Sprintf("chapter%03d.xhtml", ch)
}

// chapterID 是 manifest item 的 id；与文件名一一对应。
func chapterID(ch int) string {
	return fmt.Sprintf("ch%03d", ch)
}

// 固定模板 ────────────────────────────────────────────────

const containerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

const styleCSS = `body { font-family: serif; line-height: 1.7; margin: 1em; }
h1.book-title { font-size: 2em; text-align: center; margin: 4em 0 1em; }
.volume-divider { font-size: 1.6em; text-align: center; margin: 4em 0 1em; font-weight: bold; }
h1.chapter-title { font-size: 1.4em; text-align: center; margin: 2em 0 1.5em; }
p { text-indent: 2em; margin: 0.5em 0; }
.lang-note { text-indent: 0; font-style: italic; opacity: 0.75; margin: 1.5em 0 0.8em; }
.bilingual-divider { border: none; border-top: 1px solid #888; margin: 2em 0 1.2em; }
`

// 章节 XHTML ────────────────────────────────────────────────

func renderChapterXHTML(ch int, title string, loc chapterLocation, hasLoc bool, body string, lang Lang, translated bool, zhTitle, zhBody string) string {
	isVi := lang == LangVI || lang == LangBilingual
	var b strings.Builder
	var displayTitle, xmlLang string
	if isVi {
		xmlLang = "vi"
		displayTitle = fmt.Sprintf("Chương %d", ch)
		if title != "" {
			displayTitle = fmt.Sprintf("Chương %d: %s", ch, title)
		}
	} else {
		xmlLang = "zh-CN"
		displayTitle = fmt.Sprintf("第 %d 章", ch)
		if title != "" {
			displayTitle = fmt.Sprintf("第 %d 章 %s", ch, title)
		}
	}

	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
`, xmlLang, html.EscapeString(displayTitle))

	if hasLoc && loc.IsFirstOfVolume {
		if isVi {
			fmt.Fprintf(&b, "  <div class=\"volume-divider\">Quyển %d %s</div>\n",
				loc.VolumeIdx, html.EscapeString(strings.TrimSpace(loc.VolumeTitle)))
		} else {
			fmt.Fprintf(&b, "  <div class=\"volume-divider\">第 %d 卷 %s</div>\n",
				loc.VolumeIdx, html.EscapeString(strings.TrimSpace(loc.VolumeTitle)))
		}
	}

	fmt.Fprintf(&b, "  <h1 class=\"chapter-title\">%s</h1>\n", html.EscapeString(displayTitle))
	if !isVi || translated {
		for _, para := range splitParagraphs(body) {
			fmt.Fprintf(&b, "  <p>%s</p>\n", html.EscapeString(para))
		}
	} else if lang == LangVI {
		b.WriteString("  <p class=\"lang-note\">[Chưa dịch tiếng Việt — nguyên bản tiếng Trung]</p>\n")
		for _, para := range splitParagraphs(body) {
			fmt.Fprintf(&b, "  <p>%s</p>\n", html.EscapeString(para))
		}
	}

	if lang == LangBilingual {
		b.WriteString("  <hr class=\"bilingual-divider\"/>\n")
		b.WriteString("  <p class=\"lang-note\">[Nguyên bản tiếng Trung]</p>\n")
		zhHeading := fmt.Sprintf("第 %d 章", ch)
		if zhTitle != "" {
			zhHeading = fmt.Sprintf("第 %d 章 %s", ch, zhTitle)
		}
		fmt.Fprintf(&b, "  <h2 class=\"chapter-title\">%s</h2>\n", html.EscapeString(zhHeading))
		for _, para := range splitParagraphs(zhBody) {
			fmt.Fprintf(&b, "  <p>%s</p>\n", html.EscapeString(para))
		}
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// splitParagraphs 按空行切段；连续多空行视为一个分段。返回的段落都已 TrimSpace 且非空。
// 段内换行（单个 \n）保留为段内空格——XHTML 的 <p> 不保留换行，浏览器自动 wrap。
func splitParagraphs(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	parts := strings.Split(body, "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 段内换行变空格，避免 XHTML 渲染时丢内容
		p = strings.ReplaceAll(p, "\n", " ")
		out = append(out, p)
	}
	return out
}

// 封面 ────────────────────────────────────────────────

func renderCoverXHTML(novelName string, isVi bool) string {
	xmlLang := "zh-CN"
	coverTitle := "封面"
	if isVi {
		xmlLang = "vi"
		coverTitle = "Bìa sách"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
`, xmlLang, coverTitle)
	if name := strings.TrimSpace(novelName); name != "" {
		fmt.Fprintf(&b, "  <h1 class=\"book-title\">%s</h1>\n", html.EscapeString(name))
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// nav.xhtml（EPUB 3 navigation）────────────────────────────────────────────────

func renderNavXHTML(hasCover bool, chapters []int, titleIdx chapterTitleIndex, isVi bool) string {
	xmlLang := "zh-CN"
	navTitle := "目录"
	coverLabel := "封面"
	if isVi {
		xmlLang = "vi"
		navTitle = "Mục lục"
		coverLabel = "Bìa sách"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
  <nav epub:type="toc">
    <h1>%s</h1>
    <ol>
`, xmlLang, navTitle, navTitle)
	if hasCover {
		fmt.Fprintf(&b, "      <li><a href=\"cover.xhtml\">%s</a></li>\n", coverLabel)
	}

	// 平铺章节列表。卷/弧分组在阅读器里反而不如单层目录清爽（阅读器自己会折叠），
	// 而且 EPUB 3 nav 嵌套 ol 在某些阅读器上渲染怪。保持简单。
	for _, ch := range chapters {
		title := strings.TrimSpace(titleIdx[ch])
		var display string
		if isVi {
			display = fmt.Sprintf("Chương %d", ch)
			if title != "" {
				display = fmt.Sprintf("Chương %d: %s", ch, title)
			}
		} else {
			display = fmt.Sprintf("第 %d 章", ch)
			if title != "" {
				display = fmt.Sprintf("第 %d 章 %s", ch, title)
			}
		}
		fmt.Fprintf(&b, "      <li><a href=\"%s\">%s</a></li>\n",
			chapterFileName(ch), html.EscapeString(display))
	}

	b.WriteString(`    </ol>
  </nav>
</body>
</html>
`)
	return b.String()
}

// content.opf ────────────────────────────────────────────────

func renderOPF(book domain.BookMetadata, hasCover bool, chapters []int, isVi bool) string {
	bookID := bookIdentifier(book.Title)
	modified := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	title := strings.TrimSpace(book.Title)
	if title == "" {
		title = "Untitled"
	}
	xmlLang := "zh-CN"
	if isVi {
		xmlLang = "vi"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="%s">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:language>%s</dc:language>
    <dc:creator>ainovel-cli</dc:creator>
    <dc:description>%s</dc:description>
    <meta property="dcterms:modified">%s</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
`, xmlLang, html.EscapeString(bookID), html.EscapeString(title), xmlLang, html.EscapeString(book.Synopsis), modified)

	if hasCover {
		b.WriteString(`    <item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	for _, ch := range chapters {
		fmt.Fprintf(&b, `    <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n",
			chapterID(ch), chapterFileName(ch))
	}

	b.WriteString("  </manifest>\n  <spine>\n")
	if hasCover {
		b.WriteString(`    <itemref idref="cover"/>` + "\n")
	}
	b.WriteString(`    <itemref idref="nav"/>` + "\n")
	for _, ch := range chapters {
		fmt.Fprintf(&b, `    <itemref idref="%s"/>`+"\n", chapterID(ch))
	}
	b.WriteString("  </spine>\n</package>\n")
	return b.String()
}

// bookIdentifier 由小说名派生稳定 UUID 字符串。
//
// **只用 novelName，不掺章节列表**：作品身份应跟"是哪本书"绑定，不跟"导出范围"
// 或"导出时刻已写到第几章"绑定。重导出同一本书 ID 不变，阅读器据此识别为同一作品
// 的更新版本（更新与否由 dcterms:modified 时间戳承担）。空 novelName 共享 ID 是
// 已知边角 case：用户给两本书都不起名时责任自负。
func bookIdentifier(novelName string) string {
	h := sha1.New()
	h.Write([]byte(novelName))
	sum := h.Sum(nil)
	// 格式化为 UUID 风格（8-4-4-4-12），不要求严格 RFC 4122 — EPUB 只要求字符串唯一稳定。
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
