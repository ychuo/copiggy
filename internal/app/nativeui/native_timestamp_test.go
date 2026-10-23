package nativeui

import (
	"context"
	"fmt"
	"image"
	"reflect"
	"strconv"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestProtectedTimestampKeepsScopeBelowCrossMidnightTimes(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("Fixture local", 14*60*60)
	t.Cleanup(func() { time.Local = previousLocal })
	instant := time.Date(2026, time.October, 21, 14, 7, 9, 123000000, time.UTC)
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, height := range []int{nativeTimestampPreviewHeight, 264} {
			for _, utc := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height=%d/utc=%t", language, height, utc), func(t *testing.T) {
					s, item := semanticPreviewFixture(t, strconv.FormatInt(instant.UnixMilli(), 10))
					s.language = language
					s.appNames["com.example.Editor"] = "Fixture editor"
					if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
						t.Fatal(err)
					}
					s.filter()
					s.semanticState(item).UTC = utc
					tt := ui.NewTester(s.nativePopoverView, 400, height)
					scope := s.tr("Only in %s", "Fixture editor") + " · " + s.tr("Inactive")
					zone, other := s.tr("UTC"), instant
					if utc {
						zone, other = "UTC+14:00", instant.In(time.Local)
					}
					secondary := zone + " · " + s.nativeTimestampDate(other) + " · " + nativeTimestampClock(other)
					info, found := tt.Find(scope)
					if !found || info.Y+info.H > float32(height-20) || info.Y < 210 {
						t.Fatalf("protection scope escaped reading surface: %+v", info)
					}
					tt.Scroll(200, 150, 0, 500)
					alternate, found := tt.Find(secondary)
					if !found || alternate.Y+alternate.H > info.Y-8 {
						t.Fatalf("secondary time overlaps protection scope: secondary=%+v scope=%+v", alternate, info)
					}
					if !tt.Focused(s.tr("Content preview")) {
						t.Fatal("scrolling timestamp displaced keyboard focus")
					}
				})
			}
		}
	}
}

func TestTimestampPreviewSurfaceKeepsDarkPaperAndRoundedGlass(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, _ := semanticPreviewFixture(t, "1792645800123")
		s.viewState.Dark = dark
		tt := ui.NewTester(s.nativePopoverView, 520, nativeTimestampPreviewHeight)
		pixels := tt.Image()
		glass := pixels.RGBAAt(7, 51)
		for _, point := range []image.Point{{9, 51}, {510, 51}, {9, 300}, {510, 300}} {
			if got := pixels.RGBAAt(point.X, point.Y); got != glass {
				t.Fatalf("timestamp surface covers rounded glass corner dark=%t at %v: %v, glass=%v", dark, point, got, glass)
			}
		}
		if got := pixels.RGBAAt(260, 54); got.A < 250 || got.R > 90 || got.G > 100 || got.B > 130 {
			t.Fatalf("timestamp lost its opaque dark reading surface: %v", got)
		}
	}
}

func TestTimestampDisplayOptionsKeepPrecisionFocusAndSavedOriginal(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("Fixture local", 8*60*60)
	t.Cleanup(func() { time.Local = previousLocal })
	// A cross-midnight local/UTC pair exposes dates as well as clock values,
	// preventing the secondary time from silently implying the wrong day.
	instant := time.Date(2026, time.October, 21, 18, 7, 9, 123000000, time.UTC)
	raw := strconv.FormatInt(instant.UnixMilli(), 10)
	s, item := semanticPreviewFixture(t, raw)
	before := s.store.Records()
	copies := 0
	s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) {
		copies++
		return clipmodel.Item{}, nil
	}
	tt := ui.NewTester(s.nativePopoverView, 520, nativeTimestampPreviewHeight)
	for _, text := range []string{raw, "s", "ms", "UTC+08:00", "Oct 22, 2026", "02:07:09.123", "UTC · Oct 21, 2026 · 18:07:09.123"} {
		if !tt.HasText(text) {
			t.Fatalf("missing local timestamp presentation: %q", text)
		}
	}
	if err := tt.Click("Seconds"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(strconv.FormatInt(instant.Unix(), 10)) || !tt.HasText("02:07:09.123") {
		t.Fatal("unit choice lost the captured sub-second instant")
	}
	if err := tt.Click("Local time"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"UTC", "Oct 21, 2026", "18:07:09.123", "UTC+08:00 · Oct 22, 2026 · 02:07:09.123"} {
		if !tt.HasText(text) {
			t.Fatalf("missing UTC timestamp presentation: %q", text)
		}
	}
	if !tt.Focused("Content preview") || copies != 0 || !reflect.DeepEqual(before, s.store.Records()) {
		t.Fatal("display choices changed history, clipboard, or keyboard focus")
	}
	assertNativeCurrent(t, s, item.ID)
}

func TestTimestampCounterStaysInsideCardAndPreview(t *testing.T) {
	for _, raw := range []string{"1792645800", "1792645800123"} {
		s, item := semanticPreviewFixture(t, raw)
		tt := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
		card, found := tt.Find(s.nativeCardLabel(item))
		value, valueFound := tt.Find(raw)
		if !found || !valueFound || value.X < card.X+nativeCardPadding || value.X+value.W > card.X+card.W-nativeCardPadding || value.Y < card.Y+60 || value.Y+value.H > card.Y+100 {
			t.Fatalf("counter overlaps card chrome/ticks: counter=%+v card=%+v", value, card)
		}
		preview := ui.NewTester(s.nativePopoverView, 520, nativeTimestampPreviewHeight)
		value, valueFound = preview.Find(raw)
		if !valueFound || value.X != 30 || value.X+value.W > 490 || value.Y < 72 || value.Y+value.H > 200 {
			t.Fatalf("counter escaped left-aligned preview reading surface: %+v", value)
		}
	}
}
