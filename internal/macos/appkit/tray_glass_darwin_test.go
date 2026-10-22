//go:build darwin

package appkit

import (
	"math"
	"testing"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego/objc"
)

// Only construct an in-memory NSColor. This checks the floating-point bridge
// without constructing a window, sampling the desktop, or rendering glass.
func TestTrayGlassTintIsSubtleNeutralAndClearedForDark(t *testing.T) {
	if err := initGlassNative(); err != nil {
		t.Fatal(err)
	}
	nativeobjc.WithPool(func() {
		color := trayGlassTintColor(false)
		if color == 0 {
			t.Fatal("light glass tint was not created")
		}
		white := glassNative.getFloat(color, objc.RegisterName("whiteComponent"))
		alpha := glassNative.getFloat(color, objc.RegisterName("alphaComponent"))
		if math.Abs(white-.55) > 1e-6 || math.Abs(alpha-.14) > 1e-6 {
			t.Fatalf("native neutral tint lost its components: white=%v alpha=%v", white, alpha)
		}
		if got := trayGlassTintColor(true); got != 0 {
			t.Fatal("dark appearance retained a light-mode tint")
		}
	})
}

// Ordinary frames must not allocate another native color. No AppKit window
// is involved; the missing effect pointer is never reached on this cache hit.
func TestTrayGlassCachedLightTintDoesNotAllocateColor(t *testing.T) {
	create := glassNative.colorWhite
	defer func() { glassNative.colorWhite = create }()
	called := false
	glassNative.colorWhite = func(uintptr, objc.SEL, float64, float64) uintptr {
		called = true
		return 0
	}
	attachment := &trayGlassAttachment{tintSet: true, tintDark: false}
	updateTrayGlassTint(attachment, false)
	if called {
		t.Fatal("unchanged light appearance allocated another tint")
	}
}

// Malformed geometry must fail before dereferencing a native pointer or
// dispatching into an application loop. No window is constructed by these tests.
func TestTrayGlassRejectsUnsafeInputsBeforeDispatch(t *testing.T) {
	for _, radius := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), 1e7} {
		if _, err := SetTrayGlass(1, false, radius); err == nil {
			t.Fatalf("accepted unsafe radius %v", radius)
		}
	}
	if _, err := SetTrayGlass(0, false, 28); err == nil {
		t.Fatal("missing window must be rejected")
	}
	if err := SetNativeWindowAppearance(0, true); err == nil {
		t.Fatal("a popover without a native window cannot receive appearance")
	}
	if err := ClearTrayGlass(0); err != nil {
		t.Fatalf("clearing a missing window should be harmless: %v", err)
	}
}

func TestTrayGlassFitsRadiusAfterWindowShrinks(t *testing.T) {
	for _, tc := range []struct {
		width, height, radius, want float64
	}{
		{1180, 366, 28, 28},
		{40, 366, 28, 20},
		{1180, 30, 28, 15},
		{40, 30, 0, 0},
	} {
		got, err := trayGlassRadius(glassRect{Size: glassSize{tc.width, tc.height}}, tc.radius)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: radius %v, error %v", tc, got, err)
		}
	}
}

func TestTrayGlassRejectsInvalidNativeBounds(t *testing.T) {
	for _, frame := range []glassRect{
		{},
		{Size: glassSize{-1, 366}},
		{Size: glassSize{1180, math.NaN()}},
		{Size: glassSize{math.Inf(1), 366}},
		{Origin: glassPoint{math.NaN(), 0}, Size: glassSize{1180, 366}},
	} {
		if _, err := trayGlassRadius(frame, 28); err == nil {
			t.Fatalf("accepted unsafe native bounds %+v", frame)
		}
	}
}
