package nativeui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"

	"golang.org/x/image/vector"
)

var trayIcons = sync.OnceValue(func() [2][]byte {
	return [2][]byte{drawClipboardTrayIcon(false), drawClipboardTrayIcon(true)}
})

func clipboardTrayIcon(guard bool) []byte {
	icons := trayIcons()
	if guard {
		return icons[1]
	}
	return icons[0]
}

// The small template preserves the Little collector's raised right ear, gentle
// lean and hugged clipboard. Keep the face open and the paper sparse at 18pt.
// AppKit supplies the menu bar's ink; the guard is conveyed by shape, not color.
func drawClipboardTrayIcon(guard bool) []byte {
	const size = 72 // 18 points at 4×, also used by the standalone brand assets.
	icon := &pigTemplate{image: image.NewNRGBA(image.Rect(0, 0, size, size))}
	icon.shape(false, func(p *pigPath) {
		p.move(2.75, 5.65)
		p.curve(1.45, 4.75, .8, 2.5, 1.7, 2.05)
		p.curve(2.55, 1.65, 4.05, 2.1, 4.65, 2.75)
		p.curve(6.35, 1.75, 8.35, 1.65, 9.8, 2.1)
		p.curve(10.9, .85, 12.7, .75, 13, 1.55)
		p.curve(13.55, 2.7, 13.35, 4.15, 12.9, 4.7)
		p.curve(14.05, 6.3, 14.7, 8.4, 14.05, 10.35)
		p.curve(13.75, 11.35, 13.1, 12.25, 12.15, 12.75)
		p.curve(12.5, 15.1, 11, 16.55, 8.55, 16.55)
		p.line(5.7, 16.4)
		p.curve(3.5, 16.15, 2.65, 14.6, 3.1, 12.4)
		p.line(3.4, 11.4)
		p.curve(1.1, 10.7, 1.1, 7.5, 2.75, 5.65)
	})
	icon.shape(true, func(p *pigPath) {
		p.move(3.75, 6)
		p.curve(2.65, 5.1, 2, 3.55, 2.45, 3.15)
		p.curve(2.95, 2.9, 4.05, 3.35, 4.55, 4.05)
		p.curve(6.3, 2.8, 8.5, 2.65, 10.15, 3.45)
		p.curve(10.8, 2.35, 11.95, 1.8, 12.1, 2.3)
		p.curve(12.4, 3.15, 12.15, 4.3, 11.6, 4.8)
		p.curve(13.05, 6.6, 13.6, 8.25, 12.9, 10.1)
		p.curve(12.55, 11.05, 11.85, 11.7, 10.9, 12)
		p.curve(11.75, 14.35, 10.5, 15.5, 8.5, 15.4)
		p.line(5.9, 15.25)
		p.curve(4.05, 15.1, 3.9, 13.55, 4.8, 10.75)
		p.curve(2.2, 10.5, 2.15, 8.1, 3.75, 6)
	})
	icon.ellipse(false, 5.05, 6.55, .6, .79)
	icon.ellipse(false, 10.1, 5.9, .6, .79)
	icon.ellipse(false, 7.3, 8.45, 2.15, 1.5)
	icon.ellipse(true, 7.3, 8.45, 1.35, .7)
	icon.ellipse(false, 6.75, 8.45, .3, .46)
	icon.ellipse(false, 7.85, 8.45, .3, .46)

	// The paper leans in the same direction as the app icon. Clearing the
	// silhouette first prevents the pig's contour showing through its contents.
	icon.shape(true, func(p *pigPath) {
		p.move(9.9, 6.75)
		p.line(18, 8.4)
		p.line(16.2, 18)
		p.line(7, 16.1)
	})
	icon.shape(false, func(p *pigPath) {
		p.move(10.35, 7.75)
		p.line(16.8, 9.1)
		p.curve(17.65, 9.3, 17.65, 9.65, 17.45, 10.35)
		p.line(16.05, 16.55)
		p.curve(15.85, 17.4, 15.55, 17.5, 14.85, 17.3)
		p.line(8.5, 15.95)
		p.curve(7.65, 15.75, 7.55, 15.4, 7.75, 14.65)
		p.line(9.15, 8.5)
		p.curve(9.35, 7.65, 9.65, 7.6, 10.35, 7.75)
	})
	icon.shape(true, func(p *pigPath) {
		p.move(10.2, 8.85)
		p.line(16.4, 10.15)
		p.line(15.05, 16.15)
		p.line(8.85, 14.85)
	})
	icon.ellipse(false, 12.9, 8.15, .9, .9)
	icon.stroke(11.4, 8.45, 14.2, 9.05, 1.25)
	icon.stroke(10.8, 10.75, 14.45, 11.5, .9)
	icon.stroke(10.45, 12.25, 14.1, 13, .9)
	icon.stroke(10.1, 13.75, 12.9, 14.35, .9)
	// A single rounded paw crossing the paper gives the tiny silhouette a hug.
	icon.ellipse(true, 8.2, 11.95, 1.65, 1.3)
	icon.stroke(6.7, 11.85, 8.7, 12.25, 1.15)
	if guard {
		icon.shape(true, func(p *pigPath) {
			p.move(10.9, 11.2)
			p.line(14.4, 9.95)
			p.line(18, 11.2)
			p.line(18, 14.7)
			p.curve(17.65, 16.3, 16.2, 17.45, 14.4, 18)
			p.curve(12.6, 17.45, 11.2, 16.3, 10.9, 14.7)
		})
		icon.shape(false, func(p *pigPath) {
			p.move(11.85, 11.9)
			p.line(14.4, 10.95)
			p.line(17, 11.9)
			p.line(16.85, 14.3)
			p.curve(16.6, 15.4, 15.6, 16.35, 14.4, 16.9)
			p.curve(13.2, 16.35, 12.2, 15.4, 11.95, 14.3)
		})
		icon.shape(true, func(p *pigPath) {
			p.move(12.6, 13.75)
			p.line(13.25, 13.1)
			p.line(14, 13.9)
			p.line(15.45, 12.35)
			p.line(16.1, 13)
			p.line(14, 15.3)
		})
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, icon.image); err != nil {
		panic(err) // Encoding an allocated in-memory image cannot fail here.
	}
	return encoded.Bytes()
}

type pigTemplate struct{ image *image.NRGBA }
type pigPath struct{ raster *vector.Rasterizer }

func (p *pigPath) move(x, y float32) { p.raster.MoveTo(x*4, y*4) }
func (p *pigPath) line(x, y float32) { p.raster.LineTo(x*4, y*4) }
func (p *pigPath) curve(x1, y1, x2, y2, x, y float32) {
	p.raster.CubeTo(x1*4, y1*4, x2*4, y2*4, x*4, y*4)
}

func (icon *pigTemplate) shape(clear bool, path func(*pigPath)) {
	bounds := icon.image.Bounds()
	raster := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	path(&pigPath{raster: raster})
	raster.ClosePath()
	mask := image.NewAlpha(bounds)
	raster.Draw(mask, bounds, image.NewUniform(color.Alpha{A: 255}), image.Point{})
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			alpha := uint16(icon.image.NRGBAAt(x, y).A)
			coverage := uint16(mask.AlphaAt(x, y).A)
			if clear {
				alpha = alpha * (255 - coverage) / 255
			} else {
				alpha += (255 - alpha) * coverage / 255
			}
			icon.image.SetNRGBA(x, y, color.NRGBA{A: uint8(alpha)})
		}
	}
}

func (icon *pigTemplate) ellipse(clear bool, x, y, rx, ry float32) {
	const k = .55228475
	icon.shape(clear, func(p *pigPath) {
		p.move(x+rx, y)
		p.curve(x+rx, y+ry*k, x+rx*k, y+ry, x, y+ry)
		p.curve(x-rx*k, y+ry, x-rx, y+ry*k, x-rx, y)
		p.curve(x-rx, y-ry*k, x-rx*k, y-ry, x, y-ry)
		p.curve(x+rx*k, y-ry, x+rx, y-ry*k, x+rx, y)
	})
}

// Rounded strokes preserve their optical weight when reduced to menu-bar size.
func (icon *pigTemplate) stroke(ax, ay, bx, by, width float32) {
	length := float32(math.Hypot(float64(bx-ax), float64(by-ay)))
	dx, dy := (by-ay)*width/2/length, (bx-ax)*width/2/length
	icon.shape(false, func(p *pigPath) {
		p.move(ax+dx, ay-dy)
		p.line(bx+dx, by-dy)
		p.line(bx-dx, by+dy)
		p.line(ax-dx, ay+dy)
	})
	icon.ellipse(false, ax, ay, width/2, width/2)
	icon.ellipse(false, bx, by, width/2, width/2)
}
