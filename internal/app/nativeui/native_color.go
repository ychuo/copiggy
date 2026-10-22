package nativeui

import (
	"fmt"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeColorPreviewHeight = 232

func nativeColorCaption(value clipmodel.ColorValue) string {
	if value.A < 255 {
		return value.RGB
	}
	return fmt.Sprintf("RGB %d, %d, %d", value.R, value.G, value.B)
}

func nativeColorCard(c *ui.Context, value clipmodel.ColorValue, meta ui.Color) {
	size := nativeEmailFittedSize(value.HEX, 26, 18, nativeCardSize-2*nativeCardPadding, nativeWeightTitle)
	ui.Column(c).Fill().Gap(5).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(c, value.HEX).Font(nativeContentFont).FontSize(size).FontWeight(nativeWeightTitle).SingleLine().FontFeatures("tnum")
		ui.Text(c, nativeColorCaption(value)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).TextColor(meta).SingleLine().FontFeatures("tnum")
	})
}

// Swatches describe an exact color. Only transparency introduces a checkerboard;
// lighting, gradients and the surrounding appearance must not change its value.
func nativeColorArtwork(p *ui.Painter, r ui.Rect, value clipmodel.ColorValue) {
	if value.A == 255 {
		p.Fill(r, ui.RGB(value.R, value.G, value.B), 0)
		return
	}
	const cellSize = float32(12)
	for y := float32(0); y < r.H; y += cellSize {
		for x := float32(0); x < r.W; x += cellSize {
			cell := ui.Hex("#f8fafc")
			if (int(x/cellSize)+int(y/cellSize))%2 == 0 {
				cell = ui.Hex("#e8eaed")
			}
			p.Fill(ui.Rect{X: r.X + x, Y: r.Y + y, W: min(cellSize, r.W-x), H: min(cellSize, r.H-y)}, cell, 0)
		}
	}
	p.Fill(r, ui.RGBA(value.R, value.G, value.B, float32(value.A)/255), 0)
}

func (s *application) nativeColorActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	ui.Row(c).Height(nativeControlSize).Gap(nativeInlineGap).AlignItems(ui.Center).Padding(0, 8).Shrink(0).Children(func() {
		nativeGlyph(c, "Palette", nativeToolbarIconSize).TextColor(p.icon)
		ui.Text(c, s.tr("Color")).FontSize(nativeTextControl).FontWeight(nativeWeightControl).TextColor(p.muted).SingleLine()
	})
	ui.Row(c).Height(nativeControlSize).Padding(2).Radius(nativeControlRadius + 2).Background(p.control).Shrink(0).Children(func() {
		for index, format := range []string{"HEX", "RGB", "HSL"} {
			button := s.nativeSegment(c, s.tr(format), state.ColorFormat == index, p).Height(nativeControlSize - 4).Width(42).Disabled(s.commandBusy)
			if state.ColorFormat == index {
				button.Background(p.hover)
			}
			if button.Clicked() {
				state.ColorFormat = index
				state.CopyFormat = []clipmodel.CopyFormat{clipmodel.ColorHEX, clipmodel.ColorRGB, clipmodel.ColorHSL}[index]
				s.publish()
			}
		}
	})
}

func (s *application) nativeColorPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	value := clipmodel.Analyze(item.Text).Color
	if value == nil {
		return
	}
	state := s.semanticState(item)
	ink := panelColors(false)
	ink.text, ink.muted, ink.icon = nativeColorInk(*value), nativeColorInk(*value).Alpha(.85), nativeColorInk(*value)
	// A dark warning would disappear on a dark swatch. Protection remains
	// readable with the same contrast-tested ink as the actual color value.
	ink.warning = ink.text
	label := []string{value.HEX, value.RGB, value.HSL}[state.ColorFormat]
	size := float32(32)
	if state.ColorFormat != 0 {
		size = 26
	}
	ui.Column(c).Fill().Padding(12, 8, 8).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		// The artwork is a child of the clipping container. A Draw on the
		// rounded element itself would paint square corners over system glass.
		ui.Box(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Children(func() {
			ui.Column(c).Fill().Draw(func(painter *ui.Painter, bounds ui.Rect) {
				nativeColorArtwork(painter, bounds, *value)
			}).Padding(16).Gap(8).TextColor(ink.text).Children(func() {
				ui.Column(c).Grow(1).MinHeight(0).FillWidth().Gap(6).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, label).Font(nativeContentFont).FontSize(size).FontWeight(nativeWeightTitle).LineHeight(1.2).FillWidth().Wrap().TextAlign(ui.Center).FontFeatures("tnum")
					caption := nativeColorCaption(*value)
					if state.ColorFormat == 1 {
						caption = value.HEX
					}
					ui.Text(c, caption).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).TextColor(ink.muted).FillWidth().Wrap().TextAlign(ui.Center).FontFeatures("tnum")
				})
				s.nativePreviewScope(c, item, ink)
			})
		})
	})
}
