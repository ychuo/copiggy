package nativeui

import (
	"image"
	"path"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestFilePathCardKeepsDocumentAndNamesInsideSharedBody(t *testing.T) {
	for _, raw := range []string{
		"/Users/alex/Documents/Brief.pdf",
		"~/Downloads/An exceptionally long document name with multiple words.pdf",
		"/Users/alex/Documents/项目说明与界面设计的最终版本.pdf",
		"/Users/alex/Documents/プロジェクトの最終デザインと操作説明.pdf",
		"/Volumes/Offline/README",
	} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			t.Run(raw+"/"+language, func(t *testing.T) {
				s, item := semanticPreviewFixture(t, raw)
				s.language = language
				file := clipmodel.Analyze(raw).FilePath
				if file == nil {
					t.Fatal("fixture was not recognized as a file path")
				}
				tt := ui.NewTester(func(c *ui.Context) {
					setNativeTheme(c, nativeFilePathColors(), false)
					ui.Box(c).Fill().Draw(nativeFilePathArtwork).Children(func() {
						ui.Box(c).Absolute().Top(nativeCardPadding + nativeCardHeaderHeight + nativeCardContentGap).Left(nativeCardPadding).Right(nativeCardPadding).Bottom(nativeCardPadding + nativeCardFooterHeight + nativeCardContentGap).Children(func() {
							s.nativeFilePathCard(c, item, file)
						})
					})
				}, nativeCardSize, nativeCardSize)
				name, ok := tt.Find(file.Name)
				if !ok || name.X < nativeCardPadding+38 || name.X+name.W > nativeCardSize-nativeCardPadding || name.Y < 66 || name.Y+name.H > 134 {
					t.Fatalf("file name escaped folder content: %+v", name)
				}
				parent, ok := tt.Find(path.Base(file.Dir))
				if !ok || parent.Y < name.Y+name.H || parent.Y+parent.H > 146 || parent.X+parent.W > nativeCardSize-nativeCardPadding {
					t.Fatalf("parent directory overlaps filename or footer: name=%+v parent=%+v", name, parent)
				}
			})
		}
	}
}

func TestFilePathPreviewPreservesExactRepresentationAndLocalizedFooter(t *testing.T) {
	for _, raw := range []string{
		"/Users/alex/Documents/Brief.pdf",
		"file:///Users/alex/My%20File.pdf",
		"~/Downloads/" + strings.Repeat("a-long-name-", 25) + ".pdf",
		"/Volumes/Offline/" + strings.Repeat("项目说明", 30) + ".pdf",
	} {
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			t.Run(raw+"/"+language, func(t *testing.T) {
				s, item := semanticPreviewFixture(t, raw)
				s.language = language
				file := clipmodel.Analyze(raw).FilePath
				before := s.store.Records()
				tt := ui.NewTester(func(c *ui.Context) {
					p := panelColors(false)
					setNativeTheme(c, p, false)
					s.nativeFilePathPreview(c, item, p)
				}, 520, nativeFilePathPreviewHeight)
				for _, label := range []string{file.Name, file.Path} {
					r, ok := tt.Find(label)
					if !ok || r.X < 120 || r.X+r.W > 490 {
						t.Fatalf("file details lost the full original value or escaped paper width: %q %+v", label, r)
					}
				}
				footer, ok := tt.Find(s.tr("File path"))
				if !ok || footer.Y < nativeFilePathPreviewHeight-50 || footer.Y+footer.H > nativeFilePathPreviewHeight-16 {
					t.Fatalf("localized File path metadata missing or displaced: %+v", footer)
				}
				if _, ok := tt.Find(s.tr("Copy")); !ok {
					t.Fatal("file path preview lost its shared copy control")
				}
				if !reflect.DeepEqual(s.store.Records(), before) || item.Text != raw {
					t.Fatal("file path rendering changed original saved representations")
				}
			})
		}
	}
}

func TestFilePathPreviewKeepsRoundedLightPaperInBothThemes(t *testing.T) {
	var light *image.RGBA
	for _, dark := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, "/Users/alex/Documents/Brief.pdf")
		s.viewState.Dark = dark
		tt := ui.NewTester(func(c *ui.Context) {
			p := panelColors(dark)
			setNativeTheme(c, p, dark)
			s.nativeFilePathPreview(c, item, p)
		}, 520, nativeFilePathPreviewHeight)
		pixels := tt.Image()
		glass := pixels.RGBAAt(7, 51)
		for _, point := range []image.Point{{9, 51}, {510, 51}, {9, nativeFilePathPreviewHeight - 10}, {510, nativeFilePathPreviewHeight - 10}} {
			if got := pixels.RGBAAt(point.X, point.Y); got != glass {
				t.Fatalf("file path reading paper fills glass corner dark=%t at %v: %v, want %v", dark, point, got, glass)
			}
		}
		if pixels.RGBAAt(260, 54).A < 250 {
			t.Fatal("file path preview has no rounded opaque reading paper")
		}
		if !dark {
			light = pixels
			continue
		}
		for y := 64; y < nativeFilePathPreviewHeight-24; y++ {
			for x := 24; x < 496; x++ {
				if got, want := pixels.RGBAAt(x, y), light.RGBAAt(x, y); got != want {
					t.Fatalf("file path reading paper changes with chrome appearance at %d,%d: %v, want %v", x, y, got, want)
				}
			}
		}
	}
}
