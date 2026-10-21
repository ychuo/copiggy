//go:build darwin

package appkit

import (
	nativeobjc "copiggy/internal/macos/internal/objc"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// AppIconPNG returns the application's actual Finder icon as a 64×64 PNG.
// AppKit work runs on MyGo's main thread. Call after the MyGo application has
// started, and cache icons by bundle identifier in the presentation layer.
// Resolving an icon neither launches the application nor reads its content.
func AppIconPNG(bundleID string) (png []byte, err error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" || len(bundleID) > 4096 || strings.ContainsRune(bundleID, '\x00') {
		return nil, errors.New("invalid application bundle identifier")
	}
	if err := initAppKit(); err != nil {
		return nil, err
	}

	// RunOnMain drops work after shutdown, so do not mistake that for a
	// successful lookup of an empty image.
	err = errors.New("application icon lookup did not run")
	mygo.RunOnMain(func() {
		nativeobjc.WithPool(func() {
			bytes := []byte(bundleID)
			str := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NSString")), objc.RegisterName("alloc")),
				objc.RegisterName("initWithBytes:length:encoding:"),
				uintptr(unsafe.Pointer(unsafe.SliceData(bytes))), uintptr(len(bytes)), 4)
			runtime.KeepAlive(bytes)
			if str == 0 {
				err = errors.New("cannot encode application bundle identifier")
				return
			}
			defer nativeobjc.Send(str, objc.RegisterName("release"))
			workspace := nativeobjc.Send(uintptr(objc.GetClass("NSWorkspace")), objc.RegisterName("sharedWorkspace"))
			url := nativeobjc.Send(workspace, objc.RegisterName("URLForApplicationWithBundleIdentifier:"), str)
			path := nativeobjc.String(nativeobjc.Send(url, objc.RegisterName("path")))
			if path == "" {
				err = fmt.Errorf("application is not installed: %s", bundleID)
				return
			}
			// MyGo uses NSWorkspace's icon and picks the representation that
			// fits 64 pixels, preserving transparency and Retina sharpness.
			png, err = mygo.Shell.FileIcon(path, 64)
		})
	})
	return png, err
}
