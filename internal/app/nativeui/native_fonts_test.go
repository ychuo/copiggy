package nativeui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBundledFontsRegisterDistinctRegularAndMediumFaces(t *testing.T) {
	if err := loadNativeFonts(); err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, family := range ui.FontFamilies() {
		registered[family.Name] = family.Registered
	}
	for _, sample := range []struct{ family, language, text string }{
		{nativeInterFont, "en", "Clipboard 012345"},
		{nativeSCFont, "zh-Hans", "剪贴板记录"},
		{nativeTCFont, "zh-Hant", "剪貼簿記錄"},
		{nativeJPFont, "ja", "コピーした内容"},
	} {
		t.Run(sample.family, func(t *testing.T) {
			if !registered[sample.family] {
				t.Fatal("bundled family was not registered under its private alias")
			}
			var regular []byte
			for _, weight := range []int{nativeWeightBody, nativeWeightTitle} {
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Text(c, sample.text).Font(sample.family).Lang(sample.language).FontSize(24).FontWeight(weight).TextColor(ui.Hex("#202831"))
				}, 300, 60)
				pixels := tt.Image().Pix
				if weight == nativeWeightBody {
					regular = append([]byte(nil), pixels...)
				} else if bytes.Equal(regular, pixels) {
					t.Fatal("Regular and Medium rendered identically; requested weight was ignored")
				}
			}
		})
	}
}

func TestContentFontRemainsIndependentOfInterfaceLanguage(t *testing.T) {
	for _, content := range []string{"Hello 👋 café", "日本語のメモ", "東京"} {
		font := nativeContentFontFor(content, nativeTextPreview, nativeWeightBody)
		var reference []byte
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			tt := ui.NewTester(func(c *ui.Context) {
				setNativeTheme(c, panelColors(false), false, language)
				ui.Box(c).Lang(language).Children(func() {
					nativeContentText(c, content).FontSize(font.Size).FontWeight(font.Weight).SingleLine()
				})
			}, 400, 60)
			pixels := tt.Image().Pix
			if reference == nil {
				reference = append([]byte(nil), pixels...)
			} else if !bytes.Equal(reference, pixels) {
				t.Fatalf("menu language %s changed copied text typography", language)
			}
		}
	}
	if got := nativeContentFontFor("日本語のメモ", 17, 400); got.Lang != "ja" || !strings.HasPrefix(got.Family, nativeInterFont+", "+nativeJPFont) {
		t.Fatalf("Japanese content did not select the Japanese regional face: %+v", got)
	}
	if got := nativeContentFontFor("東京", 17, 400); got.Lang != "und" || got.Family != nativeUIFont {
		t.Fatalf("ambiguous content must have a stable fallback: %+v", got)
	}
}

func TestFontCascadesKeepLatinFirstAndRegionalFallback(t *testing.T) {
	for _, sample := range []struct{ language, regional string }{
		{"en", nativeSCFont}, {"zh-Hans", nativeSCFont}, {"zh-Hant", nativeTCFont}, {"ja", nativeJPFont},
	} {
		if !strings.HasPrefix(nativeFontForLanguage(sample.language), nativeInterFont+", "+sample.regional+", ") {
			t.Fatalf("unexpected font cascade for %s", sample.language)
		}
	}
}
