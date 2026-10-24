package nativeui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestNativeAppLinkDefaultQRIsLazyAndReturnsAfterReopening(t *testing.T) {
	raw := "sslocal://route?url=https%3A%2F%2Fexample.com%2Farticle#saved"
	s, item := semanticPreviewFixture(t, raw)
	s.preview = false
	state := s.semanticState(item)
	s.prepareNativeLinkQR(item)
	if state.LinkModeInitialized || state.QRCode != nil || state.QRTarget != "" {
		t.Fatal("selecting an App link encoded a QR before its preview opened")
	}
	writer := &nativeVariantWriter{}
	nativeVariantCopyService(s, writer)
	s.preview = true
	width, height := nativeItemPreviewSize(item)
	if width != 520 || height != 500 {
		t.Fatal("App preview did not use the compact native content width")
	}
	tt := ui.NewTester(s.nativePopoverView, width, height)
	if state.QRCode == nil || state.QRTarget != raw || !tt.HasText("Scan with your phone") {
		t.Fatal("opening App preview did not encode the saved destination")
	}
	cached := state.QRCode
	if err := tt.Click("Show link"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(raw) || state.LinkQR {
		t.Fatal("source link did not remain readable")
	}
	if err := tt.Click("Show QR code"); err != nil {
		t.Fatal(err)
	}
	if state.QRCode != cached {
		t.Fatal("display toggle unnecessarily regenerated the same saved QR")
	}
	tt.Key(0, ui.KeyEscape)
	if s.preview || s.semanticPreview.QRCode != nil {
		t.Fatal("closing App preview retained its QR")
	}
	s.preview = true
	tt.Frame()
	if !s.semanticState(item).LinkQR || !tt.HasText("Scan with your phone") || writer.calls != 0 {
		t.Fatal("reopening failed to restore QR or changed the clipboard")
	}
}

func TestNativeAppLinkQRAndSourceFitSharedChrome(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, "aweme://video/123?refer=web")
			s.language, s.viewState.Dark = language, dark
			if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"editor.app"}); err != nil {
				t.Fatal(err)
			}
			s.filter()
			width, height := nativeItemPreviewSize(item)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			qr, ok := tt.Find(s.tr("QR code"))
			if !ok || qr.W != qr.H || qr.X < nativePreviewEdgeInset || qr.Y < 50 || qr.X+qr.W > float32(width-nativePreviewEdgeInset) || qr.Y+qr.H > float32(height-84) {
				t.Fatalf("App QR does not fit the shared media area: %+v", qr)
			}
			for _, label := range []string{"Show QR code", "Show link", "Open in app", "Pin", "Delete item · ⌘⌫", "Close preview"} {
				r, ok := tt.Find(s.tr(label))
				top, height := float32(nativePreviewTopInset), float32(nativeControlSize)
				if label == "Show QR code" || label == "Show link" {
					top += 2
					height -= 4
				}
				if !ok || r.Y != top || r.H != height || r.X < 22 || r.X+r.W > float32(width-22) {
					t.Fatalf("App action %q escapes shared toolbar: %+v", label, r)
				}
			}
			copy, ok := tt.Find(s.tr("Copy"))
			if !ok || copy.W != nativeCompactControl || copy.H != nativeCompactControl || copy.Y < float32(height-60) {
				t.Fatalf("App metadata Copy left the fixed footer: %+v", copy)
			}
			if err := tt.Click(s.tr("Show link")); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(item.Text) || !tt.Focused(s.tr("Content preview")) {
				t.Fatal("App source lost text or keyboard focus")
			}
			if sourceCopy, ok := tt.Find(s.tr("Copy")); !ok || sourceCopy != copy {
				t.Fatal("switching QR/source moved the fixed metadata row")
			}
		}
	}
}

func TestNativeAppLinkDefaultQRFailureKeepsSourceAvailable(t *testing.T) {
	raw := "aweme://video/" + strings.Repeat("a", 1900)
	s, item := semanticPreviewFixture(t, raw)
	width, height := nativeItemPreviewSize(item)
	tt := ui.NewTester(s.nativePopoverView, width, height)
	if !tt.HasText("Link is too long for a QR code") {
		t.Fatal("oversized default QR did not explain its failure")
	}
	if err := tt.Click("Show link"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(raw) {
		t.Fatal("oversized App link lost its exact source preview")
	}
}

// These exercise the native controls and real copy service entirely in memory;
// they never open a window, launch another app, or access the system clipboard.
func TestNativeLinkQRModeUsesSavedDestinationAndCopiesExactOriginal(t *testing.T) {
	for _, tc := range []struct{ raw, target string }{
		{"  example.com/a?q=one#part \n", "https://example.com/a?q=one#part"},
		{"aweme://video/123?refer=web", "aweme://video/123?refer=web"},
		{"sslocal://route?url=https%3A%2F%2Fexample.com", "sslocal://route?url=https%3A%2F%2Fexample.com"},
		{"[Example](https://example.com/a_(b)#part)", "https://example.com/a_(b)#part"},
		{"https://example.com/original", "https://example.com/original"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			s, item := semanticPreviewFixture(t, tc.raw)
			if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"editor.app"}); err != nil {
				t.Fatal(err)
			}
			s.filter()
			before := s.store.Records()
			writer := &nativeVariantWriter{}
			nativeVariantCopyService(s, writer)
			pageSession := &nativeLinkPreviewState{itemID: item.ID, url: tc.target, current: "https://redirect.example/another", committed: true}
			s.linkPreview = pageSession
			width, height := nativeItemPreviewSize(item)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			if link := clipmodel.ParseLink(tc.raw); link == nil || !link.App {
				if err := tt.Click("Show QR code"); err != nil {
					t.Fatal(err)
				}
			} else {
				// A real preview synchronizer also disposes an earlier web page.
				s.syncNativeLinkPreview(item)
			}
			state := s.semanticState(item)
			if !state.LinkQR || state.QRTarget != tc.target || state.QRCode == nil || state.QRCode.failure != "" {
				t.Fatalf("QR did not use the original saved target: %+v", state)
			}
			if item.Type == clipmodel.TypeAppLink && s.linkPreview != nil || item.Type == clipmodel.TypeLink && s.linkPreview != pageSession {
				t.Fatal("QR did not preserve only the web link's existing page session")
			}
			if state.CopyFormat != "" || writer.calls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("QR mode changed the copy format, clipboard, or saved record")
			}
			if !tt.HasText("Scan with your phone") || !tt.Focused("Content preview") {
				t.Fatal("QR presentation lost its readable caption or keyboard focus")
			}
			tt.Key(0, ui.KeyEnter)
			copied, err := writer.data.Read(clipmodel.TextFormat)
			if err != nil || string(copied) != tc.raw || writer.calls != 1 || !reflect.DeepEqual(writer.scope, []string{"editor.app"}) {
				t.Fatalf("QR mode changed exact original copy: %q, calls=%d, scope=%v, err=%v", copied, writer.calls, writer.scope, err)
			}
			if !reflect.DeepEqual(before, s.store.Records()) || s.preview || s.semanticPreview.LinkQR || s.semanticPreview.QRCode != nil || s.linkPreview != nil {
				t.Fatal("copy changed history or retained QR session state")
			}
		})
	}
}

func TestNativeLinkQRRejectsUIValueAndResetsOnCloseAndNavigation(t *testing.T) {
	s, _ := semanticPreviewFixture(t, "neighboring text")
	item := nativeCaptureText(t, s, "example.com/saved", "Fixture editor")
	s.filter()
	s.selectNative(0, true)
	writer := &nativeVariantWriter{}
	nativeVariantCopyService(s, writer)
	forged := item
	forged.Text = "aweme://untrusted-route"
	forged.AllowedApps = nil
	s.toggleNativeLinkQR(forged)
	if state := s.semanticState(item); !state.LinkQR || state.QRTarget != "https://example.com/saved" {
		t.Fatal("QR trusted UI-provided content")
	}
	width, height := nativeContentPreviewSize(item.Type)
	tt := ui.NewTester(s.nativePopoverView, width, height)
	tt.Key(0, ui.KeyEscape)
	if s.preview || s.semanticPreview.LinkQR || s.semanticPreview.QRCode != nil || s.semanticPreview.QRTarget != "" {
		t.Fatal("closing preview retained QR presentation state")
	}
	s.preview = true
	tt.Frame()
	if err := tt.Click("Show QR code"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyLeft)
	if selected, _ := s.selected(); selected.ID != item.ID || s.semanticPreview.LinkQR || s.semanticPreview.QRCode != nil || s.semanticPreview.QRTarget != "" {
		t.Fatal("returning to a record retained QR presentation state")
	}
	if writer.calls != 0 {
		t.Fatal("QR toggle, close or navigation wrote the clipboard")
	}
}

func TestNativeAppLinkPreviewHasNoWebPageOrNavigationControls(t *testing.T) {
	for _, raw := range []string{"aweme://video/123", "sslocal://route?q=one"} {
		s, item := semanticPreviewFixture(t, raw)
		s.linkPreview = &nativeLinkPreviewState{itemID: "old-link", url: "https://example.com", current: "https://example.com", committed: true}
		s.syncNativeLinkPreview(item)
		if s.linkPreview != nil {
			t.Fatal("App link retained a webpage session")
		}
		width, height := nativeItemPreviewSize(item)
		tt := ui.NewTester(s.nativePopoverView, width, height)
		if !tt.HasText("Scan with your phone") || !s.semanticState(item).LinkQR {
			t.Fatalf("App link did not open its QR preview: %v", tt.Texts())
		}
		for _, label := range []string{"Open in app", "Show link", "QR code"} {
			if _, ok := tt.Find(label); !ok {
				t.Fatalf("missing explicit app action %q", label)
			}
		}
		for _, label := range []string{"Web page", "Loading page…", "Reload page", "Stop loading", "Back", "Open in browser"} {
			if _, ok := tt.Find(label); ok {
				t.Fatalf("App link exposed webpage control %q", label)
			}
		}
		if err := tt.Click("Show link"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText(raw) || s.semanticState(item).LinkQR {
			t.Fatal("original App link could not be read after the default QR")
		}
	}
}

func TestNativeBareAndWrappedLinkPresentationUsesDerivedDestination(t *testing.T) {
	for _, tc := range []struct{ raw, host, title string }{
		{"example.com/a%20b?q=one", "example.com", "a b"},
		{"www.example.com", "www.example.com", "www.example.com"},
		{"[Read this](https://example.com/page)", "example.com", "Read this"},
		{"aweme://video/123?q=one", "aweme://", "aweme://video/123"},
	} {
		item := clipmodel.Item{Type: clipmodel.TypeLink, Text: tc.raw}
		if got := nativeLinkCardHost(item); got != tc.host {
			t.Errorf("card host for %q = %q, want %q", tc.raw, got, tc.host)
		}
		if got := nativeLinkCardTitle(item, ""); got != tc.title {
			t.Errorf("card title for %q = %q, want %q", tc.raw, got, tc.title)
		}
	}
}

func TestNativeLinkOpenDestinationUsesSavedRecordAndNormalizedPageSession(t *testing.T) {
	for _, tc := range []struct {
		raw, target string
		app         bool
	}{
		{"example.com/a?q=one#part", "https://example.com/a?q=one#part", false},
		{"aweme://video/123", "aweme://video/123", true},
		{"[Read this](https://example.com/page)", "https://example.com/page", false},
	} {
		s, item := semanticPreviewFixture(t, tc.raw)
		got, app := s.nativeLinkOpenDestination(item.ID)
		if got != tc.target || app != tc.app {
			t.Fatalf("open destination for %q = %q, %v", tc.raw, got, app)
		}
		s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, url: "https://unrelated.example/", current: "https://unrelated.example/redirect"}
		if got, app := s.nativeLinkOpenDestination(item.ID); got != tc.target || app != tc.app {
			t.Fatal("open destination trusted an unrelated page session")
		}
		if got, _ := s.nativeLinkOpenDestination("missing-record"); got != "" {
			t.Fatal("open destination accepted a missing record")
		}
	}
	s, item := semanticPreviewFixture(t, "HTTP://EXAMPLE.com/original")
	original, valid := nativeLinkURL(item.Text)
	if !valid {
		t.Fatal("HTTP fixture invalid")
	}
	s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, url: original, current: "https://example.com/redirected", committed: true}
	if got, app := s.nativeLinkOpenDestination(item.ID); got != "https://example.com/redirected" || app {
		t.Fatal("uppercase original scheme lost the current page destination")
	}
	s.semanticState(item).LinkQR = true
	if got, app := s.nativeLinkOpenDestination(item.ID); got != original || app {
		t.Fatal("QR presentation opened a different page than its saved destination")
	}
	s.semanticState(item).LinkQR = false
	s.linkPreview.current = "aweme://untrusted-navigation"
	if got, _ := s.nativeLinkOpenDestination(item.ID); got != "" {
		t.Fatal("webpage navigation became an external app action")
	}
}
