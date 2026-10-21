package nativeui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"copiggy/internal/linkicons"
	"copiggy/resources"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	nativeCardSize   = 188
	nativeCardGap    = 14
	nativeTrayInset  = 22
	nativeTrayHeight = 296
	// Use the same macOS system sans-serif for controls and copied prose.
	// Core Text supplies its optical sizing and CJK/emoji fallbacks.
	nativeUIFont      = "system-ui"
	nativeContentFont = nativeUIFont
	nativeCodeFont    = "monospace"
)

// Native rendering only: these Phosphor SVGs are parsed once, with no web engine.
var nativeIcons = func() map[string]*ui.SVG {
	icons := make(map[string]*ui.SVG)
	entries, _ := resources.Icons.ReadDir("icons")
	for _, entry := range entries {
		data, err := resources.Icons.ReadFile("icons/" + entry.Name())
		if err == nil {
			icons[strings.TrimSuffix(entry.Name(), ".svg")] = ui.MustParseSVG(data)
		}
	}
	return icons
}()

type nativePanelState struct {
	SearchOpen, Original, Zoom, Dark bool
	PinnedOnly                       bool
	Scroll                           ui.ScrollState
	Focus, PreviewFocus              ui.Handle
	LastSelected, DraftCurrent       string
	TrayHint, PopoverHint            string
	Images, Icons                    map[string]*ui.Bitmap
	LinkIcons                        *linkicons.Cache
	ImageLoading                     map[string]bool
	WebsiteIcons                     map[string]*ui.Bitmap
	WebsiteHTML                      map[string][]byte
	CardBounds                       map[string]ui.Rect
	Tray                             ui.Rect
	EmptyAnimationStart              time.Time
	PinAnimationStart                time.Time
}

type nativeColors struct{ text, muted, glass, edge, control, hover, selection, paper, paperText, paperMeta, link, warning, icon ui.Color }

func panelColors(dark bool) nativeColors {
	if dark {
		return nativeColors{ui.Hex("#f4f6fa"), ui.Hex("#b6bdc6"), ui.RGBA(25, 31, 39, .42), ui.RGBA(231, 240, 250, .22), ui.RGBA(222, 234, 248, .12), ui.RGBA(230, 241, 255, .22), ui.Hex("#86bafa"), ui.Hex("#39382f"), ui.Hex("#f5edd9"), ui.Hex("#cec7b8"), ui.Hex("#2d3945"), ui.Hex("#f1c987"), ui.Hex("#adb5bf")}
	}
	return nativeColors{ui.Hex("#202831"), ui.Hex("#69727c"), ui.RGBA(248, 251, 255, .38), ui.RGBA(255, 255, 255, .66), ui.RGBA(46, 66, 86, .075), ui.RGBA(46, 66, 86, .15), ui.Hex("#3283d9"), ui.Hex("#fbf0d7"), ui.Hex("#353025"), ui.Hex("#675e4d"), ui.Hex("#f8fafc"), ui.Hex("#825019"), ui.Hex("#78818b")}
}
func (s *application) nativeState() *nativePanelState {
	if s.viewState == nil {
		dark := s.prefersDark(false)
		if s.themePreference != "light" && s.themePreference != "dark" {
			dark = mygo.Theme.IsDark()
		}
		s.viewState = &nativePanelState{Dark: dark, Images: map[string]*ui.Bitmap{}, Icons: map[string]*ui.Bitmap{}, CardBounds: map[string]ui.Rect{}}
	}
	if s.viewState.Images == nil {
		s.viewState.Images = map[string]*ui.Bitmap{}
	}
	if s.viewState.Icons == nil {
		s.viewState.Icons = map[string]*ui.Bitmap{}
	}
	if s.viewState.CardBounds == nil {
		s.viewState.CardBounds = map[string]ui.Rect{}
	}
	return s.viewState
}
func nativeGlyph(c *ui.Context, name string, size float32) ui.Element {
	return ui.Icon(c, nativeIcons[name]).Size(size, size).Shrink(0)
}
func (s *application) nativeIconButton(c *ui.Context, name, label string, size float32, p nativeColors) ui.Element {
	b := ui.ButtonBase(c).Label(label).Description(label).KeepFocus().Size(size, size).Radius(size / 2).Padding(0).AlignItems(ui.Center).Justify(ui.Center).TextColor(p.icon).Cursor(ui.CursorPointer).Shrink(0)
	if b.Hovered() || b.Pressed() {
		b.Background(p.hover)
	} else {
		b.Background(p.control)
	}
	b.Children(func() { nativeGlyph(c, name, 16) })
	b.Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	collectNativeHint(b, label, &s.nativeState().TrayHint)
	return b
}

func collectNativeHint(element ui.Element, text string, destination *string) {
	if element.Hovered() || element.FocusVisible() {
		*destination = text
	}
}
func nativeKey(c *ui.Context, key string, p nativeColors) {
	ui.Box(c).MinWidth(20).Height(20).Padding(0, 5).Radius(5).Background(p.control).AlignItems(ui.Center).Justify(ui.Center).Children(func() { ui.Text(c, key).FontSize(10).TextColor(p.muted).SingleLine() })
}
func nativeHint(c *ui.Context, keys []string, label string, p nativeColors) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, key := range keys {
			nativeKey(c, key, p)
		}
		if label != "" {
			ui.Text(c, label).FontSize(10).TextColor(p.muted).SingleLine()
		}
	})
}

// The AppKit host owns the surface material. MyGo paints only content and
// controls onto its transparent native layer.
func setNativeTheme(c *ui.Context, p nativeColors, dark bool) {
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
	t.FontSize = 13
	t.Radius = 8
	c.SetTheme(&t)
}

func (s *application) nativeView(c *ui.Context) {
	state := s.nativeState()
	state.TrayHint = ""
	defer func() {
		if !s.preview {
			s.reportNativeHint(state.TrayHint)
		}
	}()
	live := map[string]bool{}
	for _, item := range s.store.Items() {
		live[item.ID] = true
	}
	for id := range state.Images {
		if !live[id] {
			delete(state.Images, id)
		}
	}
	for id := range state.CardBounds {
		if !live[id] {
			delete(state.CardBounds, id)
		}
	}
	p := panelColors(state.Dark)
	setNativeTheme(c, p, state.Dark)
	// Keep wheel, trackpad and keyboard scrolling, but paint no scrollbar.
	theme := *c.Theme()
	theme.Scrollbar = ui.Transparent
	c.SetTheme(&theme)
	width, height := c.Size()
	tray := ui.Rect{W: width, H: height}
	state.Tray = tray
	if !state.Zoom {
		searchFocused := s.search.Focused(c)
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			s.closeNativePreview()
			state.SearchOpen = true
			s.search.Focus(c)
		}
		if c.Shortcut(ui.Cmd, ui.KeyY) {
			if s.preview {
				s.closeNativePreview()
			} else if _, ok := s.selected(); ok {
				s.preview = true
				s.publish()
			}
		}
		if c.Shortcut(0, ui.KeyEscape) {
			switch {
			case state.Zoom:
				state.Zoom = false
			case s.preview || s.draft != nil || s.busy:
				s.closeNativePreview()
				state.Focus.Focus(c)
			case s.query != "":
				s.query = ""
				s.filter()
			case state.SearchOpen:
				state.SearchOpen = false
				state.Focus.Focus(c)
			default:
				s.hide()
			}
		}
		// Clicking whitespace can clear native focus while a card stays selected.
		// Keep navigation and preview available for that selection. The search
		// editor owns typing/caret keys; focused controls get first key priority.
		if !searchFocused {
			if c.Shortcut(ui.Cmd, ui.KeyP) {
				if item, ok := s.selected(); ok {
					s.setNativePinned(item.ID, !item.Pinned)
				}
			}
			if c.Shortcut(ui.Cmd, ui.KeyBackspace) {
				if item, ok := s.selected(); ok {
					s.deleteNativeItem(item.ID)
				}
			}
			if c.Shortcut(0, ui.KeyLeft) {
				s.selectNative(max(0, s.chosen-1), s.preview)
				if !s.preview {
					state.Focus.Focus(c)
				}
			}
			if c.Shortcut(0, ui.KeyRight) {
				s.selectNative(min(len(s.results)-1, s.chosen+1), s.preview)
				if !s.preview {
					state.Focus.Focus(c)
				}
			}
		}
		if !searchFocused {
			if c.Shortcut(0, ui.KeySpace) {
				if s.preview {
					s.closeNativePreview()
					state.Focus.Focus(c)
				} else if _, ok := s.selected(); ok {
					s.preview = true
					s.publish()
				}
			}
		}
		if state.Focus.Focused(c) {
			if c.Shortcut(0, ui.KeyEnter) && !s.busy {
				s.chooseNative(state.Original)
			}
		}
		if searchFocused && c.Shortcut(0, ui.KeyDown) {
			state.Focus.Focus(c)
		}
	}
	ui.Column(c).Fill().Gap(0).Padding(1).TextColor(p.text).Children(func() {
		ui.Row(c).Height(48).Padding(6, nativeTrayInset).Gap(12).AlignItems(ui.Center).Children(func() {
			searchWidth := float32(30)
			if state.SearchOpen {
				searchWidth = min(340, tray.W-180)
			}
			search := ui.Row(c).Key("search-shell").Height(30).Radius(15).Background(p.control).Clip().AlignItems(ui.Center)
			search.Width(search.Animate("width", searchWidth, 180*time.Millisecond))
			search.Children(func() {
				if state.SearchOpen {
					ui.Row(c).Fill().Padding(0, 4, 0, 10).Gap(7).AlignItems(ui.Center).Children(func() {
						nativeGlyph(c, "MagnifyingGlass", 15).TextColor(p.icon)
						input := ui.TextInputBase(c, &s.query).Bind(&s.search).Grow(1).MinWidth(0).Height(28).FontSize(13).Label(s.tr("Search clipboard history")).Placeholder(s.tr("Search clipboard"))
						if input.Changed() {
							s.cancelNativeDraft()
							s.preview = false
							s.chosen = 0
							s.filter()
						}
						if input.Submitted() {
							s.chooseNative(true)
						}
						if s.nativeIconButton(c, "X", s.tr("Clear or close search"), 24, p).Clicked() {
							if s.query != "" {
								s.query = ""
								s.filter()
								s.search.Focus(c)
							} else {
								state.SearchOpen = false
								state.Focus.Focus(c)
							}
						}
					})
				} else if s.nativeIconButton(c, "MagnifyingGlass", s.tr("Search · ⌘F"), 30, p).Clicked() {
					s.closeNativePreview()
					state.SearchOpen = true
					s.search.Focus(c)
				}
			})
			pinLabel := s.tr("Pinned")
			if state.PinnedOnly {
				pinLabel = s.tr("All history")
			}
			pinEntry := s.nativeIconButton(c, "PushPin", pinLabel, 30, p)
			if state.PinnedOnly {
				pinEntry.Background(p.selection.Alpha(.13)).TextColor(p.selection)
			}
			if pinEntry.Clicked() {
				s.closeNativePreview()
				state.PinnedOnly = !state.PinnedOnly
				s.filter()
				state.LastSelected = ""
				state.Scroll.X = 0
				state.Focus.Focus(c)
			}
			if state.PinnedOnly {
				ui.Text(c, s.tr("Pinned")).FontSize(11).TextColor(p.muted)
			}
			ui.Spacer(c)
			ui.Text(c, s.tr("%d items", len(s.results))).FontSize(11).FontFeatures("tnum").TextColor(p.muted)
		})
		if len(s.results) == 0 {
			state.Scroll.X = 0
			state.Scroll.MaxX = 0
			ui.Column(c).Height(nativeCardSize+18).FillWidth().Padding(4, nativeTrayInset, 14).AlignItems(ui.Center).Justify(ui.Center).Children(func() { s.nativeEmpty(c, p) })
		} else {
			state.EmptyAnimationStart = time.Time{}
			state.PinAnimationStart = time.Time{}
			ui.Row(c).Height(nativeCardSize + 18).FillWidth().Children(func() {
				ui.ScrollHorizontal(c).Fill().TrackScroll(&state.Scroll).Children(func() {
					// The space belongs to the beginning and end of the strip,
					// not the viewport: intermediate cards can pass behind its edges.
					ui.Row(c).Padding(4, nativeTrayInset, 14).Gap(nativeCardGap).Children(func() {
						for index, item := range s.results {
							s.nativeCard(c, index, item, p)
						}
					})
				})
			})
		}
		ui.Row(c).Height(40).Padding(4, nativeTrayInset+2, 10).Gap(8).AlignItems(ui.Center).Children(func() {
			on := s.native.Enabled()
			track := p.control
			if on {
				track = ui.Hex("#67917d")
			}
			toggle := ui.SwitchBase(c, &on).Label(s.tr("Paste Guard")).Description(s.tr("⌃⌘C · Copy for this app only")).KeepFocus().Size(42, 24).Radius(12).Background(track).Padding(3).Children(func() {
				left := float32(3)
				icon := "ShieldSlash"
				color := ui.Hex("#78818b")
				if on {
					left = 21
					icon = "ShieldCheck"
				}
				ui.Box(c).Key("guard-knob").Absolute().Left(left).Transition(ui.ElementTransition{Position: true, Duration: 150 * time.Millisecond}).Top(3).Size(18, 18).Radius(9).Background(ui.Hex("#ffffff")).AlignItems(ui.Center).Justify(ui.Center).Children(func() { nativeGlyph(c, icon, 12).TextColor(color) })
			})
			if toggle.Changed() {
				s.toggleNativeGuard()
			}
			collectNativeHint(toggle, s.tr("⌃⌘C · Copy for this app only"), &state.TrayHint)
			ui.Spacer(c)
			if tray.W > 720 {
				nativeHint(c, []string{"←", "→"}, s.tr("Navigate"), p)
			}
			if !s.preview {
				nativeHint(c, []string{"space"}, s.tr("Preview"), p)
				nativeHint(c, []string{"↵"}, s.tr("Copy"), p)
				nativeHint(c, []string{"esc"}, s.tr("Close"), p)
			}
		})
	})
	s.syncNativeGlass()
}

func (s *application) nativeCard(c *ui.Context, index int, item clipmodel.Item, p nativeColors) {
	state := s.nativeState()
	bg, fg, meta := p.paper, p.paperText, p.paperMeta
	switch item.Type {
	case clipmodel.TypeJSON:
		bg = ui.Hex("#252d3a")
		fg = ui.Hex("#d7e1ee")
		meta = fg.Alpha(.82)
	case clipmodel.TypeLink:
		bg = p.link
		fg = p.text
		meta = p.muted
	case clipmodel.TypeImage:
		bg = ui.Transparent
		fg = ui.Hex("#ffffff")
		meta = fg
	}
	card := ui.Box(c).Key(item.ID).Size(nativeCardSize, nativeCardSize).Shrink(0).Radius(20).Background(bg).Focusable().FocusRing(false).Role(ui.RoleButton).Label(s.nativeCardLabel(item)).Cursor(ui.CursorPointer).Clip()
	if index == s.chosen {
		card.Bind(&state.Focus)
		if state.LastSelected != item.ID {
			viewport := state.Tray.W - 2
			left := nativeTrayInset + float32(index)*(nativeCardSize+nativeCardGap)
			contentWidth := 2*nativeTrayInset + float32(len(s.results))*nativeCardSize + float32(len(s.results)-1)*nativeCardGap
			maxScroll := max(0, contentWidth-viewport)
			target := state.Scroll.X
			if index == 0 {
				target = 0
			} else if index == len(s.results)-1 {
				target = maxScroll
			} else if left < target {
				target = left
			} else if left+nativeCardSize > target+viewport {
				target = left + nativeCardSize - viewport
			}
			state.Scroll.ScrollTo(target, 0, ui.Animation{Duration: 190 * time.Millisecond, Ease: ui.EaseOut})
			state.LastSelected = item.ID
		}
		card.DrawOver(func(painter *ui.Painter, r ui.Rect) {
			// Stroke draws inward already. An extra inset leaves a visible rim
			// of the card outside the selection, especially around the corners.
			painter.Stroke(r, p.selection, 20, 3)
		})
	}
	card.Children(func() {
		// MyGo's current native renderer can reuse stale card contents after a
		// theme/preview transition. Rebuild only this display subtree, retaining
		// the outer card's identity for keyboard focus and scroll position.
		contentKey := struct {
			Dark, Preview bool
			Language      string
		}{state.Dark, s.preview, s.currentLanguage()}
		ui.Box(c).Key(contentKey).Fill().Children(func() {
			headerInk, footerInk, typeInk := meta, meta, p.icon
			var imageGlass ui.Material
			if item.Type == clipmodel.TypeJSON {
				typeInk = ui.Hex("#adb5bf")
			}
			if item.Type == clipmodel.TypeImage {
				var glassInk ui.Color
				imageGlass, glassInk = nativeImageGlass(state.Dark)
				headerInk, footerInk, typeInk = glassInk, glassInk, glassInk
				// Decode only visible cards plus one neighboring card on each side.
				// Building a long history must not repeatedly decode offscreen images.
				cardLeft := state.Tray.X + nativeTrayInset + float32(index)*(nativeCardSize+nativeCardGap) - state.Scroll.X
				nearViewport := cardLeft+nativeCardSize >= state.Tray.X-(nativeCardSize+nativeCardGap) && cardLeft <= state.Tray.X+state.Tray.W+(nativeCardSize+nativeCardGap)
				var bitmap *ui.Bitmap
				if nearViewport {
					bitmap = s.nativeImage(item)
				}
				if bitmap != nil {
					// Clip only once at the card: a second rounded bitmap over a
					// colored background produces an antialiased colored fringe.
					ui.Image(c, bitmap).Absolute().Left(0).Top(0).Size(nativeCardSize, nativeCardSize).Fit(ui.Cover)
				}
			}
			content := ui.Column(c).Fill().Padding(16).Gap(10).TextColor(fg)
			if item.Type == clipmodel.TypeImage {
				// MyGo paints absolute children after normal-flow children.
				// Keep metadata absolute too, after the image, so it stays on top.
				content.Absolute().Left(0).Top(0).Size(nativeCardSize, nativeCardSize)
			}
			content.Children(func() {
				ui.Row(c).Height(20).Gap(7).AlignItems(ui.Center).Children(func() {
					typeIcon := "TextT"
					switch item.Type {
					case clipmodel.TypeJSON:
						typeIcon = "BracketsCurly"
					case clipmodel.TypeImage:
						typeIcon = "Image"
					case clipmodel.TypeLink:
						typeIcon = "LinkSimple"
					}
					header := ui.Row(c).Gap(5).Padding(2, 4).Radius(5).AlignItems(ui.Center)
					if imageGlass != nil {
						header.Material(imageGlass)
					}
					header.Children(func() {
						nativeGlyph(c, typeIcon, 16).TextColor(typeInk)
						ui.Text(c, s.relativeTime(item.CreatedAt)).FontSize(10.5).FontFeatures("tnum").TextColor(headerInk).SingleLine()
					})
					ui.Spacer(c)
					s.nativeSourceIcon(c, item, p)
				})
				switch item.Type {
				case clipmodel.TypeImage:
					ui.Spacer(c)
				case clipmodel.TypeJSON:
					var pretty bytes.Buffer
					text := item.Text
					if json.Indent(&pretty, []byte(text), "", "  ") == nil {
						text = pretty.String()
					}
					nativeJSONText(c, text, true).Font(nativeCodeFont).FontSize(11.5).FontWeight(400).LineHeight(1.5).MaxLines(5).Wrap().Grow(1).MinHeight(0)
				case clipmodel.TypeLink:
					// Constrain the body between the shared header and footer. Link
					// icon/title/domain must not push the footer past the card inset.
					ui.Column(c).Grow(1).MinHeight(0).Gap(6).Clip().Children(func() {
						if link, err := url.Parse(item.Text); err == nil {
							if bitmap := s.nativeWebsiteIcon(item); bitmap != nil {
								ui.Image(c, bitmap).Size(28, 28).Fit(ui.Contain).Label(s.tr("Website icon"))
							} else {
								nativeGlyph(c, "LinkSimple", 28).TextColor(p.icon)
							}
							title := strings.TrimPrefix(link.Path, "/")
							if title == "" {
								title = link.Host
							}
							ui.Text(c, title).Font(nativeContentFont).FontSize(16).FontWeight(500).LineHeight(1.35).MaxLines(2).Wrap()
							ui.Text(c, link.Host).FontSize(11.5).TextColor(meta).SingleLine()
						} else {
							ui.Text(c, item.Text).Font(nativeContentFont).FontSize(15).FontWeight(400).LineHeight(1.5).MaxLines(4).Wrap()
						}
					})
				default:
					size, weight := float32(15), 400
					if utf8.RuneCountInString(strings.TrimSpace(item.Text)) < 24 {
						size, weight = 18, 500
					}
					ui.Text(c, strings.TrimSpace(item.Text)).Font(nativeContentFont).FontSize(size).FontWeight(weight).LineHeight(1.5).MaxLines(4).Wrap().Grow(1).MinHeight(0)
				}
				ui.Row(c).Height(16).Gap(8).AlignItems(ui.Center).TextColor(footerInk).Children(func() {
					label := s.tr("%d characters", utf8.RuneCountInString(item.Text))
					switch item.Type {
					case clipmodel.TypeJSON:
						label = s.tr("Original")
						if item.ParentID != "" {
							label = s.tr("Formatted copy")
						}
					case clipmodel.TypeImage:
						if item.Image != nil {
							label = fmt.Sprintf("%d × %d", item.Image.Width, item.Image.Height)
						}
					case clipmodel.TypeLink:
						if link, err := url.Parse(item.Text); err == nil {
							label = strings.ToUpper(link.Scheme)
						}
					}
					footer := ui.Box(c).Padding(2, 4).Radius(5)
					if imageGlass != nil {
						footer.Material(imageGlass)
					}
					footer.Children(func() { ui.Text(c, label).FontSize(10.5).FontFeatures("tnum").SingleLine() })
					ui.Spacer(c)
					if item.Pinned || card.Hovered() || card.FocusVisible() {
						label := s.tr("Pin")
						if item.Pinned {
							label = s.tr("Unpin")
						}
						pin := ui.ButtonBase(c).Key("pin").Label(label).Description(s.tr("Pin / Unpin · ⌘P")).KeepFocus().Size(22, 22).Radius(6).Background(ui.Transparent).TextColor(p.icon).Padding(3)
						if pin.Hovered() {
							pin.Background(p.control)
						}
						if imageGlass != nil {
							pin.Material(imageGlass).TextColor(footerInk)
						}
						if item.Pinned {
							pin.TextColor(p.selection)
						}
						pin.Children(func() { nativeGlyph(c, "PushPin", 14) })
						collectNativeHint(pin, s.tr("Pin / Unpin · ⌘P"), &state.TrayHint)
						if pin.Clicked() {
							s.setNativePinned(item.ID, !item.Pinned)
						}
					}
				})
			})
		})
	})
	if card.Clicked() {
		s.selectNative(index, false)
		card.Focus()
	}
	if card.DoubleClicked() {
		s.selectNative(index, false)
		s.chooseNative(true)
	}
	if card.Shortcut(0, ui.KeySpace) {
		s.selectNative(index, true)
		s.preview = true
		s.publish()
	}
	if card.Shortcut(0, ui.KeyEnter) {
		s.selectNative(index, s.preview)
		s.chooseNative(true)
	}
	state.CardBounds[item.ID] = card.Bounds()
}
func (s *application) nativeCardLabel(item clipmodel.Item) string {
	if item.Type == clipmodel.TypeImage {
		return s.tr("Image · %s", s.sourceAppName(item))
	}
	text := item.Text
	if utf8.RuneCountInString(text) > 90 {
		text = string([]rune(text)[:90]) + "…"
	}
	return text + " · " + s.sourceAppName(item)
}
func (s *application) sourceAppName(item clipmodel.Item) string {
	if item.SourceApp == "" {
		return s.tr("Unknown app")
	}
	// Older pins stored the translated fallback. Render it in the current
	// language without rewriting provenance or translating actual app names.
	if item.SourceAppID == "" {
		for _, unknown := range translations["Unknown app"] {
			if item.SourceApp == unknown {
				return s.tr("Unknown app")
			}
		}
	}
	return item.SourceApp
}

func (s *application) nativeSourceIcon(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.nativeState()
	source := s.itemSources[item.ID]
	bitmap, known := state.Icons[source]
	if !known && len(s.appIcons[source]) > 0 {
		bitmap, _ = ui.DecodeBitmap(s.appIcons[source])
		state.Icons[source] = bitmap
	}
	sourceIcon := ui.Box(c).Size(24, 24).Shrink(0).Description(s.sourceAppName(item)).Children(func() {
		if bitmap != nil {
			ui.Image(c, bitmap).Size(24, 24).Label(s.tr("Source: %s", s.sourceAppName(item)))
		} else {
			nativeGlyph(c, "ClipboardText", 22).TextColor(p.icon).Label(s.tr("Source: %s", s.sourceAppName(item)))
		}
		if len(item.AllowedApps) > 0 {
			color := p.icon
			if !s.native.Enabled() {
				color = p.warning
			}
			ui.Box(c).Absolute().Right(-5).Top(-5).Size(15, 15).Radius(8).Background(p.link).Border(1, p.edge).AlignItems(ui.Center).Justify(ui.Center).Children(func() { nativeGlyph(c, "LockSimple", 10).TextColor(color).Label(s.scopeLabel(item.AllowedApps)) })
		}
	})
	collectNativeHint(sourceIcon, s.sourceAppName(item), &state.TrayHint)
}

var imageDecodeSlots = make(chan struct{}, 2)

func (s *application) nativeImage(item clipmodel.Item) *ui.Bitmap {
	state := s.nativeState()
	if bitmap, ok := state.Images[item.ID]; ok {
		return bitmap
	}
	decode := func() image.Image {
		record, ok := s.store.Record(item.ID)
		if !ok {
			return nil
		}
		png, err := record.Data.Read(clipmodel.PNGFormat)
		if err != nil {
			return nil
		}
		decoded, _, err := image.Decode(bytes.NewReader(png))
		if err != nil {
			return nil
		}
		return decoded
	}
	// In-memory renderer tests need deterministic frames and never touch AppKit.
	if s.panel == nil {
		decoded := decode()
		if decoded == nil {
			return nil
		}
		bitmap := ui.NewBitmap(decoded)
		state.Images[item.ID] = bitmap
		s.pruneImageCache(item.ID)
		return bitmap
	}
	if state.ImageLoading == nil {
		state.ImageLoading = map[string]bool{}
	}
	if state.ImageLoading[item.ID] {
		return nil
	}
	state.ImageLoading[item.ID] = true
	generation := s.uiGeneration
	go func() {
		imageDecodeSlots <- struct{}{}
		active := false
		mygo.RunOnMain(func() { active = !s.quitting && generation == s.uiGeneration && s.panel.IsVisible() })
		var decoded image.Image
		if active {
			decoded = decode()
		}
		<-imageDecodeSlots
		mygo.RunOnMain(func() {
			delete(state.ImageLoading, item.ID)
			if s.quitting {
				return
			}
			if generation != s.uiGeneration {
				if s.panel.IsVisible() {
					s.publish()
				}
				return
			}
			if _, exists := s.store.Record(item.ID); !exists {
				return
			}
			if decoded != nil {
				state.Images[item.ID] = ui.NewBitmap(decoded)
				s.pruneImageCache(item.ID)
			} else {
				state.Images[item.ID] = nil
			}
			s.publish()
		})
	}()
	return nil
}

func (s *application) nativeWebsiteIcon(item clipmodel.Item) *ui.Bitmap {
	if s.panel == nil || item.Type != clipmodel.TypeLink || len(item.AllowedApps) > 0 {
		return nil
	}
	state := s.nativeState()
	if state.WebsiteIcons == nil {
		state.WebsiteIcons = map[string]*ui.Bitmap{}
		state.WebsiteHTML = map[string][]byte{}
	}
	if bitmap := state.WebsiteIcons[item.ID]; bitmap != nil {
		return bitmap
	}
	if state.LinkIcons == nil {
		state.LinkIcons = linkicons.New(func() {
			mygo.RunOnMain(func() {
				if !s.quitting {
					s.publish()
				}
			})
		})
	}
	html, loaded := state.WebsiteHTML[item.ID]
	if !loaded {
		if record, ok := s.store.Record(item.ID); ok {
			html, _ = record.Data.Read(clipmodel.HTMLFormat)
		}
		state.WebsiteHTML[item.ID] = html
	}
	decoded := state.LinkIcons.Lookup(item, html)
	if decoded == nil {
		return nil
	}
	bitmap := ui.NewBitmap(decoded)
	state.WebsiteIcons[item.ID] = bitmap
	return bitmap
}

var nativeJSONTokens = regexp.MustCompile(`"(?:[^"\\]|\\.)*"\s*:|"(?:[^"\\]|\\.)*"|\b(?:true|false|null|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\b`)

func nativeJSONText(c *ui.Context, text string, dark bool) ui.Element {
	key, str, value := ui.Hex("#623466"), ui.Hex("#24543d"), ui.Hex("#76501b")
	if dark {
		key, str, value = ui.Hex("#d8a7d6"), ui.Hex("#add8c5"), ui.Hex("#edc797")
	}
	spans := []ui.Span{}
	last := 0
	for _, match := range nativeJSONTokens.FindAllStringIndex(text, -1) {
		if match[0] > last {
			spans = append(spans, ui.Span{Text: text[last:match[0]]})
		}
		token := text[match[0]:match[1]]
		color := value
		if strings.HasSuffix(token, ":") {
			color = key
		} else if strings.HasPrefix(token, "\"") {
			color = str
		}
		spans = append(spans, ui.Span{Text: token, Color: color})
		last = match[1]
	}
	if last < len(text) {
		spans = append(spans, ui.Span{Text: text[last:]})
	}
	return ui.RichText(c, spans...)
}

func (s *application) nativeEmpty(c *ui.Context, p nativeColors) {
	state := s.nativeState()
	if s.query != "" || state.PinnedOnly {
		state.EmptyAnimationStart = time.Time{}
	}
	if s.query != "" || !state.PinnedOnly {
		state.PinAnimationStart = time.Time{}
	}
	ui.Column(c).Height(nativeCardSize).Gap(8).AlignItems(ui.Center).Justify(ui.Center).TextColor(p.muted).Children(func() {
		switch {
		case s.query != "":
			nativeGlyph(c, "MagnifyingGlass", 28).TextColor(p.icon)
		case s.nativeState().PinnedOnly:
			s.nativePinMascot(c)
		default:
			s.nativeMascot(c)
		}
		text := s.tr("Your copies appear here")
		if s.query != "" {
			text = s.tr("No matches")
		} else if s.nativeState().PinnedOnly {
			text = s.tr("No pinned items")
		} else if !s.monitoring {
			text = s.tr("Recording paused")
		}
		ui.Text(c, text).FontSize(15)
		if s.query == "" && s.monitoring && !s.nativeState().PinnedOnly {
			nativeHint(c, []string{"⌃", "⌘", "C"}, s.tr("Copy for this app only"), p)
		}
	})
}
