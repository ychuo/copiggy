package nativeui

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestPhoneLocationCompletionRebuildsIdlePreview(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hant", "ja"} {
		t.Run(locale, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, "+8613702032331", "CN")
			s.language = locale
			var services ui.Services
			tt := ui.NewTester(func(c *ui.Context) {
				services = c.Services()
				s.nativePopoverView(c)
			}, 520, 360)
			tt.Advance(time.Second)
			if tt.HasText("Tianjin") {
				t.Fatal("location was present before request completion")
			}
			completeNativePhoneLocation(t, s, item)
			// Exercise the same content invalidation as Window.Invalidate,
			// consuming only scheduled frames rather than forcing Tester.Frame.
			services.Invalidate()
			tt.Advance(time.Second)
			if !tt.HasText("Tianjin") {
				t.Fatal("completed location did not appear in the idle preview")
			}
		})
	}
}

func TestPhoneLocationCompletionRecoversConsumedFrame(t *testing.T) {
	for _, locale := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		t.Run(locale, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, "+8613702032331", "CN")
			s.language = locale
			var services ui.Services
			tt := ui.NewTester(func(c *ui.Context) {
				services = c.Services()
				s.nativePopoverView(c)
			}, 520, 360)
			tt.Advance(time.Second)
			visible, requests := false, 0
			invalidate := func() {
				requests++
				// MyGo consumes a requested frame while its surface is
				// occluded. The NSPopover's temporary window does not
				// deliver the ordinary window's SurfaceShown event.
				if visible {
					services.Invalidate()
				}
			}
			want := completeNativePhoneLocation(t, s, item)
			invalidate() // The async completion's original publication is held.
			for range 3 {
				s.syncNativePhoneLocation(invalidate)
				tt.Advance(20 * time.Millisecond)
			}
			if tt.HasText(want) {
				t.Fatal("occluded preview unexpectedly consumed the completed result")
			}
			visible = true
			s.syncNativePhoneLocation(invalidate)
			tt.Advance(20 * time.Millisecond)
			if !tt.HasText(want) {
				t.Fatal("completed location stayed absent after the preview became visible")
			}
			afterRead := requests
			for range 3 {
				s.syncNativePhoneLocation(invalidate)
				tt.Advance(20 * time.Millisecond)
			}
			if requests != afterRead {
				t.Fatal("location kept requesting frames after the view consumed it")
			}
		})
	}
}

func TestPhoneLocationFrameRetryStopsForStaleOrEmptyResults(t *testing.T) {
	for _, reason := range []string{"empty", "closed", "reopened", "language", "navigation", "deleted", "number", "hiding", "quitting"} {
		t.Run(reason, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, "+8613702032331", "CN")
			completeNativePhoneLocation(t, s, item)
			entry := s.semanticPreview.PhoneLocation
			switch reason {
			case "empty":
				if !s.finishNativePhoneLocation(item.ID, entry, "") {
					t.Fatal("empty location was not accepted")
				}
			case "closed":
				s.closeNativePreview()
			case "reopened":
				s.closeNativePreview()
				s.preview = true
				s.nativePhoneLocation(item, *clipmodel.AnalyzeItem(item).Phone)
			case "language":
				s.language = "ja"
			case "navigation":
				nativeCaptureText(t, s, "+81 3 1234 5678", "Fixture editor")
				s.filter()
				s.selectNative(0, true)
			case "deleted":
				if err := s.store.Delete(context.Background(), item.ID); err != nil {
					t.Fatal(err)
				}
			case "number":
				entry.e164 = "+81312345678"
			case "hiding":
				s.hiding = true
			case "quitting":
				s.quitting = true
			}
			for range 3 {
				s.syncNativePhoneLocation(func() { t.Fatal("stale or empty result kept requesting frames") })
			}
		})
	}
}

// Headless tests explicitly complete the same offline request used by the
// worker. They never create a native window or dispatch through AppKit.
func completeNativePhoneLocation(t *testing.T, s *application, item clipmodel.Item) string {
	t.Helper()
	phone := clipmodel.AnalyzeItem(item).Phone
	if phone == nil {
		t.Fatal("fixture is not a phone")
	}
	s.nativePhoneLocation(item, *phone)
	entry := s.semanticPreview.PhoneLocation
	value := clipmodel.PhoneLocation(*phone, s.currentLanguage())
	if !s.finishNativePhoneLocation(item.ID, entry, value) {
		t.Fatal("active phone preview rejected its offline location")
	}
	return value
}

func TestPhoneLocationPreviewLanguagesAndProtectedRecords(t *testing.T) {
	for _, raw := range []string{"13702032331", "+86 137 0203 2331"} {
		for _, protected := range []bool{false, true} {
			s, item := nativeRegionalPhoneFixture(t, raw, "CN")
			if protected {
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
					t.Fatal(err)
				}
				s.filter()
				item, _ = s.selected()
			}
			writer := &nativeVariantWriter{}
			nativeVariantCopyService(s, writer)
			before := s.store.Records()
			for index, language := range supportedLanguages {
				s.language = language.Code
				tt := ui.NewTester(s.nativePopoverView, 520, 360)
				if tt.HasText("Tianjin") || tt.HasText("天津市") {
					t.Fatal("changed locale displayed the previous location before completion")
				}
				want := []string{"Tianjin", "天津市", "Tianjin", "Tianjin"}[index]
				if got := completeNativePhoneLocation(t, s, item); got != want {
					t.Fatalf("%s location = %q, want %q", language.Code, got, want)
				}
				tt.Frame()
				country, countryOK := tt.Find(nativeRegionName("CN", language.Code))
				location, locationOK := tt.Find(s.tr("Number location: %s", want))
				if !countryOK || !locationOK || location.Y < country.Y+country.H || location.X < 24 || location.X+location.W > 496 || location.Y+location.H > 310 {
					t.Fatalf("%s location is missing or outside its country metadata: country=%+v location=%+v", language.Code, country, location)
				}
				entry := s.semanticPreview.PhoneLocation
				if err := tt.Click(s.tr("Dialing number")); err != nil {
					t.Fatal(err)
				}
				if s.semanticPreview.PhoneLocation != entry || !tt.HasText(want) {
					t.Fatal("dialing selection discarded the current location")
				}
				if protected && !tt.HasText(s.scopeLabel(item.AllowedApps)) {
					t.Fatal("offline location displaced protected scope metadata")
				}
			}
			if writer.calls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("offline location or language change wrote clipboard/history")
			}
		}
	}
}

func TestPhoneLocationPreviewUsesInternationalPrefixAndOmitsCountryOnly(t *testing.T) {
	for _, tc := range []struct{ raw, locale, region, want string }{
		{"+81 3 1234 5678", "ja", "JP", "東京"},
		{"+886 2 2123 4567", "zh-Hant", "TW", "臺北"},
		{"+852 2123 4567", "zh-Hans", "HK", ""},
		{"+800 1234 5678", "en", "001", ""},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, tc.raw, "CN")
			s.language = tc.locale
			if got := completeNativePhoneLocation(t, s, item); got != tc.want {
				t.Fatalf("location = %q, want %q", got, tc.want)
			}
			tt := ui.NewTester(s.nativePopoverView, 520, 360)
			if tc.want != "" {
				if !tt.HasText(tc.want) || !tt.HasText(nativeRegionName(tc.region, tc.locale)) {
					t.Fatal("explicit international prefix did not choose country and location")
				}
			} else {
				for _, text := range tt.Texts() {
					if strings.Contains(text, s.tr("Number location: %s", "")) || strings.Contains(text, "Unknown") {
						t.Fatalf("missing detail left a placeholder: %q", text)
					}
				}
			}
		})
	}
}

func TestPhoneLocationCompletionRejectsStalePreview(t *testing.T) {
	for _, reason := range []string{"closed", "reopened", "language", "navigation", "deleted", "number", "quitting"} {
		t.Run(reason, func(t *testing.T) {
			s, item := nativeRegionalPhoneFixture(t, "+8613702032331", "CN")
			s.nativePhoneLocation(item, *clipmodel.AnalyzeItem(item).Phone)
			entry := s.semanticPreview.PhoneLocation
			switch reason {
			case "closed":
				s.closeNativePreview()
			case "reopened":
				s.closeNativePreview()
				s.preview = true
				s.nativePhoneLocation(item, *clipmodel.AnalyzeItem(item).Phone)
			case "language":
				s.language = "ja"
			case "navigation":
				nativeCaptureText(t, s, "+81 3 1234 5678", "Fixture editor")
				s.filter()
				s.selectNative(0, true)
			case "deleted":
				if err := s.store.Delete(context.Background(), item.ID); err != nil {
					t.Fatal(err)
				}
			case "number":
				entry.e164 = "+81312345678"
			case "quitting":
				s.quitting = true
			}
			if s.finishNativePhoneLocation(item.ID, entry, "Tianjin") || entry.value != "" {
				t.Fatal("stale offline result was published")
			}
		})
	}
}

func TestPhoneLocationStaysOutOfCardsAndWrapsInsidePreview(t *testing.T) {
	s, item := nativeRegionalPhoneFixture(t, "tel:13702032331;phone-context=+86;ext=1234567890", "CN")
	s.preview = false
	ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
	if s.semanticPreview.PhoneLocation != nil {
		t.Fatal("history card requested offline geocoding")
	}
	s.preview = true
	phone := clipmodel.AnalyzeItem(item).Phone
	s.nativePhoneLocation(item, *phone)
	longName := "A long municipality name, within a longer administrative district and province, including a second municipal area with another extended name"
	if !s.finishNativePhoneLocation(item.ID, s.semanticPreview.PhoneLocation, longName) {
		t.Fatal("active location request did not complete")
	}
	tt := ui.NewTester(s.nativePopoverView, 520, 360)
	location, ok := tt.Find(s.tr("Number location: %s", longName))
	if !ok || location.X < 24 || location.X+location.W > 496 || location.H <= nativeTextControl*1.35 {
		t.Fatalf("long location did not wrap inside preview width: %+v", location)
	}
	close, ok := tt.Find(s.tr("Close preview"))
	if !ok || close.Y+close.H > 48 {
		t.Fatal("long metadata displaced the fixed preview toolbar")
	}
}
