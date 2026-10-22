package nativeui

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNotePreviewKeepsExactParagraphsAndFixedMetadataFooter(t *testing.T) {
	for _, raw := range []string{
		"Make room for the next idea.\n\nKeep what matters.\nLet the rest go.",
		"  Keep whitespace.\n\n" + strings.Repeat("Long paragraphs should scroll without moving the footer.\n", 100),
		strings.Repeat("こんにちは、世界。你好，世界。👩🏽‍💻🌿\n", 100),
	} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			s, item := semanticPreviewFixture(t, raw)
			s.language = language
			item.SourceApp = "An exceptionally long original application name"
			before := s.store.Records()
			tt := ui.NewTester(func(c *ui.Context) {
				p := panelColors(false)
				setNativeTheme(c, p, false)
				s.nativeNotePreview(c, item, p)
			}, 520, nativeNotePreviewHeight)
			text, ok := tt.Find(raw)
			if !ok || text.X < 34 || text.X+text.W > 486 || text.Y < 68 {
				t.Fatalf("note lost exact text or reading inset: %+v", text)
			}
			footer, ok := tt.Find(s.tr("Plain text"))
			if !ok || footer.Y < nativeNotePreviewHeight-50 || footer.Y+footer.H > nativeNotePreviewHeight-15 {
				t.Fatalf("note footer displaced by long text: %+v", footer)
			}
			if tt.HasText(item.SourceApp) {
				t.Fatal("note preview retained redundant source information")
			}
			for _, label := range []string{"Pin", "Delete item · ⌘⌫", "Close preview"} {
				action, ok := tt.Find(s.tr(label))
				if !ok || action.Y+action.H > 42 {
					t.Fatalf("note toolbar displaced: %s %+v", label, action)
				}
			}
			if _, ok := tt.Find(s.tr("Copy")); ok {
				t.Fatal("plain text preview added a redundant copy action")
			}
			if !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("note rendering altered history")
			}
		}
	}
}

func TestNotePaperStaysWarmAndRoundedAcrossAppearance(t *testing.T) {
	var light *image.RGBA
	for _, dark := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, "A quiet native note.")
		tt := ui.NewTester(func(c *ui.Context) {
			p := panelColors(dark)
			setNativeTheme(c, p, dark)
			s.nativeNotePreview(c, item, p)
		}, 520, nativeNotePreviewHeight)
		pixels := tt.Image()
		for _, point := range []image.Point{{9, 51}, {510, 51}, {9, nativeNotePreviewHeight - 10}, {510, nativeNotePreviewHeight - 10}} {
			if pixels.RGBAAt(point.X, point.Y).A != 0 {
				t.Fatalf("note painted over glass corner at %v", point)
			}
		}
		if pixels.RGBAAt(260, 54).A < 250 {
			t.Fatal("note reading paper has no opaque rounded surface")
		}
		if !dark {
			light = pixels
			continue
		}
		for y := 64; y < nativeNotePreviewHeight-24; y++ {
			for x := 24; x < 496; x++ {
				if pixels.RGBAAt(x, y) != light.RGBAAt(x, y) {
					t.Fatalf("note paper typography or surface changes with chrome at %d,%d", x, y)
				}
			}
		}
	}
}
