package nativeui

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNativeMascotWritingRevealsInOrderAndRests(t *testing.T) {
	for _, sample := range []struct {
		at      time.Duration
		lengths [3]float32
		opacity float32
	}{
		{0, [3]float32{}, 1},
		{2175 * time.Millisecond, [3]float32{.5, 0, 0}, 1},
		{2500 * time.Millisecond, [3]float32{1, 0, 0}, 1},
		{3025 * time.Millisecond, [3]float32{1, .5, 0}, 1},
		{3400 * time.Millisecond, [3]float32{1, 1, 0}, 1},
		{3875 * time.Millisecond, [3]float32{1, 1, .5}, 1},
		{4050 * time.Millisecond, [3]float32{1, 1, 1}, 1},
		{6225 * time.Millisecond, [3]float32{1, 1, 1}, .5},
		{6400 * time.Millisecond, [3]float32{}, 0},
		{12175 * time.Millisecond, [3]float32{.5, 0, 0}, 1},
	} {
		writing, next := nativeMascotWritingAt(sample.at, false)
		if writing.Lengths != sample.lengths || math.Abs(float64(writing.Opacity-sample.opacity)) > .001 || next <= 0 {
			t.Errorf("at %v: got %+v (next %v), want %v, opacity %v", sample.at, writing, next, sample.lengths, sample.opacity)
		}
		writing, next = nativeMascotWritingAt(sample.at, true)
		if writing != (nativeMascotWriting{}) || next != 0 {
			t.Fatal("Reduce Motion must leave the clipboard still with no scheduled frames")
		}
	}
	for _, sample := range []struct{ at, sleep time.Duration }{
		{0, 2 * time.Second},
		{2350 * time.Millisecond, 500 * time.Millisecond},
		{4050 * time.Millisecond, 2 * time.Second},
		{6400 * time.Millisecond, 5600 * time.Millisecond},
	} {
		_, next := nativeMascotWritingAt(sample.at, false)
		if next != sample.sleep {
			t.Errorf("at %v: idle redraw scheduled after %v; want %v", sample.at, next, sample.sleep)
		}
	}
}

func TestNativeMascotWritingStaysOnTheClipboard(t *testing.T) {
	s := nativeViewFixture(t)
	tt := ui.NewTester(s.nativeMascot, nativeMascotSize, nativeMascotSize)
	resting := bytes.Clone(tt.Image().Pix)
	tt.Advance(5 * time.Second)
	painted := tt.Image()
	changed := 0
	for y := range nativeMascotSize {
		for x := range nativeMascotSize {
			i := painted.PixOffset(x, y)
			if bytes.Equal(resting[i:i+4], painted.Pix[i:i+4]) {
				continue
			}
			changed++
			if x < 62 || x > 85 || y < 59 || y > 83 {
				t.Fatalf("clipboard writing escaped the paper at %d,%d", x, y)
			}
			// Inspect the unmodified image under every changed pixel: the
			// writing must stay on cream paper, away from peach paws/dark edges.
			red, green, blue, alpha := resting[i], resting[i+1], resting[i+2], resting[i+3]
			if red < 210 || green < 190 || blue < 140 || red < green || green < blue || alpha != 255 {
				t.Fatalf("writing crossed the paper at %d,%d (base RGBA %d,%d,%d,%d)", x, y, red, green, blue, alpha)
			}
		}
	}
	if changed == 0 {
		t.Fatal("native overlay did not render the clipboard lines")
	}
	if bounds, ok := tt.Find("Copiggy"); !ok || bounds.W != nativeMascotSize || bounds.H != nativeMascotSize {
		t.Fatal("clipboard writing changed mascot geometry")
	}
}

func TestNativeMascotWritingFollowsTheTiltedBoard(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		setNativeTheme(c, panelColors(false), false)
		ui.Box(c).Fill().DrawOver(func(painter *ui.Painter, bounds ui.Rect) {
			paintNativeMascotWriting(painter, bounds, nativeMascotWriting{Lengths: [3]float32{1}, Opacity: 1})
		})
	}, nativeMascotSize, nativeMascotSize)
	pixels := tt.Image()
	centroidY := func(left, right int) float64 {
		var sum, weight float64
		for y := range nativeMascotSize {
			for x := left; x < right; x++ {
				alpha := float64(pixels.RGBAAt(x, y).A)
				sum += float64(y) * alpha
				weight += alpha
			}
		}
		if weight == 0 {
			t.Fatalf("missing stroke pixels between x=%d and x=%d", left, right)
		}
		return sum / weight
	}
	if fall := centroidY(79, 83) - centroidY(66, 70); fall < 2 || fall > 4 {
		t.Fatalf("clipboard line does not follow the board's downward tilt: %.2fpx", fall)
	}
}

func TestNativeMascotRestsBetweenBriefBlinks(t *testing.T) {
	for _, sample := range []struct {
		at    time.Duration
		frame string
	}{
		{0, "empty-state"},
		{5999 * time.Millisecond, "empty-state"},
		{6 * time.Second, "empty-state-blink"},
		{6140 * time.Millisecond, "empty-state"},
		{15800 * time.Millisecond, "empty-state-blink"},
		{15940 * time.Millisecond, "empty-state"},
		{16100 * time.Millisecond, "empty-state-blink"},
		{16220 * time.Millisecond, "empty-state"},
		{28 * time.Second, "empty-state-blink"},
	} {
		frame, next := nativeMascotFrame(sample.at, false)
		if frame != sample.frame || next <= 0 {
			t.Errorf("at %v: got %q, next %v; want %q", sample.at, frame, next, sample.frame)
		}
		if mascotBitmaps()[frame] == nil {
			t.Fatalf("missing native mascot frame %q", frame)
		}
		frame, next = nativeMascotFrame(sample.at, true)
		if frame != "empty-state" || next != 0 {
			t.Fatal("Reduce Motion must hold the resting pose without scheduling frames")
		}
	}
	_, wait := nativeMascotFrame(0, false)
	if wait != 6*time.Second {
		t.Fatalf("idle mascot requests frames before its first blink: %v", wait)
	}
}

func TestNativeEmptyMascotKeepsLayoutAndHonorsReducedMotion(t *testing.T) {
	s := nativeViewFixture(t)
	s.language = "en"
	tt := ui.NewTester(func(c *ui.Context) { s.nativeEmpty(c, panelColors(false)) }, 400, nativeCardSize)
	resting := bytes.Clone(tt.Image().Pix)
	bounds, ok := tt.Find("Copiggy")
	if !ok || bounds.W != nativeMascotSize || bounds.H != nativeMascotSize {
		t.Fatalf("missing or incorrectly sized mascot: %+v", bounds)
	}
	tt.Advance(6 * time.Second)
	if bytes.Equal(resting, tt.Image().Pix) {
		t.Fatal("native empty-state mascot did not blink")
	}
	blinkingBounds, _ := tt.Find("Copiggy")
	if blinkingBounds != bounds {
		t.Fatal("blink moved the empty-state layout")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if !bytes.Equal(resting, tt.Image().Pix) {
		t.Fatal("Reduce Motion did not restore the resting pose")
	}
	tt.Advance(10 * time.Second)
	if !bytes.Equal(resting, tt.Image().Pix) {
		t.Fatal("mascot animated while Reduce Motion was enabled")
	}
}
