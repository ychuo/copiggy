package nativeui

import (
	"strings"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// Only the number typography and dial-pad artwork are specific to Phone.
// Its header, footer and actions use the shared product control sizes.
const (
	nativePhoneCardNumberSize    = 24
	nativePhonePreviewNumberSize = 48
)

func nativePhoneColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#e4f8eb"), ui.Hex("#202a33"), ui.Hex("#697784")
	p.text, p.muted = p.paperText, p.paperMeta
	return p
}

// Keep the copied grouping, using a line break only at an existing boundary.
// An ungrouped number stays intact; this is not a country-code formatter.
func nativePhoneLines(display string) []string {
	if end := strings.IndexByte(display, ')'); end >= 0 && strings.TrimSpace(display[end+1:]) != "" {
		return []string{strings.TrimSpace(display[:end+1]), strings.TrimSpace(display[end+1:])}
	}
	best, distance := -1, len(display)
	for index, char := range display {
		if char != ' ' {
			continue
		}
		left, right := strings.TrimSpace(display[:index]), strings.TrimSpace(display[index+1:])
		if left == "" || right == "" {
			continue
		}
		delta := len(left) - len(right)
		if delta < 0 {
			delta = -delta
		}
		if delta < distance {
			best, distance = index, delta
		}
	}
	if best >= 0 {
		return []string{strings.TrimSpace(display[:best]), strings.TrimSpace(display[best+1:])}
	}
	return []string{display}
}

func nativePhoneNumberSize(lines []string, preferred, minimum, width float32) float32 {
	size := preferred
	for _, line := range lines {
		layout := ui.ShapeText(line, ui.Font{Family: nativeContentFont, Size: preferred, Weight: nativeWeightTitle, Features: "tnum"}, 0)
		measured, _ := layout.Size()
		if measured > width {
			size = min(size, preferred*width/measured)
		}
	}
	return max(minimum, size)
}

func nativePhoneNumber(c *ui.Context, lines []string, size float32) {
	ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
		for _, line := range lines {
			ui.Text(c, line).Font(nativeContentFont).FontSize(size).FontWeight(nativeWeightTitle).FontFeatures("tnum").LineHeight(1.15).FillWidth().SingleLine().TextAlign(ui.Center)
		}
	})
}

func nativePhoneCard(c *ui.Context, phone clipmodel.PhoneValue) {
	lines := nativePhoneLines(phone.Display)
	size := nativePhoneNumberSize(lines, nativePhoneCardNumberSize, 14, nativeCardSize-2*nativeCardPadding)
	ui.Column(c).Fill().Padding(8, 0, 0).Children(func() {
		// A single ungrouped number sits on the same visual center as two lines.
		if len(lines) == 1 {
			ui.Box(c).Height(14).Shrink(0)
		}
		nativePhoneNumber(c, lines, size)
	})
}

// Show an explicitly separated dialing prefix only. Never infer a country
// from an undelimited number, which can be ambiguous without numbering data.
func nativePhonePrefix(phone clipmodel.PhoneValue) string {
	if !phone.International || !strings.HasPrefix(phone.Display, "+") {
		return ""
	}
	end := strings.IndexAny(phone.Display, " (-.")
	if end < 2 || end > 4 {
		return ""
	}
	return phone.Display[:end]
}

func nativePhoneArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#edfcef"), To: ui.Hex("#ddf5e7"), Angle: 135}, 0)
	// End before the shared metadata row, even when it says International.
	const diameter, pitch = float32(9), float32(15)
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			p.Fill(ui.Rect{X: r.X + r.W/2 - pitch - diameter/2 + float32(col)*pitch, Y: r.Y + 116 + float32(row)*pitch, W: diameter, H: diameter}, ui.Hex("#b6e8c6").Alpha(.52), diameter/2)
		}
	}
}

func (s *application) nativePhonePreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	width, height := c.Size()
	value := clipmodel.Analyze(item.Text)
	state := s.semanticState(item)
	ink := nativePhoneColors()
	// A thin glass margin keeps the reading surface separate from AppKit's
	// continuous popover arrow, just like the other paper-based previews.
	ui.Column(c).Fill().Padding(12, 8, 8).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		// A native background follows Radius; a custom Draw callback is not
		// rounded by Element.Clip, which applies only to children.
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#fbfefa"), ui.Hex("#edf9f2"), 135).TextColor(ink.text).Padding(16).Gap(8).Children(func() {
			contentHeight := height - (12 + nativeControlSize + 8 + 8 + 32)
			if len(item.AllowedApps) > 0 {
				contentHeight -= 26
			}
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().MinHeight(max(180, contentHeight)).Gap(20).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					if phone := value.Phone; phone != nil {
						lines := nativePhoneLines(phone.Display)
						if state.Dialing {
							lines = []string{phone.Number}
						}
						size := nativePhoneNumberSize(lines, nativePhonePreviewNumberSize, 24, width-64)
						nativePhoneNumber(c, lines, size)
						ui.Column(c).FillWidth().Gap(6).AlignItems(ui.Center).TextColor(ink.muted).Children(func() {
							label := s.tr("Phone")
							if phone.International {
								label = s.tr("International")
							}
							ui.Text(c, label).FontSize(nativeTextBody).FontWeight(nativeWeightBody).SingleLine()
							ui.Text(c, phone.Number).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).FontFeatures("tnum").SingleLine()
						})
					} else {
						ui.Text(c, item.Text).FontSize(nativeTextPreview).Wrap().FillWidth()
					}
				})
			})
			s.nativePreviewScope(c, item, ink)
		})
	})
}
