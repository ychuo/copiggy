package nativeui

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Render only an in-memory material fixture. No app window or desktop capture.
func renderImageGlassFixture(dark bool, backdrop func(*ui.Painter, ui.Rect), foreground bool) *image.RGBA {
	material, ink := nativeImageGlass(dark)
	return ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			ui.Box(c).Absolute().Fill().Draw(backdrop)
			ui.Box(c).Absolute().Left(12).Top(12).Width(96).Height(28).Radius(6).Material(material).Children(func() {
				if foreground {
					ui.Box(c).Absolute().Left(46).Top(10).Size(4, 8).Background(ink)
				}
			})
		})
	}, 120, 52, 1)
}

func TestNativeImageGlassKeepsMetadataReadableAcrossImages(t *testing.T) {
	backgrounds := []ui.Color{ui.RGB(255, 0, 0), ui.RGB(0, 255, 0), ui.RGB(0, 0, 255)}
	for level := 0; level <= 255; level += 15 {
		v := uint8(level)
		backgrounds = append(backgrounds, ui.RGB(v, v, v))
	}
	for _, dark := range []bool{false, true} {
		_, ink := nativeImageGlass(dark)
		inkLum := glassTestLuminance(float64(ink.R)/255, float64(ink.G)/255, float64(ink.B)/255)
		for _, background := range backgrounds {
			img := renderImageGlassFixture(dark, func(p *ui.Painter, r ui.Rect) { p.Fill(r, background, 0) }, false)
			pixel := img.RGBAAt(60, 26)
			lum := glassTestLuminance(float64(pixel.R)/255, float64(pixel.G)/255, float64(pixel.B)/255)
			contrast := (max(inkLum, lum) + .05) / (min(inkLum, lum) + .05)
			if contrast < 4.5 {
				t.Fatalf("theme dark=%v, background=%v: metadata contrast %.2f < 4.5", dark, background, contrast)
			}
			if got := img.RGBAAt(4, 26); got != (color.RGBA{background.R, background.G, background.B, 255}) {
				t.Fatal("glass changed image pixels outside the small metadata chip")
			}
		}
	}
}

func TestNativeImageGlassBlursBackdropButKeepsForegroundSharp(t *testing.T) {
	stripes := func(p *ui.Painter, r ui.Rect) {
		for x := float32(0); x < r.W; x += 2 {
			shade := ui.RGB(0, 0, 0)
			if int(x)%4 == 0 {
				shade = ui.RGB(255, 255, 255)
			}
			p.Fill(ui.Rect{X: r.X + x, Y: r.Y, W: 2, H: r.H}, shade, 0)
		}
	}
	for _, dark := range []bool{false, true} {
		img := renderImageGlassFixture(dark, stripes, false)
		// A tint alone preserves the alternating stripes. Real blur nearly
		// eliminates that local difference, while keeping their average light.
		a, b := img.RGBAAt(46, 26), img.RGBAAt(48, 26)
		if math.Abs(float64(a.R)-float64(b.R)) > 2 {
			t.Fatalf("theme dark=%v: backdrop stripes remain sharp: %v, %v", dark, a, b)
		}
		sharp := renderImageGlassFixture(dark, stripes, true)
		_, ink := nativeImageGlass(dark)
		if got := sharp.RGBAAt(60, 26); got != (color.RGBA{ink.R, ink.G, ink.B, 255}) {
			t.Fatalf("theme dark=%v: material blurred or recolored foreground: %v", dark, got)
		}
		if got := sharp.RGBAAt(56, 26); got != img.RGBAAt(56, 26) {
			t.Fatal("foreground leaked a blur into its surrounding backdrop")
		}
	}
}

func glassTestLuminance(r, g, b float64) float64 {
	linear := func(v float64) float64 {
		if v <= .04045 {
			return v / 12.92
		}
		return math.Pow((v+.055)/1.055, 2.4)
	}
	return .2126*linear(r) + .7152*linear(g) + .0722*linear(b)
}
