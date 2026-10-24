package nativeui

import (
	"errors"
	"math"

	"copiggy/internal/clipmodel"
	"copiggy/internal/linkqr"
	"github.com/egoist/mygo/ui"
)

const nativeLinkQRMaxSize = 320

// The caller keeps this only for the selected record and normalized target.
// Generating or displaying it never changes the record's copy representation.
type nativeLinkQRCode struct {
	bitmap  *ui.Bitmap
	modules int
	failure string
}

func newNativeLinkQRCode(target string) *nativeLinkQRCode {
	code, err := linkqr.Encode(target)
	if err != nil {
		failure := "QR code unavailable"
		if errors.Is(err, linkqr.ErrTooLong) {
			failure = "Link is too long for a QR code"
		}
		return &nativeLinkQRCode{failure: failure}
	}
	return &nativeLinkQRCode{bitmap: ui.NewBitmap(code.Image()).Pixelated(), modules: code.ModuleCount()}
}

func nativeLinkQRDisplaySize(modules int, availableW, availableH float32) float32 {
	if modules <= 0 || availableW <= 0 || availableH <= 0 {
		return 0
	}
	scale := int(math.Floor(float64(min(float32(nativeLinkQRMaxSize), availableW, availableH)) / float64(modules)))
	if scale < 2 {
		return 0
	}
	return float32(modules * scale)
}

func nativeLinkQRPresentation(code *nativeLinkQRCode, availableW, availableH float32) (float32, string) {
	if code == nil {
		return 0, "QR code unavailable"
	}
	if code.failure != "" {
		return 0, code.failure
	}
	if code.bitmap != nil {
		if size := nativeLinkQRDisplaySize(code.modules, availableW, availableH); size > 0 {
			return size, ""
		}
	}
	return 0, "QR code unavailable"
}

func (s *application) nativeLinkQRContent(c *ui.Context, item clipmodel.Item, code *nativeLinkQRCode, _ nativeColors) {
	width, height := c.Size()
	viewportW, viewportH := nativeImagePreviewViewport(width, height, len(item.AllowedApps) > 0)
	const captionSpace = nativePreviewMetadataHeight + nativePreviewContentGap
	size, failure := nativeLinkQRPresentation(code, viewportW-2*nativePreviewReadingInset, viewportH-2*nativePreviewReadingInset-captionSpace)
	ink := nativeLinkColors()
	if item.Type == clipmodel.TypeAppLink {
		ink = nativeAppLinkColors()
	}
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Background(ink.paper).Padding(nativePreviewReadingInset).Gap(nativePreviewContentGap).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if failure != "" {
			ui.Text(c, s.tr(failure)).FontSize(nativeTextBase).FontWeight(nativeWeightBody).TextColor(ink.muted).Wrap().TextAlign(ui.Center).Role(ui.RoleStatus)
			return
		}
		// The image includes its own white quiet zone. Pixelated sampling and a
		// whole-module size keep every edge crisp in both appearance modes.
		ui.Image(c, code.bitmap).Size(size, size).Shrink(0).Fit(ui.Contain).Label(s.tr("QR code"))
		ui.Row(c).Height(nativePreviewMetadataHeight).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, s.tr("Scan with your phone")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).TextColor(ink.muted).SingleLine()
		})
	})
}
