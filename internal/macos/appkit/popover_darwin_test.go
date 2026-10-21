//go:build darwin

package appkit

import "testing"

// Missing or closed resources must reject work before native initialization or
// main-thread dispatch. Default tests never create an AppKit window.
func TestPopoverAppearanceMissingOwnerIsSafe(t *testing.T) {
	for _, owner := range []*PopoverAppearance{nil, {}} {
		owner.Close()
		owner.Close()
		if err := owner.SetDark(true); err == nil {
			t.Fatal("appearance update accepted a missing owner")
		}
	}
	if owner, err := ShowNativePopoverWithAppearance(false, nil); err == nil || owner != nil {
		t.Fatal("popover presentation accepted a missing callback")
	}
}
