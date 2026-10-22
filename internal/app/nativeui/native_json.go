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
	ui.Column(c).Fill().Padding(12, 8, 8).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).Shrink(0).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#343b45"), ui.Hex("#272e37"), 135).TextColor(ink.text).Padding(22).Children(func() {
			ui.ScrollBoth(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				// Keep indentation and long values intact. Horizontal scrolling
				// avoids making deeply nested code collide with the fixed footer.
				nativeJSONText(c, display, true).Font(nativeCodeFont).FontSize(14).FontWeight(nativeWeightBody).LineHeight(1.5)
			})
		})
		ui.Column(c).FillWidth().Shrink(0).Padding(0, 14, 2).Gap(6).TextColor(p.muted).Children(func() {
			ui.Row(c).Height(nativeCompactControl).Shrink(0).Gap(nativeToolbarGap).AlignItems(ui.Center).Children(func() {
				nativeGlyph(c, "File", nativeIconSize).TextColor(p.icon).Shrink(0)
				ui.Text(c, s.nativeJSONSummary(text)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Grow(1).MinWidth(0).SingleLine()
				ui.Text(c, provenance).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
			})
			s.nativePreviewScope(c, item, p)
		})
	})
}
