package nativeui

import (
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func (s *application) nativeAppLinkActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	nativeSegmentGroup(c, p).Children(func() {
		for _, option := range []struct {
			title, label string
			qr           bool
		}{{"QR", "Show QR code", true}, {"Link", "Show link", false}} {
			if s.nativeFormatSegment(c, s.tr(option.title), state.LinkQR == option.qr, p).
				Label(s.tr(option.label)).Clicked() && state.LinkQR != option.qr {
				s.toggleNativeLinkQR(item)
			}
		}
	})
}

func (s *application) nativeAppLinkPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	link := clipmodel.ParseLink(item.Text)
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() { s.nativePreviewToolbar(c, item, p) })
		if state := s.semanticState(item); state.LinkQR {
			s.nativeLinkQRContent(c, item, state.QRCode, p)
		} else if link != nil && link.App {
			s.nativeAppLinkContent(c, item, link, p)
		} else {
			s.nativeLinkQRContent(c, item, nil, p)
		}
		ui.Box(c).FillWidth().Shrink(0).Padding(0, nativePreviewReadingInset).Children(func() {
			s.nativePreviewFooter(c, item, p, func() {
				if s.nativeCopyButton(c, p).Clicked() {
					s.chooseNative(false)
				}
				nativeContentText(c, item.Text).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).Grow(1).MinWidth(0).SingleLine()
			})
		})
	})
}

func (s *application) nativeAppLinkContent(c *ui.Context, item clipmodel.Item, link *clipmodel.LinkValue, _ nativeColors) {
	ink := nativeAppLinkColors()
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Background(ink.paper).Padding(nativePreviewReadingInset).Gap(nativePreviewContentGap).Children(func() {
		nativeContentText(c, link.Scheme+"://").FontSize(nativeTextTitle).FontWeight(nativeWeightTitle).TextColor(ink.text).SingleLine()
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			nativeContentText(c, item.Text).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).TextColor(ink.text).Wrap().FillWidth()
		})
	})
}
