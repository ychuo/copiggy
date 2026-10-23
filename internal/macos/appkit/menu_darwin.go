//go:build darwin

package appkit

import (
	nativeobjc "copiggy/internal/macos/internal/objc"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// MenuModifiers use AppKit's native key-equivalent modifier masks.
type MenuModifiers uint

const (
	MenuShift   MenuModifiers = 1 << 17
	MenuControl MenuModifiers = 1 << 18
	MenuOption  MenuModifiers = 1 << 19
	MenuCommand MenuModifiers = 1 << 20
)

// MenuEntry describes a system menu row. Key is one native key-equivalent
// character (for example, " " for Space), rather than an accelerator string.
// Symbol names an optional SF Symbol; unavailable symbols are simply omitted.
type MenuEntry struct {
	ID, Label, Symbol, Key       string
	Modifiers                    MenuModifiers
	Disabled, Checked, Separator bool
	Submenu                      []MenuEntry
}

type statusMenuSelection struct {
	ids      map[uintptr]string
	selected string
}

var statusMenuNative struct {
	once          sync.Once
	err           error
	targetClass   objc.Class
	popup         func(uintptr, objc.SEL, uintptr, glassPoint, uintptr) bool
	configuration func(uintptr, objc.SEL, float64, float64) uintptr
}

// Main-thread-only sessions. The native target contains no Go pointers, and
// its sole callback records a selection without running application actions
// inside AppKit's nested menu-tracking event loop.
var statusMenuSelections = make(map[uintptr]*statusMenuSelection)

// Bound the complete tree while leaving room for country-sized submenus
// alongside the application's settings and controls.
const maxStatusMenuEntries = 1024

func initStatusMenuNative() error {
	if err := initGlassNative(); err != nil {
		return err
	}
	statusMenuNative.once.Do(func() {
		defer func() {
			if failure := recover(); failure != nil {
				statusMenuNative.err = fmt.Errorf("native status menu bridge: %v", failure)
			}
		}()
		purego.RegisterFunc(&statusMenuNative.popup, nativeobjc.MessageSend())
		purego.RegisterFunc(&statusMenuNative.configuration, nativeobjc.MessageSend())
		statusMenuNative.targetClass, statusMenuNative.err = objc.RegisterClass(
			"CopiggyStatusMenuTarget", objc.GetClass("NSObject"), nil, nil,
			[]objc.MethodDef{{
				Cmd: objc.RegisterName("copiggyMenuItemSelected:"),
				Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) {
					if selection := statusMenuSelections[uintptr(self)]; selection != nil {
						tag := nativeobjc.Send(uintptr(sender), objc.RegisterName("tag"))
						if id := selection.ids[tag]; id != "" {
							selection.selected = id
						}
					}
				},
			}},
		)
	})
	return statusMenuNative.err
}

// PopupStatusMenu presents an ordinary AppKit menu below the menu-bar item's
// MyGo screen bounds. It returns after tracking ends, with an empty ID when
// dismissed. The caller dispatches the selected action after this returns.
// The temporary menu is never installed as the application's main menu and
// registers no global shortcuts.
func PopupStatusMenu(entries []MenuEntry, dark bool, anchor mygo.Rectangle) (selectedID string, err error) {
	if err := ValidateStatusMenu(entries, anchor); err != nil {
		return "", err
	}
	if err := initStatusMenuNative(); err != nil {
		return "", err
	}
	err = errors.New("native status menu presentation did not run")
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			defer func() {
				if failure := recover(); failure != nil {
					selectedID = ""
					err = fmt.Errorf("native status menu: %v", failure)
				}
			}()
			screens := nativeobjc.Send(uintptr(objc.GetClass("NSScreen")), objc.RegisterName("screens"))
			if nativeobjc.Send(screens, objc.RegisterName("count")) == 0 {
				err = errors.New("native status menu requires a display")
				return
			}
			primary := nativeobjc.Send(screens, objc.RegisterName("objectAtIndex:"), 0)
			primaryHeight := glassNative.getRect(primary, objc.RegisterName("frame")).Size.Height
			appearance := glassAppearance(dark)
			if appearance == 0 {
				err = errors.New("native status menu appearance is unavailable")
				return
			}
			target := nativeobjc.Send(nativeobjc.Send(uintptr(statusMenuNative.targetClass), objc.RegisterName("alloc")), objc.RegisterName("init"))
			if target == 0 {
				err = errors.New("cannot allocate native status menu target")
				return
			}
			defer nativeobjc.Send(target, objc.RegisterName("release"))
			selection := &statusMenuSelection{ids: make(map[uintptr]string)}
			statusMenuSelections[target] = selection
			defer delete(statusMenuSelections, target)
			menu, buildErr := buildStatusMenu(entries, "", appearance, target, selection)
			if buildErr != nil {
				err = buildErr
				return
			}
			defer nativeobjc.Send(menu, objc.RegisterName("release"))
			statusMenuNative.popup(menu, objc.RegisterName("popUpMenuPositioningItem:atLocation:inView:"),
				0, statusMenuAnchor(anchor, primaryHeight), 0)
			selectedID, err = selection.selected, nil
		})
	})
	return selectedID, err
}

// ValidateStatusMenu checks the complete menu tree without accessing AppKit or
// opening a menu. Composed settings menus can be checked at the same boundary
// that PopupStatusMenu uses before native presentation.
func ValidateStatusMenu(entries []MenuEntry, anchor mygo.Rectangle) error {
	if anchor.Width <= 0 || anchor.Height <= 0 || anchor.Width > 1e7 || anchor.Height > 1e7 ||
		anchor.X < -1e7 || anchor.X > 1e7 || anchor.Y < -1e7 || anchor.Y > 1e7 {
		return errors.New("invalid status menu anchor")
	}
	if len(entries) == 0 {
		return errors.New("status menu is empty")
	}
	seen := make(map[string]bool)
	count := 0
	var visit func([]MenuEntry, int) error
	visit = func(items []MenuEntry, depth int) error {
		if depth > 8 {
			return errors.New("status menu nesting exceeds limit")
		}
		for _, entry := range items {
			count++
			if count > maxStatusMenuEntries {
				return errors.New("status menu item count exceeds limit")
			}
			if entry.Separator {
				continue
			}
			text := entry.Label + entry.ID + entry.Symbol + entry.Key
			if entry.Label == "" || !utf8.ValidString(text) || strings.ContainsRune(text, '\x00') {
				return errors.New("invalid status menu text")
			}
			if entry.Key != "" && (!utf8.ValidString(entry.Key) || utf8.RuneCountInString(entry.Key) != 1) {
				return errors.New("status menu key equivalent must be one character")
			}
			if entry.Modifiers & ^(MenuShift|MenuControl|MenuOption|MenuCommand) != 0 {
				return errors.New("invalid status menu key modifiers")
			}
			if entry.ID != "" {
				if seen[entry.ID] {
					return errors.New("duplicate status menu item ID")
				}
				seen[entry.ID] = true
			} else if len(entry.Submenu) == 0 && !entry.Disabled {
				return errors.New("status menu action requires an ID")
			}
			if err := visit(entry.Submenu, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(entries, 0)
}

// MyGo uses the primary screen's top-left origin; AppKit uses its bottom-left.
// Keep X and negative secondary-screen coordinates unchanged. With no view or
// positioning item, AppKit places the menu content's top edge at this point and
// handles the visible-screen edge constraints itself.
func statusMenuAnchor(anchor mygo.Rectangle, primaryHeight float64) glassPoint {
	return glassPoint{X: float64(anchor.X), Y: primaryHeight - float64(anchor.Y) - float64(anchor.Height) - 4}
}

func menuString(text string) uintptr {
	return uintptr(objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), text))
}

// Returns an owned NSMenu; each parent retains its item and submenu before we
// release the construction reference, including on partially built failures.
func buildStatusMenu(entries []MenuEntry, title string, appearance, target uintptr, selection *statusMenuSelection) (menu uintptr, err error) {
	menu = nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NSMenu")), objc.RegisterName("alloc")), objc.RegisterName("initWithTitle:"), menuString(title))
	if menu == 0 {
		return 0, errors.New("cannot allocate native status menu")
	}
	complete := false
	defer func() {
		if !complete {
			nativeobjc.Send(menu, objc.RegisterName("release"))
			menu = 0
		}
	}()
	nativeobjc.Send(menu, objc.RegisterName("setAutoenablesItems:"), 0)
	nativeobjc.Send(menu, objc.RegisterName("setAppearance:"), appearance)
	for _, entry := range entries {
		if entry.Separator {
			separator := nativeobjc.Send(uintptr(objc.GetClass("NSMenuItem")), objc.RegisterName("separatorItem"))
			nativeobjc.Send(menu, objc.RegisterName("addItem:"), separator)
			continue
		}
		if err := appendStatusMenuItem(menu, entry, appearance, target, selection); err != nil {
			return menu, err
		}
	}
	complete = true
	return menu, nil
}

func appendStatusMenuItem(menu uintptr, entry MenuEntry, appearance, target uintptr, selection *statusMenuSelection) error {
	item := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NSMenuItem")), objc.RegisterName("alloc")),
		objc.RegisterName("initWithTitle:action:keyEquivalent:"), menuString(entry.Label), 0, menuString(entry.Key))
	if item == 0 {
		return errors.New("cannot allocate native status menu item")
	}
	defer nativeobjc.Send(item, objc.RegisterName("release"))
	nativeobjc.Send(item, objc.RegisterName("setKeyEquivalentModifierMask:"), uintptr(entry.Modifiers))
	if !entry.Disabled {
		nativeobjc.Send(item, objc.RegisterName("setEnabled:"), 1)
	} else {
		nativeobjc.Send(item, objc.RegisterName("setEnabled:"), 0)
	}
	if entry.Checked {
		nativeobjc.Send(item, objc.RegisterName("setState:"), 1)
	}
	setStatusMenuSymbol(item, entry.Symbol)
	if len(entry.Submenu) > 0 {
		submenu, err := buildStatusMenu(entry.Submenu, entry.Label, appearance, target, selection)
		if err != nil {
			return err
		}
		nativeobjc.Send(item, objc.RegisterName("setSubmenu:"), submenu)
		nativeobjc.Send(submenu, objc.RegisterName("release"))
	} else if !entry.Disabled && entry.ID != "" {
		tag := uintptr(len(selection.ids) + 1)
		selection.ids[tag] = entry.ID
		nativeobjc.Send(item, objc.RegisterName("setTag:"), tag)
		nativeobjc.Send(item, objc.RegisterName("setTarget:"), target)
		nativeobjc.Send(item, objc.RegisterName("setAction:"), uintptr(objc.RegisterName("copiggyMenuItemSelected:")))
	}
	nativeobjc.Send(menu, objc.RegisterName("addItem:"), item)
	if entry.Key == " " {
		return setStatusMenuSpaceGlyph(menu, item)
	}
	return nil
}

// AppKit spells out an ASCII Space using the system language, independently
// of the application's language. Keep standard menu drawing with the neutral
// Space glyph, plus a hidden native equivalent for the actual key. Both items
// share the same target/tag/action and lifetime; no global shortcut is added.
func setStatusMenuSpaceGlyph(menu, item uintptr) error {
	shortcut := nativeobjc.Send(item, objc.RegisterName("copy"))
	if shortcut == 0 {
		return errors.New("cannot allocate native Space key equivalent")
	}
	defer nativeobjc.Send(shortcut, objc.RegisterName("release"))
	nativeobjc.Send(shortcut, objc.RegisterName("setHidden:"), 1)
	nativeobjc.Send(shortcut, objc.RegisterName("setAllowsKeyEquivalentWhenHidden:"), 1)
	nativeobjc.Send(menu, objc.RegisterName("addItem:"), shortcut)
	nativeobjc.Send(item, objc.RegisterName("setAllowsAutomaticKeyEquivalentLocalization:"), 0)
	nativeobjc.Send(item, objc.RegisterName("setKeyEquivalent:"), menuString("␣"))
	return nil
}

func setStatusMenuSymbol(item uintptr, symbol string) {
	if symbol == "" {
		return
	}
	imageClass := uintptr(objc.GetClass("NSImage"))
	symbolSelector := objc.RegisterName("imageWithSystemSymbolName:accessibilityDescription:")
	if nativeobjc.Send(imageClass, objc.RegisterName("respondsToSelector:"), uintptr(symbolSelector)) == 0 {
		return
	}
	image := nativeobjc.Send(imageClass, symbolSelector, menuString(symbol), 0)
	if image == 0 {
		return // Older systems may not contain an individual symbol.
	}
	configurationClass := uintptr(objc.GetClass("NSImageSymbolConfiguration"))
	configurationSelector := objc.RegisterName("configurationWithPointSize:weight:")
	if nativeobjc.Send(configurationClass, objc.RegisterName("respondsToSelector:"), uintptr(configurationSelector)) != 0 {
		configuration := statusMenuNative.configuration(configurationClass, configurationSelector, 16, 0)
		if configured := nativeobjc.Send(image, objc.RegisterName("imageWithSymbolConfiguration:"), configuration); configured != 0 {
			image = configured
		}
	}
	// NSImage's factory can share cached images; template only our own copy.
	image = nativeobjc.Send(image, objc.RegisterName("copy"))
	if image != 0 {
		nativeobjc.Send(image, objc.RegisterName("setTemplate:"), 1)
		nativeobjc.Send(item, objc.RegisterName("setImage:"), image)
		// Newer AppKit releases may choose icon visibility automatically.
		// Keep this optional so older systems retain standard image behavior.
		visibility := objc.RegisterName("setPreferredImageVisibility:")
		if nativeobjc.Send(item, objc.RegisterName("respondsToSelector:"), uintptr(visibility)) != 0 {
			nativeobjc.Send(item, visibility, 1) // NSMenuItemImageVisibilityVisible
		}
		nativeobjc.Send(image, objc.RegisterName("release"))
	}
}
