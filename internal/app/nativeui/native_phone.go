package nativeui

import (
	"strings"
	"unicode"
	"unicode/utf8"

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
		if !unicode.IsSpace(char) {
			continue
		}
		_, boundarySize := utf8.DecodeRuneInString(display[index:])
		left, right := strings.TrimSpace(display[:index]), strings.TrimSpace(display[index+boundarySize:])
		if left == "" || right == "" {
			continue
		}
		delta := utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
		if delta < 0 {
			delta = -delta
		}
		if delta < distance {
			best, distance = index, delta
		}
	}
	if best >= 0 {
		_, boundarySize := utf8.DecodeRuneInString(display[best:])
		return []string{strings.TrimSpace(display[:best]), strings.TrimSpace(display[best+boundarySize:])}
	}
	return []string{display}
}

// A URI, extension or full-width digit sequence can exceed the available width
// even at the minimum readable size. Split only its visual lines, preserving
// every non-whitespace character; clipboard copies still use the saved record.
func nativePhoneFittedLines(display string, preferred, minimum, width float32) ([]string, float32) {
	lines := nativePhoneLines(display)
	size := nativePhoneNumberSize(lines, preferred, minimum, width)
	var fitted []string
	for _, line := range lines {
		font := nativeContentFontFor(line, size, nativeWeightTitle)
		font.Features = "tnum"
		fits := func(value string) bool {
			measured, _ := ui.ShapeText(value, font, 0).Size()
			return measured <= width
		}
		for !fits(line) && utf8.RuneCountInString(line) > 1 {
			runes := []rune(line)
			low, high := 1, len(runes)
			for low < high {
				middle := (low + high + 1) / 2
				if fits(string(runes[:middle])) {
					low = middle
				} else {
					high = middle - 1
				}
			}
			// Keep a URI parameter together when it fits on the next line.
			for index := low - 1; index > 0; index-- {
				if runes[index] == ';' {
					low = index
					break
				}
			}
			fitted = append(fitted, string(runes[:low]))
			line = string(runes[low:])
		}
		fitted = append(fitted, line)
	}
	return fitted, size
}

func nativePhoneNumberSize(lines []string, preferred, minimum, width float32) float32 {
	size := preferred
	for _, line := range lines {
		font := nativeContentFontFor(line, preferred, nativeWeightTitle)
		font.Features = "tnum"
		layout := ui.ShapeText(line, font, 0)
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
			nativeContentText(c, line).FontSize(size).FontWeight(nativeWeightTitle).FontFeatures("tnum").LineHeight(1.15).FillWidth().SingleLine().TextAlign(ui.Center)
		}
	})
}

func nativePhoneCardLayout(phone clipmodel.PhoneValue) ([]string, float32) {
	display := phone.Display
	if strings.HasPrefix(strings.ToLower(display), "tel:") {
		// A compact card summarizes the phone URI's actual dialing target;
		// the preview below retains its complete original URI and parameters.
		display = phone.DialingNumber()
	}
	lines, size := nativePhoneFittedLines(display, nativePhoneCardNumberSize, 14, nativeCardSize-2*nativeCardPadding)
	const height = 116 - nativeCardPadding - nativeCardHeaderHeight - nativeCardContentGap - nativeInlineGap
	if float32(len(lines))*size*1.15 > height {
		// Repeated separators are valid input but must not push the compact
		// number into the dial pad or shared footer. Its complete source is
		// still available in the scrollable preview and original copy.
		return nativePhoneFittedLines(phone.DialingNumber(), nativePhoneCardNumberSize, 14, nativeCardSize-2*nativeCardPadding)
	}
	return lines, size
}

func nativePhoneCard(c *ui.Context, phone clipmodel.PhoneValue) {
	lines, size := nativePhoneCardLayout(phone)
	ui.Column(c).Fill().Padding(nativeInlineGap, 0, 0).Children(func() {
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
	value := clipmodel.AnalyzeItem(item)
	state := s.semanticState(item)
	ink := nativePhoneColors()
	location := ""
	if value.Phone != nil {
		location = s.nativePhoneLocation(item, *value.Phone)
	}
	// A thin glass margin keeps the reading surface separate from AppKit's
	// continuous popover arrow, just like the other paper-based previews.
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		// A native background follows Radius; a custom Draw callback is not
		// rounded by Element.Clip, which applies only to children.
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#fbfefa"), ui.Hex("#edf9f2"), 135).TextColor(ink.text).Padding(nativePreviewHeroInset).Gap(nativePreviewContentGap).Children(func() {
			contentHeight := height - nativePreviewChromeHeight - 2*nativePreviewHeroInset
			if len(item.AllowedApps) > 0 {
				contentHeight -= nativePreviewContentGap + nativePreviewFooterHeight(item, false)
			}
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().MinHeight(max(180, contentHeight)).Gap(20).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					if phone := value.Phone; phone != nil {
						display := phone.Display
						if state.Dialing {
							display = phone.DialingNumber()
						}
						lines, size := nativePhoneFittedLines(display, nativePhonePreviewNumberSize, 24, width-2*(nativePreviewEdgeInset+nativePreviewHeroInset))
						nativePhoneNumber(c, lines, size)
						ui.Column(c).FillWidth().Gap(6).AlignItems(ui.Center).TextColor(ink.muted).Children(func() {
							s.nativePhoneRegionMetadata(c, *phone)
							s.nativePhoneLocationMetadata(c, location)
							nativeContentText(c, phone.Number).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).FontFeatures("tnum").SingleLine()
						})
					} else {
						nativeContentText(c, item.Text).FontSize(nativeTextPreview).Wrap().FillWidth()
					}
				})
			})
			s.nativePreviewFooter(c, item, ink, nil)
		})
	})
}
