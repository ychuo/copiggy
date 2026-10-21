//go:build darwin

// Package appkit adapts the system menu, appearance, and application artwork
// used by the MyGo native presentation. It does not depend on the input guard.
package appkit

import (
	"sync"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego"
)

var appKitRuntime struct {
	once sync.Once
	err  error
}

func initAppKit() error {
	appKitRuntime.once.Do(func() {
		if err := nativeobjc.Init(); err != nil {
			appKitRuntime.err = err
			return
		}
		_, appKitRuntime.err = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	return appKitRuntime.err
}
