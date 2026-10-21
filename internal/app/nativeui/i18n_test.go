package nativeui

import (
	"errors"
	"fmt"

	"copiggy/internal/clipmodel"
	"copiggy/internal/macos/guard"
	"copiggy/internal/macos/pasteboard"
	"copiggy/internal/settings"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
)

func TestLanguageDefaultsToEnglishAndKeepsFormats(t *testing.T) {
	s := &application{}
	if s.currentLanguage() != "en" || s.tr("Search clipboard history") != "Search clipboard history" {
		t.Fatal("new installations do not default to English")
	}
	if s.tr("%d items", 1) != "1 item" || s.tr("%d items", 2) != "2 items" || s.tr("%d characters", 1) != "1 character" {
		t.Fatal("English count labels are not readable")
	}
	for _, language := range supportedLanguages {
		s.language = language.Code
		if got := s.tr("Only in %s", "TextEdit"); !strings.Contains(got, "TextEdit") || strings.Contains(got, "%!") {
			t.Fatalf("app identity was damaged in %s: %q", language.Code, got)
		}
	}
	s.language = "unsupported"
	if s.currentLanguage() != "en" || s.tr("Copy") != "Copy" {
		t.Fatal("unknown stored languages should safely fall back to English")
	}
}

func TestCatalogHasCompleteTranslationsAndMatchingPlaceholders(t *testing.T) {
	placeholder := regexp.MustCompile(`%[ds]`)
	for key, row := range translations {
		for index, text := range row {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s has no %s translation", key, supportedLanguages[index].Code)
			}
			if !slices.Equal(placeholder.FindAllString(row[0], -1), placeholder.FindAllString(text, -1)) {
				t.Errorf("%s has inconsistent %s format arguments", key, supportedLanguages[index].Code)
			}
		}
		for _, r := range row[0] {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
				t.Errorf("English entry is untranslated: %s: %q", key, row[0])
				break
			}
		}
	}
}

func TestLanguageMenuPersistsAndReloadsEachSelection(t *testing.T) {
	saved := settings.Default()
	s := &application{config: Config{SavePreferences: func(p settings.Preferences) error { saved = p.Clone(); return nil }}, viewState: &nativePanelState{}}
	s.applyPreferences(saved)
	for _, language := range supportedLanguages[1:] {
		menu := s.nativeTrayMenu()
		item := menu.ItemByID("language-" + language.Code)
		if item == nil || item.Label != language.Name {
			t.Fatalf("language choice is missing: %s", language.Code)
		}
		item.Click(nil, nil)
		if s.currentLanguage() != language.Code {
			t.Fatalf("menu selected the wrong language: want %s, got %s", language.Code, s.currentLanguage())
		}
		if s.message != s.tr("Language changed") {
			t.Fatal("language change did not produce a status in the selected language")
		}
		loaded := &application{viewState: &nativePanelState{}}
		loaded.applyPreferences(saved)
		if loaded.currentLanguage() != language.Code {
			t.Fatalf("language selection was not restored: %s", loaded.currentLanguage())
		}
		menu = loaded.nativeTrayMenu()
		for _, candidate := range supportedLanguages {
			if menu.ItemByID("language-"+candidate.Code).Checked != (candidate.Code == language.Code) {
				t.Fatalf("language menu has stale checkmarks after loading %s", language.Code)
			}
		}
	}
}

func TestInvalidLanguagePreferenceLeavesEnglishUsable(t *testing.T) {
	s := &application{}
	s.applyPreferences(settings.Preferences{Language: "unsupported"})
	if s.currentLanguage() != "en" {
		t.Fatalf("invalid language changed the default: %q", s.currentLanguage())
	}
}

func TestTranslatedStatusIsNotReplacedByLegacyLanguage(t *testing.T) {
	s := &application{language: "en"}
	s.reportNativeStatus(s.tr("Paste Guard on"))
	if s.trayStatus.title(time.Now()) != "Paste Guard on" {
		t.Fatal("translated event was not kept intact")
	}
	s.language = "ja"
	s.reportNativeStatus(s.tr("Paste blocked"))
	if s.trayStatus.title(time.Now()) != "貼り付けをブロックしました" {
		t.Fatal("translated Japanese event was replaced by legacy Chinese text")
	}
	if s.nativeErrorStatus(fmt.Errorf("start guard: %w", guard.ErrAccessibility), "Could not start Paste Guard") != s.tr("Allow Accessibility access") {
		t.Fatal("bridge error was not localized at the UI boundary")
	}
}

func TestNativeErrorsUseIdentityAndLocalizedFallbacks(t *testing.T) {
	for _, language := range supportedLanguages {
		s := &application{language: language.Code}
		for _, tc := range []struct {
			err  error
			want string
		}{
			{fmt.Errorf("platform detail: %w", guard.ErrAccessibility), "Allow Accessibility access"},
			{fmt.Errorf("platform detail: %w", guard.ErrGuardUnavailable), "Could not start Paste Guard"},
			{fmt.Errorf("platform detail: %w", pasteboard.ErrChanged), "Clipboard changed · Try again"},
			{clipmodel.ErrDraftStale, "Content changed · Try again"},
			{clipmodel.ErrDraftNotFound, "Item unavailable · Reselect"},
			{fmt.Errorf("capture failed: %w", clipmodel.ContentError("Empty image ignored")), "Empty image ignored"},
			{errors.New("accessibility occurred inside an unrelated raw backend error"), "Could not copy"},
		} {
			if got := s.nativeErrorStatus(tc.err, "Could not copy"); got != s.tr(tc.want) {
				t.Errorf("%s: %v: got %q, want %q", language.Code, tc.err, got, s.tr(tc.want))
			}
		}
	}
}

func TestUnknownSourceFollowsLanguageWithoutChangingProvenance(t *testing.T) {
	item := clipmodel.Item{Type: clipmodel.TypeImage}
	s := &application{language: "en"}
	if s.sourceAppName(item) != "Unknown app" {
		t.Fatal("missing source has no English fallback")
	}
	s.language = "ja"
	if got := s.sourceAppName(item); got != s.tr("Unknown app") || item.SourceApp != "" {
		t.Fatal("unknown source was persisted in a UI language")
	}
	item.SourceApp = translations["Unknown app"][1]
	if s.sourceAppName(item) != s.tr("Unknown app") || item.SourceApp != translations["Unknown app"][1] {
		t.Fatal("legacy unknown source should follow the UI language without rewriting the pin")
	}
	item.SourceAppID = "com.example.real-app"
	if s.sourceAppName(item) != item.SourceApp {
		t.Fatal("a real identified app must keep its name even if it resembles a fallback")
	}
	item.SourceApp = "TextEdit"
	if s.sourceAppName(item) != "TextEdit" {
		t.Fatal("real source app names must not be translated")
	}
}
