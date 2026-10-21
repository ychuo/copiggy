package nativeui

import (
	"sync"
	"time"

	"copiggy/resources"
	"github.com/egoist/mygo/ui"
)

const nativeMascotSize = 112

type nativeMascotWriting struct {
	Lengths [3]float32
	Opacity float32
}

var mascotBitmaps = sync.OnceValue(func() map[string]*ui.Bitmap {
	images := make(map[string]*ui.Bitmap, 2)
	for _, name := range []string{"empty-state", "empty-state-blink"} {
		data, err := resources.Brand.ReadFile("brand/" + name + ".png")
		if err == nil {
			images[name], _ = ui.DecodeBitmap(data)
		}
	}
	return images
})

// Spend almost all idle time resting. Only the boundaries of the brief blinks
// request another native frame; no display-rate loop runs while nothing moves.
func nativeMascotFrame(elapsed time.Duration, reducedMotion bool) (string, time.Duration) {
	if reducedMotion {
		return "empty-state", 0
	}
	const cycle = 22 * time.Second
	if elapsed < 0 {
		elapsed = 0
	}
	phase := elapsed % cycle
	blinks := [...]struct{ start, end time.Duration }{
		{6 * time.Second, 6140 * time.Millisecond},
		{15800 * time.Millisecond, 15940 * time.Millisecond},
		{16100 * time.Millisecond, 16220 * time.Millisecond},
	}
	for _, blink := range blinks {
		if phase < blink.start {
			return "empty-state", blink.start - phase
		}
		if phase < blink.end {
			return "empty-state-blink", blink.end - phase
		}
	}
	return "empty-state", cycle - phase + blinks[0].start
}

// The clipboard fills in short, separate gestures, then rests before clearing.
// Fractions describe how much of each line is revealed, independent of size.
func nativeMascotWritingAt(elapsed time.Duration, reducedMotion bool) (nativeMascotWriting, time.Duration) {
	var writing nativeMascotWriting
	if reducedMotion {
		return writing, 0
	}
	const (
		cycle = 10 * time.Second
		rest  = 2 * time.Second
		draw  = 350 * time.Millisecond
		gap   = 500 * time.Millisecond
		hold  = 2 * time.Second
		fade  = 350 * time.Millisecond
		frame = time.Second / 60
	)
	phase := max(elapsed, 0) % cycle
	writing.Opacity = 1
	for i := range writing.Lengths {
		start := rest + time.Duration(i)*(draw+gap)
		if phase < start {
			return writing, start - phase
		}
		if phase < start+draw {
			writing.Lengths[i] = float32(phase-start) / float32(draw)
			return writing, min(frame, start+draw-phase)
		}
		writing.Lengths[i] = 1
	}
	clearAt := rest + 2*(draw+gap) + draw + hold
	if phase < clearAt {
		return writing, clearAt - phase
	}
	if phase < clearAt+fade {
		writing.Opacity = 1 - float32(phase-clearAt)/float32(fade)
		return writing, min(frame, clearAt+fade-phase)
	}
	return nativeMascotWriting{}, cycle - phase + rest
}

func paintNativeMascotWriting(painter *ui.Painter, bounds ui.Rect, writing nativeMascotWriting) {
	// The pig holds its clipboard at an angle. Keep the writing parallel to
	// that paper, inset from the clip and both paws, in the square asset space.
	widths := [...]float32{.160, .147, .136}
	ink := ui.Hex("#74432b").Alpha(.90 * writing.Opacity)
	height := bounds.H * .0235
	for i, fraction := range writing.Lengths {
		if fraction <= 0 || writing.Opacity <= 0 {
			continue
		}
		line := ui.Rect{
			X: bounds.X + bounds.W*(.59-float32(i)*.01),
			Y: bounds.Y + bounds.H*(.54+float32(i)*.06),
			W: bounds.W * widths[i] * fraction,
			H: height,
		}
		painter.Transform(ui.Rotation(12), line.X, line.Y, func() {
			painter.Fill(line, ink, height/2)
		})
	}
}

func (s *application) nativeMascot(c *ui.Context) {
	state := s.nativeState()
	if state.EmptyAnimationStart.IsZero() {
		state.EmptyAnimationStart = c.Now()
	}
	elapsed, reducedMotion := c.Now().Sub(state.EmptyAnimationStart), c.Preferences().ReduceMotion
	frame, next := nativeMascotFrame(elapsed, reducedMotion)
	writing, writingNext := nativeMascotWritingAt(elapsed, reducedMotion)
	if writingNext > 0 && (next == 0 || writingNext < next) {
		next = writingNext
	}
	if next > 0 {
		c.After(next)
	}
	ui.Image(c, mascotBitmaps()[frame]).Key("empty-mascot").Size(nativeMascotSize, nativeMascotSize).Fit(ui.Contain).Shrink(0).Label("Copiggy").DrawOver(func(painter *ui.Painter, bounds ui.Rect) {
		paintNativeMascotWriting(painter, bounds, writing)
	})
}
