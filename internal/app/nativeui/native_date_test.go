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

func TestDateCardKeepsMonthOnNarrowBandAndDayAboveFooter(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		s, _ := semanticPreviewFixture(t, "2026-10-22T15:30:00+08:00")
		s.language = language
		date := clipmodel.Analyze("2026-10-22T15:30:00+08:00").Date
		tt := ui.NewTester(func(c *ui.Context) {
			setNativeTheme(c, nativeDateColors(), false)
			ui.Box(c).Fill().Draw(nativeDateArtwork).Children(func() {
				ui.Box(c).Absolute().Top(nativeCardPadding + nativeCardHeaderHeight + nativeCardContentGap).Left(nativeCardPadding).Right(nativeCardPadding).Bottom(nativeCardPadding + nativeCardFooterHeight + nativeCardContentGap).Children(func() {
					s.nativeDateCard(c, date)
				})
			})
		}, nativeCardSize, nativeCardSize)
		month, ok := tt.Find(s.nativeMonthLabel(date.Time.Month()))
		if !ok || month.Y < 35 || month.Y+month.H > nativeDateHeaderHeight {
			t.Fatalf("%s month escaped narrow header: %+v", language, month)
		}
		day, ok := tt.Find("22")
		if !ok || day.Y < nativeDateHeaderHeight || day.Y+day.H > 132 {
			t.Fatalf("%s day moved into header or caption: %+v", language, day)
		}
		caption, ok := tt.Find(s.nativeWeekdayLabel(date.Time.Weekday()) + " · 15:30")
		if !ok || caption.Y < day.Y+day.H || caption.Y+caption.H > 146 || caption.X < nativeCardPadding || caption.X+caption.W > nativeCardSize-nativeCardPadding {
			t.Fatalf("%s caption overlaps day or shared footer: caption=%+v day=%+v", language, caption, day)
		}
		for _, text := range []ui.Rect{day, caption} {
			if center := text.X + text.W/2; math.Abs(float64(center-nativeCardSize/2)) > .5 {
				t.Fatalf("%s date is not horizontally centered: %+v", language, text)
			}
		}
		if center := (day.Y + caption.Y + caption.H) / 2; math.Abs(float64(center-nativeCardSize/2)) > .5 {
			t.Fatalf("%s date is not vertically centered: day=%+v caption=%+v center=%g", language, day, caption, center)
		}
	}
}

func TestDatePreviewKeepsRoundedPaperAndAppearanceIndependentInk(t *testing.T) {
	var light *image.RGBA
	for _, dark := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, "2026-10-22T15:30:00+08:00")
		s.viewState.Dark = dark
		tt := ui.NewTester(func(c *ui.Context) {
			p := panelColors(dark)
			setNativeTheme(c, p, dark)
			s.nativeDatePreview(c, item, p)
		}, 520, nativeDatePreviewHeight)
		pixels := tt.Image()
		glass := pixels.RGBAAt(7, 51)
		for _, point := range []image.Point{{9, 51}, {510, 51}, {9, nativeDatePreviewHeight - 10}, {510, nativeDatePreviewHeight - 10}} {
			if got := pixels.RGBAAt(point.X, point.Y); got != glass {
				t.Fatalf("date paper fills glass corner dark=%t at %v: %v, want %v", dark, point, got, glass)
			}
		}
		if pixels.RGBAAt(260, 54).A < 250 {
			t.Fatal("date preview has no opaque rounded reading paper")
		}
		if !dark {
			light = pixels
			continue
		}
		for y := 64; y < nativeDatePreviewHeight-24; y++ {
			for x := 24; x < 496; x++ {
				if got, want := pixels.RGBAAt(x, y), light.RGBAAt(x, y); got != want {
					t.Fatalf("date reading surface inherits chrome appearance at %d,%d: %v, want %v", x, y, got, want)
				}
			}
		}
	}
}

func TestDatePreviewPreservesOriginalZoneAndAllDayMeaning(t *testing.T) {
	for _, raw := range []string{"2026-10-22", "2026-10-22 15:30", "2026-10-22T15:30:00+08:00", "2026-10-22T15:30:00-03:30", "2026-10-22T15:30:00Z"} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			t.Run(raw+"/"+language, func(t *testing.T) {
				s, item := semanticPreviewFixture(t, raw)
				s.language = language
				date := clipmodel.Analyze(raw).Date
				if date == nil {
					t.Fatal("fixture was not recognized as a date")
				}
				before := s.store.Records()
				tt := ui.NewTester(func(c *ui.Context) {
					p := panelColors(false)
					setNativeTheme(c, p, false)
					s.nativeDatePreview(c, item, p)
				}, 520, nativeDatePreviewHeight)
				original, ok := tt.Find(raw)
				if !ok || original.Y < nativeDatePreviewHeight-50 || original.Y+original.H > nativeDatePreviewHeight-16 {
					t.Fatalf("original date is missing or displaced from metadata: %+v", original)
				}
				if !tt.HasText(s.tr(date.Time.Weekday().String())) {
					t.Fatal("date weekday is not localized")
				}
				monthYear := map[string]string{"en": "October 2026", "zh-Hans": "2026 年 10 月", "zh-Hant": "2026 年 10 月", "ja": "2026年10月"}[language]
				if !tt.HasText(monthYear) {
					t.Fatalf("date heading lost locale ordering: want %q", monthYear)
				}
				if !date.HasTime && !tt.HasText(s.tr("All day")) {
					t.Fatal("date-only content was displayed as a timed event")
				}
				if date.HasTime && !tt.HasText("15:30") {
					t.Fatal("date preview converted the original wall-clock time")
				}
				zoneFound := false
				for _, text := range tt.Texts() {
					zoneFound = zoneFound || strings.HasPrefix(text, "UTC")
				}
				if zoneFound != date.HasZone || date.HasZone && !tt.HasText(nativeTimeZoneLabel(date.Time)) {
					t.Fatal("date preview changed or invented a timezone")
				}
				if !reflect.DeepEqual(s.store.Records(), before) {
					t.Fatal("date preview changed saved representations")
				}
			})
		}
	}
}
