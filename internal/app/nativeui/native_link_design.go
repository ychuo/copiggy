package nativeui

import (
	"net/url"
	"strings"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeLinkCoverHeight = 106

func nativeLinkColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#fdfbf8"), ui.Hex("#223338"), ui.Hex("#697784")
	p.text, p.muted, p.link = p.paperText, p.paperMeta, p.paper
	return p
}

func nativeLinkArtwork(p *ui.Painter, r ui.Rect) {
	p.Fill(r, nativeLinkColors().paper, 0)
}

func nativeLinkCardHost(item clipmodel.Item) string {
	if link, err := url.Parse(strings.TrimSpace(item.Text)); err == nil && link.Hostname() != "" {
		return link.Hostname()
	}
	return ""
}

// Use an extracted or fetched page title when available. A URL path remains
// the fallback while loading or when the page exposes no usable title.
func nativeLinkCardTitle(item clipmodel.Item, title string) string {
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	link, err := url.Parse(strings.TrimSpace(item.Text))
	if err != nil {
		return strings.TrimSpace(item.Text)
	}
	title = strings.Trim(link.EscapedPath(), "/")
	if decoded, err := url.PathUnescape(title); err == nil {
		title = decoded
	}
	if title == "" {
		title = link.Host
	}
	return title
}

func (s *application) nativeLinkSiteIcon(c *ui.Context, bitmap *ui.Bitmap, size float32, p nativeColors) {
	if bitmap != nil {
		ui.Image(c, bitmap).Size(size, size).Shrink(0).Fit(ui.Contain).Label(s.tr("Website icon"))
		return
	}
	nativeGlyph(c, "Globe", size).Shrink(0).TextColor(p.icon)
}

// The caller supplies already-cached assets. Layout must not start requests or
// inspect website contents; cover loading cannot move the common card footer.
func (s *application) nativeLinkCardBodyWithIcon(c *ui.Context, item clipmodel.Item, title string, hasCover bool, icon *ui.Bitmap) {
	p := nativeLinkColors()
	title = nativeLinkCardTitle(item, title)
	if hasCover {
		// Shared body starts at y=50. The 106px cover leaves eight pixels before
		// the compact website-icon/title row on the warm paper below it.
		ui.Column(c).Fill().MinHeight(0).Padding(nativeLinkCoverHeight+8-(nativeCardPadding+nativeCardHeaderHeight+nativeCardContentGap), 0, 0).Clip().Children(func() {
			ui.Row(c).FillWidth().MinHeight(0).Gap(8).AlignItems(ui.Start).Children(func() {
				s.nativeLinkSiteIcon(c, icon, 20, p)
				nativeContentText(c, title).FontSize(14).FontWeight(nativeWeightTitle).LineHeight(1.15).MaxLines(2).Wrap().Grow(1).MinWidth(0).TextColor(p.text)
			})
		})
		return
	}
	ui.Column(c).Fill().MinHeight(0).Padding(6, 0, 0).Gap(10).Clip().Children(func() {
		s.nativeLinkSiteIcon(c, icon, 28, p)
		nativeContentText(c, title).FontSize(nativeTextBody).FontWeight(nativeWeightTitle).LineHeight(1.25).MaxLines(2).Wrap().FillWidth().MinHeight(0).TextColor(p.text)
	})
}

func (s *application) nativeLinkPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		s.nativeLinkContent(c, item, p)
		ui.Box(c).FillWidth().Shrink(0).Padding(0, nativePreviewReadingInset).Children(func() {
			s.nativePreviewFooter(c, item, p, func() {
				if s.nativeCopyButton(c, p).Clicked() {
					s.chooseNative(false)
				}
				// Navigation changes the toolbar's location, but this control always
				// copies the exact saved record, so its label keeps the original host.
				nativeContentText(c, nativeLinkCardHost(item)).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).Grow(1).MinWidth(0).SingleLine()
			})
		})
	})
}
