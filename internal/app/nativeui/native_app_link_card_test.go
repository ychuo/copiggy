package nativeui

import (
	"image"
	"math"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestNativeAppLinkCardRouteSeparatesProtocolAndPrivateParameters(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"aweme://video/123?refer=web#section", "video/123"},
		{"sslocal://route?url=https%3A%2F%2Fexample.com&token=secret", "route"},
		{"custom-app:///route/123?mode=one", "/route/123"},
		{"my-app://route/My%20page/%E6%97%A5%E8%A8%98", "route/My page/日記"},
		{"my-app://route/a%0Ab", "route/a%0Ab"},
		{"my-app://route/a%E2%80%AEb", "route/a%E2%80%AEb"},
		{"my-app://route/a%FFb", "route/a%FFb"},
		{"[Video title](aweme://video/123?q=one)", "video/123"},
	} {
		link := clipmodel.ParseLink(tc.raw)
		if link == nil || !link.App {
			t.Fatalf("invalid app-link fixture %q", tc.raw)
		}
		if got := nativeAppLinkCardRoute(link); got != tc.want {
			t.Errorf("route(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
	if nativeAppLinkCardRoute(nil) != "" || nativeAppLinkCardRoute(clipmodel.ParseLink("https://example.com/page")) != "" {
		t.Fatal("app route accepted a missing or webpage destination")
	}
}

func TestNativeAppLinkCardHierarchyFitsSharedBodyWithoutFetchingOrMutating(t *testing.T) {
	for _, raw := range []string{
		"aweme://video/123?refer=web#part",
		"sslocal://route?url=https%3A%2F%2Fexample.com&token=secret",
		"my-app://route/" + strings.Repeat("segment/", 30) + "?private=hidden",
		"my-app://route/%E6%97%A5%E8%A8%98/%E3%83%9A%E3%83%BC%E3%82%B8?private=hidden",
		strings.Repeat("protocol", 8) + "://route/path?private=hidden",
	} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, raw)
			before := s.store.Records()
			link := clipmodel.ParseLink(raw)
			scheme, route := link.Scheme+"://", nativeAppLinkCardRoute(link)
			// Unrelated artwork/metadata must not replace an app protocol and
			// route even if a stale caller accidentally supplies website assets.
			icon := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 32, 32)))
			tt := ui.NewTester(func(c *ui.Context) {
				setNativeTheme(c, nativeLinkColors(), dark)
				ui.Box(c).Fill().Children(func() {
					ui.Box(c).Absolute().Top(50).Left(nativeCardPadding).Right(nativeCardPadding).Bottom(42).Children(func() {
						s.nativeLinkCardBodyWithIcon(c, item, "Unrelated website title", true, icon)
					})
				})
			}, nativeCardSize, nativeCardSize)
			titleBounds, titleOK := tt.Find(scheme)
			routeBounds, routeOK := tt.Find(route)
			if !titleOK || !routeOK {
				t.Fatalf("missing scheme/route for %q: %v", raw, tt.Texts())
			}
			for _, bounds := range []ui.Rect{titleBounds, routeBounds} {
				if bounds.X < nativeCardPadding || bounds.X+bounds.W > nativeCardSize-nativeCardPadding+.5 || bounds.Y < 50 || bounds.Y+bounds.H > 146.5 {
					t.Fatalf("App link content escaped shared body: %+v", bounds)
				}
			}
			if titleBounds.Y+titleBounds.H > routeBounds.Y || routeBounds.H > 2*nativeTextBody*1.25+1 {
				t.Fatalf("scheme/route hierarchy overlaps or exceeds two route lines: title=%+v route=%+v", titleBounds, routeBounds)
			}
			if tt.HasText("Unrelated website title") || tt.HasText(raw) || tt.HasText("Website icon") {
				t.Fatal("App link body exposed unrelated metadata, full query or website art")
			}
			if !reflect.DeepEqual(before, s.store.Records()) || s.linkPreview != nil || s.viewState.LinkIcons != nil {
				t.Fatal("App link layout fetched resources or changed saved records")
			}
		}
	}
}

func TestNativeAppLinkSleeveCentersFooterTextAndPin(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, "aweme://video/7540921836?private=hidden")
			s.language, s.nativeState().Dark, item.Pinned = language, dark, true
			if item.Type != clipmodel.TypeAppLink {
				t.Fatalf("app card still uses %s", item.Type)
			}
			before := s.store.Records()
			p, value := panelColors(dark), clipmodel.AnalyzeItem(item)
			background, foreground, metadata := nativeCardPalette(item.Type, value, p)
			tt := ui.NewTester(func(c *ui.Context) {
				setNativeTheme(c, p, dark, language)
				card := ui.Box(c).Fill().Radius(nativeCardRadius).Clip().Background(background)
				card.Children(func() {
					s.nativeCardContentWithAssets(c, card, item, value, foreground, metadata, p, nativeCardAssets{})
				})
			}, nativeCardSize, nativeCardSize)
			footer, hasFooter := tt.Find(s.tr("App scheme"))
			pin, hasPin := tt.Find(s.tr("Unpin"))
			if !hasFooter || !hasPin {
				t.Fatalf("missing sleeve controls for %s: %v", language, tt.Texts())
			}
			// Measure real layout, including localized text and the 22pt pin
			// target, against the visible straight pocket mouth and card edge.
			center := float64(nativeAppLinkSleeveTop+nativeCardSize) / 2
			for _, bounds := range []ui.Rect{footer, pin} {
				if math.Abs(float64(bounds.Y+bounds.H/2)-center) > .5 || bounds.Y < nativeAppLinkSleeveTop || bounds.Y+bounds.H > nativeCardSize {
					t.Fatalf("sleeve control is not vertically centered for %s dark=%t: %+v, center %.1f", language, dark, bounds, center)
				}
			}
			if footer.X != nativeCardPadding || pin.X+pin.W != nativeCardSize-nativeCardPadding {
				t.Fatalf("sleeve controls lost the shared horizontal anchors: footer=%+v pin=%+v", footer, pin)
			}
			for _, text := range []string{"aweme://", "video/7540921836"} {
				bounds, ok := tt.Find(text)
				if !ok || bounds.Y+bounds.H > nativeAppLinkSleeveTop-nativeInlineGap {
					t.Fatalf("pass content overlaps pocket: %q %+v", text, bounds)
				}
			}
			if !reflect.DeepEqual(before, s.store.Records()) || s.linkPreview != nil || s.nativeState().LinkIcons != nil {
				t.Fatal("drawing the pass changed saved content or started a website view")
			}
		}
	}
}
