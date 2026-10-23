//go:build darwin

package appkit

import (
	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego/objc"
)

// SystemRegion reads the user's regional format preference, independently of
// preferred languages and time zone. It never reads the clipboard or desktop.
func SystemRegion() (region string) {
	if initAppKit() != nil {
		return ""
	}
	defer func() {
		if recover() != nil {
			region = ""
		}
	}()
	nativeobjc.WithPool(func() {
		locale := nativeobjc.Send(uintptr(objc.GetClass("NSLocale")), objc.RegisterName("currentLocale"))
		region = nativeobjc.String(nativeobjc.Send(locale, objc.RegisterName("countryCode")))
	})
	return region
}
