package nativeui

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const (
	nativeAppLinkSleeveTop  = 136
	nativeAppLinkNotchWidth = 42
	nativeAppLinkNotchDepth = 12
)

func nativeAppLinkColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#fff8ee"), ui.Hex("#413832"), ui.Hex("#77695f")
	p.text, p.muted, p.link = p.paperText, p.paperMeta, p.paper
	p.icon = ui.Hex("#857970")
	p.control, p.hover = ui.RGBA(126, 86, 61, .10), ui.RGBA(126, 86, 61, .07)
	return p
}

// The pass and sleeve share the existing card outline. The only notch belongs
// to the center of the pocket; its side edges meet the card without cutouts.
func nativeAppLinkArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#fffaf4"), To: ui.Hex("#fff4e8"), Angle: 135}, 0)
	left, right, bottom := r.X, r.X+r.W, r.Y+r.H
	middle, top := r.X+r.W/2, r.Y+nativeAppLinkSleeveTop
	lip := func(offset float32) *ui.Path {
		y := top + offset
		half := float32(nativeAppLinkNotchWidth) / 2
		return new(ui.Path).MoveTo(left, y).LineTo(middle-half-2, y).
			QuadTo(middle-half, y, middle-half+1, y+2).
			QuadTo(middle-half+7, y+nativeAppLinkNotchDepth, middle, y+nativeAppLinkNotchDepth).
			QuadTo(middle+half-7, y+nativeAppLinkNotchDepth, middle+half-1, y+2).
			QuadTo(middle+half, y, middle+half+2, y).LineTo(right, y)
	}
	// Short contact shadows make the continuous front pocket occlude the pass.
	p.StrokePath(lip(-.7), 4, ui.Hex("#aa7956").Alpha(.035))
	p.StrokePath(lip(-.3), 1.8, ui.Hex("#aa7956").Alpha(.08))
	front := lip(0).LineTo(right, bottom).LineTo(left, bottom).Close()
	p.FillPathGradient(front, ui.LinearGradient{From: ui.Hex("#fbdcc5"), To: ui.Hex("#f3c6a8"), Angle: 135})
	p.StrokePath(lip(0), .65, ui.Hex("#dfb496").Alpha(.55))
	p.StrokePath(lip(.75), .8, ui.Hex("#fff9ed").Alpha(.94))

	// A recessed capsule sits above the common header, leaving its text and
	// original source-app icon untouched. It is paper relief, not a control.
	slot := ui.Rect{X: middle - 13, Y: r.Y + 8, W: 26, H: 7}
	p.Fill(ui.Rect{X: slot.X, Y: slot.Y + .8, W: slot.W, H: slot.H}, ui.Hex("#fffdf8"), 3.5)
	p.FillGradient(slot, ui.LinearGradient{From: ui.Hex("#dfb496"), To: ui.Hex("#f8ddc7"), Angle: 90}, 3.5)
	p.Stroke(slot, ui.Hex("#c79d7b").Alpha(.32), 3.5, .6)
}

// Center the shared metadata row in the sleeve's straight-sided area, rather
// than anchoring it to the ordinary card's bottom padding.
func nativeAppLinkFooterInset() float32 {
	return (nativeCardSize - nativeAppLinkSleeveTop - nativeCardFooterHeight) / 2
}

func nativeAppLinkCardBody(c *ui.Context, link *clipmodel.LinkValue, p nativeColors) {
	if link == nil || !link.App {
		return
	}
	// Stop the content above the sleeve even for long routes. The body is the
	// shared region beginning at y50; its extra space is reserved for the rim.
	bottom := float32(nativeAppLinkSleeveTop-nativeCardPadding-nativeCardHeaderHeight-nativeCardContentGap) - nativeInlineGap
	ui.Column(c).FillWidth().Height(bottom).MinHeight(0).Gap(nativeInlineGap).AlignItems(ui.Center).Justify(ui.Center).Clip().Children(func() {
		nativeContentText(c, link.Scheme+"://").FontSize(nativeTextTitle).FontWeight(nativeWeightTitle).LineHeight(1.25).FillWidth().MinWidth(0).Shrink(0).SingleLine().TextAlign(ui.Center).TextColor(p.text)
		nativeContentText(c, nativeAppLinkCardRoute(link)).FontSize(nativeTextBody).FontWeight(nativeWeightBody).LineHeight(1.25).MaxLines(2).Wrap().FillWidth().MinHeight(0).TextAlign(ui.Center).TextColor(p.muted)
	})
}

func nativeAppLinkCardRoute(link *clipmodel.LinkValue) string {
	if link == nil || !link.App {
		return ""
	}
	location, err := url.Parse(link.URL)
	if err != nil {
		return ""
	}
	path := location.EscapedPath()
	// Decoding must not introduce controls or directional formatting into the
	// card. The original query/fragment is available only in the full preview.
	if decoded, err := url.PathUnescape(path); err == nil && utf8.ValidString(decoded) && !strings.ContainsFunc(decoded, func(ch rune) bool {
		return unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch)
	}) {
		path = decoded
	}
	return location.Host + path
}
