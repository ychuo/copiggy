package nativeui

import (
	"reflect"
	"testing"
	"time"

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

func TestNativeWebPageSessionSurvivesQRRoundTripWithoutRecreation(t *testing.T) {
	s, item := semanticPreviewFixture(t, "https://example.com/saved?q=one#part")
	writer := &nativeVariantWriter{}
	nativeVariantCopyService(s, writer)
	const host = uintptr(37)
	state, create := s.reconcileNativeLinkPreview(item, host)
	if state == nil || !create {
		t.Fatal("first page presentation did not request its initial view")
	}
	// Model a committed page after an in-page navigation. Native creation is
	// deliberately omitted; the same session owns the real WKWebView in use.
	state.current = "https://example.com/article#reading"
	state.target, state.committed, state.loading, state.canBack = state.current, true, false, true
	unsubscribed := 0
	state.off = []func(){func() { unsubscribed++ }}
	before := *state
	before.off = nil
	tt := ui.NewTester(s.nativePopoverView, 720, 500)
	for range 2 {
		if err := tt.Click("Show QR code"); err != nil {
			t.Fatal(err)
		}
		if retained, create := s.reconcileNativeLinkPreview(item, host); retained != state || create {
			t.Fatal("QR replaced the current page session")
		}
		if !tt.HasText("Scan with your phone") || s.semanticState(item).QRTarget != item.Text {
			t.Fatal("QR lost the saved destination or native presentation")
		}
		if err := tt.Click("Show link"); err != nil {
			t.Fatal(err)
		}
		if retained, create := s.reconcileNativeLinkPreview(item, host); retained != state || create {
			t.Fatal("returning to the page requested another view/load")
		}
		after := *state
		after.off = nil
		if !reflect.DeepEqual(before, after) || state.action != "" || unsubscribed != 0 || writer.calls != 0 {
			t.Fatal("display toggle mutated page navigation, queued a reload, removed policies or wrote the clipboard")
		}
	}
	if err := tt.Click("Close preview"); err != nil {
		t.Fatal(err)
	}
	if s.linkPreview != nil || unsubscribed != 1 {
		t.Fatal("closing retained the hidden/reused page session")
	}
}

func TestNativeLinkQRDoesNotCreateAPageAndInvalidatesForeignSessions(t *testing.T) {
	for _, change := range []string{"no page yet", "different record", "changed saved URL", "different host", "hidden host", "closed preview", "hiding panel", "quitting", "App link", "unsafe destination"} {
		t.Run(change, func(t *testing.T) {
			s, item := semanticPreviewFixture(t, "https://example.com/saved")
			host := uintptr(37)
			state, create := s.reconcileNativeLinkPreview(item, host)
			if state == nil || !create {
				t.Fatal("missing initial session")
			}
			unsubscribed := 0
			state.off = []func(){func() { unsubscribed++ }}
			s.semanticState(item).LinkQR = true
			switch change {
			case "no page yet":
				s.linkPreview = nil
			case "different record":
				state.itemID = "old-record"
			case "changed saved URL":
				state.url = "https://example.com/old"
			case "different host":
				host++
			case "hidden host":
				host = 0
			case "closed preview":
				s.preview = false
			case "hiding panel":
				s.hiding = true
			case "quitting":
				s.quitting = true
			case "App link":
				item.Type, item.Text = clipmodel.TypeAppLink, "aweme://video/123"
			case "unsafe destination":
				item.Text = "https://user:password@example.com/saved"
			}
			if got, create := s.reconcileNativeLinkPreview(item, host); got != nil || create || s.linkPreview != nil {
				t.Fatal("QR kept an unrelated page or created a hidden WebView")
			}
			wantUnsubscribed := 1
			if change == "no page yet" {
				wantUnsubscribed = 0
			}
			if unsubscribed != wantUnsubscribed {
				t.Fatalf("session cleanup calls = %d, want %d", unsubscribed, wantUnsubscribed)
			}
		})
	}
}

func TestNativeHiddenPageWatchdogStopsOnlyExpiredLoadsAndKeepsSession(t *testing.T) {
	for _, committed := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, "https://example.com/saved")
		const host = uintptr(37)
		state, _ := s.reconcileNativeLinkPreview(item, host)
		state.committed = committed
		state.current, state.target, state.canBack = "https://example.com/reading#position", "https://example.com/reading#position", true
		s.semanticState(item).LinkQR = true
		stops := 0
		stop := func() { stops++ }
		if state.stopTimedOutLoad(state.started.Add(25*time.Second), stop) || stops != 0 || !state.loading {
			t.Fatal("watchdog stopped a load before its deadline")
		}
		if !state.stopTimedOutLoad(state.started.Add(26*time.Second), stop) || stops != 1 || state.loading {
			t.Fatal("hidden page did not stop its expired load")
		}
		if state.stopTimedOutLoad(state.started.Add(time.Minute), stop) || stops != 1 {
			t.Fatal("watchdog repeatedly stopped a finished load")
		}
		if committed && state.failure != "" || !committed && state.failure != "Page timed out" {
			t.Fatal("timeout hid committed content or lost an uncommitted failure")
		}
		if state.current != "https://example.com/reading#position" || state.target != state.current || !state.canBack || state.action != "" {
			t.Fatal("watchdog changed navigation or queued a reload")
		}
		if retained, create := s.reconcileNativeLinkPreview(item, host); retained != state || create {
			t.Fatal("hidden timeout replaced the page session")
		}
		s.semanticState(item).LinkQR = false
		if retained, create := s.reconcileNativeLinkPreview(item, host); retained != state || create {
			t.Fatal("returning after timeout recreated or reloaded the page")
		}
	}
}

func TestNativeLinkIconsAreBundled(t *testing.T) {
	for _, name := range []string{"ArrowClockwise", "ArrowSquareOut", "ArrowLeft", "Globe"} {
		if nativeIcons[name] == nil {
			t.Errorf("native toolbar icon %q is missing or cannot be parsed", name)
		}
	}
}
