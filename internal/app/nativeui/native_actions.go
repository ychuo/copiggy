package nativeui

import (
	"log"
	"time"

	"copiggy/internal/clipmodel"
	macos "copiggy/internal/macos/appkit"
)

func (s *application) publish() {
	s.syncNativeTray(time.Now())
	if s.panel != nil {
		s.panel.Invalidate()
	}
	if s.popover != nil {
		s.popover.Invalidate()
	}
}
func (s *application) rememberSource(item clipmodel.Item) {
	// Capture may return an older duplicate. Keep that record's provenance,
	// including an unknown source, instead of assigning the latest copier.
	id, source := item.ID, item.SourceAppID
	if source == "" {
		return
	}
	if existing := s.itemSources[id]; existing != "" {
		source = existing
	} else {
		s.itemSources[id] = source
	}
	s.appNames[source] = item.SourceApp
	if _, exists := s.appIcons[source]; !exists {
		s.appIcons[source] = nil
		if png, err := macos.AppIconPNG(source); err == nil {
			s.appIcons[source] = png
		}
	}
}
func (s *application) relativeTime(t time.Time) string {
	elapsed := time.Since(t)
	if elapsed < time.Minute {
		return s.tr("Now")
	}
	if elapsed < time.Hour {
		return t.Format("15:04")
	}
	return t.Format("01/02 15:04")
}
func (s *application) cancelNativeDraft() {
	s.uiGeneration++
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.busy = false
	if s.draft != nil {
		id := s.draft.ID
		s.perform(func() error { s.store.Cancel(id); return nil }, func(error) {})
		s.draft = nil
	}
	if s.viewState != nil {
		s.viewState.Original = false
		s.viewState.Zoom = false
	}
}
func (s *application) selectNative(index int, keepPreview bool) {
	if index < 0 || index >= len(s.results) || index == s.chosen {
		return
	}
	s.cancelNativeDraft()
	s.chosen = index
	s.preview = keepPreview
	s.message = ""
	s.publish()
}
func (s *application) closeNativePreview() {
	s.reportNativeHint("")
	s.cancelNativeDraft()
	s.preview = false
	s.message = ""
	s.destroyNativeLinkPreview()
	if s.popover != nil {
		s.popover.Hide()
	}
	if s.viewState != nil {
		s.viewState.Focus.Focus()
	}
	s.publish()
}
func (s *application) chooseNative(original bool) {
	if s.busy {
		return
	}
	if original {
		s.cancelNativeDraft()
	}
	s.choose()
	s.publish()
}
func (s *application) toggleNativeGuard() {
	if s.quitting {
		return
	}
	if s.native == nil || s.config.SetGuard == nil {
		return
	}
	s.guardPreference = !s.native.Enabled()
	err := s.config.SetGuard(s.guardPreference)
	if err != nil {
		s.reportNativeError("start paste guard", err, "Could not start Paste Guard")
	}
	s.savePreferences(func(err error) {
		if err != nil {
			s.reportNativeError("save paste guard preference", err, "Paste Guard changed · Not saved")
		}
		s.publish()
	})
	s.expireCopyFeedback()
	s.publish()
}
func (s *application) toggleNativeMonitoring() {
	if s.quitting {
		return
	}
	s.monitoring = !s.monitoring
	if s.config.SetMonitoring != nil {
		s.config.SetMonitoring(s.monitoring)
	}
	s.savePreferences(func(err error) {
		if err != nil {
			s.reportNativeError("save recording preference", err, "Recording changed · Not saved")
		}
		s.publish()
	})
	s.publish()
}
func (s *application) switchNativeTheme() {
	s.applyNativeTheme(!s.nativeState().Dark)
}
func (s *application) applyNativeTheme(dark bool) {
	if s.quitting {
		return
	}
	s.nativeState().Dark = dark
	status := "Light appearance"
	s.themePreference = "light"
	if dark {
		status = "Dark appearance"
		s.themePreference = "dark"
	}
	s.savePreferences(func(err error) {
		if err != nil {
			s.reportNativeError("save preferences", err, "Appearance changed · Not saved")
		}
		s.publish()
	})
	s.reportNativeHint("")
	s.reportNativeStatus(s.tr(status))
	s.publish()
}

// The system glass owns the MyGo native surface, including its real content.
func (s *application) syncNativeGlass() {
	if s.panel == nil {
		return
	}
	material, err := macos.SetTrayGlass(s.panel.NativeHandle(), s.nativeState().Dark, 28)
	if err != nil {
		if s.glassError != err.Error() {
			log.Printf("native tray material: %v", err)
			s.glassError = err.Error()
		}
		return
	}
	s.glassError = ""
	if material != s.glassMaterial {
		s.glassMaterial = material
		log.Printf("native tray material: %s", material)
	}
}
