package nativeui

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func nativeRegionalPhoneFixture(t *testing.T, text, region string) (*application, clipmodel.Item) {
	t.Helper()
	s := nativeViewFixture(t)
	s.language = "en"
	s.store = clipboard.NewHistory(nil, 30)
	s.store.SetRecognitionRegion(region)
	item := nativeCaptureText(t, s, text, "Fixture editor")
	if item.Type != clipmodel.TypePhone {
		t.Fatalf("phone fixture was classified as %s", item.Type)
	}
	s.filter()
	s.preview = true
	return s, item
}

func TestPhoneFlagUsesRecognizedRegionInsteadOfPreference(t *testing.T) {
	for _, tc := range []struct{ text, context, region string }{
		{"13812345678", "CN", "CN"},
		{"+81 90 1234 5678", "CN", "JP"},
		{"+1 416 555 0123", "CN", "CA"},
		{"+1 202 555 0148", "CN", "US"},
		{"+800 1234 5678", "CN", "001"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, tc.text, tc.context)
			phone := clipmodel.AnalyzeItem(item).Phone
			if phone == nil || phone.Region != tc.region {
				t.Fatalf("fixture has unexpected recognized region: %+v", phone)
			}
			// The menu and current system region can change after capture.
			s.phoneRegion, s.config.SystemRegion = "FR", "GB"
			before := s.store.Records()
			writer := &nativeVariantWriter{}
			nativeVariantCopyService(s, writer)
			for _, language := range supportedLanguages {
				s.language = language.Code
				tt := ui.NewTester(s.nativePopoverView, 520, 360)
				label := s.tr("International")
				if tc.region != "001" {
					label = nativeRegionName(tc.region, language.Code)
					if nativePhoneFlagRegion(*phone) != tc.region || nativePhoneFlag(tc.region) == nil {
						t.Fatalf("recognized %s has no flag", tc.region)
					}
				} else if nativePhoneFlagRegion(*phone) != "" || nativePhoneFlag(tc.region) != nil {
					t.Fatal("non-geographic number acquired a country flag")
				}
				bounds, ok := tt.Find(label)
				if !ok || bounds.X < 24 || bounds.X+bounds.W > 496 || bounds.Y < 66 || bounds.Y+bounds.H > 330 {
					t.Fatalf("region metadata is missing or outside the preview: %s %+v", label, bounds)
				}
				if err := tt.Click(s.tr("Dialing number")); err != nil {
					t.Fatal(err)
				}
				if _, ok := tt.Find(label); !ok {
					t.Fatal("dialing format changed the number's country")
				}
			}
			if writer.calls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("region display, locale or format selection modified clipboard/history")
			}
		})
	}
}

func TestPhoneFlagAssetsCoverSupportedRegionsAndReuseParsedImages(t *testing.T) {
	for _, region := range clipmodel.SupportedPhoneRegions() {
		flag := nativePhoneFlag(region)
		if flag == nil {
			t.Errorf("missing or invalid embedded flag for %s", region)
			continue
		}
		width, height := flag.Size()
		if width <= 0 || height <= 0 || width/height < 1.32 || width/height > 1.34 {
			t.Errorf("flag is not a usable 4:3 image: %s %v×%v", region, width, height)
		}
		if nativePhoneFlag(strings.ToLower(region)) != flag {
			t.Errorf("flag %s was reparsed instead of reused", region)
		}
	}
	for _, unknown := range []string{"", "001", "ZZ", "../cn", "CN/JP"} {
		if nativePhoneFlag(unknown) != nil {
			t.Errorf("unknown region %q acquired a flag", unknown)
		}
	}
}

func TestPhoneRegionFlagKeepsOriginalColors(t *testing.T) {
	s := &application{language: "en"}
	tt := ui.NewTester(func(c *ui.Context) {
		setNativeTheme(c, nativePhoneColors(), false, "en")
		ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).TextColor(ui.Hex("#123456")).Children(func() {
			s.nativePhoneRegionMetadata(c, clipmodel.PhoneValue{Region: "JP"})
		})
	}, 300, 50)
	name, ok := tt.Find("Japan")
	if !ok {
		t.Fatal("flag has no accessible adjacent region name")
	}
	x, y := int(name.X)-8-nativePhoneFlagWidth, 25-nativePhoneFlagHeight/2
	pixels := tt.Image()
	white, red := pixels.RGBAAt(x+2, y+2), pixels.RGBAAt(x+nativePhoneFlagWidth/2, y+nativePhoneFlagHeight/2)
	if white.R < 235 || white.G < 235 || white.B < 235 || red.R < 150 || red.G > 50 || red.B > 90 {
		t.Fatalf("flag lost its white field/red circle or inherited text tint: white=%v red=%v", white, red)
	}
}

func TestPhoneLongRepresentationsFitWithoutLosingCharacters(t *testing.T) {
	for _, raw := range []string{
		"tel:5550148;phone-context=+1-202;ext=1234567890",
		"tel:+493012345678901;ext=1234567890",
		"＋４９３０１２３４５６７８９０１",
		"＋８６\u00a0１３８\u202f１２３４−５６７８",
		"+1 (202) 555-0148 extension 1234567890",
		"+86" + strings.Repeat("-", 180) + "13812345678",
		"+86" + strings.Repeat(" ", 180) + "13812345678",
	} {
		t.Run(raw, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, raw, "CN")
			before := s.store.Records()
			phone := clipmodel.AnalyzeItem(item).Phone
			for _, display := range []string{phone.Display, phone.DialingNumber()} {
				width := float32(520 - 2*(nativePreviewEdgeInset+nativePreviewHeroInset))
				lines, size := nativePhoneFittedLines(display, nativePhonePreviewNumberSize, 24, width)
				withoutSpace := func(value string) string {
					return strings.Map(func(r rune) rune {
						if unicode.IsSpace(r) {
							return -1
						}
						return r
					}, value)
				}
				if withoutSpace(strings.Join(lines, "")) != withoutSpace(display) {
					t.Fatal("visual wrapping removed a number or URI character")
				}
				for _, line := range lines {
					font := nativeContentFontFor(line, size, nativeWeightTitle)
					font.Features = "tnum"
					measured, _ := ui.ShapeText(line, font, 0).Size()
					if measured > width+.1 {
						t.Fatalf("phone line still clips at its minimum font: %q width=%v limit=%v", line, measured, width)
					}
				}
			}
			tray := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
			card, ok := tray.Find(s.nativeCardLabel(item))
			if !ok {
				t.Fatal("phone card is missing")
			}
			lines, _ := nativePhoneCardLayout(*phone)
			for _, line := range lines {
				bounds, ok := tray.Find(line)
				if !ok || bounds.X < card.X+nativeCardPadding || bounds.X+bounds.W > card.X+card.W-nativeCardPadding || bounds.Y+bounds.H > card.Y+116 {
					t.Fatalf("long phone display escaped card number area: %q %+v", line, bounds)
				}
			}
			if !reflect.DeepEqual(before, s.store.Records()) || item.Text != raw {
				t.Fatal("phone wrapping changed saved original representations")
			}
		})
	}
}
