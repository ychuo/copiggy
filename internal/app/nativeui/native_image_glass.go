package nativeui

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// nativeImageGlass gives small image labels their own neutral, frosted surface.
// The blur reads pixels already painted by the card image; children are drawn
// afterwards, so their glyphs remain sharp. Keep this on the metadata's bounds,
// never on full-width strips or on the image itself.
func nativeImageGlass(dark bool) (ui.Material, ui.Color) {
	if dark {
		return nativeImageGlassMaterial{
			tint: ui.RGBA(62, 69, 79, .86),
			rim:  ui.RGBA(225, 230, 237, .16),
		}, ui.RGB(239, 242, 246)
	}
	return nativeImageGlassMaterial{
		tint: ui.RGBA(218, 222, 228, .84),
		rim:  ui.RGBA(255, 255, 255, .30),
	}, ui.RGB(61, 69, 81)
}

type nativeImageGlassMaterial struct {
	tint, rim ui.Color
}

func (g nativeImageGlassMaterial) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	// The metadata chips have uniform corners. Using the same continuous
	// rounded shape for the blur, tint and inner rim avoids a doubled contour.
	radius := radii[0]
	glass.Blur{Radius: 6}.PaintMaterial(p, box, radii)
	p.Fill(box, g.tint, radius)
	p.Stroke(box, g.rim, radius, .5)
}
