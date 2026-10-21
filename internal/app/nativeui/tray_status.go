package nativeui

import (
	"errors"
	"log"
	"strings"
	"time"
	"unicode"

	"copiggy/internal/clipmodel"
	macos "copiggy/internal/macos/guard"
	"copiggy/internal/macos/pasteboard"
	"github.com/egoist/mygo"
)

const trayEventDuration = 3 * time.Second

// UI-thread state. Only guard events and actionable failures enter the menu bar.
// Repeated important events own their expiry; routine actions never replace them.
type trayStatusState struct {
	event, eventKind        string
	eventUntil              time.Time
	messageSeen, shownTitle string
	iconSet, iconGuard      bool
}

func (state *trayStatusState) reportEvent(text string, now time.Time) {
	state.event = compactTrayText(text)
	state.eventKind = ""
	if state.event == "" {
		state.eventUntil = time.Time{}
	} else {
		state.eventUntil = now.Add(trayEventDuration)
	}
}

func (state *trayStatusState) title(now time.Time) string {
	if now.Before(state.eventUntil) {
		return state.event
	}
	return ""
}

// Native controls retain their accessibility labels, but hovering or focusing
// them no longer changes the menu-bar title or opens any separate hint surface.
func (s *application) reportNativeHint(_ string) {}

// Existing callers may pass an English source key or an already-localized label.
// Match exact catalog entries only; wording prefixes and translated fragments
// must never decide whether a message is important.
func importantTrayStatus(text string) bool {
	for _, key := range []string{
		"Paste Guard on", "Paste Guard off", "Paste Guard is off",
		"Paste Guard changed · Not saved", "Allow Accessibility access",
		"Could not start Paste Guard", "Restart Paste Guard", "Paste blocked",
		"Source app only", "Protected copy failed", "Shortcut unavailable",
		"Could not copy", "Copy the image again", "Could not enhance content",
		"Clipboard changed · Try again", "Content changed · Try again",
		"Item unavailable · Reselect", "Could not open browser",
		"Could not save pin", "Could not load pins", "Pin limit reached",
		"Could not delete item", "Could not save history", "Could not load history",
		"Could not save retention", "Recording changed · Not saved",
		"Language changed · Not saved", "Appearance changed · Not saved",
		"Clipboard content unavailable", "Image data is incomplete",
		"Unsupported clipboard content", "Text exceeds 1 MiB · Not recorded",
		"Image exceeds 24 MP · Not recorded", "Content exceeds 16 MiB · Not recorded",
	} {
		if text == key {
			return true
		}
		for _, translated := range translations[key] {
			if text != "" && text == translated {
				return true
			}
		}
	}
	return false
}

// Keep the latest result for local state without announcing every successful
// action. Silent results must not clear a still-visible guard event or failure.
func (s *application) reportNativeStatus(text string) {
	s.reportNativeStatusWithPolicy(text, importantTrayStatus(text))
}

func (s *application) reportNativeStatusWithPolicy(text string, important bool) {
	now := time.Now()
	s.message = text
	s.trayStatus.messageSeen = text
	if important && text != "" {
		s.trayStatus.reportEvent(s.compactNativeStatus(text), now)
	}
	s.syncNativeTray(now)
}

// Native events carry their identity separately from translated, formatted text.
// In particular an application name in a protected-copy result is not parsed.
func (s *application) reportNativeEvent(event macos.Event) {
	important := false
	switch event.Kind {
	case "enabled", "stopped", "permission-required", "disabled-timeout", "disabled-user", "blocked", "copy-protected", "copy-failed":
		important = true
	}
	s.reportNativeStatusWithPolicy(s.nativeEventStatus(event), important)
	if important {
		s.trayStatus.eventKind = event.Kind
	}
}

func (s *application) syncNativeTray(now time.Time) {
	state := &s.trayStatus
	if s.message != state.messageSeen {
		state.messageSeen = s.message
		if importantTrayStatus(s.message) {
			state.reportEvent(s.compactNativeStatus(s.message), now)
		}
	}
	if s.tray == nil {
		return
	}
	guard := s.native != nil && s.native.Enabled()
	if !state.iconSet || state.iconGuard != guard {
		if err := s.tray.SetIcon(clipboardTrayIcon(guard), true); err != nil {
			log.Printf("menu bar icon: %v", err)
		} else {
			state.iconSet, state.iconGuard = true, guard
		}
	}
	title := state.title(now)
	if title != state.shownTitle {
		s.tray.SetTitle(title)
		state.shownTitle = title
	}
}

// Build when opening so actual monitoring/appearance/guard states are reflected
// even after a system event disabled the guard while the panel was hidden.
func (s *application) nativeTrayMenu() *mygo.Menu {
	monitorLabel := s.tr("Resume Recording")
	if s.monitoring {
		monitorLabel = s.tr("Pause Recording")
	}
	dark := s.nativeState().Dark
	guard := s.native != nil && s.native.Enabled()
	languages := make([]*mygo.MenuItem, 0, len(supportedLanguages))
	for _, language := range supportedLanguages {
		languages = append(languages, &mygo.MenuItem{
			ID: "language-" + language.Code, Label: language.Name,
			Type: mygo.MenuItemRadio, Checked: s.currentLanguage() == language.Code,
			Click: func(*mygo.MenuItem, *mygo.Window) { s.setLanguage(language.Code) },
		})
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{ID: "open", Label: s.tr("Open Clipboard"), Accelerator: summonShortcut, Click: func(*mygo.MenuItem, *mygo.Window) { s.show() }},
		mygo.Separator(),
		{ID: "monitor", Label: monitorLabel, Click: func(*mygo.MenuItem, *mygo.Window) { s.toggleNativeMonitoring() }},
		{ID: "guard", Label: s.tr("Paste Guard"), Type: mygo.MenuItemCheckbox, Checked: guard, Click: func(*mygo.MenuItem, *mygo.Window) { s.toggleNativeGuard() }},
		{ID: "retention", Label: s.tr("Keep History"), Submenu: []*mygo.MenuItem{
			{ID: "retention-1", Label: s.tr("1 Day"), Type: mygo.MenuItemRadio, Checked: s.retentionDays == 1, Click: func(*mygo.MenuItem, *mygo.Window) { s.setHistoryRetention(1) }},
			{ID: "retention-7", Label: s.tr("7 Days"), Type: mygo.MenuItemRadio, Checked: s.retentionDays == 7, Click: func(*mygo.MenuItem, *mygo.Window) { s.setHistoryRetention(7) }},
			{ID: "retention-30", Label: s.tr("30 Days"), Type: mygo.MenuItemRadio, Checked: s.retentionDays == 30, Click: func(*mygo.MenuItem, *mygo.Window) { s.setHistoryRetention(30) }},
			{ID: "retention-0", Label: s.tr("Forever"), Type: mygo.MenuItemRadio, Checked: s.retentionDays == 0, Click: func(*mygo.MenuItem, *mygo.Window) { s.setHistoryRetention(0) }},
			{ID: "retention-pins", Label: s.tr("Pinned items stay"), Disabled: true},
		}},
		mygo.Separator(),
		{ID: "appearance", Label: s.tr("Appearance"), Submenu: []*mygo.MenuItem{
			{ID: "light", Label: s.tr("Light"), Type: mygo.MenuItemRadio, Checked: !dark, Click: func(*mygo.MenuItem, *mygo.Window) { s.applyNativeTheme(false) }},
			{ID: "dark", Label: s.tr("Dark"), Type: mygo.MenuItemRadio, Checked: dark, Click: func(*mygo.MenuItem, *mygo.Window) { s.applyNativeTheme(true) }},
		}},
		{ID: "language", Label: s.tr("Language"), Submenu: languages},
		mygo.Separator(),
		{ID: "quit", Label: s.tr("Quit Copiggy"), Accelerator: "Cmd+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}

// Localize source-key statuses; formatted/localized action results remain intact.
func (s *application) compactNativeStatus(text string) string {
	if _, known := translations[text]; known {
		return s.tr(text)
	}
	return text
}

// Keep diagnostics out of the menu bar. Error identities, never translated
// prefixes, select the concise product message in the current language.
func (s *application) reportNativeError(operation string, err error, fallback string) {
	log.Printf("%s: %v", operation, err)
	s.reportNativeStatusWithPolicy(s.nativeErrorStatus(err, fallback), true)
}

func (s *application) nativeErrorStatus(err error, fallback string) string {
	var captureErr clipmodel.ContentError
	switch {
	case errors.Is(err, macos.ErrAccessibility):
		return s.tr("Allow Accessibility access")
	case errors.Is(err, macos.ErrGuardUnavailable):
		return s.tr("Could not start Paste Guard")
	case errors.Is(err, pasteboard.ErrChanged), errors.Is(err, macos.ErrCopyVersionChanged):
		return s.tr("Clipboard changed · Try again")
	case errors.Is(err, clipmodel.ErrDraftStale):
		return s.tr("Content changed · Try again")
	case errors.Is(err, clipmodel.ErrItemNotFound), errors.Is(err, clipmodel.ErrDraftNotFound):
		return s.tr("Item unavailable · Reselect")
	case errors.Is(err, clipmodel.ErrUnsupportedContent), errors.Is(err, clipmodel.ErrUnknownEnhancement):
		return s.tr("Could not enhance content")
	case errors.Is(err, macos.ErrCopyGuardDisabled):
		return s.tr("Paste Guard is off")
	case errors.Is(err, macos.ErrCopyExpired), errors.Is(err, macos.ErrCopyStale), errors.Is(err, macos.ErrCopySourceChanged), errors.Is(err, macos.ErrCopyUnchanged):
		return s.tr("Protected copy failed")
	case errors.As(err, &captureErr):
		return s.tr(string(captureErr))
	default:
		return s.tr(fallback)
	}
}

func compactTrayText(text string) string {
	text = strings.Join(strings.FieldsFunc(text, unicode.IsSpace), " ")
	// Keep menu-bar messages compact without cutting English at a CJK-sized
	// character limit. CJK glyphs occupy roughly twice the horizontal space.
	width := 0
	for index, r := range text {
		advance := 1
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			advance = 2
		}
		if width+advance > 40 {
			return strings.TrimSpace(text[:index]) + "…"
		}
		width += advance
	}
	return text
}

func (s *application) nativeEventStatus(event macos.Event) string {
	switch event.Kind {
	case "enabled":
		return s.tr("Paste Guard on")
	case "stopped":
		return s.tr("Paste Guard off")
	case "permission-required":
		return s.tr("Allow Accessibility access")
	case "disabled-timeout", "disabled-user":
		return s.tr("Restart Paste Guard")
	case "blocked":
		return s.tr("Paste blocked")
	case "copy-protected":
		if event.Target.Name != "" {
			return s.tr("Only in %s", event.Target.Name)
		}
		return s.tr("Source app only")
	case "copy-failed":
		return s.tr("Protected copy failed")
	case "copy-pending":
		return s.tr("Copying for %s", event.Target.Name)
	default:
		log.Printf("native event %s: %s", event.Kind, event.Message)
		return ""
	}
}
