package nativeui

import (
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestNativeLinkURLPolicy(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
		allowed   bool
	}{
		{"https://example.com/docs?q=two%20words#section", "https://example.com/docs?q=two%20words#section", true},
		{"  HTTP://example.com:8080/page  ", "http://example.com:8080/page", true},
		{"http://127.0.0.1:8000/page", "http://127.0.0.1:8000/page", true},
		{"https://example.com", "https://example.com", true},
		{"file:///etc/passwd", "", false},
		{"javascript:alert(1)", "", false},
		{"data:text/html,hello", "", false},
		{"about:blank", "", false},
		{"mygo://localhost", "", false},
		{"mailto:test@example.com", "", false},
		{"/docs", "", false},
		{"https:///docs", "", false},
		{"https://user:password@example.com", "", false},
		{"https://example.com/\npath", "", false},
	} {
		got, allowed := nativeLinkURL(tc.raw)
		if got != tc.want || allowed != tc.allowed {
			t.Errorf("nativeLinkURL(%q) = %q, %v; want %q, %v", tc.raw, got, allowed, tc.want, tc.allowed)
		}
	}
}

func nativeLinkFixture(t *testing.T) (*application, clipmodel.Item) {
	t.Helper()
	s := nativeViewFixture(t)
	s.language = "en"
	for i, item := range s.results {
		if item.Type == clipmodel.TypeLink {
			s.selectNative(i, true)
			return s, item
		}
	}
	panic("fixture has no link")
}

func TestNativeLinkPreviewWaitsForVisiblePopover(t *testing.T) {
	s, item := nativeLinkFixture(t)
	current, _ := s.store.Current()
	// Building either native view never creates WebKit or sends a request. The
	// watcher will load only after an actual user-opened NSPopover is visible.
	_ = ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	tt := ui.NewTester(s.nativePopoverView, 720, 500)
	if !tt.HasText("Loading page…") || !tt.HasText("github.com") {
		t.Fatalf("missing native loading state or link location: %v", tt.Texts())
	}
	for _, label := range []string{"Reload page", "Open in browser", "Close preview"} {
		control, found := tt.Find(label)
		if !found || control.Y+control.H > 48 {
			t.Fatalf("missing native top action %q: %+v", label, control)
		}
	}
	s.syncNativeLinkPreview(item) // No real popup exists in a pure UI test.
	if s.linkPreview != nil {
		t.Fatal("link loaded without a visible native popup")
	}
	assertNativeCurrent(t, s, current.ID)
}

func TestNativeLinkErrorRetryUsesFailedDestination(t *testing.T) {
	s, item := nativeLinkFixture(t)
	failed := "https://example.com/next?q=private#section"
	s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, url: item.Text, current: item.Text, target: failed, failure: "Page unavailable"}
	tt := ui.NewTester(s.nativePopoverView, 720, 500)
	if !tt.HasText("Page unavailable") || !tt.HasText("example.com") {
		t.Fatalf("missing error or failed destination: %v", tt.Texts())
	}
	if err := tt.Click("Retry"); err != nil {
		t.Fatal(err)
	}
	if s.linkPreview.action != "retry" || s.linkPreview.retryURL() != failed {
		t.Fatal("retry lost the page the user was navigating to")
	}
	if !tt.Focused("Content preview") {
		t.Fatal("native retry button took preview keyboard focus")
	}
	if err := tt.Click("Close preview"); err != nil {
		t.Fatal(err)
	}
	if s.preview || s.linkPreview != nil {
		t.Fatal("close retained the link page session")
	}
}

func TestNativeLinkToolbarQueuesNavigationWithoutTouchingClipboard(t *testing.T) {
	s, item := nativeLinkFixture(t)
	current, _ := s.store.Current()
	s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, url: item.Text, current: item.Text, target: item.Text, loading: true, canBack: true}
	tt := ui.NewTester(s.nativePopoverView, 720, 500)
	if err := tt.Click("Stop loading"); err != nil {
		t.Fatal(err)
	}
	if s.linkPreview.action != "stop" {
		t.Fatal("loading action did not queue stop")
	}
	s.linkPreview.loading = false
	s.linkPreview.committed = true
	tt.Frame()
	if err := tt.Click("Reload page"); err != nil {
		t.Fatal(err)
	}
	if s.linkPreview.action != "reload" {
		t.Fatal("reload action did not queue reload")
	}
	if err := tt.Click("Back"); err != nil {
		t.Fatal(err)
	}
	if s.linkPreview.action != "back" {
		t.Fatal("back action did not queue back navigation")
	}
	assertNativeCurrent(t, s, current.ID)
}

func TestNativeLinkCleanupInvalidatesCallbacksBeforeUnsubscribe(t *testing.T) {
	s, item := nativeLinkFixture(t)
	called := 0
	s.linkPreview = &nativeLinkPreviewState{itemID: item.ID, off: []func(){func() {
		called++
		if s.linkPreview != nil {
			t.Error("callbacks can still modify the old preview during cleanup")
		}
	}}}
	s.destroyNativeLinkPreview()
	s.destroyNativeLinkPreview()
	if called != 1 || s.linkPreview != nil {
		t.Fatal("cleanup is not idempotent")
	}
}

func TestNativeLinkIconsAreBundled(t *testing.T) {
	for _, name := range []string{"ArrowClockwise", "ArrowSquareOut", "ArrowLeft", "Globe"} {
		if nativeIcons[name] == nil {
			t.Errorf("native toolbar icon %q is missing or cannot be parsed", name)
		}
	}
}
