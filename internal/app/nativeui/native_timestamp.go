package nativeui

import (
	"strconv"
	"strings"
	"time"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

const nativeTimestampPreviewHeight = 310

// A timestamp keeps its instrument-like blue surface in both appearances.
// Only the containing glass and its toolbar follow the selected theme.
func nativeTimestampColors() nativeColors {
	p := panelColors(true)
	p.paper, p.paperText, p.paperMeta = ui.Hex("#303b4a"), ui.Hex("#f1f5fc"), ui.Hex("#afc0d8")
	p.text, p.muted, p.icon = p.paperText, p.paperMeta, ui.Hex("#c0cddd")
	return p
}

func nativeTimestampArtwork(p *ui.Painter, r ui.Rect) {
	p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#3c4a60"), To: ui.Hex("#29313b"), Angle: 135}, 0)
	for i := 0; i < 17; i++ {
		height, ink := float32(5), ui.Hex("#9bb5d7")
		if i%4 == 0 {
			height = 8
		}
		if i == 8 {
			height, ink = 18, ui.Hex("#edf4ff")
		}
		p.Fill(ui.Rect{X: r.X + 22 + float32(i)*9, Y: r.Y + 110 - height/2, W: 1.4, H: height}, ink, .7)
	}
}

func (s *application) nativeTimestampCard(c *ui.Context, item clipmodel.Item, stamp *clipmodel.TimestampValue) {
	ink := nativeTimestampColors()
	size := float32(22)
	if stamp.Unit == "ms" {
		size = 17
	}
	local := stamp.Time.In(time.Local)
	ui.Box(c).Fill().Children(func() {
		ui.Text(c, strings.TrimSpace(item.Text)).Absolute().Top(18).Left(0).Right(0).Font(nativeCodeFont).FontSize(size).FontWeight(nativeWeightTitle).FontFeatures("tnum").LineHeight(1.15).SingleLine().TextAlign(ui.Center)
		ui.Text(c, local.Format(s.tr("Jan 2"))+" · "+local.Format("15:04")).Absolute().Top(76).Left(0).Right(0).FontSize(nativeTextControl).FontWeight(nativeWeightBody).FontFeatures("tnum").TextColor(ink.paperMeta).SingleLine().TextAlign(ui.Center)
	})
}

func (s *application) nativeTimestampActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	nativeSegmentGroup(c, p).Children(func() {
		for index, unit := range []struct{ symbol, label string }{{"s", "Seconds"}, {"ms", "Milliseconds"}} {
			selected := state.Milliseconds == (index == 1)
			button := s.nativeFormatSegment(c, unit.symbol, selected, p).Label(s.tr(unit.label)).Width(36).Disabled(s.commandBusy)
			if button.Clicked() {
				state.Milliseconds = index == 1
				state.CopyFormat = clipmodel.TimestampSeconds
				if state.Milliseconds {
					state.CopyFormat = clipmodel.TimestampMilliseconds
				}
				s.publish()
			}
		}
	})
	stamp := clipmodel.Analyze(item.Text).Timestamp
	if stamp == nil {
		return
	}
	label, zone := s.tr("Local time"), nativeTimeZoneLabel(stamp.Time.In(time.Local))
	if state.UTC {
		label, zone = s.tr("UTC"), s.tr("UTC")
	}
	button := ui.ButtonBase(c).Label(label).Height(nativeControlSize).Padding(0, 8).Radius(nativeControlRadius).Gap(nativeInlineGap).AlignItems(ui.Center).KeepFocus().Shrink(0).TextColor(p.muted).Background(ui.Transparent)
	if button.Hovered() || button.Pressed() {
		button.Background(p.hover)
	}
	button.Children(func() {
		ui.Text(c, zone).FontSize(nativeTextControl).FontWeight(nativeWeightControl).FontFeatures("tnum").SingleLine()
		nativeGlyph(c, "ArrowsLeftRight", nativeIconSize).TextColor(p.icon)
	})
	if button.Clicked() {
		state.UTC = !state.UTC
		s.publish()
	}
}

func (s *application) nativeTimestampDate(t time.Time) string {
	return t.Format(s.tr("Jan 2, 2006"))
}

func nativeTimestampClock(t time.Time) string {
	// Keep captured sub-second precision visible in the converted time even
	// while the numeric display is switched from milliseconds to seconds.
	return t.Format("15:04:05.999")
}

func nativeTimestampNumberSize(value string, width float32) float32 {
	const preferred = float32(42)
	layout := ui.ShapeText(value, ui.Font{Family: nativeCodeFont, Size: preferred, Weight: nativeWeightTitle, Features: "tnum"}, 0)
	measured, _ := layout.Size()
	if measured <= width {
		return preferred
	}
	return max(18, preferred*width/measured)
}

func (s *application) nativeTimestampPreview(c *ui.Context, item clipmodel.Item, p nativeColors) {
	width, height := c.Size()
	stamp := clipmodel.Analyze(item.Text).Timestamp
	state, ink := s.semanticState(item), nativeTimestampColors()
	nativePreviewLayout(c).Children(func() {
		nativePreviewToolbarRow(c).Children(func() {
			s.nativePreviewToolbar(c, item, p)
		})
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Radius(nativePreviewPaperRadius).Clip().Gradient(ui.Hex("#2c3541"), ui.Hex("#3d5273"), 135).TextColor(ink.text).Padding(nativePreviewDataInset).Gap(nativePreviewContentGap).Children(func() {
			contentHeight := height - (nativePreviewChromeHeight + 2*nativePreviewDataInset)
			if len(item.AllowedApps) > 0 {
				contentHeight -= nativePreviewFooterHeight(item, false) + nativePreviewContentGap
			}
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).MinHeight(max(0, contentHeight)).FillWidth().Justify(ui.Center).Gap(12).Children(func() {
					if stamp == nil {
						ui.Text(c, item.Text).Font(nativeCodeFont).FontSize(nativeTextPreview).Wrap().FillWidth()
						return
					}
					number := stamp.Seconds
					if state.Milliseconds {
						number = stamp.Milliseconds
					}
					date, alternate := stamp.Time.In(time.Local), stamp.Time.UTC()
					if state.UTC {
						date, alternate = stamp.Time.UTC(), stamp.Time.In(time.Local)
					}
					counter := strconv.FormatInt(number, 10)
					ui.Text(c, counter).Font(nativeCodeFont).FontSize(nativeTimestampNumberSize(counter, width-2*(nativePreviewEdgeInset+nativePreviewDataInset))).FontWeight(nativeWeightTitle).FontFeatures("tnum").LineHeight(1.15).FillWidth().SingleLine()
					ui.Column(c).Gap(3).FillWidth().Children(func() {
						ui.Text(c, s.nativeTimestampDate(date)).FontSize(23).FontWeight(nativeWeightValue).FontFeatures("tnum").SingleLine()
						ui.Text(c, nativeTimestampClock(date)).FontSize(23).FontWeight(nativeWeightValue).FontFeatures("tnum").SingleLine()
					})
					zone := s.tr("UTC")
					if state.UTC {
						zone = nativeTimeZoneLabel(alternate)
					}
					other := zone + " " + nativeTimestampClock(alternate)
					if date.Format("2006-01-02") != alternate.Format("2006-01-02") {
						other = zone + " · " + s.nativeTimestampDate(alternate) + " · " + nativeTimestampClock(alternate)
					}
					ui.Text(c, other).FontSize(nativeTextBody).FontWeight(nativeWeightBody).FontFeatures("tnum").TextColor(ink.muted).Wrap().FillWidth()
				})
			})
			s.nativePreviewFooter(c, item, ink, nil)
		})
	})
}
