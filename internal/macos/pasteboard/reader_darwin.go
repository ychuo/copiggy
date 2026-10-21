//go:build darwin

package pasteboard

import (
	"fmt"
	"sync"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var readerRuntime struct {
	once                              sync.Once
	err                               error
	pasteboard, workspace, runningApp objc.Class
	selectors                         struct{ general, change, shared, frontmost, bundle, name, pid, appByPID objc.SEL }
}

// Reader observes actual native state. Creation opens no windows, reads no
// clipboard contents, and never asks for authorization.
type Reader struct{}

func New() (*Reader, error) {
	readerRuntime.once.Do(func() {
		defer func() {
			if failure := recover(); failure != nil {
				readerRuntime.err = fmt.Errorf("native pasteboard: %v", failure)
			}
		}()
		if err := nativeobjc.Init(); err != nil {
			readerRuntime.err = err
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			readerRuntime.err = err
			return
		}
		r := &readerRuntime
		r.pasteboard, r.workspace, r.runningApp = objc.GetClass("NSPasteboard"), objc.GetClass("NSWorkspace"), objc.GetClass("NSRunningApplication")
		r.selectors.general = objc.RegisterName("generalPasteboard")
		r.selectors.change = objc.RegisterName("changeCount")
		r.selectors.shared = objc.RegisterName("sharedWorkspace")
		r.selectors.frontmost = objc.RegisterName("frontmostApplication")
		r.selectors.bundle = objc.RegisterName("bundleIdentifier")
		r.selectors.name = objc.RegisterName("localizedName")
		r.selectors.pid = objc.RegisterName("processIdentifier")
		r.selectors.appByPID = objc.RegisterName("runningApplicationWithProcessIdentifier:")
	})
	if readerRuntime.err != nil {
		return nil, readerRuntime.err
	}
	return &Reader{}, nil
}

func (r *Reader) ChangeCount() (version int64) {
	nativeobjc.WithPool(func() {
		board := nativeobjc.Send(uintptr(readerRuntime.pasteboard), readerRuntime.selectors.general)
		version = int64(nativeobjc.Send(board, readerRuntime.selectors.change))
	})
	return version
}

func (r *Reader) Frontmost() (app App) {
	nativeobjc.WithPool(func() {
		workspace := nativeobjc.Send(uintptr(readerRuntime.workspace), readerRuntime.selectors.shared)
		app = nativeApp(nativeobjc.Send(workspace, readerRuntime.selectors.frontmost))
	})
	return app
}

func (r *Reader) TargetApp(pid int) (app App) {
	if pid <= 0 {
		return r.Frontmost()
	}
	nativeobjc.WithPool(func() {
		app = nativeApp(nativeobjc.Send(uintptr(readerRuntime.runningApp), readerRuntime.selectors.appByPID, uintptr(pid)))
		app.PID = pid
	})
	return app
}

func nativeApp(running uintptr) App {
	return App{
		BundleID: nativeobjc.String(nativeobjc.Send(running, readerRuntime.selectors.bundle)),
		Name:     nativeobjc.String(nativeobjc.Send(running, readerRuntime.selectors.name)),
		PID:      int(int32(nativeobjc.Send(running, readerRuntime.selectors.pid))),
	}
}
