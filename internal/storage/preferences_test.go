package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/settings"
)

func TestPreferencesRoundTripAllChoicesAndPrivatePermissions(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"/"+theme, func(t *testing.T) {
				paused, guard, days := false, false, 7
				p := settings.Preferences{Language: language, Theme: theme, RecordingEnabled: &paused, PasteGuardEnabled: &guard, RetentionDays: &days}
				path := filepath.Join(t.TempDir(), "nested", "preferences.json")
				if err := WritePreferences(path, p); err != nil {
					t.Fatal(err)
				}
				loaded, err := ReadPreferences(path)
				if err != nil || !reflect.DeepEqual(p, loaded) {
					t.Fatalf("preferences changed: %+v %v", loaded, err)
				}
				stat, _ := os.Stat(path)
				if stat.Mode().Perm() != 0600 {
					t.Fatal("preferences permissions not private")
				}
			})
		}
	}
}
func TestLegacyPreferencesKeepSystemAppearanceAndDefaults(t *testing.T) {
	for _, raw := range []string{"", `{"language":"ja"}`, `{"language":"zh-Hant","theme":""}`, `{"language":"zh-Hans","theme":"future"}`} {
		t.Run(raw, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "preferences.json")
			if raw != "" {
				if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err := ReadPreferences(path)
			if err != nil {
				t.Fatal(err)
			}
			if p.Dark(false) || !p.Dark(true) || !settings.Enabled(p.RecordingEnabled) || !settings.Enabled(p.PasteGuardEnabled) || settings.Retention(p.RetentionDays) != 30 {
				t.Fatalf("legacy defaults changed: %+v", p)
			}
			p.Language = "en"
			if err = WritePreferences(path, p); err != nil {
				t.Fatal(err)
			}
			again, err := ReadPreferences(path)
			if err != nil || again.Theme != "" {
				t.Fatal("language update changed system-theme fallback")
			}
		})
	}
}
func TestInvalidPreferenceInputsAndUnknownLanguage(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"unknown language", `{"language":"future-language"}`, true},
		{"wrong type", `{"language":4}`, false},
		{"malformed", `{`, false},
		{"too large", strings.Repeat(" ", 64*1024+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "preferences.json")
			if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := ReadPreferences(path)
			if tc.valid {
				if err != nil || p.Language != "en" {
					t.Fatalf("unknown language fallback: %+v %v", p, err)
				}
			} else if err == nil {
				t.Fatal("malformed preference accepted")
			}
		})
	}
}
func TestPreferenceSaveFailurePreservesPreviousFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	p := settings.Preferences{Language: "ja", Theme: "dark"}
	if err := WritePreferences(path, p); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := WritePreferences(filepath.Join(path, "preferences.json"), settings.Default()); err == nil {
		t.Fatal("impossible preference path succeeded")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed preference save changed original")
	}
}
