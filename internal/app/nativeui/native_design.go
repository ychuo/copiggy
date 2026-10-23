package nativeui

import "github.com/egoist/mygo/ui"

// Shared visual roles for every card, preview and panel control. Type-specific
// artwork and display typography are content, not alternative control systems.
// The product-wide specification lives in docs/design/design-system.md.
const (
	nativeCardSize         = 188
	nativeCardRadius       = 20
	nativeCardPadding      = 16
	nativeCardGap          = 14
	nativeCardContentGap   = 10
	nativeCardHeaderHeight = 24
	nativeCardFooterHeight = 16
	nativeTrayInset        = 22
	nativeTrayHeight       = 296

	nativeIconSize           = 16
	nativeToolbarIconSize    = 18
	nativeSourceIconSize     = 24
	nativeBadgeSize          = 15
	nativeBadgeIconSize      = 10
	nativeControlSize        = 30
	nativeCompactControl     = 22
	nativeControlRadius      = 6
	nativePreviewPaperRadius = 12
	nativeToolbarGap         = 8
	nativeInlineGap          = 6
	nativeKeySize            = 20

	// Preview composition roles keep controls and metadata stable while each
	// content type retains its own artwork and display typography.
	nativePreviewTopInset       = 12
	nativePreviewEdgeInset      = 8
	nativePreviewSectionGap     = 8
	nativePreviewReadingInset   = 14
	nativePreviewHeroInset      = 18
	nativePreviewDataInset      = 22
	nativePreviewContentGap     = 12
	nativePreviewMetadataHeight = nativeCompactControl
	nativePreviewScopeGap       = 6
	nativePreviewChromeHeight   = nativePreviewTopInset + nativeControlSize + nativePreviewSectionGap + nativePreviewEdgeInset

	nativeCodeFont      = "monospace"
	nativeTextHint      = 10
	nativeTextMeta      = 11
	nativeTextControl   = 12
	nativeTextBase      = 13
	nativeTextBody      = 15
	nativeTextTitle     = 18
	nativeTextPreview   = 17
	nativeWeightBody    = 400
	nativeWeightTitle   = 500
	nativeWeightControl = 500
	nativeWeightValue   = 500
)

type nativeColors struct{ text, muted, edge, control, hover, selection, paper, paperText, paperMeta, link, warning, icon ui.Color }

func nativePreviewLayout(c *ui.Context) ui.Element {
	return ui.Column(c).Fill().Padding(nativePreviewTopInset, nativePreviewEdgeInset, nativePreviewEdgeInset).Gap(nativePreviewSectionGap)
}

func nativePreviewToolbarRow(c *ui.Context) ui.Element {
	return ui.Box(c).Height(nativeControlSize).Shrink(0).FillWidth().Padding(0, nativePreviewReadingInset)
}

// The header is taller than the footer, so the shared body is slightly below
// the square's center. Center short hero content on the card itself without
// moving the independent header, metadata or pin controls.
func nativeCardHero(c *ui.Context) ui.Element {
	offset := float32(nativeCardFooterHeight-nativeCardHeaderHeight) / 2
	return ui.Column(c).Absolute().Left(0).Right(0).Top(offset).Bottom(-offset).AlignItems(ui.Center).Justify(ui.Center)
}

func panelColors(dark bool) nativeColors {
	if dark {
		return nativeColors{
			text: ui.Hex("#f4f6fa"), muted: ui.Hex("#b6bdc6"),
			edge: ui.RGBA(231, 240, 250, .22), control: ui.RGBA(222, 234, 248, .12),
			hover: ui.RGBA(230, 241, 255, .22), selection: ui.Hex("#86bafa"),
			paper: ui.Hex("#39382f"), paperText: ui.Hex("#f5edd9"), paperMeta: ui.Hex("#cec7b8"),
			link: ui.Hex("#2d3945"), warning: ui.Hex("#f1c987"), icon: ui.Hex("#adb5bf"),
		}
	}
	return nativeColors{
		text: ui.Hex("#202831"), muted: ui.Hex("#69727c"),
		edge: ui.RGBA(255, 255, 255, .66), control: ui.RGBA(46, 66, 86, .075),
		hover: ui.RGBA(46, 66, 86, .15), selection: ui.Hex("#3283d9"),
		paper: ui.Hex("#fbf0d7"), paperText: ui.Hex("#353025"), paperMeta: ui.Hex("#675e4d"),
		link: ui.Hex("#f8fafc"), warning: ui.Hex("#825019"), icon: ui.Hex("#78818b"),
	}
}

// The AppKit host owns the surface material. MyGo paints only content and
// controls onto its transparent native layer.
func setNativeTheme(c *ui.Context, p nativeColors, dark bool, language ...string) {
	_ = loadNativeFonts()
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
	}
	t.Background = ui.Transparent
	t.Text = p.text
	t.TextMuted = p.muted
	t.Surface = p.control
	t.SurfaceHover = p.hover
	t.SurfacePressed = p.hover
	t.Accent = p.selection
	// Tooltip's default text falls back to Background, which is transparent
	// for our native material. Give both tooltip roles explicit opaque colors.
	t.Inverse = p.link
	t.InverseText = p.text
	t.Font = nativeUIFont
	if len(language) > 0 {
		t.Font = nativeFontForLanguage(language[0])
	}
	t.FontSize = nativeTextBase
	t.Radius = 8
	c.SetTheme(&t)
}
