package nativeui

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"image"
	"math"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestSecretPreviewsKeepOpaqueRoundedReadingSurfaces(t *testing.T) {
	for _, raw := range []string{"OTP: 382941", "ghp_" + strings.Repeat("z", 36), semanticPrivateKeyFixture()} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, raw)
			s.viewState.Dark = dark
			height := nativeSecretPreviewHeight(item.Type)
			tt := ui.NewTester(s.nativePopoverView, 520, height)
			pixels := tt.Image()
			glass := pixels.RGBAAt(7, 51)
			for _, point := range []image.Point{{9, 51}, {510, 51}, {9, height - 10}, {510, height - 10}} {
				if got := pixels.RGBAAt(point.X, point.Y); got != glass {
					t.Fatalf("%s paper covers glass corner dark=%t point=%v: %v vs %v", item.Type, dark, point, got, glass)
				}
			}
			paper := pixels.RGBAAt(260, 54)
			if paper.A < 250 {
				t.Fatalf("%s reading surface is translucent: %v", item.Type, paper)
			}
			if item.Type == clipmodel.TypeOTP && (paper.R < 220 || paper.B < 230) {
				t.Fatalf("OTP paper lost its lavender appearance: %v", paper)
			}
			if item.Type != clipmodel.TypeOTP && (paper.R > 90 || paper.G > 100 || paper.B > 130) {
				t.Fatalf("credential preview lost its charcoal reading surface: %v", paper)
			}
		}
	}
}

func TestSecretRevealNeverDisplacesMetadataOrInventsProtection(t *testing.T) {
	longKey := string(pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte(strings.Repeat("keyfixture", 1800))}))
	for _, raw := range []string{"OTP: 38294175", "ghp_" + strings.Repeat("z", 250), longKey} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			for _, protected := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", clipmodel.Analyze(raw).Type, language, protected), func(t *testing.T) {
					s, item := semanticPreviewFixture(t, raw)
					s.language = language
					s.appNames["com.example.Editor"] = "Fixture editor"
					if protected {
						if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
							t.Fatal(err)
						}
						s.filter()
					}
					before, copies := s.store.Records(), 0
					s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) {
						copies++
						return clipmodel.Item{}, nil
					}
					height := nativeSecretPreviewHeight(item.Type)
					tt := ui.NewTester(s.nativePopoverView, 520, height)
					secret := clipmodel.Analyze(raw).Secret
					if secret == nil {
						t.Fatal("credential fixture was not recognized")
					}
					assertChrome := func() {
						t.Helper()
						close, found := tt.Find(s.tr("Close preview"))
						if !found || close.Y < 0 || close.Y+close.H > 48 {
							t.Fatalf("credential displaced preview toolbar: %+v", close)
						}
						format := s.tr(secret.Kind)
						if item.Type == clipmodel.TypeOTP {
							format = s.tr("%d digits", secret.Length)
						}
						footer, found := tt.Find(format)
						metadataBottom := float32(height - 26)
						if protected {
							metadataBottom -= 28
						}
						if !found || footer.Y < metadataBottom-22 || footer.Y+footer.H > metadataBottom {
							t.Fatalf("credential displaced format metadata: %+v", footer)
						}
						scope := s.tr("Only in %s", "Fixture editor")
						if tt.HasText(scope) != protected {
							t.Fatal("credential appearance invented or hid a protection scope")
						}
						if protected {
							row, found := tt.Find(scope + " · " + s.tr("Inactive"))
							if !found || row.H != 22 || row.Y+row.H != float32(height-26) || row.Y < footer.Y+footer.H+6 {
								t.Fatalf("protection scope escaped reading surface: %+v", row)
							}
						}
					}
					assertHidden := func() {
						t.Helper()
						_, exposesAccessibleValue := tt.Find(secret.Value)
						if tt.HasText(secret.Value) || exposesAccessibleValue {
							t.Fatal("hidden secret entered visible text or accessibility")
						}
					}
					if item.Type == clipmodel.TypeOTP {
						if _, found := tt.Find(secret.Value); !found {
							t.Fatal("verification code is not readable by default")
						}
						if _, found := tt.Find(s.tr("Reveal")); found {
							t.Fatal("visible verification code has a redundant reveal action")
						}
						assertChrome()
						if copies != 0 || !reflect.DeepEqual(before, s.store.Records()) {
							t.Fatal("displaying the code changed history or clipboard")
						}
						return
					}
					assertHidden()
					assertChrome()
					if err := tt.Click(s.tr("Reveal")); err != nil {
						t.Fatal(err)
					}
					if !tt.HasText(raw) {
						t.Fatal("revealed credential changed the exact saved representation")
					}
					tt.Scroll(260, 140, 0, 500)
					assertChrome()
					if err := tt.Click(s.tr("Hide")); err != nil {
						t.Fatal(err)
					}
					assertHidden()
					if copies != 0 || !reflect.DeepEqual(before, s.store.Records()) || !tt.Focused(s.tr("Content preview")) {
						t.Fatal("credential display changed history, clipboard, or keyboard focus")
					}
				})
			}
		}
	}
}

func TestPrivateKeyHiddenBoundariesDescribeOnlyValidatedFormat(t *testing.T) {
	for _, kind := range []string{"PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY", "DSA PRIVATE KEY", "OPENSSH PRIVATE KEY"} {
		bytes := []byte(strings.Repeat("keyfixture", 8))
		if kind == "OPENSSH PRIVATE KEY" {
			bytes = append([]byte("openssh-key-v1\x00"), bytes...)
		}
		raw := string(pem.EncodeToMemory(&pem.Block{Type: kind, Headers: map[string]string{"Comment": "hidden-header-fixture"}, Bytes: bytes}))
		s, item := semanticPreviewFixture(t, raw)
		if item.Type != clipmodel.TypePrivateKey {
			t.Fatal("private key fixture did not parse")
		}
		tt := ui.NewTester(s.nativePopoverView, 520, nativeSecretPreviewHeight(item.Type))
		for _, boundary := range []string{"-----BEGIN " + kind + "-----", "-----END " + kind + "-----"} {
			if !tt.HasText(boundary) {
				t.Fatalf("hidden key omitted its validated boundary: %q", boundary)
			}
		}
		if tt.HasText("hidden-header-fixture") || tt.HasText(string(bytes)) || tt.HasText(raw) {
			t.Fatal("hidden key leaked metadata or payload outside the format boundary")
		}
	}
}

func TestSecretMalformedAndMismatchedAnnotationsRemainHiddenAfterReveal(t *testing.T) {
	for _, kind := range []clipmodel.ContentType{clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey} {
		for _, raw := range []string{"malformed-private-value", "OTP: 382941", "ghp_" + strings.Repeat("z", 36)} {
			if clipmodel.Analyze(raw).Type == kind {
				continue
			}
			s, item := semanticPreviewFixture(t, raw)
			s.results[0].Type = kind
			s.semanticState(item).Revealed = true
			tt := ui.NewTester(s.nativePopoverView, 520, nativeSecretPreviewHeight(kind))
			if tt.HasText(raw) || !tt.HasText("Hidden content") {
				t.Fatalf("stale %s annotation exposed parsed or raw unrelated content", kind)
			}
			if _, exposed := tt.Find(raw); exposed {
				t.Fatal("unrecognized credential entered accessibility")
			}
		}
	}
}

func TestOTPVisibleDigitsKeepEqualSpacingAndEightDigitFit(t *testing.T) {
	for _, code := range []string{"1234", "12345", "123456", "1234567", "12345678"} {
		s, item := semanticPreviewFixture(t, "OTP: "+code)
		tt := ui.NewTester(s.nativePopoverView, 520, nativeSecretPreviewHeight(item.Type))
		var previous ui.Rect
		var steps []float32
		for index, digit := range code {
			box, found := tt.Find(string(digit))
			if !found || box.X < 40 || box.X+box.W > 480 || box.Y < 105 || box.Y+box.H > 230 {
				t.Fatalf("OTP digit escaped its reading slot: count=%d digit=%c rect=%+v", len(code), digit, box)
			}
			if index > 0 {
				steps = append(steps, box.X-previous.X)
			}
			previous = box
		}
		for _, step := range steps {
			if math.Abs(float64(step-steps[0])) > 0.5 {
				t.Fatalf("OTP digits are not equally spaced: count=%d steps=%v", len(code), steps)
			}
		}
	}
}

func TestCompactSecretPreviewKeepsChromeWhileReadingAreaScrolls(t *testing.T) {
	for _, raw := range []string{"OTP: 38294175", semanticPrivateKeyFixture()} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			for _, dark := range []bool{false, true} {
				s, item := semanticPreviewFixture(t, raw)
				s.language, s.viewState.Dark = language, dark
				s.appNames["com.example.Editor"] = "Fixture editor"
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
					t.Fatal(err)
				}
				s.filter()
				tt := ui.NewTester(s.nativePopoverView, 520, 232)
				for _, reveal := range []bool{false, true} {
					if reveal && item.Type != clipmodel.TypeOTP {
						if err := tt.Click(s.tr("Reveal")); err != nil {
							t.Fatal(err)
						}
					}
					scope := s.tr("Only in %s", "Fixture editor") + " · " + s.tr("Inactive")
					before, found := tt.Find(scope)
					if !found || before.Y+before.H > 214 || before.Y < 175 {
						t.Fatalf("compact protected %s lost footer: %+v", item.Type, before)
					}
					tt.Scroll(260, 120, 0, 500)
					after, found := tt.Find(scope)
					if !found || after != before {
						t.Fatal("scrolling sensitive content moved its protection metadata")
					}
					if item.Type == clipmodel.TypePrivateKey && !reveal {
						end, found := tt.Find("-----END PRIVATE KEY-----")
						if !found || end.Y+end.H > before.Y-8 {
							t.Fatalf("masked PEM content overlaps fixed footer: end=%+v scope=%+v", end, before)
						}
					}
				}
			}
		}
	}
}

func TestCredentialRevealBelongsToLeftActionGroup(t *testing.T) {
	for _, raw := range []string{"ghp_" + strings.Repeat("z", 36), semanticPrivateKeyFixture()} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			s, item := semanticPreviewFixture(t, raw)
			s.language = language
			tt := ui.NewTester(s.nativePopoverView, 520, nativeSecretPreviewHeight(item.Type))
			reveal, revealFound := tt.Find(s.tr("Reveal"))
			pin, pinFound := tt.Find(s.tr("Pin"))
			if !revealFound || !pinFound || reveal.X > 30 || pin.X < reveal.X+reveal.W+50 {
				t.Fatalf("credential toolbar lost left reveal and right management groups: reveal=%+v pin=%+v", reveal, pin)
			}
			if tt.HasText(s.tr("Token")) {
				t.Fatal("token toolbar retained its redundant title")
			}
		}
	}
}

func TestCredentialPartialShowsOnlySixCharacterEnds(t *testing.T) {
	key := semanticPrivateKeyFixture()
	block, _ := pem.Decode([]byte(key))
	encoded := base64.StdEncoding.EncodeToString(block.Bytes)
	for _, tc := range []struct{ raw, payload string }{
		{"ghp_abcdefghijklmnopqrstuvwx0123456789", "ghp_abcdefghijklmnopqrstuvwx0123456789"},
		{"Authorization: Bearer ghp_abcdefghijklmnopqrstuvwx0123456789", "ghp_abcdefghijklmnopqrstuvwx0123456789"},
		{key, encoded},
	} {
		value := clipmodel.Analyze(tc.raw)
		item := clipmodel.Item{Type: value.Type, Text: tc.raw}
		got := nativeSecretPartial(item, value)
		lines := strings.Split(got, "\n")
		wantRows := 3
		if item.Type == clipmodel.TypePrivateKey {
			wantRows = 4
		}
		if len(lines) != wantRows || lines[0] != tc.payload[:6]+strings.Repeat("•", 10) || lines[len(lines)-1] != strings.Repeat("•", 10)+tc.payload[len(tc.payload)-6:] {
			t.Fatalf("credential does not preserve its identifying ends in a multiline block: %q", got)
		}
		for _, middle := range lines[1 : len(lines)-1] {
			if middle != strings.Repeat("•", 16) {
				t.Fatalf("credential middle row contains more than a mask: %q", middle)
			}
		}
		if strings.Contains(got, tc.payload[6:len(tc.payload)-6]) {
			t.Fatal("credential abbreviation exposed the masked payload")
		}
		item.Type = clipmodel.TypeText
		if nativeSecretPartial(item, value) != "" {
			t.Fatal("stale type annotation exposed credential ends")
		}
	}
}

func TestSecretCardMainContentIsCentered(t *testing.T) {
	for _, raw := range []string{"OTP: 12345678", "ghp_" + strings.Repeat("z", 36), semanticPrivateKeyFixture()} {
		s, item := semanticPreviewFixture(t, raw)
		value := clipmodel.Analyze(raw)
		label := nativeSecretPartial(item, value)
		if item.Type == clipmodel.TypeOTP {
			label = value.Secret.Value
		}
		tt := ui.NewTester(func(c *ui.Context) {
			p := nativeSecretCardColors(item.Type)
			card := ui.Box(c).Fill()
			card.Children(func() { s.nativeCardContentWithAssets(c, card, item, value, p.text, p.muted, p, nativeCardAssets{}) })
		}, nativeCardSize, nativeCardSize)
		box, found := tt.Find(label)
		if !found || math.Abs(float64(box.X+box.W/2-nativeCardSize/2)) > 0.5 || math.Abs(float64(box.Y+box.H/2-nativeCardSize/2)) > 0.5 {
			t.Fatalf("secret main content is not centered: kind=%s box=%+v", item.Type, box)
		}
		if item.Type != clipmodel.TypeOTP {
			rows := len(strings.Split(label, "\n"))
			if box.X < nativeCardPadding || box.X+box.W > nativeCardSize-nativeCardPadding || box.H < float32(rows*13) || box.Y < 50 || box.Y+box.H > 146 {
				t.Fatalf("multiline credential escaped its card body: kind=%s box=%+v", item.Type, box)
			}
			preview := ui.NewTester(s.nativePopoverView, 520, nativeSecretPreviewHeight(item.Type))
			block, found := preview.Find(label)
			if !found || block.H < float32(rows*20) || block.X < 26 || block.X+block.W > 494 {
				t.Fatalf("credential preview did not retain its multiline mask: kind=%s box=%+v", item.Type, block)
			}
		}
	}
}
