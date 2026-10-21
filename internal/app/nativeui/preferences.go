package nativeui

import (
	"copiggy/internal/settings"
	"log"
	"time"
)

const defaultRetentionDays = settings.DefaultRetentionDays

func (s *application) applyPreferences(p settings.Preferences) {
	p = settings.Normalize(p)
	s.language, s.themePreference = p.Language, p.Theme
	s.monitoring, s.guardPreference = settings.Enabled(p.RecordingEnabled), settings.Enabled(p.PasteGuardEnabled)
	s.retentionDays = settings.Retention(p.RetentionDays)
	if s.viewState != nil {
		s.viewState.Dark = p.Dark(s.viewState.Dark)
	}
}
func (s *application) preferenceSnapshot() settings.Preferences {
	recording, guard, days := s.monitoring, s.guardPreference, s.retentionDays
	return settings.Preferences{Language: s.currentLanguage(), Theme: s.themePreference, RecordingEnabled: &recording, PasteGuardEnabled: &guard, RetentionDays: &days}
}
func (s *application) savePreferences(done func(error)) {
	s.preferenceGeneration++
	p := s.preferenceSnapshot()
	s.perform(func() error {
		if s.config.SavePreferences == nil {
			return nil
		}
		return s.config.SavePreferences(p)
	}, done)
}
func (s *application) prefersDark(systemDark bool) bool {
	return (settings.Preferences{Theme: s.themePreference}).Dark(systemDark)
}
func (s *application) setHistoryRetention(days int) {
	if s.quitting {
		return
	}
	if !settings.ValidRetentionDays(days) || s.retentionDays == days {
		return
	}
	s.preferenceGeneration++
	generation := s.preferenceGeneration
	previous := s.retentionDays
	s.retentionDays = days
	p := s.preferenceSnapshot()
	saved := false
	s.perform(func() error {
		if s.config.SavePreferences != nil {
			if err := s.config.SavePreferences(p); err != nil {
				return err
			}
		}
		saved = true
		s.store.SetRetention(days)
		_, err := s.store.Expire(backgroundContext(), time.Now())
		return err
	}, func(err error) {
		if err != nil {
			// A saved preference can outlive a failed expiry; the service keeps all
			// records until its next successful transaction.
			log.Printf("save history retention: %v", err)
			if !saved && generation == s.preferenceGeneration && s.retentionDays == days {
				s.retentionDays = previous
			}
			s.reportNativeError("save history retention", err, "Could not save retention")
		}
		s.Changed()
	})
}
