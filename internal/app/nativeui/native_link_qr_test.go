package nativeui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"copiggy/internal/linkqr"
	"github.com/egoist/mygo/ui"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func TestLinkQRDisplayUsesWholeModulesAndMinimumReadableSize(t *testing.T) {
	for _, modules := range []int{29, 37, 57, 101, linkqr.MaxModuleCount} {
		for _, available := range []float32{120, 319, 320, 500} {
			size := nativeLinkQRDisplaySize(modules, available, available)
			if size == 0 {
				if available >= float32(modules*2) {
					t.Fatalf("readable code rejected: modules=%d available=%g", modules, available)
				}
				continue
			}
			if size > available || size > nativeLinkQRMaxSize || int(size)%modules != 0 || size/float32(modules) < 2 {
				t.Fatalf("fractional, clipped or too-small modules: modules=%d size=%g", modules, size)
			}
		}
	}
	for _, input := range [][3]float32{{0, 320, 320}, {37, 0, 320}, {37, 320, -1}} {
		if got := nativeLinkQRDisplaySize(int(input[0]), input[1], input[2]); got != 0 {
			t.Fatalf("invalid viewport produced %g", got)
		}
	}
}

func TestLinkQRNativeRenderingRemainsScannableInBothThemes(t *testing.T) {
	for _, target := range []string{
		"https://example.com/path?q=测试#section",
		"aweme://aweme/detail/123456789?source=copy",
		"https://example.com/" + strings.Repeat("a", 1600),
	} {
		for _, dark := range []bool{false, true} {
			s := nativeViewFixture(t)
			s.language, s.viewState.Dark = "en", dark
			code := newNativeLinkQRCode(target)
			if code.failure != "" {
				t.Fatal(code.failure)
			}
			kind, ink := clipmodel.TypeLink, nativeLinkColors()
			if link := clipmodel.ParseLink(target); link != nil && link.App {
				kind, ink = clipmodel.TypeAppLink, nativeAppLinkColors()
			}
			item := clipmodel.Item{ID: "QR fixture", Type: kind, Text: target, AllowedApps: []string{"com.example.Editor"}}
			view := func(c *ui.Context) {
				p := panelColors(dark)
				setNativeTheme(c, p, dark, s.currentLanguage())
				nativePreviewLayout(c).Children(func() {
					nativePreviewToolbarRow(c)
					s.nativeLinkQRContent(c, item, code, p)
					ui.Box(c).Height(nativePreviewMetadataHeight*2 + nativePreviewScopeGap).Shrink(0)
				})
			}
			tt := ui.NewTester(view, 520, 500)
			r, ok := tt.Find("QR code")
			if !ok || r.W != r.H || int(r.W)%code.modules != 0 || r.X < 22 || r.Y < 64 || r.X+r.W > 498 || r.Y+r.H > 420 {
				t.Fatalf("QR symbol escaped content area: %+v", r)
			}
			pixels := tt.Image()
			wantPaper := color.RGBA{R: ink.paper.R, G: ink.paper.G, B: ink.paper.B, A: 255}
			if got := pixels.RGBAAt(20, 80); got != wantPaper {
				t.Fatalf("QR reading surface is not opaque native paper (dark=%t): got=%v want=%v", dark, got, wantPaper)
			}
			if pixels.RGBAAt(7, 80).A != 0 {
				t.Fatal("QR paper covered the common glass margin")
			}
			if texts := tt.Texts(); len(texts) != 2 || texts[0] != "QR code" || texts[1] != "Scan with your phone" {
				t.Fatalf("QR body contains content beyond its scan caption: %v", texts)
			}
			// Decode the exact native rendered pixels, including the quiet zone;
			// this catches image filtering or appearance tinting regressions.
			bounds := image.Rect(int(math.Floor(float64(r.X))), int(math.Floor(float64(r.Y))), int(math.Ceil(float64(r.X+r.W))), int(math.Ceil(float64(r.Y+r.H))))
			binary, err := gozxing.NewBinaryBitmapFromImage(pixels.SubImage(bounds))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := qrcode.NewQRCodeReader().Decode(binary, nil)
			if err != nil {
				t.Fatalf("native QR at %gpx is not scannable (dark=%t): %v", r.W, dark, err)
			}
			if decoded.GetText() != target {
				t.Fatalf("rendering changed the QR target: %q", decoded.GetText())
			}
		}
	}
}

func TestLinkQROverflowHasLocalizedFailureWithoutPartialSymbol(t *testing.T) {
	code := newNativeLinkQRCode("https://example.com/" + strings.Repeat("a", 1800))
	if code.bitmap != nil || code.modules != 0 || code.failure != "Link is too long for a QR code" {
		t.Fatalf("overflow produced partial content: %+v", code)
	}
	if code := newNativeLinkQRCode(""); code.bitmap != nil || code.failure != "QR code unavailable" {
		t.Fatalf("invalid content was not rejected: %+v", code)
	}
}
