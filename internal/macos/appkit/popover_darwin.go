//go:build darwin

package appkit

import (
	nativeobjc "copiggy/internal/macos/internal/objc"
	"errors"
	"fmt"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"sync/atomic"
)

// PopoverAppearance owns an opaque retained NSPopover reference. Copies share
// the same ownership token, so Close is safe to call repeatedly or after copying.
// All Objective-C access happens on the main thread.
type PopoverAppearance struct{ reference *atomic.Uintptr }

// ShowNativePopoverWithAppearance runs a synchronous MyGo popover Show and
// returns an opaque owner for one retained NSPopover, not its NSWindow. The closure must
// return that flyout's NativeHandle after Show. Matching this explicit window
// keeps unrelated notifications from changing another popover's appearance.
//
// MyGo exposes the popover's window but not its owning NSPopover. The public
// will-show notification supplies that owner; observe only during this Show,
// then set the matching owner's appearance before returning to the event loop.
// Close the returned owner when it is replaced or the flyout is destroyed.
// Setup failures do not call show.
func ShowNativePopoverWithAppearance(dark bool, show func() uintptr) (popover *PopoverAppearance, err error) {
	if show == nil {
		return nil, errors.New("native popover show callback is missing")
	}
	if err = initGlassNative(); err != nil {
		return nil, err
	}
	err = errors.New("native popover show did not run")
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			defer func() {
				if failure := recover(); failure != nil {
					err = fmt.Errorf("native popover appearance: %v", failure)
				}
			}()
			appearance := glassAppearance(dark)
			if appearance == 0 {
				err = errors.New("native popover appearance is unavailable")
				return
			}
			candidates := make(map[uintptr]bool)
			defer func() {
				for candidate := range candidates {
					nativeobjc.Send(candidate, objc.RegisterName("release"))
				}
			}()
			block := objc.NewBlock(func(_ objc.Block, notification objc.ID) {
				candidate := nativeobjc.Send(uintptr(notification), objc.RegisterName("object"))
				if candidate != 0 && !candidates[candidate] && nativeobjc.Send(candidate, objc.RegisterName("isKindOfClass:"), uintptr(objc.GetClass("NSPopover"))) != 0 {
					// Keep the notification object alive through Show; do not
					// change it until its content window matches the caller's.
					nativeobjc.Send(candidate, objc.RegisterName("retain"))
					candidates[candidate] = true
				}
			})
			defer block.Release()
			center := nativeobjc.Send(uintptr(objc.GetClass("NSNotificationCenter")), objc.RegisterName("defaultCenter"))
			name := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "NSPopoverWillShowNotification")
			observer := nativeobjc.Send(center, objc.RegisterName("addObserverForName:object:queue:usingBlock:"), uintptr(name), 0, 0, uintptr(block))
			if observer == 0 {
				err = errors.New("cannot observe native popover presentation")
				return
			}
			defer nativeobjc.Send(center, objc.RegisterName("removeObserver:"), observer)
			window := show()
			if window == 0 {
				err = errors.New("native popover did not create a window")
				return
			}
			var matched uintptr
			for candidate := range candidates {
				controller := nativeobjc.Send(candidate, objc.RegisterName("contentViewController"))
				content := nativeobjc.Send(controller, objc.RegisterName("view"))
				if nativeobjc.Send(content, objc.RegisterName("window")) == window {
					if matched != 0 {
						err = errors.New("native popover window has ambiguous owners")
						return
					}
					matched = candidate
				}
			}
			if matched == 0 {
				err = errors.New("native popover presentation owner was not observed")
				return
			}
			setGlassViewAppearance(matched, appearance)
			nativeobjc.Send(matched, objc.RegisterName("retain"))
			reference := &atomic.Uintptr{}
			reference.Store(matched)
			popover, err = &PopoverAppearance{reference: reference}, nil
		})
	})
	return popover, err
}

// SetDark updates the actual NSPopover that owns both the material and arrow.
func (p *PopoverAppearance) SetDark(dark bool) error {
	if p == nil || p.reference == nil || p.reference.Load() == 0 {
		return errors.New("native popover reference is missing or closed")
	}
	if err := initGlassNative(); err != nil {
		return err
	}
	err := errors.New("native popover appearance update did not run")
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			handle := p.reference.Load()
			if handle == 0 {
				err = errors.New("native popover reference has been closed")
				return
			}
			appearance := glassAppearance(dark)
			if appearance == 0 {
				err = errors.New("native popover appearance is unavailable")
				return
			}
			setGlassViewAppearance(handle, appearance)
			err = nil
		})
	})
	return err
}

// Close releases this object's retained reference at most once. It neither
// dismisses the popover nor changes MyGo's ownership of it.
func (p *PopoverAppearance) Close() {
	if p == nil || p.reference == nil || p.reference.Load() == 0 {
		return
	}
	mygo.RunOnMain(func() {
		if handle := p.reference.Swap(0); handle != 0 {
			nativeobjc.Send(handle, objc.RegisterName("release"))
		}
	})
}
