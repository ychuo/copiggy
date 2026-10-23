package nativeui

import (
	"strings"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// Email keeps only content-specific typography and paper geometry here.
// All controls use the product-wide roles in native_design.go.
const (
	nativeEmailCardTitleSize  = 34
	nativeEmailCardDomainSize = 17
	nativeEmailPreviewHeight  = 264
	nativeEmailPreviewTitle   = 46
	nativeEmailPreviewDomain  = 28
)

func nativeEmailColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#eaf3ff"), ui.Hex("#202a33"), ui.Hex("#697784")
	p.text, p.muted = p.paperText, p.paperMeta
	p.link, p.edge = ui.Hex("#f3f7fc"), ui.Hex("#d2deea")
	p.control, p.hover = ui.RGBA(75, 100, 125, .10), ui.RGBA(75, 100, 125, .08)
	return p
}

func nativeEmailCard(c *ui.Context, email clipmodel.EmailValue, meta ui.Color) {
	// Keep long account names out of the folds. The preview retains the full
	// address; this fitting only affects the card's display typography.
	size := float32(nativeEmailCardTitleSize)
	if utf8.RuneCountInString(email.Local) > 10 {
		size = 24
	}
	size = nativeEmailFittedSize(email.Local, size, 14, nativeCardSize-40, nativeWeightTitle)
	domain := "@" + email.Domain
	domainSize := nativeEmailFittedSize(domain, nativeEmailCardDomainSize, 10.5, nativeCardSize-40, nativeWeightBody)
	nativeCardHero(c).Padding(0, 2).Gap(2).Children(func() {
		nativeContentText(c, email.Local).FontSize(size).FontWeight(nativeWeightTitle).LineHeight(1.15).FillWidth().SingleLine().TextAlign(ui.Center)
		nativeContentText(c, domain).FontSize(domainSize).FontWeight(nativeWeightBody).LineHeight(1.2).TextColor(meta).FillWidth().SingleLine().TextAlign(ui.Center)
	})
}

func nativeEmailFittedSize(text string, preferred, minimum, width float32, weight int) float32 {
	layout := ui.ShapeText(text, nativeContentFontFor(text, preferred, weight), 0)
	measured, _ := layout.Size()
	if measured <= width {
		return preferred
	}
	return max(minimum, preferred*width/measured)
}

// Separate paper faces meet at the diagonal creases. Painting each face once
// avoids the gray X made by overlapping translucent triangles.
func nativeEmailArtwork(p *ui.Painter, r ui.Rect) {
	paperTop, paperBottom := ui.Hex("#f3f8ff"), ui.Hex("#dfedff")
	leftTop, leftBottom := ui.Hex("#dcecff"), ui.Hex("#bed7f5")
	rightTop, rightBottom := ui.Hex("#e4f1ff"), ui.Hex("#c9dff8")
	frontTop, frontBottom := ui.Hex("#c4ddfa"), ui.Hex("#d5e7fc")
	crease, highlight, shadow := ui.Hex("#acc9eb"), ui.RGBA(255, 255, 255, .62), ui.Hex("#6f97c6")
	p.FillGradient(r, ui.LinearGradient{From: paperTop, To: paperBottom, Angle: 145}, 0)
	left, right, bottom := r.X, r.X+r.W, r.Y+r.H
	middle, shoulder, tip := r.X+r.W/2, r.Y+r.H*.60, r.Y+r.H*.96
	leftFace := new(ui.Path).MoveTo(left, shoulder).LineTo(middle, tip).LineTo(middle, bottom).LineTo(left, bottom).Close()
	rightFace := new(ui.Path).MoveTo(middle, tip).LineTo(right, shoulder).LineTo(right, bottom).LineTo(middle, bottom).Close()
	p.FillPathGradient(leftFace, ui.LinearGradient{From: leftTop, To: leftBottom, Angle: 135})
	p.FillPathGradient(rightFace, ui.LinearGradient{From: rightTop, To: rightBottom, Angle: 225})
	// Painter.Line is axis-aligned; a V crease must be a stroked path.
	creasePath := new(ui.Path).MoveTo(left, shoulder).LineTo(middle, tip).LineTo(right, shoulder)
	lightPath := new(ui.Path).MoveTo(left, shoulder+1).LineTo(middle, tip+1).LineTo(right, shoulder+1)
	p.StrokePath(creasePath, 5, shadow.Alpha(.025))
	p.StrokePath(creasePath, 2.5, shadow.Alpha(.055))
	p.StrokePath(lightPath, .7, highlight)
	p.StrokePath(creasePath, .7, crease.Alpha(.85))
	// A rounded front flap covers the crossed tips, with a restrained paper
	// shadow and one illuminated edge rather than a heavy outline.
	flapLeft, flapRight, peak := r.X+r.W*.32, r.X+r.W*.68, r.Y+r.H*.86
	flap := new(ui.Path).MoveTo(flapLeft, bottom).LineTo(middle-4, peak+3).QuadTo(middle, peak-1, middle+4, peak+3).LineTo(flapRight, bottom).Close()
	edge := new(ui.Path).MoveTo(flapLeft, bottom).LineTo(middle-4, peak+3).QuadTo(middle, peak-1, middle+4, peak+3).LineTo(flapRight, bottom)
	p.StrokePath(edge, 6, shadow.Alpha(.025))
	p.StrokePath(edge, 3, shadow.Alpha(.06))
	p.StrokePath(edge, 1.5, shadow.Alpha(.09))
	p.FillPathGradient(flap, ui.LinearGradient{From: frontTop, To: frontBottom, Angle: 180})
	p.StrokePath(edge, .7, crease.Alpha(.8))
}

func (s *application) nativeEmailPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	_, height := c.Size()
	value := clipmodel.AnalyzeItem(item)
	ink := nativeEmailColors()
	// The popover and arrow remain system glass. Only the letter's reading
	// surface gets paper, so desktop detail cannot compete with the address.
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		// The divider has space on both sides. Use the compact section gap
		// here so a short protected address still fits without scrolling.
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Background(ui.Hex("#fdfbf8")).TextColor(ink.text).Padding(nativePreviewReadingInset).Gap(nativePreviewSectionGap).Children(func() {
			contentHeight := height - nativePreviewChromeHeight - 2*nativePreviewReadingInset - 2*nativePreviewSectionGap - .5 - nativePreviewFooterHeight(item, true)
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().MinHeight(max(80, contentHeight)).Padding(nativePreviewReadingInset, 8, 0).Gap(3).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					if email := value.Email; email != nil {
						size := float32(nativeEmailPreviewTitle)
						if utf8.RuneCountInString(email.Local) > 10 {
							size = 30
						}
						if utf8.RuneCountInString(email.Local) > 24 {
							size = 24
						}
						nativeContentText(c, email.Local).FontSize(size).FontWeight(nativeWeightTitle).LineHeight(1.15).FillWidth().Wrap().TextAlign(ui.Center)
						nativeContentText(c, "@"+email.Domain).FontSize(nativeEmailPreviewDomain).FontWeight(nativeWeightBody).LineHeight(1.2).TextColor(ink.muted).FillWidth().Wrap().TextAlign(ui.Center)
					} else {
						nativeContentText(c, item.Text).FontSize(nativeTextPreview).Wrap().FillWidth()
					}
				})
			})
			ui.Box(c).Height(.5).Shrink(0).FillWidth().Background(ink.muted.Alpha(.18))
			s.nativePreviewFooter(c, item, ink, func() {
				if s.nativeCopyButton(c, ink).Clicked() {
					s.chooseNative(false)
				}
				nativeContentText(c, strings.TrimSpace(item.Text)).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).Grow(1).MinWidth(0).SingleLine()
				ui.Text(c, s.tr("Email address")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
			})
		})
	})
}
