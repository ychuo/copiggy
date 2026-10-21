//go:build darwin

package appkit

import (
	nativeobjc "copiggy/internal/macos/internal/objc"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Shared AppKit geometry and runtime helpers for the tray material and native popovers.
type glassPoint struct{ X, Y float64 }
type glassSize struct{ Width, Height float64 }
type glassRect struct {
	Origin glassPoint
	Size   glassSize
}

var glassNative struct {
	once            sync.Once
	err             error
	getRect         func(uintptr, objc.SEL) glassRect
	setRect         func(uintptr, objc.SEL, glassRect)
	initRect        func(uintptr, objc.SEL, glassRect) uintptr
	getFloat        func(uintptr, objc.SEL) float64
	setFloat        func(uintptr, objc.SEL, float64)
	aquaName        uintptr
	darkName        uintptr
	continuousCurve uintptr
}

func initGlassNative() error {
	if err := initAppKit(); err != nil {
		return err
	}
	glassNative.once.Do(func() {
		defer func() {
			if failure := recover(); failure != nil {
				glassNative.err = fmt.Errorf("macOS glass bridge: %v", failure)
			}
		}()
		if _, err := purego.Dlopen("/System/Library/Frameworks/QuartzCore.framework/QuartzCore", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			glassNative.err = err
			return
		}
		rectSend := nativeobjc.MessageSend()
		if runtime.GOARCH == "amd64" {
			var err error
			rectSend, err = purego.Dlsym(purego.RTLD_DEFAULT, "objc_msgSend_stret")
			if err != nil {
				glassNative.err = err
				return
			}
		}
		g := &glassNative
		purego.RegisterFunc(&g.getRect, rectSend)
		purego.RegisterFunc(&g.setRect, nativeobjc.MessageSend())
		purego.RegisterFunc(&g.initRect, nativeobjc.MessageSend())
		purego.RegisterFunc(&g.getFloat, nativeobjc.MessageSend())
		purego.RegisterFunc(&g.setFloat, nativeobjc.MessageSend())
		for name, destination := range map[string]*uintptr{
			"NSAppearanceNameAqua":     &g.aquaName,
			"NSAppearanceNameDarkAqua": &g.darkName,
			"kCACornerCurveContinuous": &g.continuousCurve,
		} {
			address, err := purego.Dlsym(purego.RTLD_DEFAULT, name)
			if err != nil {
				g.err = err
				return
			}
			// These frameworks export NSString * globals, not the NSString itself.
			*destination = **(**uintptr)(unsafe.Pointer(&address))
			if *destination == 0 {
				g.err = fmt.Errorf("native appearance constant %s is unavailable", name)
				return
			}
		}
	})
	return glassNative.err
}

func glassAppearance(dark bool) uintptr {
	name := glassNative.aquaName
	if dark {
		name = glassNative.darkName
	}
	return nativeobjc.Send(uintptr(objc.GetClass("NSAppearance")), objc.RegisterName("appearanceNamed:"), name)
}

func findGlassSurfaceView(view uintptr, depth int) uintptr {
	if view == 0 || depth > 8 {
		return 0
	}
	if nativeobjc.Send(view, objc.RegisterName("isKindOfClass:"), uintptr(objc.GetClass("MyGoSurfaceView"))) != 0 {
		return view
	}
	children := nativeobjc.Send(view, objc.RegisterName("subviews"))
	count := min(nativeobjc.Send(children, objc.RegisterName("count")), 64)
	for index := uintptr(0); index < count; index++ {
		child := nativeobjc.Send(children, objc.RegisterName("objectAtIndex:"), index)
		if found := findGlassSurfaceView(child, depth+1); found != 0 {
			return found
		}
	}
	return 0
}
