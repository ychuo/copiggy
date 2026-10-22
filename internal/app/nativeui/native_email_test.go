package nativeui

import (
	"image"
	"math"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func TestEmailCardCentersAddressWithinSquare(t *testing.T) {
	for _, raw := range []string{"hello@example.com", "First.Last+tag@EXAMPLE.com", "a.longer.account.name@example.com"} {
		for _, dark := range []bool{false, true} {
			s := nativeViewFixture(t)
			s.viewState.Dark = dark
			item := clipmodel.Item{ID: "email", Type: clipmodel.TypeEmail, Text: raw, Pinned: true}
			value, p := clipmodel.Analyze(raw), panelColors(dark)
			background, foreground, metadata := nativeCardPalette(item.Type, value, p)
			tt := ui.NewTester(func(c *ui.Context) {
				setNativeTheme(c, p, dark)
				card := ui.Box(c).Fill().Radius(nativeCardRadius).Clip().Background(background)
				card.Children(func() {
					s.nativeCardContentWithAssets(c, card, item, value, foreground, metadata, p, nativeCardAssets{})
				})
			}, nativeCardSize, nativeCardSize)
			local, localFound := tt.Find(value.Email.Local)
			domain, domainFound := tt.Find("@" + value.Email.Domain)
			if !localFound || !domainFound {
				t.Fatalf("address content missing for %q", raw)
			}
			for _, text := range []ui.Rect{local, domain} {
				if center := text.X + text.W/2; math.Abs(float64(center-nativeCardSize/2)) > .5 {
					t.Fatalf("address is not horizontally centered: %q %+v", raw, text)
				}
			}
			if center := (local.Y + domain.Y + domain.H) / 2; math.Abs(float64(center-nativeCardSize/2)) > .5 {
				t.Fatalf("address is not vertically centered: %q local=%+v domain=%+v center=%g", raw, local, domain, center)
			}
		}
	}
}

func TestEmailEnvelopeHasNoHorizontalCreaseAcrossPaper(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(nativeCardSize, nativeCardSize).Draw(func(p *ui.Painter, r ui.Rect) {
			nativeEmailArtwork(p, r)
		})
	}, nativeCardSize, nativeCardSize)
	// This is an in-memory paint check, not a desktop screenshot. At the
	// shoulder height the middle of the letter is still uninterrupted
	// paper; only its far left and right corners meet the diagonal folds.
	pixels := tt.Image()
	for x := 72; x <= 116; x += 11 {
		above := pixels.RGBAAt(x, 110)
		for y := 111; y <= 115; y++ {
			pixel := pixels.RGBAAt(x, y)
			for channel, delta := range []int{int(pixel.R) - int(above.R), int(pixel.G) - int(above.G), int(pixel.B) - int(above.B)} {
				if delta < -8 || delta > 8 {
					t.Fatalf("horizontal crease through paper: x=%d y=%d channel=%d delta=%d", x, y, channel, delta)
				}
			}
		}
	}
}

// Switching appearance must never put dark-mode white text on the envelope or
// recolor the reading paper. These renders stay in memory and touch no desktop.
func TestEmailReadingSurfacesStayStableAcrossAppearance(t *testing.T) {
	var cardPixels, previewPixels [2]*image.RGBA
	for index, dark := range []bool{false, true} {
		s, item := semanticPreviewFixture(t, "hello@example.com")
		s.viewState.Dark = dark
		tray := ui.NewTester(s.nativeView, 1140, nativeTrayHeight)
		card, ok := tray.Find(s.nativeCardLabel(item))
		if !ok {
			t.Fatal("email card missing")
		}
		// Exclude the themed selection outline and outside rounded corners.
		r := image.Rect(int(card.X)+20, int(card.Y)+20, int(card.X+card.W)-20, int(card.Y+card.H)-20)
		cardPixels[index] = tray.Image().SubImage(r).(*image.RGBA)
		preview := ui.NewTester(s.nativePopoverView, 520, nativeEmailPreviewHeight)
		previewPixels[index] = preview.Image().SubImage(image.Rect(24, 64, 496, 240)).(*image.RGBA)
	}
	for name, samples := range map[string][2]*image.RGBA{"card": cardPixels, "preview": previewPixels} {
		light, dark := samples[0], samples[1]
		for y := 0; y < light.Rect.Dy(); y++ {
			for x := 0; x < light.Rect.Dx(); x++ {
				a := light.RGBAAt(light.Rect.Min.X+x, light.Rect.Min.Y+y)
				b := dark.RGBAAt(dark.Rect.Min.X+x, dark.Rect.Min.Y+y)
				if a != b {
					t.Fatalf("%s inherits chrome appearance at (%d,%d): light=%v dark=%v", name, x, y, a, b)
				}
			}
		}
	}
}
