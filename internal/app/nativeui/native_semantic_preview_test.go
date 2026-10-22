package nativeui

import (
	"context"
	"encoding/pem"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"copiggy/internal/macos/guard"
	"github.com/egoist/mygo/ui"
)

func semanticPreviewFixture(t *testing.T, text string) (*application, clipmodel.Item) {
	t.Helper()
	s := nativeViewFixture(t)
	s.language = "en"
	s.store = clipboard.NewHistory(nil, 30)
	item := nativeCaptureText(t, s, text, "Fixture editor")
	s.filter()
	s.preview = true
	return s, item
}

func semanticPrivateKeyFixture() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte(strings.Repeat("privatefixture", 6))}))
}

func TestSemanticPreviewToolbarFitsLanguagesAndThemes(t *testing.T) {
	cases := []struct {
		text    string
		kind    clipmodel.ContentType
		actions []string
	}{
		{"hello@example.com", clipmodel.TypeEmail, []string{"Mail"}},
		{"+1 (202) 555-0148", clipmodel.TypePhone, []string{"Call", "Dialing number"}},
		{"31.2304, 121.4737", clipmodel.TypeCoordinates, []string{"Maps"}},
		{"2026-10-22T15:30:00+08:00", clipmodel.TypeDate, []string{"Calendar"}},
		{"1792645800", clipmodel.TypeTimestamp, []string{"Seconds", "Milliseconds", "Local time"}},
		{"#D78379", clipmodel.TypeColor, []string{"HEX", "RGB", "HSL"}},
		{"/Users/alex/Documents/Brief.pdf", clipmodel.TypeFilePath, []string{"Finder"}},
		{"Verification code: 382941", clipmodel.TypeOTP, nil},
		{"ghp_" + strings.Repeat("z", 36), clipmodel.TypeToken, []string{"Reveal"}},
		{semanticPrivateKeyFixture(), clipmodel.TypePrivateKey, []string{"Reveal"}},
	}
	for _, tc := range cases {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			for _, dark := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/dark=%t", tc.kind, language, dark), func(t *testing.T) {
					s, item := semanticPreviewFixture(t, tc.text)
					s.language, s.viewState.Dark = language, dark
					if item.Type != tc.kind {
						t.Fatalf("fixture type is %s, want %s", item.Type, tc.kind)
					}
					width, height := nativeContentPreviewSize(item.Type)
					tt := ui.NewTester(s.nativePopoverView, width, height)
					if !tt.Focused(s.tr("Content preview")) {
						t.Fatal("preview did not acquire its keyboard target")
					}
					actions := append(append([]string{}, tc.actions...), "Pin", "Delete item · ⌘⌫", "Close preview")
					var previous ui.Rect
					for index, source := range actions {
						r, ok := tt.Find(s.tr(source))
						if !ok || r.X < 20 || r.X+r.W > 501 || r.Y < 0 || r.Y+r.H > 48 {
							t.Fatalf("action %q escapes native toolbar: %+v", source, r)
						}
						if index > 0 && r.X < previous.X+previous.W-1 {
							t.Fatalf("toolbar actions overlap: %q=%+v previous=%+v", source, r, previous)
						}
						previous = r
					}
					assertNativeCurrent(t, s, item.ID)
				})
			}
		}
	}
}

type semanticPreviewGuard struct {
	policy guard.Policy
}

func (g *semanticPreviewGuard) Enabled() bool                { return true }
func (g *semanticPreviewGuard) CurrentPolicy() guard.Policy  { return g.policy }
func (g *semanticPreviewGuard) ChangeCount() int64           { return g.policy.Version }
func (g *semanticPreviewGuard) SetPanelVisible(visible bool) {}

func TestSecretPreviewRevealIsBoundToSessionAndNeverWrites(t *testing.T) {
	for _, raw := range []string{"ghp_" + strings.Repeat("z", 36), semanticPrivateKeyFixture()} {
		t.Run(string(clipmodel.Analyze(raw).Type), func(t *testing.T) {
			s, _ := semanticPreviewFixture(t, "ordinary neighboring item")
			policy := guard.Policy{Version: 74, AllowedApps: []string{"com.example.Editor"}}
			s.native = &semanticPreviewGuard{policy: policy}
			s.guardPreference = true
			item := nativeCapture(t, s, clipmodel.TextData(raw), clipmodel.Source{Name: "Fixture editor", BundleID: "com.example.Editor"}, "com.example.Editor")
			s.filter()
			s.selectNative(0, true)
			before := s.store.Records()
			copyCalls, guardCalls := 0, 0
			s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) {
				copyCalls++
				return clipmodel.Item{}, nil
			}
			s.config.SetGuard = func(bool) error { guardCalls++; return nil }
			secret := clipmodel.Analyze(raw).Secret
			width, height := nativeContentPreviewSize(item.Type)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			assertHidden := func() {
				t.Helper()
				_, exposed := tt.Find(secret.Value)
				if s.semanticPreview.Revealed || exposed || tt.HasText(secret.Value) || tt.HasText(raw) {
					t.Fatal("secret is visible without an explicit reveal in this session")
				}
				if _, ok := tt.Find("Reveal"); !ok {
					t.Fatal("reveal control is missing")
				}
			}
			assertHidden()
			if err := tt.Click("Reveal"); err != nil {
				t.Fatal(err)
			}
			revealed := tt.HasText(secret.Value)
			if item.Type == clipmodel.TypeOTP {
				_, revealed = tt.Find(secret.Value)
			}
			if !s.semanticPreview.Revealed || !revealed || !tt.Focused("Content preview") {
				t.Fatal("explicit reveal did not display the value while retaining keyboard focus")
			}
			tt.Key(0, ui.KeyRight)
			if s.chosen != 1 || !s.preview {
				t.Fatal("revealed preview lost history navigation")
			}
			tt.Key(0, ui.KeyLeft)
			assertHidden()
			if err := tt.Click("Reveal"); err != nil {
				t.Fatal(err)
			}
			tt.Key(0, ui.KeyEscape)
			if s.preview || s.semanticPreview.Revealed {
				t.Fatal("closing retained secret reveal state")
			}
			s.preview = true
			tt.Frame()
			assertHidden()
			assertNativeCurrent(t, s, item.ID)
			if copyCalls != 0 || guardCalls != 0 || !reflect.DeepEqual(s.store.Records(), before) || !reflect.DeepEqual(s.native.CurrentPolicy(), policy) || !s.guardPreference {
				t.Fatal("preview display changed clipboard/history or revision-bound protection")
			}
		})
	}
}

func TestSemanticDisplayChoicesPreserveOriginalRepresentation(t *testing.T) {
	for _, tc := range []struct {
		text    string
		buttons []string
	}{
		{"rgba(215, 131, 121, 0.5)", []string{"RGB", "HSL", "HEX"}},
		{"1792645800123", []string{"Seconds", "Milliseconds", "Local time", "UTC"}},
		{"+1 (202) 555-0148", []string{"Dialing number", "Original number"}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			s, item := semanticPreviewFixture(t, tc.text)
			before := s.store.Records()
			calls := 0
			s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) { calls++; return clipmodel.Item{}, nil }
			tt := ui.NewTester(s.nativePopoverView, 520, 360)
			for _, label := range tc.buttons {
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				if !tt.Focused("Content preview") {
					t.Fatal("display option stranded keyboard focus")
				}
			}
			if value := clipmodel.Analyze(tc.text).Color; value != nil && !tt.HasText(value.HEX) {
				t.Fatal("color display did not return to HEX")
			}
			if value := clipmodel.Analyze(tc.text).Timestamp; value != nil {
				if !tt.HasText("1792645800123") || !s.semanticPreview.Milliseconds || s.semanticPreview.UTC {
					t.Fatal("timestamp units or local/UTC toggle did not update presentation")
				}
			}
			if calls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("display formatting changed saved representations")
			}
			assertNativeCurrent(t, s, item.ID)
		})
	}
}

func TestLongSemanticContentDoesNotDisplacePreviewChrome(t *testing.T) {
	email := strings.Repeat("a", 64) + "@" + strings.Repeat("b", 60) + "." + strings.Repeat("c", 60) + ".example.com"
	file := "/Users/alex/" + strings.Repeat("long-directory/", 150) + strings.Repeat("document", 15) + ".pdf"
	for _, text := range []string{email, file} {
		t.Run(string(clipmodel.Analyze(text).Type), func(t *testing.T) {
			s, item := semanticPreviewFixture(t, text)
			width, height := nativeContentPreviewSize(item.Type)
			tt := ui.NewTester(s.nativePopoverView, width, height)
			footerLabel := item.Text
			if item.Type == clipmodel.TypeFilePath {
				footerLabel = s.tr("File path")
			}
			assertChrome := func() {
				t.Helper()
				close, ok := tt.Find("Close preview")
				if !ok || close.Y < 0 || close.Y+close.H > 48 {
					t.Fatalf("long content displaced toolbar: %+v", close)
				}
				footer, ok := tt.Find(footerLabel)
				if !ok || footer.Y < float32(height-60) || footer.Y+footer.H > float32(height-15) {
					t.Fatalf("long content displaced metadata: %+v", footer)
				}
			}
			assertChrome()
			tt.Scroll(260, 180, 0, 500)
			assertChrome()
			assertNativeCurrent(t, s, item.ID)
		})
	}
}

func TestMalformedSensitivePresentationNeverFallsBackToRawText(t *testing.T) {
	for _, kind := range []clipmodel.ContentType{clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey} {
		s, _ := semanticPreviewFixture(t, "unrecognized-private-value")
		// Simulate a legacy/stale type annotation. Rendering must fail closed;
		// real actions still consult the authoritative record, never this view.
		s.results[0].Type = kind
		tt := ui.NewTester(s.nativePopoverView, 520, 360)
		if tt.HasText("unrecognized-private-value") || !tt.HasText("Hidden content") {
			t.Fatalf("%s rendered an unparseable secret as plain text", kind)
		}
	}
}
