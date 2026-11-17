package tui

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
)

type fakeTransRuntime struct {
	profile    domain.TranslationProfile
	saveGlobal bool
	setCalls   int
}

func (f *fakeTransRuntime) TranslationProfile() domain.TranslationProfile {
	return f.profile
}

func (f *fakeTransRuntime) SetTranslationProfile(p domain.TranslationProfile, saveGlobal bool) error {
	f.profile = p.Normalized()
	f.saveGlobal = saveGlobal
	f.setCalls++
	return nil
}

func TestTransSwitchCyclingAndApply(t *testing.T) {
	rt := &fakeTransRuntime{
		profile: domain.TranslationProfile{
			Genre:         domain.TransGenreAuto,
			Tone:          domain.TransToneNormal,
			AutoTranslate: domain.AutoTranslateAfterArc,
		},
	}
	st := newTransSwitchState(rt)
	// Chuyển sang fantasy (3 bước tiến từ auto)
	for i := 0; i < 3; i++ {
		st.cycle(1)
	}
	st.moveFocus(1) // Tone
	st.cycle(1)     // normal -> mature
	st.moveFocus(1) // Auto
	st.cycle(1)     // after_arc -> after_commit
	st.moveFocus(1) // Scope
	st.cycle(1)     // saveGlobal = true

	if err := st.apply(rt); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rt.profile.Genre != domain.TransGenreFantasy || rt.profile.Tone != domain.TransToneMature || rt.profile.AutoTranslate != domain.AutoTranslateAfterCommit || !rt.saveGlobal {
		t.Fatalf("unexpected applied profile: %+v saveGlobal=%v", rt.profile, rt.saveGlobal)
	}

	out := renderTransSwitchBar(100, st)
	if !strings.Contains(out, "Fantasy") || !strings.Contains(out, "18+") {
		t.Fatalf("rendered switch bar missing labels:\n%s", out)
	}
}

func TestApplyTransArgsInline(t *testing.T) {
	rt := &fakeTransRuntime{
		profile: domain.TranslationProfile{}.Normalized(),
	}
	if err := applyTransArgs(rt, []string{"genre=fantasy", "tone=mature", "auto=manual", "--global"}); err != nil {
		t.Fatalf("applyTransArgs: %v", err)
	}
	if rt.profile.Genre != domain.TransGenreFantasy || rt.profile.Tone != domain.TransToneMature || rt.profile.AutoTranslate != domain.AutoTranslateManual || !rt.saveGlobal {
		t.Fatalf("unexpected inline profile: %+v global=%v", rt.profile, rt.saveGlobal)
	}
	if err := applyTransArgs(rt, []string{"genre=unknown"}); err == nil {
		t.Fatal("expected error for unknown genre")
	}
}
