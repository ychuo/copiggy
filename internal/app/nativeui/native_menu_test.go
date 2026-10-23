package nativeui

import (
	"testing"

	macos "copiggy/internal/macos/appkit"
	"copiggy/internal/settings"
	"github.com/egoist/mygo"
)

// Exercise the complete production menu at its platform boundary. Checking a
// small hand-written menu missed that all region submenus count toward the
// native limit and caused a fallback that lost icons and the Space glyph.
func TestCompleteSettingsMenuUsesNativePresentation(t *testing.T) {
	for _, language := range supportedLanguages {
		t.Run(language.Code, func(t *testing.T) {
			s := &application{language: language.Code, config: Config{SystemRegion: "US"}}
			s.applyPreferences(settings.Preferences{Language: language.Code, Theme: "light"})
			entries := s.nativeMenuEntries(s.nativeTrayMenu().Items())
			if err := macos.ValidateStatusMenu(entries, mygo.Rectangle{X: 100, Width: 24, Height: 24}); err != nil {
				t.Fatalf("complete settings menu would lose icons and Space glyph through fallback: %v", err)
			}
			foundOpen := false
			for _, entry := range entries {
				if !entry.Separator && entry.Symbol == "" {
					t.Errorf("settings item %q has no native symbol", entry.ID)
				}
				if entry.ID == "open" {
					foundOpen = true
					if entry.Key != " " || entry.Modifiers != macos.MenuControl|macos.MenuOption|macos.MenuShift {
						t.Error("open shortcut no longer reaches native Space glyph handling")
					}
				}
			}
			if !foundOpen {
				t.Fatal("settings menu is missing Open Clipboard")
			}
		})
	}
}
