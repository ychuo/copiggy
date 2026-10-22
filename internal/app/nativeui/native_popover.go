package nativeui

import (
	"log"
	"math"
	"time"

	"copiggy/internal/clipmodel"
	macos "copiggy/internal/macos/appkit"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Synchronize between frames, after card layout is known. Constructing a second
// native surface from inside the parent's UI build would re-enter its renderer.
// The existing main-thread clipboard watcher calls this every 20 ms.
func (s *application) syncNativePopover() {
	if s.panel == nil || s.quitting || s.hiding {
		s.destroyNativeLinkPreview()
		return
	}
	item, selected := s.selected()
	if !s.preview || !selected || !s.panel.IsVisible() {
		s.destroyNativeLinkPreview()
		if s.popover != nil && s.popover.IsVisible() {
			s.popover.Hide()
		}
		return
	}
	state := s.nativeState()
	anchor, ok := nativePreviewAnchor(state.CardBounds[item.ID], state.Tray)
	if !ok {
		return // ScrollIntoView will produce the selected card's next bounds.
	}
	width, height := nativeContentPreviewSize(item.Type)
	display := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	width, height = min(width, display.WorkArea.Width-48), min(height, display.WorkArea.Height-80)
	if s.popover == nil {
		popup := mygo.NewFlyout(mygo.FlyoutOptions{
			Parent: s.panel, Anchor: anchor, Placement: mygo.PlacementTop,
			Width: width, Height: height, Hidden: true, Focusable: true,
			Popover: true, Content: ui.View(s.nativePopoverView),
		})
		s.popover = popup
		s.popoverAnchor = anchor
		s.popoverWidth, s.popoverHeight = width, height
		s.popoverAppearance = nil
		popup.OnFocus(s.nativeWindowFocused)
		popup.OnBlur(s.nativeWindowBlurred)
		// AppKit's outside-click dismissal destroys this flyout. Programmatic
		// Hide keeps it reusable, so only this callback clears the pointer.
		popup.OnClosed(func() {
			if s.popover != popup {
				return
			}
			s.destroyNativeLinkPreview()
			s.popover = nil
			s.popoverAppearance.Close()
			s.popoverAppearance = nil
			s.cancelNativeDraft()
			s.preview = false
			state.Focus.Focus()
			s.publish()
			s.nativeWindowBlurred()
		})
	}
	popup := s.popover
	if anchor != s.popoverAnchor {
		popup.SetAnchor(anchor)
		s.popoverAnchor = anchor
	}
	if width != s.popoverWidth || height != s.popoverHeight {
		popup.SetSize(width, height)
		s.popoverWidth, s.popoverHeight = width, height
	}
	if !popup.IsVisible() {
		// NSPopover owns the appearance of its material and arrow. Its
		// temporary NSWindow is not the owner and may be replaced on reopen.
		// Observe this exact Show operation to retain the matching popover.
		appearance, err := macos.ShowNativePopoverWithAppearance(state.Dark, func() uintptr {
			popup.Show()
			return popup.NativeHandle()
		})
		if err != nil {
			log.Printf("native popover appearance: %v", err)
			if !popup.IsVisible() {
				popup.Show()
			}
		} else {
			previous := s.popoverAppearance
			s.popoverAppearance, s.popoverDark = appearance, state.Dark
			previous.Close()
		}
		popup.SetTitle("Copiggy · " + s.tr("Content preview"))
		state.PreviewFocus.Focus()
		popup.Invalidate()
		log.Printf("native preview: NSPopover, %dx%d", width, height)
	}
	if s.popoverAppearance != nil && s.popoverDark != state.Dark {
		if err := s.popoverAppearance.SetDark(state.Dark); err != nil {
			log.Printf("native popover appearance: %v", err)
		} else {
			s.popoverDark = state.Dark
		}
	}
	// Embedded WebKit needs the real window adopted by popup.Show(). A hidden
	// NSPopover has no NSWindow, and reopening may allocate a different one.
	s.syncNativeLinkPreview(item)
}

// Use the same content dimensions for the native host and headless layout
// checks so compact surfaces are exercised at their actual available height.
func nativeContentPreviewSize(kind clipmodel.ContentType) (int, int) {
	switch kind {
	case clipmodel.TypeText:
		return 520, nativeNotePreviewHeight
	case clipmodel.TypeJSON:
		return 520, nativeJSONPreviewHeight
	case clipmodel.TypeFilePath:
		return 520, nativeFilePathPreviewHeight
	case clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey:
		return 520, nativeSecretPreviewHeight(kind)
	case clipmodel.TypeEmail:
		return 520, nativeEmailPreviewHeight
	case clipmodel.TypeColor:
		return 520, nativeColorPreviewHeight
	case clipmodel.TypeDate:
		return 520, nativeDatePreviewHeight
	case clipmodel.TypeTimestamp:
		return 520, nativeTimestampPreviewHeight
	case clipmodel.TypeImage:
		return 520, nativeImagePreviewHeight
	case clipmodel.TypeLink:
		return 720, 500
	case clipmodel.TypeCoordinates:
		return nativeMapPreviewWidth, nativeMapPreviewHeight
	default:
		return 520, 360
	}
}

// Anchor only to the visible part of a card. AppKit owns the arrow position and
// may flip the popover when a display edge leaves too little space above it.
func nativePreviewAnchor(card, tray ui.Rect) (mygo.Rectangle, bool) {
	left, top := max(card.X, tray.X), max(card.Y, tray.Y)
	right, bottom := min(card.X+card.W, tray.X+tray.W), min(card.Y+card.H, tray.Y+tray.H)
	if card.W <= 0 || card.H <= 0 || right <= left || bottom <= top {
		return mygo.Rectangle{}, false
	}
	return mygo.Rectangle{X: int(math.Round(float64(left))), Y: int(math.Round(float64(top))),
		Width: max(1, int(math.Round(float64(right-left)))), Height: max(1, int(math.Round(float64(bottom-top))))}, true
}

func (s *application) nativeWindowFocused() {
	s.focusGeneration++
	s.native.SetPanelVisible(true)
}

func (s *application) reconcileNativeFocus() {
	// NSPopover does not emit all ordinary window focus callbacks. Reconcile
	// before confirming protected copies, so a stale UI state cannot cancel a
	// new source-app copy transaction after the user has switched out.
	if s.panel != nil {
		focused := s.panel.IsVisible() && (s.panel.IsFocused() || (s.popover != nil && s.popover.IsVisible() && s.popover.IsFocused()))
		s.native.SetPanelVisible(focused)
	}
}

func (s *application) nativeWindowBlurred() {
	// Clear the exemption immediately, before deferring the visibility check.
	// Events addressed to our process already have their own guard exemption.
	s.native.SetPanelVisible(false)
	s.focusGeneration++
	if s.hiding || s.quitting || s.panel == nil {
		return
	}
	generation := s.focusGeneration
	time.AfterFunc(30*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if s.quitting || s.hiding || generation != s.focusGeneration || !s.panel.IsVisible() {
				return
			}
			focused := s.panel.IsFocused() || (s.popover != nil && s.popover.IsVisible() && s.popover.IsFocused())
			s.native.SetPanelVisible(focused)
			if !focused {
				s.hide()
			}
		})
	})
}
