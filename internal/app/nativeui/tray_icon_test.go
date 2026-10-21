package nativeui

import (
	"bytes"
	"testing"

	"copiggy/resources"
)

func TestTrayBrandAssetsMatchRuntimeTemplates(t *testing.T) {
	for _, variant := range []struct {
		name  string
		guard bool
	}{
		{"tray-normal.png", false},
		{"tray-protected.png", true},
	} {
		t.Run(variant.name, func(t *testing.T) {
			asset, err := resources.Brand.ReadFile("brand/" + variant.name)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(asset, clipboardTrayIcon(variant.guard)) {
				t.Fatal("standalone brand asset differs from the native menu-bar icon")
			}
		})
	}
}
