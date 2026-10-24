package nativeui

import (
	"context"
	"image"
	"math"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func TestNativeAppLinkPreviewKeepsDenseQRScannableAboveProtection(t *testing.T) {
	for _, raw := range []string{
		"aweme://video/7540921836?refer=copy#saved",
		"sslocal://route/" + strings.Repeat("a", 1600),
	} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, raw)
			if item.Type != clipmodel.TypeAppLink {
				t.Fatalf("fixture was not a distinct App link: %s", item.Type)
			}
			s.viewState.Dark = dark
			if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"editor.app"}); err != nil {
				t.Fatal(err)
			}
			s.filter()
			width, height := nativeItemPreviewSize(item)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			code := s.semanticState(item).QRCode
			r, ok := tt.Find("QR code")
			if !ok || code == nil || int(r.W)%code.modules != 0 || r.W/float32(code.modules) < 2 {
				t.Fatalf("App QR lost complete readable modules: %+v", r)
			}
			caption, found := tt.Find("Scan with your phone")
			copy, hasCopy := tt.Find("Copy")
			if !found || !hasCopy || r.Y < 50 || r.Y+r.H > caption.Y || caption.Y+caption.H > copy.Y-nativePreviewSectionGap {
				t.Fatalf("App QR displaced stationary footer: QR=%+v, caption=%+v, copy=%+v", r, caption, copy)
			}
			pixels := tt.Image()
			bounds := image.Rect(int(math.Floor(float64(r.X))), int(math.Floor(float64(r.Y))), int(math.Ceil(float64(r.X+r.W))), int(math.Ceil(float64(r.Y+r.H))))
			binary, err := gozxing.NewBinaryBitmapFromImage(pixels.SubImage(bounds))
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := qrcode.NewQRCodeReader().Decode(binary, nil)
			if err != nil || decoded.GetText() != raw {
				t.Fatalf("App QR failed independent decode (dark=%v): %v", dark, err)
			}
			if code.modules == 157 && r.W != 314 {
				t.Fatal("dense App QR did not retain its full readable size")
			}
		}
	}
}

func TestNativeAppLinkQRRemovesProtocolHeaderAndKeepsOriginalFooter(t *testing.T) {
	for _, tc := range []struct{ raw, route string }{
		{"aweme://video/7540921836?refer=copy#saved", "video/7540921836"},
		{"sslocal:///search/results?q=one", "search/results"},
		{"aweme://video/path%20with%20spaces", "video/path%20with%20spaces"},
	} {
		s, item := semanticPreviewFixture(t, tc.raw)
		width, height := nativeItemPreviewSize(item)
		tt := ui.NewTester(s.nativePopoverView, width, height)
		link := clipmodel.ParseLink(tc.raw)
		for _, text := range tt.Texts() {
			if text == link.Scheme+"://" || text == tc.route {
				t.Fatalf("QR preview retained a redundant protocol header: %q", text)
			}
		}
		if !tt.HasText(tc.raw) || !tt.HasText("Scan with your phone") || s.semanticState(item).QRTarget != tc.raw {
			t.Fatal("removing the header lost the original footer, caption, or QR target")
		}
	}
}
