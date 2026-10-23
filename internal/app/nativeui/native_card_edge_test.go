package nativeui

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// Inspect only in-memory native cards, never the desktop or real clipboard.
// Each opaque card must apply its outer coverage once, regardless of how many
// artwork layers paint inside it. Repeated coverage produces stair-stepped,
// overly opaque corners and a pale fringe where colored artwork covers paper.
func TestNativeCardOuterCoverageIsIndependentOfArtworkLayers(t *testing.T) {
	for _, raw := range []string{"2026-10-22T15:30:00+08:00", "hello@example.com", "+1 (202) 555-0148", "#D78379", "image"} {
		for _, scale := range []float32{1, 2} {
			for _, selected := range []bool{false, true} {
				for _, offset := range []float32{0, .25, .75} {
					t.Run(fmt.Sprintf("%s/%gx/selected=%t/offset=%g", raw, scale, selected, offset), func(t *testing.T) {
						var s *application
						var item clipmodel.Item
						if raw == "image" {
							s, item = nativeImagePreviewFixture(t, nativeCardSize, nativeCardSize, color.RGBA{R: 100, G: 140, B: 180, A: 255})
						} else {
							s, item = semanticPreviewFixture(t, raw)
						}
						s.chosen = -1
						if selected {
							s.chosen = 0
						}
						const size = nativeCardSize + 2
						got := ui.Render(func(c *ui.Context) {
							c.Root().Background(ui.Transparent)
							ui.Row(c).Padding(0, offset).Children(func() { s.nativeCard(c, 0, item, panelColors(false)) })
						}, size, size, scale)
						want := ui.Render(func(c *ui.Context) {
							c.Root().Background(ui.Transparent)
							ui.Row(c).Padding(0, offset).Children(func() {
								ink := ui.Hex("#ffffff")
								if selected {
									ink = panelColors(false).selection
								}
								box := ui.Box(c).Size(nativeCardSize, nativeCardSize).Radius(nativeCardRadius).Background(ink)
								nativeCardDepth(box, false)
							})
						}, size, size, scale)
						for y := range got.Bounds().Dy() {
							for x := range got.Bounds().Dx() {
								a, b := got.RGBAAt(x, y).A, want.RGBAAt(x, y).A
								if int(a)-int(b) > 2 || int(b)-int(a) > 2 {
									t.Fatalf("edge coverage at (%d,%d): got %d, single contour %d", x, y, a, b)
								}
								if selected && a > 0 && a < 255 {
									pixel, blue := got.RGBAAt(x, y), want.RGBAAt(x, y)
									for _, pair := range [][2]uint8{{pixel.R, blue.R}, {pixel.G, blue.G}, {pixel.B, blue.B}} {
										delta := int(pair[0]) - int(pair[1])
										if delta < -3 || delta > 3 {
											t.Fatalf("paper rim at selection contour (%d,%d): %v, expected blue %v at alpha %d", x, y, pixel, blue, a)
										}
									}
								}
							}
						}
						if selected {
							// The selection still meets the top contour, with no
							// exposed paper rim from an inset or a moved layer.
							ink := got.RGBAAt(int((nativeCardSize/2+offset)*scale), int(scale))
							selection := panelColors(false).selection
							if ink != (color.RGBA{R: selection.R, G: selection.G, B: selection.B, A: 255}) {
								t.Fatalf("selection no longer meets outer edge: %v", ink)
							}
						}
					})
				}
			}
		}
	}
}

func TestNativeCardDepthStaysOutsideContentAndFitsStrip(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scale := range []float32{1, 2} {
			s, item := semanticPreviewFixture(t, "#808080")
			s.chosen = -1
			s.nativeState().Dark = dark
			const inset = 22
			pixels := ui.Render(func(c *ui.Context) {
				c.Root().Background(ui.Transparent)
				ui.Row(c).Padding(4, inset, 14).Children(func() { s.nativeCard(c, 0, item, panelColors(dark)) })
			}, nativeCardSize+2*inset, nativeCardSize+18, scale)
			at := func(x, y float32) color.RGBA { return pixels.RGBAAt(int(x*scale), int(y*scale)) }
			x := float32(inset + nativeCardSize/2)
			if center := at(x, 4+nativeCardSize/2+30); center != (color.RGBA{128, 128, 128, 255}) {
				t.Fatalf("depth tinted the swatch at %gx, dark=%t: %v", scale, dark, center)
			}
			for _, point := range []ui.Point{
				{X: x, Y: 4}, {X: x, Y: 4 + nativeCardSize - 1/scale},
				{X: inset, Y: 4 + nativeCardSize/2}, {X: inset + nativeCardSize - 1/scale, Y: 4 + nativeCardSize/2},
			} {
				if edge := at(point.X, point.Y); edge != (color.RGBA{128, 128, 128, 255}) {
					t.Fatalf("reflection should leave side midpoints unchanged at %gx: %v: %v", scale, point, edge)
				}
			}
			near, far := at(x, 4+nativeCardSize+1).A, at(x, 4+nativeCardSize+7).A
			if near == 0 || near > 20 || near <= far {
				t.Fatalf("shadow should be light and soften away from card at %gx: near=%d far=%d", scale, near, far)
			}
			for y := 0; y < pixels.Bounds().Dy(); y++ {
				for x := 0; x < pixels.Bounds().Dx(); x++ {
					if y != 0 && y != pixels.Bounds().Dy()-1 {
						continue
					}
					if a := pixels.RGBAAt(x, y).A; a > 1 {
						t.Fatalf("shadow cut off at strip boundary (%d,%d), %gx: alpha=%d", x, y, scale, a)
					}
				}
			}
		}
	}
}

func TestNativeCardEdgeLightIsLimitedToOppositeCorners(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		render := func(light bool) *image.RGBA {
			return ui.Render(func(c *ui.Context) {
				c.Root().Background(ui.Transparent)
				ui.Box(c).Fill().Radius(nativeCardRadius).Clip().Children(func() {
					ui.Box(c).Fill().Background(ui.Hex("#808080")).Effect(nativeCardComposite{}).Children(func() {
						if light {
							nativeCardEdgeLight(c)
						}
					})
				})
			}, nativeCardSize, nativeCardSize, scale)
		}
		lit, plain := render(true), render(false)
		upperLeft, lowerRight := 0, 0
		for y := range lit.Bounds().Dy() {
			for x := range lit.Bounds().Dx() {
				delta := int(lit.RGBAAt(x, y).R) - int(plain.RGBAAt(x, y).R)
				if delta == 0 {
					continue
				}
				if delta < 0 {
					t.Fatalf("corner reflection darkened the face at %gx (%d,%d)", scale, x, y)
				}
				fx, fy := float32(x)/scale, float32(y)/scale
				switch {
				case fx < 44 && fy < 44:
					upperLeft += delta
				case fx >= nativeCardSize-44 && fy >= nativeCardSize-44:
					lowerRight += delta
				default:
					t.Fatalf("reflection escaped the two corner regions at %gx (%d,%d)", scale, x, y)
				}
			}
		}
		if lowerRight == 0 || upperLeft <= lowerRight {
			t.Fatalf("expected a stronger upper-left glint and a faint lower-right glint at %gx: %d, %d", scale, upperLeft, lowerRight)
		}
	}
}

func TestNativeCardCompositePreservesMetadataBlurAndSharpInk(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		render := func(composite bool) *image.RGBA {
			return ui.Render(func(c *ui.Context) {
				c.Root().Background(ui.Transparent)
				box := ui.Box(c).Fill()
				if composite {
					box.Effect(nativeCardComposite{})
				}
				box.Children(func() {
					ui.Box(c).Absolute().Fill().Draw(func(p *ui.Painter, r ui.Rect) {
						for x := float32(0); x < r.W; x += 2 {
							shade := ui.Hex("#000000")
							if int(x)%4 == 0 {
								shade = ui.Hex("#ffffff")
							}
							p.Fill(ui.Rect{X: x, W: 2, H: r.H}, shade, 0)
						}
					})
					material, ink := nativeImageGlass(false)
					ui.Box(c).Absolute().Left(12).Top(12).Size(96, 28).Radius(6).Material(material).Children(func() {
						ui.Box(c).Absolute().Left(46).Top(10).Size(4, 8).Background(ink)
					})
				})
			}, 120, 52, scale)
		}
		got, want := render(true), render(false)
		for y := range got.Bounds().Dy() {
			for x := range got.Bounds().Dx() {
				a, b := got.RGBAAt(x, y), want.RGBAAt(x, y)
				if a != b {
					t.Fatalf("compositing changed backdrop blur or sharp ink at %gx (%d,%d): %v, want %v", scale, x, y, a, b)
				}
			}
		}
	}
}
