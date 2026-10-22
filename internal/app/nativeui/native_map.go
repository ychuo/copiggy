package nativeui

import (
	"fmt"
	"strings"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const (
	nativeMapPreviewWidth     = 520
	nativeMapPreviewHeight    = 264
	nativeMapImageWidth       = nativeMapPreviewWidth - 16
	nativeMapImageHeight      = nativeMapPreviewHeight - 12 - nativeControlSize - 16 - nativeCompactControl - 8
	nativeMapAttributionInset = 24
)

func nativeMapColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#eff3ed"), ui.Hex("#202a33"), ui.Hex("#697784")
	p.text, p.muted = p.paperText, p.paperMeta
	return p
}

// Map content remains a light, readable map in either chrome appearance.
// A single key function keeps loading/failure and completed images in sync.
func nativeMapAssetKey(id string, zoom int, preview bool) string {
	return fmt.Sprintf("map:%s:%t:%t:%d", id, false, preview, max(-2, min(3, zoom)))
}

func (s *application) nativeMapStatus(item clipmodel.Item, zoom int, preview bool) string {
	if len(item.AllowedApps) > 0 {
		return s.tr("Protected location")
	}
	if s.nativeMapUnavailable(item, zoom, preview) {
		return s.tr("Map unavailable")
	}
	return s.tr("Loading map…")
}

func nativeMapMarker(p *ui.Painter, bounds ui.Rect, scale float32) {
	// The snapshot's requested coordinate is at its center. Anchor the SVG's
	// tip (13, 34) there, rather than centering its complete bounding box.
	x, y := bounds.X+bounds.W/2, bounds.Y+bounds.H/2
	p.Fill(ui.Rect{X: x - 7*scale, Y: y - 2*scale, W: 14 * scale, H: 5 * scale}, ui.RGBA(36, 49, 53, .13), 2.5*scale)
	// This is two-color artwork, not an icon mask: keep its white rim and
	// center instead of tinting every SVG shape to the same coral color.
	p.Image(nativeIcons["MapPinFill"], ui.Rect{X: x - 13*scale, Y: y - 34*scale, W: 26 * scale, H: 36 * scale}, ui.Contain)
}

// Contain preserves the complete system snapshot, including MapKit labels.
// The surrounding clipped element supplies the continuous rounded shape.
func nativeMapImage(c *ui.Context, bitmap *ui.Bitmap, scale float32) {
	ui.Box(c).Fill().Draw(func(painter *ui.Painter, bounds ui.Rect) {
		painter.Image(bitmap, bounds, ui.Contain)
		nativeMapMarker(painter, bounds, scale)
	})
}

func (s *application) nativeMapCardFallback(c *ui.Context, item clipmodel.Item, p nativeColors) {
	ui.Column(c).Fill().Gap(8).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		nativeGlyph(c, "MapPin", 28).TextColor(p.icon)
		ui.Text(c, s.nativeMapStatus(item, 0, false)).FontSize(nativeTextMeta).TextColor(p.muted).TextAlign(ui.Center).Wrap().Role(ui.RoleStatus)
	})
}

func (s *application) nativeMapPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	bitmap := s.nativeMapPreviewBitmap(item, state.MapZoom)
	ink := nativeMapColors()
	ui.Column(c).Fill().Padding(12, 8, 8).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Box(c).Grow(1).MinHeight(80).FillWidth().Radius(nativePreviewPaperRadius).Clip().Background(ink.paper).Label(s.tr("Map preview")).Children(func() {
			if bitmap != nil {
				nativeMapImage(c, bitmap, 1.3)
			} else {
				ui.Column(c).Fill().Gap(8).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					nativeGlyph(c, "MapPin", 28).TextColor(ink.icon)
					ui.Text(c, s.nativeMapStatus(item, state.MapZoom, true)).FontSize(nativeTextBase).TextColor(ink.muted).Role(ui.RoleStatus)
				})
			}
			if len(item.AllowedApps) == 0 {
				material, _ := nativeImageGlass(false)
				ui.Column(c).Absolute().Right(10).Bottom(nativeMapAttributionInset).Radius(nativeControlRadius).Material(material).Children(func() {
					if s.nativePreviewButton(c, "Plus", s.tr("Zoom in"), ink).Disabled(state.MapZoom >= 3).Clicked() {
						state.MapZoom++
						s.publish()
					}
					if s.nativePreviewButton(c, "Minus", s.tr("Zoom out"), ink).Disabled(state.MapZoom <= -2).Clicked() {
						state.MapZoom--
						s.publish()
					}
				})
			}
		})
		ui.Row(c).Height(nativeCompactControl).Shrink(0).Padding(0, 14).Gap(nativeToolbarGap).AlignItems(ui.Center).TextColor(p.muted).Children(func() {
			if s.nativeCopyButton(c, p).Clicked() {
				s.chooseNative(false)
			}
			ui.Text(c, strings.TrimSpace(item.Text)).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).FontFeatures("tnum").Grow(1).MinWidth(0).SingleLine()
			ui.Text(c, "WGS 84").FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
		})
		if len(item.AllowedApps) > 0 {
			ui.Box(c).FillWidth().Padding(0, 14).Children(func() { s.nativePreviewScope(c, item, p) })
		}
	})
}
