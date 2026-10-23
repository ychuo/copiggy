package nativeui

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestPhonePreviewPaperKeepsRoundedGlassMargin(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, _ := semanticPreviewFixture(t, "+1 (202) 555-0148")
		s.viewState.Dark = dark
		tt := ui.NewTester(s.nativePopoverView, 520, 360)
		// Inspect the real preview's paint output only in memory. The four
		// paper corners must expose the same transparent glass margin as the
		// adjacent outside pixel, while the middle of its top edge is paper.
		pixels := tt.Image()
		glass := pixels.RGBAAt(7, 51)
		for _, point := range []image.Point{{9, 51}, {510, 51}, {9, 350}, {510, 350}} {
			if got := pixels.RGBAAt(point.X, point.Y); got != glass {
				t.Fatalf("paper fills rounded corner in dark=%t at %v: got=%v glass=%v", dark, point, got, glass)
			}
		}
		if got := pixels.RGBAAt(260, 54); got.A < 250 {
			t.Fatalf("reading paper is missing or translucent: %v", got)
		}
	}
}

func TestPhonePresentationPreservesCopiedGroups(t *testing.T) {
	for _, tc := range []struct {
		input, prefix string
		lines         []string
	}{
		{"+1 (202) 555-0148", "+1", []string{"+1 (202)", "555-0148"}},
		{"+44 20 7946 0958", "+44", []string{"+44 20", "7946 0958"}},
		{"+12025550148", "", []string{"+12025550148"}},
		{"(202) 555-0148", "", []string{"(202)", "555-0148"}},
		{"+1202 555 0148", "", []string{"+1202", "555 0148"}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			value := clipmodel.Analyze(tc.input).Phone
			if value == nil {
				t.Fatal("fixture was not recognized as a phone number")
			}
			lines := nativePhoneLines(value.Display)
			if !reflect.DeepEqual(lines, tc.lines) || nativePhonePrefix(*value) != tc.prefix {
				t.Fatalf("unexpected phone presentation: lines=%q prefix=%q", lines, nativePhonePrefix(*value))
			}
			// Joining visual lines may change whitespace, but never punctuation,
			// digits or the original value kept in the semantic model.
			if strings.ReplaceAll(strings.Join(lines, ""), " ", "") != strings.ReplaceAll(tc.input, " ", "") || value.Display != tc.input {
				t.Fatal("display grouping rewrote the copied number")
			}
		})
	}
}

func TestPhoneNumberFitsCardAboveDialPad(t *testing.T) {
	for _, raw := range []string{"+1 (202) 555-0148", "+86 138 0013 8000", "+493012345678901", "(202) 555-0148"} {
		t.Run(raw, func(t *testing.T) {
			s, item := semanticPreviewFixture(t, raw)
			tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
			card, ok := tt.Find(s.nativeCardLabel(item))
			if !ok {
				t.Fatal("phone card is missing")
			}
			phone := clipmodel.Analyze(raw).Phone
			if phone == nil {
				t.Fatal("fixture was not recognized as a phone number")
			}
			lines := nativePhoneLines(phone.Display)
			for _, line := range lines {
				r, found := tt.Find(line)
				if !found || r.X < card.X+nativeCardPadding || r.X+r.W > card.X+card.W-nativeCardPadding || r.Y < card.Y+50 || r.Y+r.H > card.Y+116 {
					t.Fatalf("number escaped its area above the dial pad: line=%q text=%+v card=%+v", line, r, card)
				}
			}
			preview := ui.NewTester(s.nativePopoverView, 520, 360)
			for _, line := range lines {
				r, found := preview.Find(line)
				if !found || r.X < 24 || r.X+r.W > 496 || r.Y < 66 || r.Y+r.H > 310 {
					t.Fatalf("preview number escaped reading surface: %q %+v", line, r)
				}
			}
			assertNativeCurrent(t, s, item.ID)
		})
	}
}
