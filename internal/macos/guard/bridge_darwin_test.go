//go:build darwin && integration

package guard

import (
	"copiggy/internal/macos/pasteboard"
	"testing"
	"time"

	"github.com/ebitengine/purego"
)

func memoryKey(t *testing.T, key uint16, down bool, modifiers uint64, pid int) uintptr {
	t.Helper()
	var create func(uintptr, uint16, bool) uintptr
	var field func(uintptr, uint32, int64)
	purego.RegisterLibFunc(&create, purego.RTLD_DEFAULT, "CGEventCreateKeyboardEvent")
	purego.RegisterLibFunc(&field, purego.RTLD_DEFAULT, "CGEventSetIntegerValueField")
	event := create(0, key, down)
	if event == 0 {
		t.Fatal("cannot construct an in-memory keyboard event")
	}
	t.Cleanup(func() { cfRelease(event) })
	cgSetEventFlags(event, modifiers)
	field(event, eventTargetPID, int64(pid))
	return event
}

// These opt-in integration tests construct CGEvents only in this process,
// but their callbacks read the real clipboard generation and foreground app.
// They never post events or write the pasteboard; default tests exclude them.
func TestNativeSyntheticEventDecisions(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var create func(uintptr, uint16, bool) uintptr
	var flags func(uintptr, uint64)
	var field func(uintptr, uint32, int64)
	purego.RegisterLibFunc(&create, purego.RTLD_DEFAULT, "CGEventCreateKeyboardEvent")
	purego.RegisterLibFunc(&flags, purego.RTLD_DEFAULT, "CGEventSetFlags")
	purego.RegisterLibFunc(&field, purego.RTLD_DEFAULT, "CGEventSetIntegerValueField")
	down, up := create(0, KeyV, true), create(0, KeyV, false)
	if down == 0 || up == 0 {
		t.Fatal("cannot construct in-memory keyboard events")
	}
	defer cfRelease(down)
	defer cfRelease(up)
	flags(down, ModifierCmd)
	flags(up, 0) // Command released before V: paired up must still be blocked.
	// An unresolvable, nonzero target must not be mistaken for the allowlisted
	// frontmost app. No process receives these events.
	field(down, eventTargetPID, 2147483647)
	field(up, eventTargetPID, 2147483647)
	version := b.ChangeCount()
	b.SetPolicy(version, []string{"local.copiggy.synthetic.allowed"})
	b.enabled.Store(true) // isolated callback state, without a native tap
	if result := b.handleEvent(keyDown, down); result != 0 {
		if b.ChangeCount() != version {
			t.Skip("clipboard changed concurrently; no clipboard writes were performed")
		}
		t.Fatal("disallowed native down must be dropped")
	}
	if result := b.handleEvent(keyUp, up); result != 0 {
		t.Fatal("paired native up must be dropped")
	}
	b.SetPanelVisible(true)
	if result := b.handleEvent(keyDown, down); result != 0 {
		if b.ChangeCount() != version {
			t.Skip("clipboard changed concurrently; no clipboard writes were performed")
		}
		t.Fatal("stale panel focus must not exempt a known external target")
	}
	if result := b.handleEvent(keyUp, up); result != 0 {
		t.Fatal("external target's paired key-up must still be dropped")
	}
	field(down, eventTargetPID, 0)
	if result := b.handleEvent(keyDown, down); result != down {
		t.Fatal("panel editing without a target PID must preserve the original event")
	}
	b.SetPanelVisible(false)
	field(down, eventTargetPID, int64(b.pid))
	if result := b.handleEvent(keyDown, down); result != down {
		t.Fatal("an event addressed to this process must pass")
	}
	field(down, eventTargetPID, 2147483647)
	b.SetPolicy(version-1, []string{"local.copiggy.synthetic.allowed"})
	if result := b.handleEvent(keyDown, down); result != down {
		t.Fatal("a different clipboard generation must pass")
	}
	b.handleEvent(tapDisabledTimeout, 0)
	if b.Enabled() {
		t.Fatal("timeout must visibly disable protection")
	}
}

func TestNativeProtectedCopyNormalizesOneChordAndPairedKeyUp(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	source := b.Frontmost()
	if source.BundleID == "" || source.PID <= 0 || source.PID == b.pid {
		t.Skip("no external foreground application for read-only source identity check")
	}
	b.copies.setActive(true)
	b.enabled.Store(true) // direct callback test only; no tap installed
	b.SetPolicy(7, []string{"old.app"})
	down := memoryKey(t, KeyC, true, ModifierCtrl|ModifierCmd, source.PID)
	up := memoryKey(t, KeyC, false, ModifierCtrl|ModifierCmd, source.PID)
	if result := b.handleEvent(keyDown, down); result != down {
		t.Fatal("protected copy must return its original event")
	}
	first, ok := b.PendingCopy()
	if !ok {
		if current := b.Frontmost(); current.PID != source.PID {
			t.Skip("foreground changed during source identity check")
		}
		t.Fatal("protected chord did not create a pending request")
	}
	if cgEventFlags(down)&ModifierCtrl != 0 || cgEventFlags(down)&ModifierCmd == 0 {
		t.Fatal("protected copy did not become standard Command+C")
	}
	cgSetEventFlags(down, ModifierCtrl|ModifierCmd)
	if b.handleEvent(keyDown, down) != 0 {
		t.Fatal("a held chord delivered a second copy command")
	}
	if current, ok := b.PendingCopy(); !ok || current.ID != first.ID {
		t.Fatal("autorepeat changed the transaction token")
	}
	if b.handleEvent(keyUp, up) != up || cgEventFlags(up)&ModifierCtrl != 0 {
		t.Fatal("matching key-up was not normalized")
	}
	cgSetEventFlags(down, ModifierCtrl|ModifierCmd)
	b.handleEvent(keyDown, down)
	second, ok := b.PendingCopy()
	if !ok || second.ID == first.ID {
		t.Fatal("a second physical chord did not replace the old request")
	}
	cgSetEventFlags(up, ModifierCtrl|ModifierCmd)
	b.handleEvent(keyUp, up)
	plain := memoryKey(t, KeyC, true, ModifierCmd, source.PID)
	if b.handleEvent(keyDown, plain) != plain {
		t.Fatal("ordinary copy was intercepted")
	}
	if _, ok := b.PendingCopy(); ok {
		t.Fatal("ordinary copy left an old protected transaction pending")
	}
	if b.CurrentPolicy().Version != 7 {
		t.Fatal("cancelled pending copy erased the old policy")
	}
}

func TestNativePendingCopyTemporarilyBlocksOtherAppsAndCancelsOnFocus(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	b.copies.setActive(true)
	b.enabled.Store(true)
	baseline := b.ChangeCount()
	request, _ := b.copies.begin(pasteboard.App{BundleID: "synthetic.source", PID: 123}, baseline, time.Now())
	down := memoryKey(t, KeyV, true, ModifierCmd, 2147483647)
	up := memoryKey(t, KeyV, false, 0, 2147483647)
	if b.handleEvent(keyDown, down) != 0 || b.handleEvent(keyUp, up) != 0 {
		t.Fatal("pending copy allowed an unconfirmed paste into another application")
	}
	if current, ok := b.PendingCopy(); !ok || current.ID != request.ID {
		t.Fatal("a blocked paste incorrectly cancelled its pending protection")
	}
	b.SetPanelVisible(true)
	if _, ok := b.PendingCopy(); ok {
		t.Fatal("opening a nonactivating panel left the old source request valid")
	}
	b.SetPanelVisible(false)
	b.copies.begin(pasteboard.App{BundleID: "synthetic.source", PID: 123}, baseline, time.Now())
	cut := memoryKey(t, KeyX, true, ModifierCmd, 2147483647)
	if b.handleEvent(keyDown, cut) != cut {
		t.Fatal("ordinary cut was intercepted")
	}
	if _, ok := b.PendingCopy(); ok {
		t.Fatal("ordinary cut did not cancel the old pending request")
	}
}

func TestNativePendingCopyKeepsProtectionForKnownExternalTargetWithStalePanelFocus(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	b.copies.setActive(true)
	b.enabled.Store(true)
	b.SetPanelVisible(true)
	// A native source event can arrive before the main-thread focus snapshot
	// catches up. Starting the request after that snapshot avoids modelling an
	// actual new panel focus, which intentionally cancels pending copies.
	request, _ := b.copies.begin(pasteboard.App{BundleID: "synthetic.source", PID: 123}, b.ChangeCount(), time.Now())
	down := memoryKey(t, KeyV, true, ModifierCmd, 2147483647)
	up := memoryKey(t, KeyV, false, 0, 2147483647)
	if b.handleEvent(keyDown, down) != 0 || b.handleEvent(keyUp, up) != 0 {
		t.Fatal("stale panel focus allowed a pending copy into a known external application")
	}
	if current, ok := b.PendingCopy(); !ok || current.ID != request.ID {
		t.Fatal("external recipient was mistaken for panel input and cancelled the pending copy")
	}
}

func TestNativePendingCopyExpiryAndGuardStopKeepOldPolicy(t *testing.T) {
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	b.copies.setActive(true)
	b.enabled.Store(true)
	b.SetPolicy(7, []string{"old.app"})
	source := pasteboard.App{BundleID: "synthetic.source", PID: 123}
	b.copies.begin(source, 10, time.Now().Add(-ProtectedCopyTimeout))
	if _, ok := b.PendingCopy(); ok {
		t.Fatal("expired request was returned to the caller")
	}
	b.copies.begin(source, 10, time.Now())
	b.handleEvent(tapDisabledTimeout, 0)
	if _, ok := b.PendingCopy(); ok {
		t.Fatal("system-disabled guard left a pending request")
	}
	if b.CurrentPolicy().Version != 7 {
		t.Fatal("expiry or shutdown cleared an older confirmed policy")
	}
}
