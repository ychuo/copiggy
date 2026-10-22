package nativeui

import (
	"strconv"
	"strings"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const (
	nativeDatePreviewHeight = 300
	nativeDateHeaderHeight  = 55
	nativeDateCardDaySize   = 50
	nativeDateTileSize      = 128
)

func nativeDateColors() nativeColors {
	p := panelColors(false)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#fffcf9"), ui.Hex("#202a33"), ui.Hex("#697784")
	p.text, p.muted = p.paperText, p.paperMeta
	return p
}

// The whole card is the calendar page. Its narrow month band and large day
// remain the same paper colors in both appearances; only outer chrome changes.
func nativeDateArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#fffefd"), To: ui.Hex("#fffaf5"), Angle: 135}, 0)
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: nativeDateHeaderHeight}, ui.LinearGradient{From: ui.Hex("#fb8175"), To: ui.Hex("#f36f65"), Angle: 135}, 0)
}

func (s *application) nativeDateCard(c *ui.Context, date *clipmodel.DateValue) {
	if date == nil {
		return
	}
	ink := nativeDateColors()
	// The shared body begins below the source-app header. Anchor the month
	// to the card's coral band, not to changes in the day/caption text metrics.
	ui.Text(c, s.nativeMonthLabel(date.Time.Month())).Absolute().Left(0).Right(0).Top(36 - nativeCardPadding - nativeCardHeaderHeight - nativeCardContentGap).Height(18).Font(nativeContentFont).FontSize(12).FontWeight(nativeWeightTitle).TextColor(ui.Hex("#fffaf8")).TextAlign(ui.Center)
	nativeCardHero(c).Gap(0).TextColor(ink.text).Children(func() {
		ui.Text(c, strconv.Itoa(date.Time.Day())).Font(nativeContentFont).FontSize(nativeDateCardDaySize).FontWeight(nativeWeightTitle).FontFeatures("tnum").LineHeight(1.08).FillWidth().TextAlign(ui.Center)
		caption := s.nativeWeekdayLabel(date.Time.Weekday())
		if date.HasTime {
			caption += " · " + date.Time.Format("15:04")
		}
		ui.Text(c, caption).Font(nativeContentFont).FontSize(nativeTextControl).FontWeight(nativeWeightValue).FontFeatures("tnum").TextColor(ink.muted).FillWidth().SingleLine().TextAlign(ui.Center)
	})
}

func (s *application) nativeDatePreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	_, height := c.Size()
	date := clipmodel.Analyze(item.Text).Date
	ink := nativeDateColors()
	ui.Column(c).Fill().Padding(12, 8, 8).Gap(8).Children(func() {
		ui.Box(c).Height(nativeControlSize).FillWidth().Padding(0, 14).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#fffefd"), ui.Hex("#fdf9f5"), 135).TextColor(ink.text).Padding(14).Gap(10).Children(func() {
			contentHeight := height - (12 + nativeControlSize + 8 + 8 + 28 + 10 + nativeCompactControl)
			if len(item.AllowedApps) > 0 {
				contentHeight -= 26
			}
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Row(c).FillWidth().MinHeight(max(nativeDateTileSize+8, contentHeight)).Padding(4, 16).Gap(24).AlignItems(ui.Center).Children(func() {
					if date == nil {
						ui.Text(c, item.Text).FontSize(nativeTextPreview).Wrap().FillWidth()
						return
					}
					s.nativeDateTile(c, date, ink)
					ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
						ui.Text(c, date.Time.Format(s.tr("January 2006"))).Font(nativeContentFont).FontSize(22).FontWeight(nativeWeightTitle).LineHeight(1.2).Wrap().FillWidth()
						ui.Text(c, s.tr(date.Time.Weekday().String())).Font(nativeContentFont).FontSize(nativeTextTitle).FontWeight(nativeWeightBody).TextColor(ink.muted).Wrap().FillWidth()
						clock := s.tr("All day")
						if date.HasTime {
							clock = date.Time.Format("15:04")
						}
						ui.Text(c, clock).Font(nativeContentFont).FontSize(21).FontWeight(nativeWeightBody).FontFeatures("tnum").SingleLine()
					})
				})
			})
			ui.Row(c).Height(nativeCompactControl).Shrink(0).Gap(nativeToolbarGap).AlignItems(ui.Center).TextColor(ink.muted).Children(func() {
				if s.nativeCopyButton(c, ink).Clicked() {
					s.chooseNative(false)
				}
				ui.Text(c, strings.TrimSpace(item.Text)).FontSize(nativeTextMeta).FontWeight(nativeWeightValue).FontFeatures("tnum").Grow(1).MinWidth(0).SingleLine()
				// A date without an explicit timezone is not a UTC instant. Keep
				// the original untouched and avoid inventing an offset for it.
				if date != nil && date.HasTime && date.HasZone {
					ui.Text(c, nativeTimeZoneLabel(date.Time)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).FontFeatures("tnum").Shrink(0).SingleLine()
				}
			})
			s.nativePreviewScope(c, item, ink)
		})
	})
}

func (s *application) nativeDateTile(c *ui.Context, date *clipmodel.DateValue, ink nativeColors) {
	// Clip child fills to the same rounded tile; custom drawing on the tile
	// itself would bypass that clip and bring back square paper corners.
	ui.Column(c).Size(nativeDateTileSize, nativeDateTileSize).Shrink(0).Radius(nativePreviewPaperRadius).Clip().Background(ui.Hex("#ffffff")).Shadow(0, 3, 10, 0, ui.RGBA(92, 64, 49, .07)).Children(func() {
		ui.Box(c).Height(33).FillWidth().Shrink(0).Gradient(ui.Hex("#fb8175"), ui.Hex("#f36f65"), 135).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, s.nativeMonthLabel(date.Time.Month())).Font(nativeContentFont).FontSize(16).FontWeight(nativeWeightTitle).TextColor(ui.Hex("#fffaf8")).SingleLine()
		})
		ui.Box(c).Grow(1).MinHeight(0).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, strconv.Itoa(date.Time.Day())).Font(nativeContentFont).FontSize(64).FontWeight(nativeWeightTitle).FontFeatures("tnum").LineHeight(1.08).TextColor(ink.text)
		})
	})
}
