package nativeui

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNativePinMascotPressesThenRests(t *testing.T) {
	for _, sample := range []struct {
		at    time.Duration
		frame string
		next  time.Duration
	}{
		{-time.Second, "pin-ready", 1800 * time.Millisecond},
		{0, "pin-ready", 1800 * time.Millisecond},
		{1799 * time.Millisecond, "pin-ready", time.Millisecond},
		{1800 * time.Millisecond, "pin-pressed", 1500 * time.Millisecond},
		{3299 * time.Millisecond, "pin-pressed", time.Millisecond},
		{3300 * time.Millisecond, "pin-ready", 12500 * time.Millisecond},
		{14 * time.Second, "pin-ready", 1800 * time.Millisecond},
		{15800 * time.Millisecond, "pin-pressed", 1500 * time.Millisecond},
	} {
		frame, next := nativePinMascotFrame(sample.at, false)
		if frame != sample.frame || next != sample.next {
			t.Errorf("at %v: got %q, next %v; want %q, next %v", sample.at, frame, next, sample.frame, sample.next)
		}
		frame, next = nativePinMascotFrame(sample.at, true)
		if frame != "pin-pressed" || next != 0 {
			t.Fatal("Reduce Motion must keep the completed pin pose without scheduled frames")
		}
	}
}

func TestNativePinMascotPreservesGeometryAndReducedMotion(t *testing.T) {
	for _, name := range []string{"pin-ready", "pin-pressed"} {
		if pinMascotBitmaps()[name] == nil {
			t.Fatalf("missing native pin mascot frame %q", name)
		}
	}
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativePinMascot, nativeMascotSize, nativeMascotSize)
	ready := bytes.Clone(tt.Image().Pix)
	bounds, ok := tt.Find("Copiggy")
	if !ok || bounds.W != nativeMascotSize || bounds.H != nativeMascotSize {
		t.Fatalf("missing or incorrectly sized pin mascot: %+v", bounds)
	}
	tt.Advance(1800 * time.Millisecond)
	pressed := bytes.Clone(tt.Image().Pix)
	if bytes.Equal(ready, pressed) {
		t.Fatal("pin mascot did not show the pressed pose")
	}
	pressedBounds, _ := tt.Find("Copiggy")
	if pressedBounds != bounds {
		t.Fatal("pinning moved the empty-state layout")
	}
	tt.Advance(1500 * time.Millisecond)
	if !bytes.Equal(ready, tt.Image().Pix) {
		t.Fatal("pin mascot did not return to its resting pose")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if !bytes.Equal(pressed, tt.Image().Pix) {
		t.Fatal("Reduce Motion did not display the completed pin pose")
	}
	tt.Advance(20 * time.Second)
	if !bytes.Equal(pressed, tt.Image().Pix) {
		t.Fatal("pin mascot animated while Reduce Motion was enabled")
	}
}
