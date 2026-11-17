package domain

import (
	"fmt"
	"strings"
)

// TranslationGenre xác định hệ quy chiếu văn phong, xưng hô và tên riêng khi dịch sang tiếng Việt.
type TranslationGenre string

const (
	TransGenreAuto     TranslationGenre = "auto"
	TransGenreTienHiep TranslationGenre = "tien_hiep"
	TransGenreCoTrang  TranslationGenre = "co_trang"
	TransGenreFantasy  TranslationGenre = "fantasy"
	TransGenreHienDai  TranslationGenre = "hien_dai"
)

func (g TranslationGenre) Normalize() TranslationGenre {
	v := TranslationGenre(strings.ToLower(strings.TrimSpace(string(g))))
	if v == "" {
		return TransGenreAuto
	}
	return v
}

func (g TranslationGenre) Valid() bool {
	switch g.Normalize() {
	case TransGenreAuto, TransGenreTienHiep, TransGenreCoTrang, TransGenreFantasy, TransGenreHienDai:
		return true
	default:
		return false
	}
}

// TranslationTone kiểm soát sắc thái/độ tuổi khi cầu nối đầu vào và dịch sang tiếng Việt.
type TranslationTone string

const (
	TransToneNormal TranslationTone = "normal"
	TransToneMature TranslationTone = "mature"
)

func (t TranslationTone) Normalize() TranslationTone {
	v := TranslationTone(strings.ToLower(strings.TrimSpace(string(t))))
	if v == "" {
		return TransToneNormal
	}
	return v
}

func (t TranslationTone) Valid() bool {
	switch t.Normalize() {
	case TransToneNormal, TransToneMature:
		return true
	default:
		return false
	}
}

// AutoTranslateMode quyết định thời điểm tự động dịch các chương đã hoàn thiện.
type AutoTranslateMode string

const (
	AutoTranslateAfterArc    AutoTranslateMode = "after_arc"
	AutoTranslateAfterCommit AutoTranslateMode = "after_commit"
	AutoTranslateManual      AutoTranslateMode = "manual"
)

func (m AutoTranslateMode) Normalize() AutoTranslateMode {
	v := AutoTranslateMode(strings.ToLower(strings.TrimSpace(string(m))))
	if v == "" {
		return AutoTranslateAfterArc
	}
	return v
}

func (m AutoTranslateMode) Valid() bool {
	switch m.Normalize() {
	case AutoTranslateAfterArc, AutoTranslateAfterCommit, AutoTranslateManual:
		return true
	default:
		return false
	}
}

// TranslationProfile lưu bộ công tắc dịch thuật riêng cho từng cuốn truyện (meta/run.json)
// hoặc làm cấu hình mặc định trong config.json.
type TranslationProfile struct {
	Genre         TranslationGenre  `json:"genre,omitempty"`
	Tone          TranslationTone   `json:"tone,omitempty"`
	AutoTranslate AutoTranslateMode `json:"auto_translate,omitempty"`
}

func (p TranslationProfile) Normalized() TranslationProfile {
	return TranslationProfile{
		Genre:         p.Genre.Normalize(),
		Tone:          p.Tone.Normalize(),
		AutoTranslate: p.AutoTranslate.Normalize(),
	}
}

func (p TranslationProfile) Validate() error {
	if p.Genre != "" && !p.Genre.Valid() {
		return fmt.Errorf("không hỗ trợ thể loại dịch %q (hợp lệ: auto/tien_hiep/co_trang/fantasy/hien_dai)", p.Genre)
	}
	if p.Tone != "" && !p.Tone.Valid() {
		return fmt.Errorf("không hỗ trợ chế độ nội dung %q (hợp lệ: normal/mature)", p.Tone)
	}
	if p.AutoTranslate != "" && !p.AutoTranslate.Valid() {
		return fmt.Errorf("không hỗ trợ chế độ tự động dịch %q (hợp lệ: after_arc/after_commit/manual)", p.AutoTranslate)
	}
	return nil
}
