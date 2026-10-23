package nativeui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sync"
	"unicode"

	"copiggy/internal/macos/appkit"
	"copiggy/resources"
	"github.com/egoist/mygo/ui"
)

const (
	nativeInterFont = "Copiggy Inter"
	nativeSCFont    = "Copiggy Noto Sans SC"
	nativeTCFont    = "Copiggy Noto Sans TC"
	nativeJPFont    = "Copiggy Noto Sans JP"
	nativeUIFont    = nativeInterFont + ", " + nativeSCFont + ", " + nativeTCFont + ", " + nativeJPFont + ", system-ui"
)

// Use private registration aliases so an installed font with a different
// version cannot change layout. Static faces are intentional: the pinned MyGo
// renderer selects registered weights but does not instantiate variable axes.
var loadNativeFonts = sync.OnceValue(func() error {
	var failures []error
	for _, family := range []struct{ directory, alias string }{
		{"inter", nativeInterFont}, {"noto-sans-sc", nativeSCFont},
		{"noto-sans-tc", nativeTCFont}, {"noto-sans-jp", nativeJPFont},
	} {
		files, err := fs.Glob(resources.Fonts, "fonts/"+family.directory+"/*.ttf")
		if err != nil || len(files) != 2 {
			failures = append(failures, fmt.Errorf("font family %s must contain Regular and Medium faces", family.directory))
			continue
		}
		for _, file := range files {
			data, err := resources.Fonts.ReadFile(file)
			if err == nil {
				err = ui.RegisterFont(data, family.alias)
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("register %s: %w", file, err))
			}
		}
	}
	err := errors.Join(failures...)
	if err != nil {
		// A failed face falls back through the remaining families. Keep startup
		// usable, and leave technical details out of the menu-bar status.
		log.Printf("load bundled fonts: %v", err)
	}
	return err
})

func nativeFontForLanguage(language string) string {
	switch language {
	case "ja":
		return nativeInterFont + ", " + nativeJPFont + ", " + nativeSCFont + ", " + nativeTCFont + ", system-ui"
	case "zh-Hant":
		return nativeInterFont + ", " + nativeTCFont + ", " + nativeSCFont + ", " + nativeJPFont + ", system-ui"
	default:
		return nativeUIFont
	}
}

var nativeContentLanguages = struct {
	sync.Mutex
	values map[[32]byte]string
}{values: make(map[[32]byte]string)}

// Content language never inherits the menu language. A bounded on-device hint
// selects the regional CJK face; ambiguous Han text uses the stable SC cascade.
// Cache only sample hashes and language tags, not additional clipboard strings.
func nativeContentLanguage(text string) string {
	hasCJK, count := false, 0
	for index, char := range text {
		if count == 1024 {
			text = text[:index]
			break
		}
		count++
		hasCJK = hasCJK || unicode.In(char, unicode.Han, unicode.Hiragana, unicode.Katakana)
	}
	if !hasCJK {
		return "und"
	}
	key := sha256.Sum256([]byte(text))
	nativeContentLanguages.Lock()
	defer nativeContentLanguages.Unlock()
	if language, ok := nativeContentLanguages.values[key]; ok {
		return language
	}
	language := appkit.TextLanguage(text)
	if len(nativeContentLanguages.values) >= 256 {
		clear(nativeContentLanguages.values)
	}
	nativeContentLanguages.values[key] = language
	return language
}

func nativeContentText(c *ui.Context, text string) ui.Element {
	language := nativeContentLanguage(text)
	return ui.Text(c, text).Font(nativeFontForLanguage(language)).Lang(language)
}

func nativeContentFontFor(text string, size float32, weight int) ui.Font {
	_ = loadNativeFonts()
	language := nativeContentLanguage(text)
	return ui.Font{Family: nativeFontForLanguage(language), Size: size, Weight: weight, Lang: language}
}
