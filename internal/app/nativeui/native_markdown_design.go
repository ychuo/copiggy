package nativeui

import "github.com/egoist/mygo/ui"

const nativeMarkdownPaperEdge = 10

// Markdown has its own warm-white document surface. The coral binding edge is
// decoration only; source/protection icons and every shared control stay intact.
func nativeMarkdownColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#fffaf6"), ui.Hex("#202b35"), ui.Hex("#718092")
	p.text, p.muted = p.paperText, p.paperMeta
	return p
}

func nativeMarkdownArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#fffdf9"), To: ui.Hex("#fff8f3"), Angle: 135}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: nativeMarkdownPaperEdge, H: r.H},
		ui.LinearGradient{From: ui.Hex("#ff9a80"), To: ui.Hex("#ff8068"), Angle: 90}, 0)
}

// A child edge is clipped by its rounded paper parent. Drawing the edge in the
// parent's paint callback would paint beyond that parent's own rounded corners.
func nativeMarkdownPaperEdgeView(c *ui.Context) {
	ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(nativeMarkdownPaperEdge).
		Gradient(ui.Hex("#ff9a80"), ui.Hex("#ff8068"), 90)
}
