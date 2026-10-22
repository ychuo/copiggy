//go:build darwin

package appkit

import (
	nativeobjc "copiggy/internal/macos/internal/objc"
	"errors"
	"math"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

const trayGlassAutoresizing = 2 | 16 // NSViewWidthSizable | NSViewHeightSizable

type trayGlassAttachment struct {
	parent, surface, effect uintptr
	surfaceAutoresizing     uintptr
	material                string
	tintSet, tintDark       bool

	parentWantsLayer, parentMasksToBounds, parentCornerCurve uintptr
	parentCornerRadius                                       float64
	clipFrame                                                glassRect
	clipRadius                                               float64
	clipSet                                                  bool
}

// Main-thread-only ownership. Both views we reparent are retained until Clear:
// AppKit's contentView setter can remove a view from its previous superview.
var trayGlassAttachments = make(map[uintptr]*trayGlassAttachment)

// MakeTrayBorderless removes the window chrome before attaching the glass.
// MyGo's Frameless option hides the title bar but retains NSWindowStyleMaskTitled,
// whose frame outline has its own corner radius. A truly borderless panel lets
// NSGlassEffectView supply the single visible outline instead of nesting the
// glass inside a second, differently rounded system frame.
func MakeTrayBorderless(window uintptr) error {
	if window == 0 {
		return errors.New("cannot configure a missing tray window")
	}
	if err := initGlassNative(); err != nil {
		return err
	}
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			const frameStyles = 1 | 2 | 4 | 8 | (1 << 15) // Titled, Closable, Miniaturizable, Resizable, FullSizeContentView
			style := nativeobjc.Send(window, objc.RegisterName("styleMask"))
			// Keep NonactivatingPanel exactly as it was at creation: changing it
			// at runtime breaks the WindowServer's panel keyboard semantics.
			if borderless := style &^ uintptr(frameStyles); borderless != style {
				nativeobjc.Send(window, objc.RegisterName("setStyleMask:"), borderless)
			}
			nativeobjc.Send(window, objc.RegisterName("invalidateShadow"))
		})
	})
	return nil
}

// SetTrayGlass fills a transparent MyGo Native UI window with a system material.
// On macOS 26+, the MyGoSurfaceView is NSGlassEffectView.contentView, so its
// contents participate in the system effect rather than merely covering it.
// Older systems use an NSVisualEffectView containing the same native surface.
// It returns "liquid-glass" or "visual-effect" after successfully attaching.
//
// Use this only for the single-surface tray window, not an NSPopover (which
// supplies its own material). The UI root must draw a transparent background.
// Call again for appearance/size changes; ordinary resizing also autoresizes
// both views. ClearTrayGlass must run before MyGo destroys the window.
func SetTrayGlass(window uintptr, dark bool, radius float64) (string, error) {
	if window == 0 {
		return "", errors.New("cannot attach tray glass to a missing native window")
	}
	if !validTrayGlassNumber(radius) || radius < 0 {
		return "", errors.New("invalid tray glass corner radius")
	}
	if err := initGlassNative(); err != nil {
		return "", err
	}
	var material string
	err := errors.New("native tray glass update did not run")
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() { material, err = setTrayGlass(window, dark, radius) })
	})
	return material, err
}

func setTrayGlass(window uintptr, dark bool, radius float64) (string, error) {
	parent := nativeobjc.Send(window, objc.RegisterName("contentView"))
	if parent == 0 {
		return "", errors.New("tray window has no content view")
	}
	frame := glassNative.getRect(parent, objc.RegisterName("bounds"))
	radius, err := trayGlassRadius(frame, radius)
	if err != nil {
		return "", err
	}
	appearance := glassAppearance(dark)
	if appearance == 0 {
		return "", errors.New("tray glass appearance is unavailable")
	}
	attachment := trayGlassAttachments[window]
	if attachment != nil && attachment.parent != parent {
		return "", errors.New("tray content view changed; clear its glass before replacing content")
	}
	transaction := uintptr(objc.GetClass("CATransaction"))
	nativeobjc.Send(transaction, objc.RegisterName("begin"))
	nativeobjc.Send(transaction, objc.RegisterName("setDisableActions:"), 1)
	shapeChanged := false
	defer func() {
		nativeobjc.Send(transaction, objc.RegisterName("commit"))
		if shapeChanged {
			nativeobjc.Send(window, objc.RegisterName("invalidateShadow"))
		}
	}()
	if attachment == nil {
		surface := findGlassSurfaceView(parent, 0)
		if surface == 0 || nativeobjc.Send(surface, objc.RegisterName("superview")) != parent {
			return "", errors.New("tray glass requires one direct MyGo ui.View surface")
		}
		material, class := "liquid-glass", uintptr(objc.GetClass("NSGlassEffectView"))
		if class == 0 {
			material, class = "visual-effect", uintptr(objc.GetClass("NSVisualEffectView"))
		}
		if class == 0 {
			return "", errors.New("macOS provides no supported tray material view")
		}
		effect := glassNative.initRect(nativeobjc.Send(class, objc.RegisterName("alloc")), objc.RegisterName("initWithFrame:"), frame)
		if effect == 0 {
			return "", errors.New("cannot allocate tray material view")
		}
		attachment = &trayGlassAttachment{
			parent: parent, surface: surface, effect: effect, material: material,
			surfaceAutoresizing: nativeobjc.Send(surface, objc.RegisterName("autoresizingMask")),
		}
		nativeobjc.Send(parent, objc.RegisterName("retain"))
		nativeobjc.Send(surface, objc.RegisterName("retain"))
		nativeobjc.Send(effect, objc.RegisterName("setAutoresizingMask:"), trayGlassAutoresizing)
		nativeobjc.Send(effect, objc.RegisterName("setAppearance:"), appearance)
		nativeobjc.Send(parent, objc.RegisterName("addSubview:positioned:relativeTo:"), effect, ^uintptr(0), surface)
		wasFirstResponder := nativeobjc.Send(window, objc.RegisterName("firstResponder")) == surface
		nativeobjc.Send(surface, objc.RegisterName("removeFromSuperview"))
		nativeobjc.Send(surface, objc.RegisterName("setAutoresizingMask:"), trayGlassAutoresizing)
		if material == "liquid-glass" {
			nativeobjc.Send(effect, objc.RegisterName("setStyle:"), 0) // NSGlassEffectViewStyleRegular
			nativeobjc.Send(effect, objc.RegisterName("setContentView:"), surface)
		} else {
			nativeobjc.Send(effect, objc.RegisterName("setBlendingMode:"), 0) // BehindWindow
			nativeobjc.Send(effect, objc.RegisterName("setState:"), 1)        // Active
			nativeobjc.Send(effect, objc.RegisterName("setMaterial:"), 6)     // Popover
			nativeobjc.Send(effect, objc.RegisterName("setWantsLayer:"), 1)
			nativeobjc.Send(nativeobjc.Send(effect, objc.RegisterName("layer")), objc.RegisterName("setMasksToBounds:"), 1)
			nativeobjc.Send(effect, objc.RegisterName("addSubview:"), surface)
		}
		trayGlassAttachments[window] = attachment
		if wasFirstResponder {
			nativeobjc.Send(window, objc.RegisterName("makeFirstResponder:"), surface)
		}
	}
	setGlassViewAppearance(attachment.effect, appearance)
	setGlassViewFrame(attachment.effect, frame)
	// NSGlassEffectView owns content placement. Match its content rectangle
	// without reparenting it or assuming the private internal view hierarchy.
	contentFrame := glassRect{Size: frame.Size}
	setGlassViewFrame(attachment.surface, contentFrame)
	if attachment.material == "liquid-glass" {
		updateTrayGlassTint(attachment, dark)
		setGlassCornerRadius(attachment.effect, radius)
	} else {
		setGlassCornerRadius(nativeobjc.Send(attachment.effect, objc.RegisterName("layer")), radius)
	}
	shapeChanged = updateTrayGlassClip(attachment, frame, radius)
	nativeobjc.Send(window, objc.RegisterName("setOpaque:"), 0)
	nativeobjc.Send(window, objc.RegisterName("setBackgroundColor:"), nativeobjc.Send(uintptr(objc.GetClass("NSColor")), objc.RegisterName("clearColor")))
	return attachment.material, nil
}

// A large Regular glass surface becomes very pale over white windows. Give
// light mode a restrained neutral tint through the material's own public API;
// do not cover it with a painted overlay or replace its adaptive glass style.
// Dark mode keeps the system's untinted glass. Resizing and ordinary frames
// must not allocate colors or reset an unchanged native effect property.
func updateTrayGlassTint(attachment *trayGlassAttachment, dark bool) {
	if attachment.tintSet && attachment.tintDark == dark {
		return
	}
	nativeobjc.Send(attachment.effect, objc.RegisterName("setTintColor:"), trayGlassTintColor(dark))
	attachment.tintSet, attachment.tintDark = true, dark
}

func trayGlassTintColor(dark bool) uintptr {
	if dark {
		return 0
	}
	return glassNative.colorWhite(uintptr(objc.GetClass("NSColor")), objc.RegisterName("colorWithWhite:alpha:"), .55, .14)
}

// The material's cornerRadius shapes the glass, but the window's root NSView
// still has rectangular bounds. Clip the final composition at that root, as
// MyGo does for its rounded flyouts, without altering the glass's private layers.
// Cache geometry so ordinary renders don't invalidate the window shadow.
func updateTrayGlassClip(attachment *trayGlassAttachment, frame glassRect, radius float64) bool {
	if !attachment.clipSet {
		attachment.parentWantsLayer = nativeobjc.Send(attachment.parent, objc.RegisterName("wantsLayer"))
		nativeobjc.Send(attachment.parent, objc.RegisterName("setWantsLayer:"), 1)
		layer := nativeobjc.Send(attachment.parent, objc.RegisterName("layer"))
		attachment.parentCornerRadius = glassNative.getFloat(layer, objc.RegisterName("cornerRadius"))
		attachment.parentMasksToBounds = nativeobjc.Send(layer, objc.RegisterName("masksToBounds"))
		attachment.parentCornerCurve = nativeobjc.Send(nativeobjc.Send(layer, objc.RegisterName("cornerCurve")), objc.RegisterName("retain"))
		nativeobjc.Send(layer, objc.RegisterName("setCornerCurve:"), glassNative.continuousCurve)
		nativeobjc.Send(layer, objc.RegisterName("setMasksToBounds:"), 1)
	}
	changed := !attachment.clipSet || attachment.clipFrame != frame || attachment.clipRadius != radius
	if changed {
		setGlassCornerRadius(nativeobjc.Send(attachment.parent, objc.RegisterName("layer")), radius)
		attachment.clipFrame, attachment.clipRadius, attachment.clipSet = frame, radius, true
	}
	return changed
}

// ClearTrayGlass removes our material wrapper and restores the MyGo surface to
// its original parent with its current size. It is idempotent while the window
// exists. Do not pass an NSWindow pointer after MyGo has destroyed it.
func ClearTrayGlass(window uintptr) error {
	if window == 0 {
		return nil
	}
	if err := initGlassNative(); err != nil {
		return err
	}
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			attachment := trayGlassAttachments[window]
			if attachment == nil {
				return
			}
			delete(trayGlassAttachments, window)
			frame := glassNative.getRect(attachment.effect, objc.RegisterName("frame"))
			wasFirstResponder := nativeobjc.Send(window, objc.RegisterName("firstResponder")) == attachment.surface
			if attachment.material == "liquid-glass" {
				nativeobjc.Send(attachment.effect, objc.RegisterName("setContentView:"), 0)
			} else {
				nativeobjc.Send(attachment.surface, objc.RegisterName("removeFromSuperview"))
			}
			nativeobjc.Send(attachment.parent, objc.RegisterName("addSubview:positioned:relativeTo:"), attachment.surface, 1, attachment.effect)
			setGlassViewFrame(attachment.surface, frame)
			nativeobjc.Send(attachment.surface, objc.RegisterName("setAutoresizingMask:"), attachment.surfaceAutoresizing)
			nativeobjc.Send(attachment.effect, objc.RegisterName("removeFromSuperview"))
			if attachment.clipSet {
				layer := nativeobjc.Send(attachment.parent, objc.RegisterName("layer"))
				setGlassCornerRadius(layer, attachment.parentCornerRadius)
				nativeobjc.Send(layer, objc.RegisterName("setCornerCurve:"), attachment.parentCornerCurve)
				nativeobjc.Send(layer, objc.RegisterName("setMasksToBounds:"), attachment.parentMasksToBounds)
				nativeobjc.Send(attachment.parentCornerCurve, objc.RegisterName("release"))
				nativeobjc.Send(attachment.parent, objc.RegisterName("setWantsLayer:"), attachment.parentWantsLayer)
				nativeobjc.Send(window, objc.RegisterName("invalidateShadow"))
			}
			if wasFirstResponder {
				nativeobjc.Send(window, objc.RegisterName("makeFirstResponder:"), attachment.surface)
			}
			nativeobjc.Send(attachment.effect, objc.RegisterName("release"))
			nativeobjc.Send(attachment.surface, objc.RegisterName("release"))
			nativeobjc.Send(attachment.parent, objc.RegisterName("release"))
		})
	})
	return nil
}

// SetNativeWindowAppearance overrides only the supplied NSWindow and its root
// content view. An NSPopover's material belongs to the NSPopover itself; use
// PopoverAppearance.SetDark for that owner instead of its backing window.
func SetNativeWindowAppearance(window uintptr, dark bool) error {
	if window == 0 {
		return errors.New("cannot set appearance before the native window exists")
	}
	if err := initGlassNative(); err != nil {
		return err
	}
	err := errors.New("native window appearance update did not run")
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			appearance := glassAppearance(dark)
			if appearance == 0 {
				err = errors.New("native window appearance is unavailable")
				return
			}
			setGlassViewAppearance(window, appearance)
			setGlassViewAppearance(nativeobjc.Send(window, objc.RegisterName("contentView")), appearance)
			err = nil
		})
	})
	return err
}

func setGlassViewAppearance(view, appearance uintptr) {
	if view != 0 && nativeobjc.Send(view, objc.RegisterName("appearance")) != appearance {
		nativeobjc.Send(view, objc.RegisterName("setAppearance:"), appearance)
	}
}

// Avoid repeatedly sending setFrame: while rendering: MyGo's setFrameSize:
// emits resize/invalidation even if the size is unchanged.
func setGlassViewFrame(view uintptr, frame glassRect) {
	if glassNative.getRect(view, objc.RegisterName("frame")) != frame {
		glassNative.setRect(view, objc.RegisterName("setFrame:"), frame)
	}
}

func setGlassCornerRadius(view uintptr, radius float64) {
	if glassNative.getFloat(view, objc.RegisterName("cornerRadius")) != radius {
		glassNative.setFloat(view, objc.RegisterName("setCornerRadius:"), radius)
	}
}

func validTrayGlassNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value) <= 1e6
}

func trayGlassRadius(frame glassRect, radius float64) (float64, error) {
	for _, value := range []float64{frame.Origin.X, frame.Origin.Y, frame.Size.Width, frame.Size.Height, radius} {
		if !validTrayGlassNumber(value) {
			return 0, errors.New("invalid native tray material geometry")
		}
	}
	if frame.Size.Width <= 0 || frame.Size.Height <= 0 || radius < 0 {
		return 0, errors.New("native tray material requires a positive content size and nonnegative radius")
	}
	return min(radius, frame.Size.Width/2, frame.Size.Height/2), nil
}
