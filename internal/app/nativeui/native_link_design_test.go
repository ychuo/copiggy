package nativeui

import (
	"context"
	"image"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestLinkCardTitleUsesOnlySavedMetadataOrURL(t *testing.T) {
	for _, tc := range []struct{ raw, title, want string }{
		{"https://example.com/notes/slow%20morning?secret=unchanged#part", "Notes on a slower morning", "Notes on a slower morning"},
		{"https://example.com/notes/slow%20morning?secret=unchanged#part", "", "notes/slow morning"},
		{"https://example.com/", "", "example.com"},
		{"https://example.com/%E6%97%A5%E8%A8%98", "", "日記"},
	} {
		item := clipmodel.Item{Type: clipmodel.TypeLink, Text: tc.raw}
		if got := nativeLinkCardTitle(item, tc.title); got != tc.want {
			t.Fatalf("title=%q, want %q", got, tc.want)
		}
		if nativeLinkCardHost(item) != "example.com" || item.Text != tc.raw {
			t.Fatal("link title extraction changed the record or exposed query/fragment as metadata")
		}
	}
}

func TestLinkCardTitleFitsUnderCoverAndLeavesSharedFooter(t *testing.T) {
	icon := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 32, 32)))
	for _, title := range []string{
		"Notes on a slower morning",
		"An exceptionally long title " + strings.Repeat("with more detail ", 12),
		"设计与开发的记录，以及一个更从容的早晨",
		"いつもよりゆっくり過ごす朝の記録",
	} {
		for _, covered := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, "https://example.com/notes")
			before := s.store.Records()
			tt := ui.NewTester(func(c *ui.Context) {
				setNativeTheme(c, nativeLinkColors(), false)
				ui.Box(c).Fill().Children(func() {
					ui.Box(c).Absolute().Top(50).Left(nativeCardPadding).Right(nativeCardPadding).Bottom(42).Children(func() {
						s.nativeLinkCardBodyWithIcon(c, item, title, covered, icon)
					})
				})
			}, nativeCardSize, nativeCardSize)
			r, ok := tt.Find(nativeLinkCardTitle(item, title))
			if !ok || r.X < nativeCardPadding || r.X+r.W > nativeCardSize-nativeCardPadding || r.Y+r.H > 146.5 {
				t.Fatalf("title overlaps shared footer or escapes card, cover=%t title=%q: %+v texts=%v", covered, title, r, tt.Texts())
			}
			if covered && r.Y < nativeLinkCoverHeight+8 {
				t.Fatalf("title overlaps cover: %+v", r)
			}
			if !reflect.DeepEqual(before, s.store.Records()) || s.linkPreview != nil || s.viewState.LinkIcons != nil {
				t.Fatal("link layout fetched resources or modified captured content")
			}
		}
	}
}

func TestLinkPreviewKeepsCurrentHostSeparateFromSavedCopy(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, "https://original.example.com/document?query=kept#section")
			s.language, s.viewState.Dark = language, dark
			host := strings.Repeat("a", 60) + "." + strings.Repeat("b", 60) + ".example.com:8443"
			s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, url: item.Text, current: "https://" + host + "/reading", target: "https://" + host + "/reading", failure: "Page unavailable", canBack: true}
			before := s.store.Records()
			tt := ui.NewTester(s.nativePopoverView, 720, 500)
			var previous ui.Rect
			for i, label := range []string{"Back", "Reload page", "Open in browser", "Pin", "Delete item · ⌘⌫", "Close preview"} {
				r, ok := tt.Find(s.tr(label))
				if !ok || r.X < 22 || r.X+r.W > 699 || r.Y < 12 || r.Y+r.H > 42 {
					t.Fatalf("link toolbar lost a fixed action in %s dark=%t: %s %+v", language, dark, label, r)
				}
				if i > 0 && r.X < previous.X+previous.W {
					t.Fatalf("long hostname displaced toolbar action %s: %+v", label, r)
				}
				previous = r
			}
			destination, ok := tt.Find(host)
			open, _ := tt.Find(s.tr("Open in browser"))
			if !ok || destination.X+destination.W > open.X-8 {
				t.Fatalf("hostname overlaps right-side actions: host=%+v open=%+v", destination, open)
			}
			original, ok := tt.Find("original.example.com")
			copy, copyFound := tt.Find(s.tr("Copy"))
			if !ok || !copyFound || original.Y < 450 || original.X < copy.X+copy.W || original.Y+original.H > 492 {
				t.Fatalf("copy footer no longer identifies the original record: original=%+v copy=%+v", original, copy)
			}
			if err := tt.Click(s.tr("Retry")); err != nil {
				t.Fatal(err)
			}
			if !tt.Focused(s.tr("Content preview")) || s.linkPreview.action != "retry" || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("preview retry changed clipboard history or lost keyboard focus")
			}
		}
	}
}

func TestLinkPreviewIconFollowsExactDisplayedOrigin(t *testing.T) {
	s, item := semanticPreviewFixture(t, "https://example.com/notes")
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://example.com/new", true},
		{"https://EXAMPLE.COM/new", true},
		{"http://example.com/new", false},
		{"https://example.com:8443/new", false},
		{"https://another.example.com/new", false},
	} {
		s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, url: item.Text, current: tc.url, target: tc.url}
		if got := s.nativeLinkShowsOriginalOrigin(item); got != tc.want {
			t.Fatalf("cached original favicon used for %s: %t", tc.url, got)
		}
	}
}

func TestLinkPreviewLoadingSurfaceRetainsRoundedCorners(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, "https://example.com/notes")
		tt := ui.NewTester(func(c *ui.Context) {
			p := panelColors(dark)
			setNativeTheme(c, p, dark)
			s.nativeLinkPreview(c, item, p)
		}, 720, 500)
		pixels := tt.Image()
		glass := pixels.RGBAAt(7, 51)
		for _, point := range []image.Point{{9, 51}, {710, 51}} {
			if got := pixels.RGBAAt(point.X, point.Y); got != glass {
				t.Fatalf("link content paints over rounded corner at %v: %v, want %v", point, got, glass)
			}
		}
		if pixels.RGBAAt(360, 54).A < 250 {
			t.Fatal("link loading surface has no legible paper background")
		}
	}
}

func TestLinkQRUsesCompactChromeAndReturnsToRetainedPage(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, "example.com/original?q=one#saved")
			s.language, s.viewState.Dark = language, dark
			if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"editor.app"}); err != nil {
				t.Fatal(err)
			}
			s.filter()
			writer := &nativeVariantWriter{}
			nativeVariantCopyService(s, writer)
			page := &nativeLinkPreviewState{itemID: item.ID, url: "https://" + item.Text, current: "https://reading.example/next", committed: true, canBack: true}
			s.linkPreview = page
			width, height := s.nativeCurrentPreviewSize(item)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			if err := tt.Click(s.tr("Show QR code")); err != nil {
				t.Fatal(err)
			}
			width, height = s.nativeCurrentPreviewSize(item)
			if width != 520 || height != 500 {
				t.Fatalf("QR did not use compact preview dimensions: %dx%d", width, height)
			}
			tt.SetSize(width, height)
			var previous ui.Rect
			for i, label := range []string{"Show link", "Show QR code", "Open in browser", "Pin", "Delete item · ⌘⌫", "Close preview"} {
				r, ok := tt.Find(s.tr(label))
				if !ok || r.X < 22 || r.X+r.W > 498 || r.Y < 12 || r.Y+r.H > 42 || i > 0 && r.X < previous.X+previous.W {
					t.Fatalf("compact QR toolbar overlaps in %s dark=%t: %s %+v", language, dark, label, r)
				}
				previous = r
			}
			for _, label := range []string{"Back", "Reload page", "Website icon", "reading.example"} {
				if _, found := tt.Find(s.tr(label)); found {
					t.Fatalf("QR kept redundant page chrome: %s", label)
				}
			}
			original, ok := tt.Find(item.Text)
			copy, hasCopy := tt.Find(s.tr("Copy"))
			if !ok || !hasCopy || original.Y < copy.Y || original.Y+original.H > copy.Y+copy.H || original.X < copy.X+copy.W || original.X+original.W > 498 {
				t.Fatalf("QR footer lost the exact original URL: source=%+v copy=%+v", original, copy)
			}
			qr, hasQR := tt.Find(s.tr("QR code"))
			caption, hasCaption := tt.Find(s.tr("Scan with your phone"))
			if !hasQR || !hasCaption || qr.W != qr.H || qr.Y+qr.H > caption.Y || caption.Y+caption.H > copy.Y-nativePreviewSectionGap {
				t.Fatalf("QR or caption overlaps fixed protection footer: qr=%+v caption=%+v copy=%+v", qr, caption, copy)
			}
			if err := tt.Click(s.tr("Show link")); err != nil {
				t.Fatal(err)
			}
			width, height = s.nativeCurrentPreviewSize(item)
			if width != 720 || height != 500 {
				t.Fatalf("page did not regain its reading width: %dx%d", width, height)
			}
			tt.SetSize(width, height)
			if !tt.HasText("reading.example") || s.linkPreview != page || page.action != "" || writer.calls != 0 {
				t.Fatal("QR round trip lost the current page, requested reload, or wrote the clipboard")
			}
		}
	}
}
