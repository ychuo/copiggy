package nativeui

import (
	"context"
	"encoding/pem"
	"image"
	"strings"
	"testing"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestCardPinKeepsSamePositionAcrossContentAndMapLoading(t *testing.T) {
	bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, nativeCardSize, nativeCardSize)))
	cases := []struct {
		name, text string
		kind       clipmodel.ContentType
		assets     nativeCardAssets
	}{
		{"text", "Make room for the next idea.", clipmodel.TypeText, nativeCardAssets{}},
		{"json", `{"project":"Copiggy","native":true}`, clipmodel.TypeJSON, nativeCardAssets{}},
		{"email", "hello@example.com", clipmodel.TypeEmail, nativeCardAssets{}},
		{"phone", "+1 (202) 555-0148", clipmodel.TypePhone, nativeCardAssets{}},
		{"color", "#D78379", clipmodel.TypeColor, nativeCardAssets{}},
		{"date", "2026-10-22T15:30:00+08:00", clipmodel.TypeDate, nativeCardAssets{}},
		{"timestamp", "1792645800", clipmodel.TypeTimestamp, nativeCardAssets{}},
		{"file-path", "/Users/alex/Documents/Brief.pdf", clipmodel.TypeFilePath, nativeCardAssets{}},
		{"otp", "Verification code: 382941", clipmodel.TypeOTP, nativeCardAssets{}},
		{"token", "ghp_" + strings.Repeat("z", 36), clipmodel.TypeToken, nativeCardAssets{}},
		{"private-key", semanticPrivateKeyFixture(), clipmodel.TypePrivateKey, nativeCardAssets{}},
		{"map-loading", "31.2304, 121.4737", clipmodel.TypeCoordinates, nativeCardAssets{}},
		{"map-ready", "31.2304, 121.4737", clipmodel.TypeCoordinates, nativeCardAssets{bitmap: bitmap}},
		{"image", "", clipmodel.TypeImage, nativeCardAssets{bitmap: bitmap}},
		{"link", "https://example.com", clipmodel.TypeLink, nativeCardAssets{}},
		{"link-cover", "https://example.com/articles/morning", clipmodel.TypeLink, nativeCardAssets{linkCover: bitmap, linkIcon: bitmap, linkTitle: "Notes on a slower morning"}},
	}
	var reference ui.Rect
	for _, tc := range cases {
		for _, dark := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				s := nativeViewFixture(t)
				s.language, s.viewState.Dark = "en", dark
				item := clipmodel.Item{ID: "fixture", Text: tc.text, Type: tc.kind, Pinned: true}
				value, p := clipmodel.Analyze(tc.text), panelColors(dark)
				background, foreground, metadata := nativeCardPalette(tc.kind, value, p)
				tt := ui.NewTester(func(c *ui.Context) {
					setNativeTheme(c, p, dark)
					card := ui.Box(c).Fill().Radius(nativeCardRadius).Clip().Background(background)
					card.Children(func() { s.nativeCardContentWithAssets(c, card, item, value, foreground, metadata, p, tc.assets) })
				}, nativeCardSize, nativeCardSize)
				pin, found := tt.Find("Unpin")
				if !found {
					t.Fatal("pinned card is missing its control")
				}
				if reference.W == 0 {
					reference = pin
				}
				if pin != reference {
					t.Fatalf("pin moved with content type or map loading: got %+v, shared %+v", pin, reference)
				}
			})
		}
	}
}

func TestSemanticCardsKeepFrameAndFooterGeometry(t *testing.T) {
	cases := []struct {
		text string
		kind clipmodel.ContentType
	}{
		{"hello@example.com", clipmodel.TypeEmail},
		{"+1 (202) 555-0148", clipmodel.TypePhone},
		{"31.2304, 121.4737", clipmodel.TypeCoordinates},
		{"2026-10-22T15:30:00+08:00", clipmodel.TypeDate},
		{"1792645800", clipmodel.TypeTimestamp},
		{"#D78379", clipmodel.TypeColor},
		{"/Users/alex/Documents/Brief.pdf", clipmodel.TypeFilePath},
		{"Verification code: 123456", clipmodel.TypeOTP},
		{"ghp_" + strings.Repeat("a", 36), clipmodel.TypeToken},
		{string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte(strings.Repeat("fixture", 12))})), clipmodel.TypePrivateKey},
	}
	for _, tc := range cases {
		for _, dark := range []bool{false, true} {
			t.Run(string(tc.kind), func(t *testing.T) {
				s := nativeViewFixture(t)
				s.language, s.viewState.Dark = "en", dark
				s.store = clipboard.NewHistory(nil, 30)
				item := nativeCaptureText(t, s, tc.text, "Fixture app")
				if item.Type != tc.kind {
					t.Fatalf("fixture has type %s, want %s", item.Type, tc.kind)
				}
				if err := s.store.SetPinned(context.Background(), item.ID, true); err != nil {
					t.Fatal(err)
				}
				s.filter()
				tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
				card, found := tt.Find(s.nativeCardLabel(item))
				if !found || card.W != nativeCardSize || card.H != nativeCardSize {
					t.Fatalf("card is not square: %+v", card)
				}
				pin, ok := tt.Find("Unpin")
				if !ok || pin.Y+pin.H > card.Y+card.H-12 || pin.X+pin.W > card.X+card.W-12 {
					t.Fatalf("pin escaped the shared footer: pin=%+v card=%+v", pin, card)
				}
				if tc.kind == clipmodel.TypeDate {
					month, ok := tt.Find("OCT")
					if !ok || month.Y+month.H > card.Y+55 {
						t.Fatalf("month escaped compact coral header: month=%+v card=%+v", month, card)
					}
				}
				// Browsing and rendering never change the exact saved original.
				stored, ok := s.store.Record(item.ID)
				if !ok || stored.Item.Text != tc.text {
					t.Fatal("display formatting replaced captured text")
				}
			})
		}
	}
}

func TestSensitiveCardsShowCodeOrPartialCredentialWithoutExposingMiddle(t *testing.T) {
	for _, raw := range []string{
		"Verification code: 382941",
		"ghp_" + strings.Repeat("z", 36),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte(strings.Repeat("privatefixture", 6))})),
	} {
		s := nativeViewFixture(t)
		s.language = "en"
		s.store = clipboard.NewHistory(nil, 30)
		item := nativeCaptureText(t, s, raw, "Fixture app")
		s.filter()
		tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
		secret := clipmodel.Analyze(raw).Secret
		if secret == nil {
			t.Fatal("test fixture was not recognized")
		}
		if item.Type == clipmodel.TypeOTP {
			if _, found := tt.Find(secret.Value); !found {
				t.Fatal("verification card does not expose its readable digits")
			}
			continue
		}
		allText := append(tt.Texts(), s.nativeCardLabel(item))
		for _, visible := range allText {
			if strings.Contains(visible, secret.Value) || strings.Contains(visible, raw) {
				t.Fatalf("sensitive card exposed captured data through text/accessibility: %q", visible)
			}
		}
		if len(item.AllowedApps) != 0 {
			t.Fatal("sensitive styling invented an application restriction")
		}
		if !tt.HasText(nativeSecretPartial(item, clipmodel.Analyze(raw))) {
			t.Fatal("credential card is missing its identifying ends")
		}
	}
}

func TestColorInkFollowsSwatchAndTransparency(t *testing.T) {
	for _, tc := range []struct {
		text     string
		lightInk bool
	}{
		{"#ffffff", false}, {"#000000", true}, {"#D78379", false}, {"#00000000", false},
	} {
		value := clipmodel.Analyze(tc.text).Color
		if value == nil {
			t.Fatal("invalid color fixture")
		}
		ink := nativeColorInk(*value)
		if (ink.R > 200) != tc.lightInk {
			t.Errorf("%s chose unreadable ink: %+v", tc.text, ink)
		}
	}
}
