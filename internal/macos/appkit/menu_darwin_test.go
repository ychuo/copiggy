//go:build darwin

package appkit

import (
	"strconv"
	"testing"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// Construct menu objects only: no menu is shown, no event is posted, and the
// target only records a selection instead of running an application action.
func TestStatusMenuSpaceGlyphPreservesNativeShortcut(t *testing.T) {
	if err := initStatusMenuNative(); err != nil {
		t.Fatal(err)
	}
	nativeobjc.WithPool(func() {
		target := nativeobjc.Send(nativeobjc.Send(uintptr(statusMenuNative.targetClass), objc.RegisterName("alloc")), objc.RegisterName("init"))
		defer nativeobjc.Send(target, objc.RegisterName("release"))
		selection := &statusMenuSelection{ids: make(map[uintptr]string)}
		statusMenuSelections[target] = selection
		defer delete(statusMenuSelections, target)
		modifiers := MenuControl | MenuOption | MenuShift
		menu, err := buildStatusMenu([]MenuEntry{
			{ID: "open", Label: "Open Clipboard", Symbol: "clipboard", Key: " ", Modifiers: modifiers},
			{ID: "quit", Label: "Quit Copiggy", Symbol: "power", Key: "q", Modifiers: MenuCommand},
		}, "", 0, target, selection)
		if err != nil {
			t.Fatal(err)
		}
		defer nativeobjc.Send(menu, objc.RegisterName("release"))
		var visible, shortcut, quit uintptr
		for i, count := uintptr(0), nativeobjc.Send(menu, objc.RegisterName("numberOfItems")); i < count; i++ {
			item := nativeobjc.Send(menu, objc.RegisterName("itemAtIndex:"), i)
			key := nativeobjc.String(nativeobjc.Send(item, objc.RegisterName("keyEquivalent")))
			switch key {
			case "␣":
				visible = item
			case " ":
				shortcut = item
			case "q":
				quit = item
			}
		}
		if visible == 0 || shortcut == 0 || quit == 0 {
			t.Fatalf("Space must have a visible glyph and real native equivalent; visible=%v shortcut=%v quit=%v", visible != 0, shortcut != 0, quit != 0)
		}
		for _, item := range []uintptr{visible, shortcut, quit} {
			if nativeobjc.Send(item, objc.RegisterName("image")) == 0 {
				t.Fatal("native menu item lost its system symbol")
			}
		}
		if nativeobjc.Send(visible, objc.RegisterName("isHidden")) != 0 ||
			nativeobjc.Send(visible, objc.RegisterName("allowsAutomaticKeyEquivalentLocalization")) != 0 {
			t.Fatal("Space glyph must remain visible and must not be remapped by keyboard layout")
		}
		if nativeobjc.Send(shortcut, objc.RegisterName("isHidden")) == 0 ||
			nativeobjc.Send(shortcut, objc.RegisterName("allowsKeyEquivalentWhenHidden")) == 0 {
			t.Fatal("real Space equivalent must work without creating a duplicate visible row")
		}
		for _, item := range []uintptr{visible, shortcut} {
			if got := nativeobjc.Send(item, objc.RegisterName("keyEquivalentModifierMask")); got != uintptr(modifiers) {
				t.Fatalf("Space modifiers = %x, want %x", got, modifiers)
			}
			if got := nativeobjc.String(nativeobjc.Send(item, objc.RegisterName("title"))); got != "Open Clipboard" {
				t.Fatalf("shortcut was appended to title: %q", got)
			}
			selection.selected = ""
			action := objc.SEL(nativeobjc.Send(item, objc.RegisterName("action")))
			nativeobjc.Send(nativeobjc.Send(item, objc.RegisterName("target")), action, item)
			if selection.selected != "open" {
				t.Fatalf("Space item dispatched %q, want open", selection.selected)
			}
		}
		if nativeobjc.Send(quit, objc.RegisterName("isHidden")) != 0 ||
			nativeobjc.Send(quit, objc.RegisterName("keyEquivalentModifierMask")) != uintptr(MenuCommand) {
			t.Fatal("ordinary command shortcut changed")
		}
	})
}

func TestStatusMenuDisabledSpaceCannotDispatch(t *testing.T) {
	if err := initStatusMenuNative(); err != nil {
		t.Fatal(err)
	}
	nativeobjc.WithPool(func() {
		selection := &statusMenuSelection{ids: make(map[uintptr]string)}
		menu, err := buildStatusMenu([]MenuEntry{
			{ID: "open", Label: "Open Clipboard", Key: " ", Modifiers: MenuControl | MenuOption | MenuShift, Disabled: true},
		}, "", 0, 0, selection)
		if err != nil {
			t.Fatal(err)
		}
		defer nativeobjc.Send(menu, objc.RegisterName("release"))
		if count := nativeobjc.Send(menu, objc.RegisterName("numberOfItems")); count != 2 {
			t.Fatalf("expected visible Space glyph and hidden shortcut, got %d items", count)
		}
		for i := uintptr(0); i < 2; i++ {
			item := nativeobjc.Send(menu, objc.RegisterName("itemAtIndex:"), i)
			if nativeobjc.Send(item, objc.RegisterName("isEnabled")) != 0 || nativeobjc.Send(item, objc.RegisterName("action")) != 0 {
				t.Fatal("disabled Space item or its shortcut can still dispatch")
			}
		}
	})
}

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
	if err := ValidateStatusMenu(entries, mygo.Rectangle{X: 100, Width: 24, Height: 24}); err != nil {
		t.Fatal(err)
	}
}

func TestStatusMenuBoundsTheCompleteTree(t *testing.T) {
	// Nested items and separators both consume capacity. A large submenu must
	// not bypass the limit, while a complete tree at the limit remains valid.
	children := make([]MenuEntry, maxStatusMenuEntries)
	for i := range children {
		children[i] = MenuEntry{ID: "choice-" + strconv.Itoa(i), Label: "Choice"}
	}
	children[0] = MenuEntry{Separator: true}
	entries := []MenuEntry{{ID: "choices", Label: "Choices", Submenu: children[:maxStatusMenuEntries-1]}}
	anchor := mygo.Rectangle{X: 100, Width: 24, Height: 24}
	if err := ValidateStatusMenu(entries, anchor); err != nil {
		t.Fatalf("menu at the tree limit was rejected: %v", err)
	}
	entries[0].Submenu = children
	if err := ValidateStatusMenu(entries, anchor); err == nil {
		t.Fatal("oversized nested menu was accepted")
	}
}
