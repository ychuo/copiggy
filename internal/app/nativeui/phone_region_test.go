package nativeui

import (
	"errors"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/settings"
)

func TestPhoneRegionMenuPersistsSeparatelyFromLanguage(t *testing.T) {
	saved := settings.Preferences{Language: "ja", Theme: "dark"}
	s := &application{config: Config{SystemRegion: "CN", SavePreferences: func(p settings.Preferences) error { saved = p; return nil }}}
	s.applyPreferences(saved)
	menu := s.nativeTrayMenu()
	if !menu.ItemByID("phone-region-").Checked {
		t.Fatal("default does not follow system region")
	}
	for _, code := range clipmodel.SupportedPhoneRegions() {
		if menu.ItemByID("phone-region-"+code) == nil {
			t.Fatalf("missing selectable region %s", code)
		}
	}
	menu.ItemByID("phone-region-CN").Click(nil, nil)
	if saved.PhoneRegion != "CN" || saved.Language != "ja" || saved.Theme != "dark" || s.trayStatus.title(time.Now()) != "" {
		t.Fatal("region selection changed other preferences or announced success")
	}
	restored := &application{}
	restored.applyPreferences(saved)
	if !restored.nativeTrayMenu().ItemByID("phone-region-CN").Checked {
		t.Fatal("region selection was not restored")
	}
	restored.language = "en"
	if restored.preferenceSnapshot().PhoneRegion != "CN" {
		t.Fatal("language changed the telephone region")
	}
}

func TestPhoneRegionSaveFailureRestoresPreviousSelection(t *testing.T) {
	s := &application{phoneRegion: "CN", config: Config{SavePreferences: func(settings.Preferences) error { return errors.New("disk unavailable") }}}
	s.setPhoneRegion("JP")
	if s.phoneRegion != "CN" || s.trayStatus.title(time.Now()) != s.tr("Could not save phone region") {
		t.Fatal("failed region save was accepted or hidden")
	}
}
