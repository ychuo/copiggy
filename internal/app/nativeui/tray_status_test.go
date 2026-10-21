package nativeui

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"copiggy/internal/macos/guard"
	"copiggy/internal/macos/pasteboard"
	"github.com/egoist/mygo"
)

func TestTrayHoverHintsStaySilent(t *testing.T) {
	for _, language := range supportedLanguages {
		s := &application{language: language.Code}
		s.reportNativeHint(s.tr("Search · ⌘F"))
		if s.trayStatus.title(time.Now()) != "" {
			t.Fatal("hover changed the menu-bar title")
		}
		s.reportNativeStatus(s.tr("Paste blocked"))
		until := s.trayStatus.eventUntil
		s.reportNativeHint(s.tr("Format JSON"))
		s.reportNativeHint("")
		if s.trayStatus.title(time.Now()) != s.tr("Paste blocked") || s.trayStatus.eventUntil != until {
			t.Fatal("hover changed an important event or its expiry")
		}
	}
}

func TestTrayImportantEventsOwnTheirExpiry(t *testing.T) {
	var state trayStatusState
	now := time.Unix(100, 0)
	state.reportEvent("Paste blocked", now)
	if state.title(now.Add(2500*time.Millisecond)) != "Paste blocked" {
		t.Fatal("important event disappeared early")
	}
	state.reportEvent("Paste blocked", now.Add(2500*time.Millisecond))
	if state.title(now.Add(4*time.Second)) != "Paste blocked" {
		t.Fatal("the first expiry cleared a repeated, newer event")
	}
	if state.title(now.Add(6*time.Second)) != "" {
		t.Fatal("event text did not return to the icon-only state")
	}
}

func TestRoutineStatusesNeverReachMenuBar(t *testing.T) {
	for _, language := range supportedLanguages {
		for _, key := range []string{
			"Copied", "Pinned", "Unpinned", "Deleted", "Recording paused", "Recording resumed",
			"Dark appearance", "Light appearance", "Language changed", "Retention updated", "Cancelled",
		} {
			s := &application{language: language.Code}
			text := s.tr(key)
			s.reportNativeStatus(text)
			if s.message != text || s.trayStatus.title(time.Now()) != "" {
				t.Fatalf("routine %s result was announced in %s", key, language.Code)
			}
			// A subsequent sync or a language switch cannot resurrect a quiet result.
			s.trayStatus.messageSeen = ""
			s.language = "en"
			s.syncNativeTray(time.Now())
			if s.trayStatus.title(time.Now()) != "" {
				t.Fatalf("sync announced localized routine result %q", text)
			}
		}
	}
}

func TestRoutineStatusesDoNotReplaceOrRenewImportantEvents(t *testing.T) {
	s := &application{}
	s.reportNativeStatus("Paste blocked")
	until := s.trayStatus.eventUntil
	s.reportNativeStatus("Copied")
	if s.trayStatus.title(time.Now()) != "Paste blocked" || s.trayStatus.eventUntil != until {
		t.Fatal("ordinary success replaced or renewed important feedback")
	}
	s.syncNativeTray(until.Add(time.Second))
	if s.trayStatus.title(until.Add(time.Second)) != "" {
		t.Fatal("quiet result kept an old important event visible")
	}
}

func TestImportantStatusesAndNativeEventsStayVisibleInEveryLanguage(t *testing.T) {
	for _, language := range supportedLanguages {
		s := &application{language: language.Code}
		for _, key := range []string{"Paste Guard on", "Paste Guard off", "Allow Accessibility access", "Could not save history", "Pin limit reached", "Language changed · Not saved"} {
			s.reportNativeStatus(s.tr(key))
			if s.trayStatus.title(time.Now()) != compactTrayText(s.tr(key)) {
				t.Fatalf("important %s result was silenced in %s", key, language.Code)
			}
		}
		s.reportNativeError("test copy", errors.New("raw platform details"), "Could not copy")
		if s.trayStatus.title(time.Now()) != s.tr("Could not copy") {
			t.Fatal("typed error report was silenced or exposed raw diagnostics")
		}
		s.reportNativeEvent(guard.Event{Kind: "copy-protected", Target: pasteboard.App{Name: "Copied"}})
		if s.trayStatus.title(time.Now()) != s.tr("Only in %s", "Copied") || s.trayStatus.eventKind != "copy-protected" {
			t.Fatal("formatted protected-copy result lost its semantic identity")
		}
		until := s.trayStatus.eventUntil
		s.reportNativeEvent(guard.Event{Kind: "copy-pending", Target: pasteboard.App{Name: "TextEdit"}})
		if s.trayStatus.eventUntil != until || s.trayStatus.eventKind != "copy-protected" {
			t.Fatal("copy progress replaced the important result")
		}
	}
	for _, text := range []string{"", "Could not copy: arbitrary raw details", "Copied safely", "unknown status"} {
		if importantTrayStatus(text) {
			t.Fatalf("policy guessed importance from message wording: %q", text)
		}
	}
}

func TestTraySyncDoesNotRepublishAnUnchangedMessage(t *testing.T) {
	s := &application{language: "zh-Hans", message: "Copy the image again"}
	now := time.Unix(100, 0)
	s.syncNativeTray(now)
	if got := s.trayStatus.title(now); got != "图片不可用，请重新复制" {
		t.Fatalf("message was not reported as a compact status: %q", got)
	}
	s.syncNativeTray(now.Add(4 * time.Second))
	if got := s.trayStatus.title(now.Add(4 * time.Second)); got != "" {
		t.Fatalf("polling renewed an unchanged error forever: %q", got)
	}
	s.message = ""
	s.syncNativeTray(now.Add(5 * time.Second))
	s.message = "Copy the image again"
	s.syncNativeTray(now.Add(6 * time.Second))
	if s.trayStatus.title(now.Add(6*time.Second)) == "" {
		t.Fatal("a new occurrence after clearing the error was suppressed")
	}
}

func TestTrayMenuReflectsMonitoringAndExplicitAppearance(t *testing.T) {
	s := &application{language: "zh-Hans", monitoring: true, viewState: &nativePanelState{Dark: false}}
	menu := s.nativeTrayMenu()
	if menu.ItemByID("monitor").Label != "暂停记录" || !menu.ItemByID("light").Checked || menu.ItemByID("dark").Checked {
		t.Fatal("menu did not reflect recording and light appearance")
	}
	if item := menu.ItemByID("guard"); item.Type != mygo.MenuItemCheckbox || item.Checked {
		t.Fatal("a missing native guard was falsely shown as enabled")
	}
	menu.ItemByID("dark").Click(nil, nil)
	if !s.viewState.Dark || s.message != "已切换深色" {
		t.Fatal("explicit dark menu action did not update local appearance")
	}
	s.monitoring = false
	menu = s.nativeTrayMenu()
	if menu.ItemByID("monitor").Label != "继续记录" || !menu.ItemByID("dark").Checked || menu.ItemByID("light").Checked {
		t.Fatal("reopened menu kept stale monitoring/appearance values")
	}
	menu.ItemByID("light").Click(nil, nil)
	if s.viewState.Dark || s.message != "已切换浅色" {
		t.Fatal("explicit light menu action did not update local appearance")
	}
}

func TestTrayTemplateIconsAreTransparentAndGuardStatesDiffer(t *testing.T) {
	plain, protected := clipboardTrayIcon(false), clipboardTrayIcon(true)
	if bytes.Equal(plain, protected) {
		t.Fatal("guard state did not change the menu-bar icon")
	}
	for _, encoded := range [][]byte{plain, protected} {
		img, err := png.Decode(bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 72 || img.Bounds().Dy() != 72 {
			t.Fatal("template icon is not a square high-resolution image")
		}
		ink, transparent := 0, 0
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				r, g, b, a := img.At(x, y).RGBA()
				if r != 0 || g != 0 || b != 0 {
					t.Fatal("template image contains colored pixels")
				}
				if a > 0 {
					ink++
				} else {
					transparent++
				}
			}
		}
		if ink < 200 || transparent < 1000 {
			t.Fatal("template has no meaningful pig silhouette or transparency")
		}
	}
}

func TestTrayStatusMessagesStayShortAndReadable(t *testing.T) {
	for _, language := range supportedLanguages {
		s := &application{language: language.Code}
		for _, key := range []string{
			"Clipboard changed · Try again", "Text exceeds 1 MiB · Not recorded",
			"Image exceeds 24 MP · Not recorded", "Content exceeds 16 MiB · Not recorded",
			"Could not save history", "Could not load history", "Retention updated",
			"Could not save retention", "Recording changed · Not saved", "Paste Guard changed · Not saved",
		} {
			message := s.tr(key)
			if compactTrayText(message) != message {
				t.Fatalf("%s truncates the status message %q", language.Code, message)
			}
		}
		for _, event := range []guard.Event{
			{Kind: "enabled"}, {Kind: "stopped"}, {Kind: "permission-required"},
			{Kind: "disabled-timeout"}, {Kind: "disabled-user"}, {Kind: "blocked"},
			{Kind: "copy-protected", Target: pasteboard.App{Name: "Visual Studio Code"}}, {Kind: "copy-failed"},
		} {
			message := compactTrayText(s.nativeEventStatus(event))
			if message == "" || !utf8.ValidString(message) || utf8.RuneCountInString(message) > 41 || strings.ContainsAny(message, "\r\n") {
				t.Fatalf("event produced an invalid menu-bar status: %+v: %q", event, message)
			}
		}
	}
	if compactTrayText("第一行\n第二行\t第三行") != "第一行 第二行 第三行" {
		t.Fatal("multiline message was not normalized for a single-line menu bar")
	}
}
