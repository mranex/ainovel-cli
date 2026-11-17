package trans

import (
	"fmt"
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func genreRuleBlock(genre domain.TranslationGenre) string {
	switch genre.Normalize() {
	case domain.TransGenreTienHiep:
		return `### Thể loại: Tiên hiệp / Huyền huyễn cổ phong (tien_hiep)
- **Tên riêng, tông môn, địa danh, cảnh giới, công pháp, pháp bảo**: Dịch theo âm **Hán-Việt** chuẩn xác, trang trọng (ví dụ: Trúc Cơ, Kim Đan, Nguyên Anh, Thanh Vân Tông, Linh Khí).
- **Đại từ nhân xưng**: Giữ sắc thái cổ phong tu chân (*ta - ngươi - hắn - nàng - y - đạo hữu - tiền bối - vãn bối - tại hạ - các hạ - bản tọa - lão phu*). Tránh lạm dụng *tôi/bạn/anh/em* hiện đại.`
	case domain.TransGenreCoTrang:
		return `### Thể loại: Cổ trang / Kiếm hiệp / Cung đình (co_trang)
- **Tên riêng, danh hiệu, địa danh, chiêu thức**: Dùng âm **Hán-Việt** tao nhã, chuẩn cổ trang/kiếm hiệp.
- **Đại từ nhân xưng**: Linh hoạt theo vai vế (*ta - ngươi - chàng - nàng - thiếp - huynh - muội - tỷ - đệ - tại hạ - các hạ - bổn vương - trẫm - nô tỳ*). Văn phong mượt mà, giàu chất kiếm hiệp/cổ phong.`
	case domain.TransGenreFantasy:
		return `### Thể loại: Fantasy / Dị giới / Light Novel phương Tây (fantasy)
- **QUY TẮC TÊN RIÊNG BẮT BUỘC (KHỬ HÁN-VIỆT TÊN TÂY)**: Nguyên tác viết bằng chữ Hán buộc phải phiên âm tên phương Tây (như 艾莉丝, 亚瑟, 莱因哈特, 哥布林, 精灵). Khi dịch sang tiếng Việt, PHẢI **khôi phục về tên gốc tiếng Anh / Latin hoặc dịch nghĩa Fantasy chuẩn** (ví dụ: *Alice, Arthur, Reinhardt, Goblin, Elf, Mana*), **TUYỆT ĐỐI KHÔNG** phiên âm kiểu Hán-Việt sượng trân như *Ngải Lị Ti*, *Á Sắt*, *Ca Bố Lâm*!
- **Chiêu thức / Phép thuật / Chức nghiệp**: Dịch nghĩa tiếng Việt gọn đẹp hoặc giữ thuật ngữ Fantasy quen thuộc (*Hỏa Cầu Thuật / Fireball, Ma pháp sư, Kỵ sĩ, Hiệp sĩ, Ma thú, Dung hợp*).
- **Đại từ nhân xưng**: Dùng văn phong hiện đại / Light Novel (*tôi - cậu - anh - cô - hắn - nàng - ngài - tiểu thư - thiếu gia - công chúa*), tránh đại từ tu tiên (*đạo hữu, bản tọa*).`
	case domain.TransGenreHienDai:
		return `### Thể loại: Đô thị / Hiện đại / Đời thường (hien_dai)
- **Văn phong & Đối thoại**: Dùng tiếng Việt hiện đại tự nhiên như tiểu thuyết xuất bản / truyện dài hiện đại. Tuyệt đối tránh từ ngữ convert hay cổ trang (*nữ nhân này, nam nhân kia, không khỏi có chút, hít sâu một ngụm khí lạnh, bổn thiếu gia*).
- **Đại từ nhân xưng**: Phù hợp quan hệ và tuổi tác hiện đại (*tôi - anh - em - cô ấy - cậu - tớ - mày - tao - ông - bà - chú - cháu*).
- **Tên người**: Nếu tên Trung Quốc hiện đại giữ Hán-Việt quen thuộc (Lâm Trần, Tô Mộc), nếu là tên phương Tây trong bối cảnh quốc tế thì khôi phục về dạng Latin chuẩn.`
	default: // auto
		return `### Thể loại: Tự động nhận diện (auto)
- Ở bước khởi tạo từ điển hoặc theo ngữ cảnh truyện, tự xác định thể loại thực sự là một trong 4 nhóm: **tien_hiep** (Tiên hiệp/Huyền huyễn), **co_trang** (Cổ trang/Kiếm hiệp), **fantasy** (Tây phương/Dị giới/Light Novel), hoặc **hien_dai** (Đô thị/Hiện đại).
- Áp dụng nhất quán triệt để quy tắc tên riêng & xưng hô của nhóm đã chọn (đặc biệt: nếu là Fantasy Tây phương phải khôi phục tên gốc Latin/Anh như *Alice, Arthur, Goblin*, tuyệt đối không đọc âm Hán-Việt thô).`
	}
}

func toneRuleBlock(tone domain.TranslationTone) string {
	if tone.Normalize() == domain.TransToneMature {
		return `### Sắc thái / Độ tuổi: 18+ Trưởng thành (mature)
- Đây là tác phẩm văn học dành cho độc giả trưởng thành (18+).
- **Dịch trung thực 100% nguyên tác**: Giữ trọn vẹn mọi diễn biến tình cảm, cảnh nóng, bầu không khí nhục cảm, cảm quan cơ thể, rung động tâm lý và xung đột dữ dội trong bản gốc tiếng Trung. TUYỆT ĐỐI KHÔNG tự ý cắt bỏ, làm mờ, nói giảm nói tránh hay "kiểm duyệt" nội dung đã có trong nguyên tác.
- **Văn phong văn học quyến rũ, tự nhiên**: Dùng ngôn từ văn học tình cảm/trưởng thành tiếng Việt mượt mà, giàu gợi cảm và nhịp điệu; tránh dùng thuật ngữ giải phẫu y khoa khô cứng hay từ ngữ thô kệch làm mất cảm xúc văn chương.`
	}
	return `### Sắc thái / Độ tuổi: Phổ thông (normal)
- Giữ đúng mức độ tình cảm, kịch tính và sắc thái nguyên tác bằng văn phong tiểu thuyết phổ thông tinh tế, tự nhiên, giàu cảm xúc.`
}

// SeedSystemPrompt tạo system prompt cho bước dịch BookMetadata + Volumes + dựng Glossary gốc.
func SeedSystemPrompt(genre domain.TranslationGenre, tone domain.TranslationTone) string {
	return fmt.Sprintf(`Bạn là Biên dịch viên Văn học Trung → Việt hàng đầu, chuyên chuyển ngữ tiểu thuyết dài kỳ sang tiếng Việt xuất bản mượt mà, tuyệt đối không mang "mùi convert / QT".

Nhiệm vụ lần này: Phân tích hồ sơ tác phẩm tiếng Trung (Tên truyện, Văn án, Tên các quyển, Danh sách nhân vật, Quy tắc thế giới, Tiền đề) để tạo **Hồ sơ bản dịch & Từ điển nhất quán (Glossary)** sang tiếng Việt.

%s

%s

### Quy chuẩn đầu ra XML (BẮT BUỘC dùng đúng các thẻ chữ thường sau, ngoài thẻ không viết thừa)
<resolved_genre>Chọn ĐÚNG 1 trong 4 giá trị: tien_hiep | co_trang | fantasy | hien_dai</resolved_genre>
<book_title>Tên truyện tiếng Việt thật kêu, tự nhiên</book_title>
<book_synopsis>Văn án / tóm tắt tiếng Việt lôi cuốn, mượt mà</book_synopsis>
<volumes>
Mỗi dòng một quyển (nếu có danh sách quyển), định dạng:
1 => Tên quyển 1 tiếng Việt
</volumes>
<pronoun_guide>
Hướng dẫn ngắn gọn (3-6 gạch đầu dòng) quy ước xưng hô cốt lõi giữa nhân vật chính và các tuyến nhân vật để giữ đồng bộ 100%% qua hàng trăm chương.
</pronoun_guide>
<glossary>
Mỗi dòng một thuật ngữ quan trọng (tên nhân vật, biệt danh, địa danh, tông môn, cảnh giới, chiêu thức, vật phẩm đặc thù), định dạng:
Chữ Hán => Bản dịch tiếng Việt chuẩn | loại (character/place/term/title/skill) | ghi chú ngắn (nếu cần)
</glossary>`, genreRuleBlock(genre), toneRuleBlock(tone))
}

// ChapterSystemPrompt tạo system prompt cho bước dịch từng chương hoàn thiện sang tiếng Việt.
func ChapterSystemPrompt(genre domain.TranslationGenre, tone domain.TranslationTone) string {
	return fmt.Sprintf(`Bạn là Biên dịch viên Văn học Trung → Việt xuất sắc. Hãy dịch toàn bộ chương truyện tiếng Trung được cung cấp sang **tiếng Việt văn học trau chuốt, giàu nhịp điệu, đọc như tác phẩm viết trực tiếp bằng tiếng Việt**.

### Nguyên tắc vàng chống "mùi convert / QT"
1. **Dịch thoát cấu trúc Hán ngữ**: Không giữ trật tự từ Hán-Việt gượng gạo (tránh tuyệt đối kiểu *đích, liễu, tại... thời điểm, đối với... tới nói, không khỏi có chút, hít sâu một ngụm khí lạnh, nhếch miệng cười lạnh, trong mắt lóe qua một tia...* lặp máy móc). Phải tách ghép câu linh hoạt theo ngữ pháp tự nhiên của tiếng Việt.
2. **Dịch trọn vẹn 100%% nội dung**: Không tóm tắt, không bỏ đoạn, không cắt cảnh, không sót thoại hay chi tiết miêu tả nào của nguyên tác.
3. **Tuân thủ tuyệt đối Từ điển (Glossary) & Quy tắc xưng hô** được cung cấp trong ngữ cảnh để tên riêng và xưng hô thống nhất từ chương 1 tới chương cuối; bám theo giọng văn và xưng hô ở đoạn cuối chương trước (nếu có).
4. **Hành động & Đối thoại**: Thoại phải mang đúng khí chất nhân vật, có ngữ điệu tự nhiên tiếng Việt; miêu tả chiến đấu hoặc tâm lý phải có tiết tấu nhanh/chậm khớp với văn cảnh.

%s

%s

### Quy chuẩn đầu ra XML (BẮT BUỘC dùng đúng 3 thẻ sau, đủ đóng mở, không chèn bình luận thừa bên ngoài)
<title>Chỉ ghi tiêu đề chương bằng tiếng Việt (KHÔNG kèm "Chương N:")</title>
<body>
Toàn bộ nội dung chính văn tiếng Việt của chương (đã bỏ dòng tiêu đề đầu chương để tránh lặp), phân đoạn sạch đẹp bằng dòng trống.
</body>
<glossary_updates>
Chỉ liệt kê các tên riêng / thuật ngữ / chiêu thức / địa danh MỚI xuất hiện lần đầu trong chương này mà chưa có trong Glossary (nếu không có để trống thẻ), mỗi dòng theo định dạng:
Chữ Hán => Bản dịch tiếng Việt | loại | ghi chú ngắn
</glossary_updates>`, genreRuleBlock(genre), toneRuleBlock(tone))
}

func formatSeedUserPayload(titleZh, synopsisZh, premise string, volumes []domain.VolumeOutline, chars []domain.Character, rules []domain.WorldRule, preferredGenre domain.TranslationGenre) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Thể loại người dùng chọn: %s\n", preferredGenre.Normalize())
	fmt.Fprintf(&b, "Tên truyện gốc: %s\n", strings.TrimSpace(titleZh))
	fmt.Fprintf(&b, "Văn án gốc:\n%s\n\n", strings.TrimSpace(synopsisZh))
	if strings.TrimSpace(premise) != "" {
		fmt.Fprintf(&b, "Tiền đề / Bối cảnh (Premise):\n%s\n\n", truncateRunesStr(strings.TrimSpace(premise), 2400))
	}
	if len(volumes) > 0 {
		b.WriteString("Danh sách quyển:\n")
		for _, v := range volumes {
			fmt.Fprintf(&b, "- Quyển %d: %s (Chủ đề: %s)\n", v.Index, strings.TrimSpace(v.Title), strings.TrimSpace(v.Theme))
		}
		b.WriteByte('\n')
	}
	if len(chars) > 0 {
		b.WriteString("Nhân vật chính/phụ:\n")
		for _, c := range chars {
			fmt.Fprintf(&b, "- %s", strings.TrimSpace(c.Name))
			if len(c.Aliases) > 0 {
				fmt.Fprintf(&b, " (biệt danh: %s)", strings.Join(c.Aliases, ", "))
			}
			if c.Role != "" {
				fmt.Fprintf(&b, " [%s]", c.Role)
			}
			if c.Description != "" {
				fmt.Fprintf(&b, ": %s", truncateRunesStr(strings.TrimSpace(c.Description), 180))
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(rules) > 0 {
		b.WriteString("Quy tắc thế giới:\n")
		for i, r := range rules {
			if i >= 20 {
				break
			}
			fmt.Fprintf(&b, "- [%s] %s\n", strings.TrimSpace(r.Category), truncateRunesStr(strings.TrimSpace(r.Rule), 160))
		}
	}
	return b.String()
}

func formatChapterUserPayload(chapter int, titleZh, bodyZh string, glossary *Glossary, prevTailVi string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Thông tin chương cần dịch\n- Chương số: %d\n", chapter)
	if strings.TrimSpace(titleZh) != "" {
		fmt.Fprintf(&b, "- Tiêu đề gốc tiếng Trung: %s\n", strings.TrimSpace(titleZh))
	}
	if glossary != nil {
		if strings.TrimSpace(glossary.PronounGuide) != "" {
			fmt.Fprintf(&b, "\n## Quy ước xưng hô cố định\n%s\n", strings.TrimSpace(glossary.PronounGuide))
		}
		if len(glossary.Entries) > 0 {
			b.WriteString("\n## Từ điển thuật ngữ & tên riêng (BẮT BUỘC tuân thủ)\n")
			for i, e := range glossary.Entries {
				if i >= 160 {
					break
				}
				fmt.Fprintf(&b, "- %s => %s", e.Source, e.Target)
				if e.Kind != "" || e.Note != "" {
					fmt.Fprintf(&b, " (%s%s%s)", e.Kind, joinCommaIfBoth(e.Kind, e.Note), e.Note)
				}
				b.WriteByte('\n')
			}
		}
	}
	if strings.TrimSpace(prevTailVi) != "" {
		fmt.Fprintf(&b, "\n## Đoạn cuối bản dịch Chương %d (chỉ để tham khảo giọng văn & xưng hô nối tiếp, KHÔNG dịch lại)\n%s\n", chapter-1, strings.TrimSpace(prevTailVi))
	}
	fmt.Fprintf(&b, "\n## Nguyên tác Chương %d cần dịch trọn vẹn\n%s\n", chapter, strings.TrimSpace(bodyZh))
	return b.String()
}

func joinCommaIfBoth(a, b string) string {
	if strings.TrimSpace(a) != "" && strings.TrimSpace(b) != "" {
		return ", "
	}
	return ""
}

func truncateRunesStr(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
