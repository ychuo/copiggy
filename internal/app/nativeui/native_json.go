package nativeui

import (
	"bytes"
	"encoding/json"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeJSONPreviewHeight = 328

func nativeJSONColors() nativeColors {
	p := panelColors(true)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#2c333d"), ui.Hex("#eef2fa"), ui.Hex("#b7c1d3")
	p.text, p.muted = p.paperText, p.paperMeta
	p.icon = ui.Hex("#b5bfce")
	return p
}

func nativeJSONArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#353d48"), To: ui.Hex("#252c35"), Angle: 135}, 0)
}

func nativeJSONDisplay(text string) string {
	var pretty bytes.Buffer
	if json.Indent(&pretty, []byte(text), "", "  ") == nil {
		return pretty.String()
	}
	return text
}

func nativeJSONCard(c *ui.Context, item clipmodel.Item) {
	nativeJSONText(c, nativeJSONDisplay(item.Text), true).Font(nativeCodeFont).FontSize(11.5).FontWeight(nativeWeightBody).LineHeight(1.4).MaxLines(5).Wrap().Fill().MinHeight(0).Clip()
}

func (s *application) nativeJSONSummary(text string) string {
	// Count only the top-level collection. Numbers retain their exact spelling
	// during decoding, including values outside the float64 integer range.
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.UseNumber()
	var value any
	if !json.Valid([]byte(text)) || decoder.Decode(&value) != nil {
		return s.tr("JSON")
	}
	switch value := value.(type) {
	case map[string]any:
		return s.tr("%d properties", len(value))
	case []any:
		return s.tr("%d items", len(value))
	default:
		// A scalar is valid JSON but has neither properties nor collection items.
		return s.tr("JSON")
	}
}

func (s *application) nativeJSONPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.nativeState()
	text, display, provenance := item.Text, nativeJSONDisplay(item.Text), s.tr("Original")
	if item.ParentID != "" {
		provenance = s.tr("Formatted copy")
	}
	if s.draft != nil {
		if state.Original {
			// Comparison shows the untouched source, including its original
			// whitespace. The default preview may indent for readability only.
			display, provenance = item.Text, s.tr("Original")
		} else {
			text, display, provenance = s.draft.Text, s.draft.Text, s.tr("Formatted draft")
		}
	}
	ink := nativeJSONColors()
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#343b45"), ui.Hex("#272e37"), 135).TextColor(ink.text).Padding(nativePreviewReadingInset).Gap(nativePreviewContentGap).Children(func() {
			ui.ScrollBoth(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				// Keep indentation and long values intact. Horizontal scrolling
				// avoids making deeply nested code collide with the fixed footer.
				ui.Box(c).Padding(nativePreviewDataInset - nativePreviewReadingInset).Children(func() {
					nativeJSONText(c, display, true).Font(nativeCodeFont).FontSize(14).FontWeight(nativeWeightBody).LineHeight(1.5)
				})
			})
			ui.Box(c).Height(.5).Shrink(0).FillWidth().Background(ink.muted.Alpha(.18))
			s.nativePreviewFooter(c, item, ink, func() {
				ui.Text(c, s.nativeJSONSummary(text)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Grow(1).MinWidth(0).SingleLine()
				ui.Text(c, provenance).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
			})
		})
	})
}
