package nativeui

import (
	"sync"
	"time"

	"copiggy/resources"
	"github.com/egoist/mygo/ui"
)

var pinMascotBitmaps = sync.OnceValue(func() map[string]*ui.Bitmap {
	images := make(map[string]*ui.Bitmap, 2)
	for _, name := range []string{"pin-ready", "pin-pressed"} {
		data, err := resources.Brand.ReadFile("brand/" + name + ".png")
		if err == nil {
			images[name], _ = ui.DecodeBitmap(data)
		}
	}
	return images
})

// A brief push demonstrates pinning, followed by a long rest. Frame changes
// alone schedule redraws; the pinned-history empty state has no idle frame loop.
func nativePinMascotFrame(elapsed time.Duration, reducedMotion bool) (string, time.Duration) {
	if reducedMotion {
		return "pin-pressed", 0
	}
	const (
		cycle   = 14 * time.Second
		pressAt = 1800 * time.Millisecond
		restAt  = 3300 * time.Millisecond
	)
	phase := max(elapsed, 0) % cycle
	switch {
	case phase < pressAt:
		return "pin-ready", pressAt - phase
	case phase < restAt:
		return "pin-pressed", restAt - phase
	default:
		return "pin-ready", cycle - phase + pressAt
	}
}

func (s *application) nativePinMascot(c *ui.Context) {
	state := s.nativeState()
	if state.PinAnimationStart.IsZero() {
		state.PinAnimationStart = c.Now()
	}
	frame, next := nativePinMascotFrame(c.Now().Sub(state.PinAnimationStart), c.Preferences().ReduceMotion)
	if next > 0 {
		c.After(next)
	}
	ui.Image(c, pinMascotBitmaps()[frame]).Key("pin-mascot").Size(nativeMascotSize, nativeMascotSize).Fit(ui.Contain).Shrink(0).Label("Copiggy")
}
