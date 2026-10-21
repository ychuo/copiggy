//go:build darwin

package appkit

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestStatusMenuAnchorUsesPrimaryScreenCoordinates(t *testing.T) {
	for _, test := range []struct {
		name   string
		anchor mygo.Rectangle
		want   glassPoint
	}{
		{"primary", mygo.Rectangle{X: 1100, Y: 0, Width: 22, Height: 24}, glassPoint{1100, 1052}},
		{"display left", mygo.Rectangle{X: -500, Y: 180, Width: 22, Height: 24}, glassPoint{-500, 872}},
		{"display above", mygo.Rectangle{X: 500, Y: -900, Width: 22, Height: 24}, glassPoint{500, 1952}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := statusMenuAnchor(test.anchor, 1080); got != test.want {
				t.Fatalf("anchor = %+v, want %+v", got, test.want)
			}
		})
	}
}

// Invalid menus must fail before dispatching to the application loop. These
// checks do not construct a native menu, inspect displays or open any window.
func TestStatusMenuRejectsInvalidInputBeforeDispatch(t *testing.T) {
	anchor := mygo.Rectangle{X: 100, Width: 24, Height: 24}
	for _, entries := range [][]MenuEntry{
		nil,
		{{Label: "Missing action"}},
		{{ID: "one", Label: "One"}, {ID: "one", Label: "Duplicate"}},
		{{ID: "one", Label: "One", Key: "Ctrl+Space"}},
		{{ID: "one", Label: "One", Modifiers: 1}},
		{{ID: "one", Label: "Broken\x00label"}},
		{{ID: "one", Label: "Broken\xfflabel"}},
	} {
		if _, err := PopupStatusMenu(entries, false, anchor); err == nil {
			t.Fatalf("invalid menu accepted: %+v", entries)
		}
	}
	if _, err := PopupStatusMenu([]MenuEntry{{ID: "one", Label: "One"}}, false, mygo.Rectangle{}); err == nil {
		t.Fatal("missing anchor was accepted")
	}
}

func TestStatusMenuAcceptsGroupedLocalizedSettings(t *testing.T) {
	entries := []MenuEntry{
		{ID: "open", Label: "Open Clipboard", Symbol: "doc.on.clipboard", Key: " ", Modifiers: MenuControl | MenuOption | MenuShift},
		{Separator: true},
		{ID: "guard", Label: "Paste Guard", Symbol: "checkmark.shield", Checked: true},
		{ID: "language", Label: "Language", Symbol: "globe", Submenu: []MenuEntry{
			{ID: "en", Label: "English", Checked: true},
			{ID: "zh", Label: "简体中文"},
			{ID: "ja", Label: "日本語"},
		}},
		{Label: "Pinned items stay", Disabled: true},
	}
	if err := validateStatusMenu(entries, mygo.Rectangle{X: 100, Width: 24, Height: 24}); err != nil {
		t.Fatal(err)
	}
}
