package nativeui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// Content has its own visual language inside a shared card frame. These values
// are derived for display only; neither rendering nor selecting a card rewrites
// its captured representation or protection scope.
func nativeContentColors(kind clipmodel.ContentType, fallback nativeColors) nativeColors {
	switch kind {
	case clipmodel.TypeText:
		return nativeNoteColors()
	case clipmodel.TypeMarkdown:
		return nativeMarkdownColors()
	case clipmodel.TypeAppLink:
		return nativeAppLinkColors()
	case clipmodel.TypeJSON:
		return nativeJSONColors()
	case clipmodel.TypeLink:
		return nativeLinkColors()
	case clipmodel.TypeEmail:
		return nativeEmailColors()
	case clipmodel.TypePhone:
		return nativePhoneColors()
	case clipmodel.TypeCoordinates:
		return nativeMapColors()
	case clipmodel.TypeDate:
		return nativeDateColors()
	case clipmodel.TypeTimestamp:
		return nativeTimestampColors()
	case clipmodel.TypeFilePath:
		return nativeFilePathColors()
	case clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey:
		return nativeSecretCardColors(kind)
	default:
		return fallback
	}
}

func nativeCardPalette(kind clipmodel.ContentType, value clipmodel.Analysis, p nativeColors) (background, foreground, metadata ui.Color) {
	p = nativeContentColors(kind, p)
	background, foreground, metadata = p.paper, p.paperText, p.paperMeta
	switch kind {
	case clipmodel.TypeImage:
		background, foreground, metadata = ui.Transparent, ui.Hex("#ffffff"), ui.Hex("#ffffff")
	case clipmodel.TypeColor:
		if value.Color != nil {
			background, foreground = nativeColorSurface(*value.Color), nativeColorInk(*value.Color)
			metadata = foreground.Alpha(.84)
		}
	}
	return
}

func nativeTypeIcon(kind clipmodel.ContentType) string {
	switch kind {
	case clipmodel.TypeJSON:
		return "BracketsCurly"
	case clipmodel.TypeMarkdown:
		return "Markdown"
	case clipmodel.TypeAppLink:
		return "ArrowSquareOut"
	case clipmodel.TypeImage:
		return "Image"
	case clipmodel.TypeLink:
		return "LinkSimple"
	case clipmodel.TypeEmail:
		return "Envelope"
	case clipmodel.TypePhone:
		return "Phone"
	case clipmodel.TypeCoordinates:
		return "MapPin"
	case clipmodel.TypeDate:
		return "Calendar"
	case clipmodel.TypeTimestamp:
		return "Clock"
	case clipmodel.TypeColor:
		return "Palette"
	case clipmodel.TypeFilePath:
		return "Folder"
	case clipmodel.TypeOTP:
		return "Password"
	case clipmodel.TypeToken, clipmodel.TypePrivateKey:
		return "Key"
	default:
		return "TextT"
	}
}

func (s *application) nativeTypeLabel(kind clipmodel.ContentType) string {
	switch kind {
	case clipmodel.TypeMarkdown:
		return s.tr("Markdown")
	case clipmodel.TypeAppLink:
		return s.tr("App scheme")
	case clipmodel.TypeEmail:
		return s.tr("Email")
	case clipmodel.TypePhone:
		return s.tr("Phone")
	case clipmodel.TypeCoordinates:
		return s.tr("Coordinates")
	case clipmodel.TypeDate:
		return s.tr("Date")
	case clipmodel.TypeTimestamp:
		return s.tr("Timestamp")
	case clipmodel.TypeColor:
		return s.tr("Color")
	case clipmodel.TypeFilePath:
		return s.tr("File path")
	case clipmodel.TypeOTP:
		return s.tr("Verification code")
	case clipmodel.TypeToken:
		return s.tr("Token")
	case clipmodel.TypePrivateKey:
		return s.tr("Private key")
	default:
		return string(kind)
	}
}

func (s *application) nativeCardContent(c *ui.Context, card ui.Element, index int, item clipmodel.Item, value clipmodel.Analysis, fg, meta ui.Color, p nativeColors) {
	state := s.nativeState()
	left := state.Tray.X + nativeTrayInset + float32(index)*(nativeCardSize+nativeCardGap) - state.Scroll.X
	nearViewport := left+nativeCardSize >= state.Tray.X-(nativeCardSize+nativeCardGap) && left <= state.Tray.X+state.Tray.W+(nativeCardSize+nativeCardGap)
	var assets nativeCardAssets
	if nearViewport {
		switch item.Type {
		case clipmodel.TypeImage:
			assets.bitmap = s.nativeImage(item)
		case clipmodel.TypeCoordinates:
			assets.bitmap = s.nativeMapBitmap(item)
		case clipmodel.TypeLink:
			assets.linkTitle, assets.linkCover = s.nativeLinkCardMetadata(item)
			assets.linkIcon = s.nativeWebsiteIcon(item)
		}
	}
	s.nativeCardContentWithAssets(c, card, item, value, fg, meta, p, assets)
}

type nativeCardAssets struct {
	bitmap, linkCover, linkIcon *ui.Bitmap
	linkTitle                   string
}

// Keep layout independent of asynchronous resource acquisition. The same card
// geometry applies to loading content and completed native snapshots.
func (s *application) nativeCardContentWithAssets(c *ui.Context, card ui.Element, item clipmodel.Item, value clipmodel.Analysis, fg, meta ui.Color, p nativeColors, assets nativeCardAssets) {
	state := s.nativeState()
	bitmap, linkCover, linkTitle := assets.bitmap, assets.linkCover, assets.linkTitle
	if item.Type != clipmodel.TypeLink {
		linkCover, linkTitle, assets.linkIcon = nil, "", nil
	}
	// Selection belongs to the tray; light content surfaces retain their own
	// contrasting ink and controls when the outer glass uses dark mode.
	p = nativeContentColors(item.Type, p)

	glassCard := item.Type == clipmodel.TypeImage || item.Type == clipmodel.TypeCoordinates && bitmap != nil
	var metadataGlass ui.Material
	headerInk, footerInk, typeInk := meta, meta, p.icon
	if glassCard || linkCover != nil {
		metadataGlass, headerInk = nativeImageGlass(state.Dark && item.Type != clipmodel.TypeCoordinates)
		typeInk = headerInk
		if glassCard {
			footerInk = headerInk
		}
	} else if item.Type == clipmodel.TypeColor {
		typeInk = fg
	} else if item.Type == clipmodel.TypeJSON || item.Type == clipmodel.TypeTimestamp || item.Type == clipmodel.TypeToken {
		typeInk = ui.Hex("#adb5bf")
	} else if item.Type == clipmodel.TypeDate {
		headerInk, typeInk = ui.Hex("#fffaf8"), ui.Hex("#fffaf8")
	}
	ui.Box(c).Fill().Draw(func(painter *ui.Painter, bounds ui.Rect) {
		nativeCardArtwork(painter, bounds, item.Type, value, fg, state.Dark)
		if bitmap != nil {
			if item.Type == clipmodel.TypeCoordinates {
				painter.Image(bitmap, bounds, ui.Contain)
				nativeMapMarker(painter, bounds, 1)
			} else {
				painter.Image(bitmap, bounds, ui.Cover)
			}
		}
		if linkCover != nil {
			painter.Image(linkCover, ui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: nativeLinkCoverHeight}, ui.Cover)
		}
	}).Children(func() {
		bottomInset := float32(nativeCardPadding)
		if item.Type == clipmodel.TypeAppLink {
			bottomInset = nativeAppLinkFooterInset()
		}
		ui.Column(c).Fill().Padding(nativeCardPadding, nativeCardPadding, bottomInset).Gap(nativeCardContentGap).TextColor(fg).Children(func() {
			ui.Row(c).Height(nativeCardHeaderHeight).Gap(nativeInlineGap).AlignItems(ui.Center).Children(func() {
				header := ui.Row(c).Gap(5).Radius(5).AlignItems(ui.Center)
				if metadataGlass != nil {
					header.Padding(2, 4).Material(metadataGlass)
				}
				header.Children(func() {
					nativeGlyph(c, nativeTypeIcon(item.Type), nativeIconSize).TextColor(typeInk)
					ui.Text(c, s.relativeTime(item.CreatedAt)).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).FontFeatures("tnum").TextColor(headerInk).SingleLine()
				})
				ui.Spacer(c)
				s.nativeSourceIcon(c, item, p)
			})
			body := ui.Box(c).Grow(1).MinHeight(0).FillWidth()
			body.Children(func() {
				s.nativeContentCardBody(c, item, value, fg, meta, p, bitmap != nil, linkTitle, linkCover != nil, assets.linkIcon)
			})
			ui.Row(c).Height(nativeCardFooterHeight).Gap(nativeToolbarGap).AlignItems(ui.Center).TextColor(footerInk).Children(func() {
				label := s.nativeContentCardFooter(item, value)
				if label != "" {
					footer := ui.Box(c).Radius(5).MinWidth(0).Shrink(1)
					if glassCard {
						footer.Padding(2, 4).Material(metadataGlass)
					}
					footer.Children(func() {
						weight := nativeWeightBody
						if item.Type == clipmodel.TypeCoordinates {
							weight = nativeWeightValue
						}
						ui.Text(c, label).FontSize(nativeTextMeta).FontWeight(weight).FontFeatures("tnum").SingleLine()
					})
				}
				ui.Spacer(c)
				if item.Pinned || card.Hovered() || card.FocusVisible() {
					label := s.tr("Pin")
					if item.Pinned {
						label = s.tr("Unpin")
					}
					ink := p.icon
					if item.Type == clipmodel.TypeColor {
						ink = fg
					}
					pin := ui.ButtonBase(c).Key("pin").Label(label).Description(s.tr("Pin / Unpin · ⌘P")).KeepFocus().Size(nativeCompactControl, nativeCompactControl).Shrink(0).Radius(nativeControlRadius).Background(ui.Transparent).TextColor(ink).Padding(3)
					if pin.Hovered() {
						pin.Background(p.control)
					}
					if glassCard {
						pin.Material(metadataGlass).TextColor(footerInk)
					}
					pinName := "PushPin"
					if item.Pinned {
						pinName = "PushPinFill"
					}
					pin.Children(func() { nativeGlyph(c, pinName, nativeIconSize) })
					if pin.Clicked() {
						s.setNativePinned(item.ID, !item.Pinned)
					}
				}
			})
		})
	})
}

func (s *application) nativeContentCardBody(c *ui.Context, item clipmodel.Item, value clipmodel.Analysis, fg, meta ui.Color, p nativeColors, hasBitmap bool, linkTitle string, hasCover bool, linkIcon *ui.Bitmap) {
	switch item.Type {
	case clipmodel.TypeImage:
		if !hasBitmap {
			s.nativeImageCardFallback(c, p)
		}
	case clipmodel.TypeJSON:
		nativeJSONCard(c, item)
	case clipmodel.TypeMarkdown:
		nativeMarkdownCard(c, item, value.Markdown)
	case clipmodel.TypeAppLink:
		nativeAppLinkCardBody(c, value.Link, p)
	case clipmodel.TypeLink:
		s.nativeLinkCardBodyWithIcon(c, item, linkTitle, hasCover, linkIcon)
	case clipmodel.TypeEmail:
		if email := value.Email; email != nil {
			nativeEmailCard(c, *email, meta)
		}
	case clipmodel.TypePhone:
		if phone := value.Phone; phone != nil {
			nativePhoneCard(c, *phone)
		}
	case clipmodel.TypeCoordinates:
		if !hasBitmap && value.Coordinates != nil {
			s.nativeMapCardFallback(c, item, p)
		}
	case clipmodel.TypeDate:
		s.nativeDateCard(c, value.Date)
	case clipmodel.TypeTimestamp:
		if stamp := value.Timestamp; stamp != nil {
			s.nativeTimestampCard(c, item, stamp)
		}
	case clipmodel.TypeColor:
		if color := value.Color; color != nil {
			nativeColorCard(c, *color, meta)
		}
	case clipmodel.TypeFilePath:
		s.nativeFilePathCard(c, item, value.FilePath)
	case clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey:
		s.nativeSecretCard(c, item, value)
	default:
		nativeNoteCard(c, item)
	}
}

func (s *application) nativeContentCardFooter(item clipmodel.Item, value clipmodel.Analysis) string {
	switch item.Type {
	case clipmodel.TypeJSON:
		return s.nativeJSONSummary(item.Text)
	case clipmodel.TypeMarkdown:
		return s.tr("Markdown")
	case clipmodel.TypeAppLink:
		return s.tr("App scheme")
	case clipmodel.TypeImage:
		if item.Image != nil {
			return fmt.Sprintf("%d × %d", item.Image.Width, item.Image.Height)
		}
		return ""
	case clipmodel.TypeLink:
		return nativeLinkCardHost(item)
	case clipmodel.TypeEmail, clipmodel.TypeColor:
		return ""
	case clipmodel.TypePhone:
		if value.Phone != nil && value.Phone.International {
			if prefix := nativePhonePrefix(*value.Phone); prefix != "" {
				return prefix
			}
			return s.tr("International")
		}
		return s.tr("Phone")
	case clipmodel.TypeCoordinates:
		if value.Coordinates != nil {
			return fmt.Sprintf("%.4f, %.4f", value.Coordinates.Latitude, value.Coordinates.Longitude)
		}
	case clipmodel.TypeDate:
		if value.Date != nil {
			if value.Date.HasTime && value.Date.HasZone {
				return nativeTimeZoneLabel(value.Date.Time)
			}
			return fmt.Sprint(value.Date.Time.Year())
		}
	case clipmodel.TypeTimestamp:
		if value.Timestamp != nil && value.Timestamp.Unit == "ms" {
			return s.tr("Milliseconds")
		}
		return s.tr("Seconds")
	case clipmodel.TypeFilePath:
		if value.FilePath != nil && value.FilePath.Ext != "" {
			return strings.ToUpper(strings.TrimPrefix(value.FilePath.Ext, "."))
		}
		return s.tr("File path")
	case clipmodel.TypeOTP:
		if value.Secret != nil {
			return s.tr("%d digits", value.Secret.Length)
		}
		return s.tr("Verification code")
	case clipmodel.TypeToken, clipmodel.TypePrivateKey:
		if value.Secret != nil {
			return s.tr(value.Secret.Kind)
		}
		return s.nativeTypeLabel(item.Type)
	default:
		return s.tr("Plain text")
	}
	return ""
}

func (s *application) nativeMonthLabel(month time.Month) string {
	if s.currentLanguage() == "en" {
		return strings.ToUpper(month.String()[:3])
	}
	return s.tr(month.String())
}

func (s *application) nativeWeekdayLabel(day time.Weekday) string {
	if s.currentLanguage() == "en" {
		return day.String()[:3]
	}
	return s.tr(day.String())
}

func nativeTimeZoneLabel(t time.Time) string {
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign, offset = "-", -offset
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, offset/3600, offset%3600/60)
}

// Alpha colors are previewed over a neutral checkerboard. Contrast is chosen
// against its middle tone rather than against the translucent desktop.
func nativeColorSurface(value clipmodel.ColorValue) ui.Color {
	a := float64(value.A) / 255
	channel := func(v uint8) uint8 { return uint8(math.Round(float64(v)*a + 240*(1-a))) }
	return ui.RGB(channel(value.R), channel(value.G), channel(value.B))
}

func nativeColorInk(value clipmodel.ColorValue) ui.Color {
	background := nativeColorSurface(value)
	luma := func(c ui.Color) float64 {
		channel := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= .04045 {
				return x / 12.92
			}
			return math.Pow((x+.055)/1.055, 2.4)
		}
		return .2126*channel(c.R) + .7152*channel(c.G) + .0722*channel(c.B)
	}
	dark, light := ui.Hex("#222831"), ui.Hex("#fffdf9")
	lum := luma(background)
	if (lum+.05)/(luma(dark)+.05) >= (luma(light)+.05)/(lum+.05) {
		return dark
	}
	return light
}

func nativeCardArtwork(p *ui.Painter, r ui.Rect, kind clipmodel.ContentType, value clipmodel.Analysis, ink ui.Color, dark bool) {
	switch kind {
	case clipmodel.TypeText:
		nativeNoteArtwork(p, r)
	case clipmodel.TypeMarkdown:
		nativeMarkdownArtwork(p, r)
	case clipmodel.TypeAppLink:
		nativeAppLinkArtwork(p, r)
	case clipmodel.TypeJSON:
		nativeJSONArtwork(p, r)
	case clipmodel.TypeLink:
		nativeLinkArtwork(p, r)
	case clipmodel.TypeEmail:
		nativeEmailArtwork(p, r)
	case clipmodel.TypePhone:
		nativePhoneArtwork(p, r)
	case clipmodel.TypeDate:
		nativeDateArtwork(p, r)
	case clipmodel.TypeTimestamp:
		nativeTimestampArtwork(p, r)
	case clipmodel.TypeColor:
		if color := value.Color; color != nil {
			nativeColorArtwork(p, r, *color)
		}
	case clipmodel.TypeFilePath:
		nativeFilePathArtwork(p, r)
	case clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey:
		nativeSecretArtwork(p, r, kind)
	}
}
