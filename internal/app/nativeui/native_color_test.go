package nativeui

import (
	"context"
	"image"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestColorPreviewKeepsExactSwatchInsideRoundedGlassMargin(t *testing.T) {
	for _, raw := range []string{"#D78379", "#FFFFFF", "#121A28", "rgba(215, 131, 121, 0.5)"} {
		for _, dark := range []bool{false, true} {
			s, _ := semanticPreviewFixture(t, raw)
			s.viewState.Dark = dark
			tt := ui.NewTester(s.nativePopoverView, 520, nativeColorPreviewHeight)
			pixels := tt.Image()
			glass := pixels.RGBAAt(7, 51)
			for _, point := range []image.Point{{9, 51}, {510, 51}, {9, nativeColorPreviewHeight - 10}, {510, nativeColorPreviewHeight - 10}} {
				if got := pixels.RGBAAt(point.X, point.Y); got != glass {
					t.Fatalf("color=%q dark=%t fills glass corner %v: %v vs %v", raw, dark, point, got, glass)
				}
			}
			if raw == "#D78379" {
				got := pixels.RGBAAt(260, 54)
				if got.R != 215 || got.G != 131 || got.B != 121 || got.A != 255 {
					t.Fatalf("swatch changed copied RGB color in dark=%t: %v", dark, got)
				}
			}
		}
	}
}

func TestColorPreviewFormatsFitCompactSurfaceAndKeepFocus(t *testing.T) {
	for _, raw := range []string{"#D78379", "rgba(215, 131, 121, 0.5)"} {
		s, item := semanticPreviewFixture(t, raw)
		if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
			t.Fatal(err)
		}
		s.filter()
		value := clipmodel.Analyze(raw).Color
		tt := ui.NewTester(s.nativePopoverView, 520, nativeColorPreviewHeight)
		for _, format := range []struct{ name, display string }{{"RGB", value.RGB}, {"HSL", value.HSL}, {"HEX", value.HEX}} {
			if err := tt.Click(format.name); err != nil {
				t.Fatal(err)
			}
			if !tt.Focused("Content preview") {
				t.Fatal("format switch lost navigation focus")
			}
			label, found := tt.Find(format.display)
			if !found || label.X < 24 || label.X+label.W > 496 || label.Y < 66 || label.Y+label.H > nativeColorPreviewHeight-50 {
				t.Fatalf("%s label escaped compact protected preview: %+v", format.name, label)
			}
		}
		assertNativeCurrent(t, s, item.ID)
	}
}
