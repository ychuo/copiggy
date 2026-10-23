package nativeui

import (
	"path"
	"strings"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeFilePathPreviewHeight = 300

func nativeFilePathColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#ffe9af"), ui.Hex("#202a33"), ui.Hex("#756e60")
	p.text, p.muted = p.paperText, ui.Hex("#697784")
	return p
}

func nativeFilePathArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#fff3d5"), To: ui.Hex("#ffe8ae"), Angle: 135}, 0)
	// The small back tab sits behind one continuous folder face. Its seam
	// starts below the shared header and never crosses the source-app icon.
	tab := new(ui.Path).MoveTo(r.X, r.Y+42).LineTo(r.X+68, r.Y+42).
		QuadTo(r.X+75, r.Y+42, r.X+79, r.Y+49).
		QuadTo(r.X+83, r.Y+54, r.X+91, r.Y+54).
		LineTo(r.X, r.Y+54).Close()
	p.FillPathGradient(tab, ui.LinearGradient{From: ui.Hex("#ffdf94"), To: ui.Hex("#f5cf7d"), Angle: 160})
	front := new(ui.Path).MoveTo(r.X, r.Y+54).LineTo(r.X+r.W-20, r.Y+54).
		QuadTo(r.X+r.W, r.Y+54, r.X+r.W, r.Y+74).
		LineTo(r.X+r.W, r.Y+r.H).LineTo(r.X, r.Y+r.H).Close()
	p.FillPathGradient(front, ui.LinearGradient{From: ui.Hex("#ffedbd"), To: ui.Hex("#ffe1a1"), Angle: 135})
	seam := new(ui.Path).MoveTo(r.X, r.Y+54).LineTo(r.X+r.W-20, r.Y+54).
		QuadTo(r.X+r.W, r.Y+54, r.X+r.W, r.Y+74)
	p.StrokePath(seam, .7, ui.Hex("#fff9e7").Alpha(.92))
}

// The illustration is derived solely from the copied path. Rendering a path
// must not inspect its target, resolve a home directory or trigger network I/O.
func nativeFileDocument(c *ui.Context, extension string, width, height float32) {
	extension = strings.ToUpper(strings.TrimPrefix(extension, "."))
	ui.Box(c).Size(width, height).Shrink(0).Draw(nativeFileDocumentArtwork).Children(func() {
		if extension != "" {
			size := nativeEmailFittedSize(extension, width*.2, 5, width-8, nativeWeightValue)
			ui.Text(c, extension).Absolute().Left(4).Right(4).Bottom(height * .09).FontSize(size).FontWeight(nativeWeightValue).TextColor(ui.Hex("#798392")).SingleLine().TextAlign(ui.Center)
		}
	})
}

func nativeFileDocumentArtwork(p *ui.Painter, r ui.Rect) {
	left, top, right, bottom := r.X+1, r.Y+1, r.X+r.W-1, r.Y+r.H-2
	fold := r.W * .29
	page := new(ui.Path).MoveTo(left+2, top).LineTo(right-fold, top).
		LineTo(right, top+fold).LineTo(right, bottom-2).
		QuadTo(right, bottom, right-2, bottom).LineTo(left+2, bottom).
		QuadTo(left, bottom, left, bottom-2).LineTo(left, top+2).
		QuadTo(left, top, left+2, top).Close()
	p.StrokePath(page, 3, ui.Hex("#7e7462").Alpha(.035))
	p.FillPathGradient(page, ui.LinearGradient{From: ui.Hex("#ffffff"), To: ui.Hex("#eef1f5"), Angle: 135})
	p.StrokePath(page, .6, ui.Hex("#bbc4d0").Alpha(.7))
	folded := new(ui.Path).MoveTo(right-fold, top).LineTo(right-fold, top+fold-2).
		QuadTo(right-fold, top+fold, right-fold+2, top+fold).
		LineTo(right, top+fold).Close()
	p.StrokePath(folded, 2, ui.Hex("#8994a3").Alpha(.10))
	p.FillPathGradient(folded, ui.LinearGradient{From: ui.Hex("#dbe1ea"), To: ui.Hex("#ffffff"), Angle: 135})
	// A restrained document motif remains recognizable for every extension.
	// It intentionally does not impersonate another application's file icon.
	lineX, lineW := left+r.W*.19, r.W*.48
	for i := 0; i < 3; i++ {
		w := lineW
		if i == 2 {
			w *= .7
		}
		p.Fill(ui.Rect{X: lineX, Y: top + r.H*.40 + float32(i)*r.H*.09, W: w, H: max(1, r.W*.025)}, ui.Hex("#92a1b3").Alpha(.55), .6)
	}
}

func (s *application) nativeFilePathCard(c *ui.Context, item clipmodel.Item, file *clipmodel.FilePathValue) {
	if file == nil {
		return
	}
	ink := nativeFilePathColors()
	ui.Row(c).Fill().Gap(11).Padding(12, 0, 0).AlignItems(ui.Center).TextColor(ink.text).Children(func() {
		nativeFileDocument(c, file.Ext, 38, 51)
		ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
			size := float32(nativeTextTitle)
			if utf8.RuneCountInString(file.Name) > 15 {
				size = nativeTextBody
			}
			nativeContentText(c, file.Name).FontSize(size).FontWeight(nativeWeightTitle).LineHeight(1.2).MaxLines(2).Wrap().FillWidth()
			nativeContentText(c, path.Base(file.Dir)).FontSize(nativeTextControl).FontWeight(nativeWeightBody).TextColor(ink.paperMeta).SingleLine().FillWidth()
		})
	})
}

func (s *application) nativeFilePathPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	_, height := c.Size()
	file := clipmodel.Analyze(item.Text).FilePath
	ink := nativeFilePathColors()
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#fffefd"), ui.Hex("#f8f9fb"), 135).TextColor(ink.text).Padding(nativePreviewReadingInset).Gap(nativePreviewContentGap).Children(func() {
			contentHeight := height - nativePreviewChromeHeight - 2*nativePreviewReadingInset - nativePreviewContentGap - nativePreviewFooterHeight(item, true)
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Row(c).FillWidth().MinHeight(max(110, contentHeight)).Padding(12, 8).Gap(24).AlignItems(ui.Center).Children(func() {
					if file == nil {
						nativeContentText(c, item.Text).FontSize(nativeTextPreview).Wrap().FillWidth()
						return
					}
					nativeFileDocument(c, file.Ext, 70, 92)
					ui.Column(c).Grow(1).MinWidth(0).Gap(10).Children(func() {
						nativeContentText(c, file.Name).FontSize(25).FontWeight(nativeWeightTitle).LineHeight(1.2).Wrap().FillWidth()
						nativeContentText(c, file.Path).FontSize(nativeTextPreview).FontWeight(nativeWeightBody).LineHeight(1.4).TextColor(ink.muted).Wrap().FillWidth()
					})
				})
			})
			s.nativePreviewFooter(c, item, ink, func() {
				if s.nativeCopyButton(c, ink).Clicked() {
					s.chooseNative(false)
				}
				ui.Text(c, s.tr("File path")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).SingleLine()
			})
		})
	})
}
