package nativeui

import (
	"log"

	macos "copiggy/internal/macos/appkit"
	"github.com/egoist/mygo"
)

// Keep actions and localized labels in one menu model. AppKit adds its image
// column and key equivalents; selection invokes the existing action only after
// native menu tracking has ended.
func (s *application) showNativeTrayMenu() {
	s.hide()
	menu := s.nativeTrayMenu()
	selected, err := macos.PopupStatusMenu(s.nativeMenuEntries(menu.Items()), s.nativeState().Dark, s.tray.Bounds())
	if err != nil {
		log.Printf("native settings menu: %v", err)
		s.tray.SetMenu(menu)
		defer s.tray.SetMenu(nil)
		s.tray.PopUpMenu()
		return
	}
	if item := menu.ItemByID(selected); item != nil && !item.Disabled && !item.Hidden && item.Click != nil {
		item.Click(item, nil)
	}
}

func (s *application) nativeMenuEntries(items []*mygo.MenuItem) []macos.MenuEntry {
	symbols := map[string]string{
		"open":         "clipboard",
		"monitor":      "pause",
		"guard":        "shield",
		"retention":    "clock",
		"appearance":   "circle.lefthalf.filled",
		"light":        "sun.max",
		"dark":         "moon",
		"language":     "globe",
		"phone-region": "phone",
		"quit":         "power",
	}
	if !s.monitoring {
		symbols["monitor"] = "play"
	}
	entries := make([]macos.MenuEntry, 0, len(items))
	for _, item := range items {
		if item.Hidden {
			continue
		}
		entry := macos.MenuEntry{
			ID: item.ID, Label: item.Label, Symbol: symbols[item.ID],
			Disabled: item.Disabled, Checked: item.Checked,
			Separator: item.Type == mygo.MenuItemSeparator,
			Submenu:   s.nativeMenuEntries(item.Submenu),
		}
		if item.ID == "guard" && item.Checked {
			entry.Symbol = "checkmark.shield"
		}
		switch item.Accelerator {
		case summonShortcut:
			entry.Key = " "
			entry.Modifiers = macos.MenuControl | macos.MenuOption | macos.MenuShift
		case "Cmd+Q":
			entry.Key = "q"
			entry.Modifiers = macos.MenuCommand
		}
		entries = append(entries, entry)
	}
	return entries
}
