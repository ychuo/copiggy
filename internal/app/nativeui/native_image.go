package nativeui

import (
	"fmt"
	"math"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeImagePreviewHeight = 420

// Image controls stay in the same preview: fitting and magnifying never create
// a second mode, take history focus, or change the saved representation.
func (s *application) nativeImageActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	if s.nativePreviewButton(c, "MagnifyingGlassMinus", s.tr("Zoom out"), p).Disabled(state.ImageZoom <= 0).Clicked() {
		state.ImageZoom = max(0, state.ImageZoom-1)
		s.publish()
	}
	if s.nativePreviewButton(c, "MagnifyingGlassPlus", s.tr("Zoom in"), p).Disabled(state.ImageZoom >= 4).Clicked() {
		state.ImageZoom = min(4, state.ImageZoom+1)
		s.publish()
	}
	if s.nativeSegment(c, s.tr("Fit"), false, p).Clicked() {
		state.ImageZoom = 0
		s.publish()
	}
}

func (s *application) nativeImageCardFallback(c *ui.Context, p nativeColors) {
	// A missing bitmap may still be decoding. Keep the card neutral until the
	// decoder publishes its result instead of announcing an error prematurely.
	ui.Box(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		nativeGlyph(c, "Image", 28).TextColor(p.icon)
	})
}

func (s *application) nativeImagePreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	s.nativeImagePreviewWithBitmap(c, item, p, s.nativeImage(item))
}

func nativeImagePreviewViewport(width, height float32, protected bool) (float32, float32) {
	// Padding + toolbar + metadata + the two gaps. A protected record adds one
	// fixed-height scope row; the image is the only part allowed to shrink.
	remaining := height - nativePreviewChromeHeight - nativePreviewMetadataHeight - nativePreviewSectionGap
	if protected {
		remaining -= nativePreviewMetadataHeight + nativePreviewScopeGap
	}
	return float32(max(0, width-2*nativePreviewEdgeInset)), float32(max(0, remaining))
}

// Use the decoded bitmap dimensions, not a possibly stale metadata hint. Even
// panoramic or tiny images have a finite, aspect-preserving fit at every zoom.
func nativeImageDisplaySize(width, height int, viewportW, viewportH float32, zoom int) (float32, float32) {
	if width <= 0 || height <= 0 || viewportW <= 0 || viewportH <= 0 {
		return 0, 0
	}
	fit := min(float64(viewportW)/float64(width), float64(viewportH)/float64(height))
	scale := fit * math.Pow(1.5, float64(max(0, min(4, zoom))))
	return float32(float64(width) * scale), float32(float64(height) * scale)
}

func (s *application) nativeImagePreviewWithBitmap(c *ui.Context, item clipmodel.Item, p nativeColors, bitmap *ui.Bitmap) {
	state := s.semanticState(item)
	width, height := c.Size()
	viewportW, viewportH := nativeImagePreviewViewport(width, height, len(item.AllowedApps) > 0)
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Box(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Children(func() {
			if bitmap == nil {
				ui.Column(c).Fill().Gap(8).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					nativeGlyph(c, "Image", 28).TextColor(p.icon)
					if !s.nativeState().ImageLoading[item.ID] {
						ui.Text(c, s.tr("Image unavailable")).FontSize(nativeTextBase).TextColor(p.muted).Role(ui.RoleStatus)
					}
				})
				return
			}
			imageW, imageH := bitmap.Size()
			displayW, displayH := nativeImageDisplaySize(imageW, imageH, viewportW, viewportH, state.ImageZoom)
			image := func() {
				ui.Box(c).Size(displayW, displayH).Shrink(0).Radius(nativePreviewPaperRadius).Clip().Children(func() {
					ui.Image(c, bitmap).Fill().Fit(ui.Contain).Label(s.tr("Copied image"))
				})
			}
			if state.ImageZoom == 0 {
				ui.Box(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Children(image)
			} else {
				ui.ScrollBoth(c).Fill().Children(func() {
					// Keep any letterboxed axis centered while the overflowing axis
					// remains fully reachable by native trackpad scrolling.
					ui.Box(c).Size(max(viewportW, displayW), max(viewportH, displayH)).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(image)
				})
			}
		})
		ui.Box(c).FillWidth().Shrink(0).Padding(0, nativePreviewReadingInset).Children(func() {
			s.nativePreviewFooter(c, item, p, func() {
				if item.Image != nil {
					ui.Text(c, fmt.Sprintf("%d × %d · PNG", item.Image.Width, item.Image.Height)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).FontFeatures("tnum").SingleLine()
				}
			})
		})
	})
}
