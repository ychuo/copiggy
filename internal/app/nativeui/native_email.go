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
	nativeEmailPaperInset     = 8
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
		ui.Text(c, email.Local).Font(nativeContentFont).FontSize(size).FontWeight(nativeWeightTitle).LineHeight(1.15).FillWidth().SingleLine().TextAlign(ui.Center)
		ui.Text(c, domain).Font(nativeContentFont).FontSize(domainSize).FontWeight(nativeWeightBody).LineHeight(1.2).TextColor(meta).FillWidth().SingleLine().TextAlign(ui.Center)
	})
}

func nativeEmailFittedSize(text string, preferred, minimum, width float32, weight int) float32 {
	layout := ui.ShapeText(text, ui.Font{Family: nativeContentFont, Size: preferred, Weight: weight}, 0)
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
	value := clipmodel.Analyze(item.Text)
	ink := nativeEmailColors()
	// The popover and arrow remain system glass. Only the letter's reading
	// surface gets paper, so desktop detail cannot compete with the address.
	ui.Column(c).Fill().Padding(12, nativeEmailPaperInset, nativeEmailPaperInset).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Background(ui.Hex("#fdfbf8")).TextColor(ink.text).Padding(14).Gap(8).Children(func() {
			contentHeight := height - (12 + nativeControlSize + 2*nativeEmailPaperInset + 28 + 16 + .5 + nativeCompactControl)
			if len(item.AllowedApps) > 0 {
				contentHeight -= 26
			}
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().MinHeight(max(80, contentHeight)).Padding(20, 8, 0).Gap(3).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					if email := value.Email; email != nil {
						size := float32(nativeEmailPreviewTitle)
						if utf8.RuneCountInString(email.Local) > 10 {
							size = 30
						}
						if utf8.RuneCountInString(email.Local) > 24 {
							size = 24
						}
						ui.Text(c, email.Local).Font(nativeContentFont).FontSize(size).FontWeight(nativeWeightTitle).LineHeight(1.15).FillWidth().Wrap().TextAlign(ui.Center)
						ui.Text(c, "@"+email.Domain).Font(nativeContentFont).FontSize(nativeEmailPreviewDomain).FontWeight(nativeWeightBody).LineHeight(1.2).TextColor(ink.muted).FillWidth().Wrap().TextAlign(ui.Center)
					} else {
						ui.Text(c, item.Text).Font(nativeContentFont).FontSize(nativeTextPreview).Wrap().FillWidth()
					}
				})
			})
			ui.Box(c).Height(.5).Shrink(0).FillWidth().Background(ink.muted.Alpha(.18))
			ui.Row(c).Height(nativeCompactControl).Shrink(0).Gap(nativeToolbarGap).AlignItems(ui.Center).TextColor(ink.muted).Children(func() {
				if s.nativeCopyButton(c, ink).Clicked() {
					s.chooseNative(false)
				}
				ui.Text(c, strings.TrimSpace(item.Text)).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).Grow(1).MinWidth(0).SingleLine()
				ui.Text(c, s.tr("Email address")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
			})
			s.nativePreviewScope(c, item, ink)
		})
	})
}
