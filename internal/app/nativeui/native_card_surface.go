package nativeui

import "github.com/egoist/mygo/ui"

// Paint outside the card's clip. The short shadows fit the strip's existing
// vertical padding and add depth without tinting the artwork or changing layout.
func nativeCardDepth(card ui.Element, dark bool) {
	ambient, contact := float32(.08), float32(.04)
	if dark {
		ambient, contact = .10, .06
	}
	card.Shadow(0, 2, 4, -1, ui.RGBA(0, 0, 0, ambient)).
		Shadow(0, .5, 1.5, 0, ui.RGBA(0, 0, 0, contact))
}

// Two short reflections catch the upper-left and lower-right corners, fading
// completely before the middle of each side. Keep them inside the composite,
// below selection, so they do not introduce another outer antialiasing pass.
func nativeCardEdgeLight(c *ui.Context) {
	clear := ui.RGBA(255, 255, 255, 0)
	ui.Box(c).Absolute().Left(0).Top(0).Fill().PassThrough().Radius(nativeCardRadius).
		Border(.65, ui.Transparent).BorderConicGradient(ui.ConicGradient{
		Stops: []ui.GradientStop{
			{Color: clear, At: 0},
			{Color: clear, At: .335},
			{Color: ui.RGBA(255, 255, 255, .18), At: .375},
			{Color: clear, At: .415},
			{Color: clear, At: .835},
			{Color: ui.RGBA(255, 255, 255, .45), At: .875},
			{Color: clear, At: .915},
			{Color: clear, At: 1},
		},
	})
}

// nativeCardComposite resolves overlapping artwork into a single surface before
// the parent applies its rounded clip. MyGo retains this layer at the display's
// scale on the GPU; text, image metadata blur and hit testing stay native.
// Keep the layer rectangular: rounding it here would apply edge coverage twice.
type nativeCardComposite struct{}

func (nativeCardComposite) EffectMargin() ui.Point { return ui.Point{} }

func (nativeCardComposite) PaintEffect(p *ui.Painter, box ui.Rect, content *ui.Bitmap, inset ui.Point) {
	// The retained canvas is pixel aligned, while scrolling can put the box
	// between pixels. Preserve that offset instead of stretching its pixels.
	scale := p.Scale()
	width, height := content.Size()
	area := ui.Rect{X: box.X - inset.X/scale, Y: box.Y - inset.Y/scale, W: float32(width) / scale, H: float32(height) / scale}
	p.Image(content, area, ui.FillBox)
}
