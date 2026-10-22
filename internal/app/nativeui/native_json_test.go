package nativeui

import (
	"context"
	"image"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestJSONSummaryCountsTopLevelCollectionWithoutChangingNumbers(t *testing.T) {
	s := nativeViewFixture(t)
	s.language = "en"
	for _, tc := range []struct{ raw, want string }{
		{`{"project":"Copiggy","theme":"light","pinned":true}`, "3 properties"},
		{`{"only":{"nested":1,"more":2}}`, "1 property"},
		{`{}`, "0 properties"},
		{`[1,{"nested":2},3]`, "3 items"},
		{`[1]`, "1 item"},
		{`[]`, "0 items"},
		{`{"large":123456789012345678901234567890123456789}`, "1 property"},
		{`"text"`, "JSON"}, {`true`, "JSON"}, {`null`, "JSON"}, {`1234`, "JSON"},
		{`{"broken":`, "JSON"}, {`{} []`, "JSON"},
	} {
		if got := s.nativeJSONSummary(tc.raw); got != tc.want {
			t.Errorf("JSON summary %q = %q; want %q", tc.raw, got, tc.want)
		}
	}
}

func TestJSONPreviewLongCodeKeepsToolbarAndMetadataStationary(t *testing.T) {
	for _, raw := range []string{
		`{"project":"Copiggy","theme":"light","pinned":true}`,
		`{"long":"` + strings.Repeat("こんにちは🌿你好", 250) + `"}`,
		strings.Repeat(`{"nested":`, 120) + `{"value":1}` + strings.Repeat("}", 120),
	} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			for _, dark := range []bool{false, true} {
				s, item := semanticPreviewFixture(t, raw)
				s.language = language
				before := s.store.Records()
				tt := ui.NewTester(func(c *ui.Context) {
					p := panelColors(dark)
					setNativeTheme(c, p, dark)
					s.nativeJSONPreview(c, item, p)
				}, 520, nativeJSONPreviewHeight)
				for _, key := range []string{"Format JSON", "Pin", "Delete item · ⌘⌫", "Close preview"} {
					r, ok := tt.Find(s.tr(key))
					if !ok || r.X < 22 || r.X+r.W > 498 || r.Y < 12 || r.Y+r.H > 42 {
						t.Fatalf("long code displaced toolbar %s: %+v", key, r)
					}
				}
				for _, footer := range []string{s.nativeJSONSummary(raw), s.tr("Original")} {
					r, ok := tt.Find(footer)
					if !ok || r.Y < nativeJSONPreviewHeight-40 || r.Y+r.H > nativeJSONPreviewHeight-10 {
						t.Fatalf("long code displaced JSON footer %s: %+v", footer, r)
					}
				}
				if !reflect.DeepEqual(before, s.store.Records()) {
					t.Fatal("display indentation changed saved JSON")
				}
			}
		}
	}
}

func TestJSONPreviewComparisonKeepsUnmodifiedOriginalAndDraftSeparate(t *testing.T) {
	raw := "{\"project\":\"Copiggy\",  \"pinned\":true}\n"
	s, item := semanticPreviewFixture(t, raw)
	draft, err := s.store.Enhance(context.Background(), item.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	s.draft = &draft
	before := s.store.Records()
	for _, original := range []bool{false, true} {
		s.nativeState().Original = original
		tt := ui.NewTester(func(c *ui.Context) {
			setNativeTheme(c, panelColors(false), false)
			s.nativeJSONPreview(c, item, panelColors(false))
		}, 520, nativeJSONPreviewHeight)
		want, provenance := draft.Text, "Formatted draft"
		if original {
			want, provenance = raw, "Original"
		}
		if !tt.HasText(want) || !tt.HasText(provenance) {
			t.Fatalf("comparison lost original whitespace or draft provenance: original=%t texts=%v", original, tt.Texts())
		}
		if !reflect.DeepEqual(before, s.store.Records()) {
			t.Fatal("comparison wrote a result before explicit confirmation")
		}
	}
}

func TestJSONReadingSurfaceKeepsDarkRoundedCornersInBothThemes(t *testing.T) {
	var light *image.RGBA
	for _, dark := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, `{"project":"Copiggy"}`)
		tt := ui.NewTester(func(c *ui.Context) {
			p := panelColors(dark)
			setNativeTheme(c, p, dark)
			s.nativeJSONPreview(c, item, p)
		}, 520, nativeJSONPreviewHeight)
		pixels := tt.Image()
		for _, point := range []image.Point{{9, 51}, {510, 51}} {
			if pixels.RGBAAt(point.X, point.Y).A != 0 {
				t.Fatalf("JSON paints over glass corner at %v", point)
			}
		}
		if pixels.RGBAAt(260, 54).A < 250 {
			t.Fatal("JSON dark reading surface missing")
		}
		if !dark {
			light = pixels
			continue
		}
		for y := 64; y < 275; y++ {
			for x := 24; x < 496; x++ {
				if pixels.RGBAAt(x, y) != light.RGBAAt(x, y) {
					t.Fatalf("JSON syntax ink or reading surface changes with chrome at %d,%d", x, y)
				}
			}
		}
	}
}
