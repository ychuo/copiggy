package nativeui

import (
	"strings"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeNotePreviewHeight = 328

func nativeNoteColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#fff1d8"), ui.Hex("#272f3a"), ui.Hex("#6b7584")
	p.text, p.muted = p.paperText, p.paperMeta
	return p
}

func nativeNoteArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#fff6e3"), To: ui.Hex("#ffedcc"), Angle: 135}, 0)
}

func nativeNoteCard(c *ui.Context, item clipmodel.Item) {
	text := strings.TrimSpace(item.Text)
	size, weight := float32(nativeTextBody), nativeWeightBody
	if utf8.RuneCountInString(text) < 24 {
		size, weight = nativeTextTitle, nativeWeightTitle
	}
	nativeContentText(c, text).FontSize(size).FontWeight(weight).LineHeight(1.4).MaxLines(4).Wrap().Fill().MinHeight(0).Clip()
}

// A note is paper with its original paragraphs, not an editor. The content can
// scroll while its type and protection metadata stay in their fixed footer.
func (s *application) nativeNotePreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	ink := nativeNoteColors()
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Background(ui.Hex("#fdfbf8")).TextColor(ink.text).Padding(nativePreviewReadingInset).Gap(nativePreviewContentGap).Children(func() {
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Box(c).FillWidth().Padding(8, nativePreviewReadingInset, 6).Children(func() {
					nativeContentText(c, item.Text).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).LineHeight(1.5).FillWidth().Wrap()
				})
			})
			ui.Box(c).Height(.5).Shrink(0).FillWidth().Background(ink.muted.Alpha(.18))
			s.nativePreviewFooter(c, item, ink, func() {
				ui.Text(c, s.tr("%d characters", utf8.RuneCountInString(item.Text))).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).FontFeatures("tnum").Grow(1).MinWidth(0).SingleLine()
				ui.Text(c, s.tr("Plain text")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).SingleLine().Shrink(0)
			})
		})
	})
}
