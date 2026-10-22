package nativeui

import (
	"bytes"
	"image"
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

// Native rendering only: shared SVG glyphs are parsed once, with no web engine.
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
	SearchOpen, Original, Dark bool
	PinnedOnly                 bool
	Scroll                     ui.ScrollState
	Focus, PreviewFocus        ui.Handle
	LastSelected               string
	Images, Icons              map[string]*ui.Bitmap
	LinkIcons                  *linkicons.Cache
	ImageLoading               map[string]bool
	WebsiteIcons               map[string]*ui.Bitmap
	WebsiteHTML                map[string][]byte
	CardBounds                 map[string]ui.Rect
	Tray                       ui.Rect
	EmptyAnimationStart        time.Time
	PinAnimationStart          time.Time
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
	b.Children(func() { nativeGlyph(c, name, nativeIconSize) })
	b.Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
	return b
}
func nativeKey(c *ui.Context, key string, p nativeColors) {
	ui.Box(c).MinWidth(nativeKeySize).Height(nativeKeySize).Padding(0, 5).Radius(5).Background(p.control).AlignItems(ui.Center).Justify(ui.Center).Children(func() { ui.Text(c, key).FontSize(nativeTextHint).TextColor(p.muted).SingleLine() })
}
func nativeHint(c *ui.Context, keys []string, label string, p nativeColors) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, key := range keys {
			nativeKey(c, key, p)
		}
		if label != "" {
			ui.Text(c, label).FontSize(nativeTextHint).TextColor(p.muted).SingleLine()
		}
	})
}

func (s *application) nativeView(c *ui.Context) {
	state := s.nativeState()
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
	ui.Column(c).Fill().Gap(0).Padding(1).TextColor(p.text).Children(func() {
		ui.Row(c).Height(48).Padding(6, nativeTrayInset).Gap(12).AlignItems(ui.Center).Children(func() {
			searchWidth := float32(nativeControlSize)
			if state.SearchOpen {
				searchWidth = min(340, tray.W-180)
			}
			search := ui.Row(c).Key("search-shell").Height(nativeControlSize).Radius(nativeControlSize / 2).Background(p.control).Clip().AlignItems(ui.Center)
			search.Width(search.Animate("width", searchWidth, 180*time.Millisecond))
			search.Children(func() {
				if state.SearchOpen {
					ui.Row(c).Fill().Padding(0, 4, 0, 10).Gap(7).AlignItems(ui.Center).Children(func() {
						nativeGlyph(c, "MagnifyingGlass", nativeIconSize).TextColor(p.icon)
						input := ui.TextInputBase(c, &s.query).Bind(&s.search).Grow(1).MinWidth(0).Height(28).FontSize(nativeTextBase).Label(s.tr("Search clipboard history")).Placeholder(s.tr("Search clipboard"))
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
				} else if s.nativeIconButton(c, "MagnifyingGlass", s.tr("Search · ⌘F"), nativeControlSize, p).Clicked() {
					s.closeNativePreview()
					state.SearchOpen = true
					s.search.Focus(c)
				}
			})
			pinLabel := s.tr("Pinned")
			if state.PinnedOnly {
				pinLabel = s.tr("All history")
			}
			pinEntry := s.nativeIconButton(c, "PushPin", pinLabel, nativeControlSize, p)
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
				ui.Text(c, s.tr("Pinned")).FontSize(nativeTextMeta).TextColor(p.muted)
			}
			ui.Spacer(c)
			ui.Text(c, s.tr("%d items", len(s.results))).FontSize(nativeTextMeta).FontFeatures("tnum").TextColor(p.muted)
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
	analysis := clipmodel.Analysis{Type: item.Type}
	if item.Type != clipmodel.TypeText && item.Type != clipmodel.TypeJSON && item.Type != clipmodel.TypeImage && item.Type != clipmodel.TypeLink {
		analysis = clipmodel.Analyze(item.Text)
	}
	bg, fg, meta := nativeCardPalette(item.Type, analysis, p)
	card := ui.Box(c).Key(item.ID).Size(nativeCardSize, nativeCardSize).Shrink(0).Radius(nativeCardRadius).Background(bg).Focusable().FocusRing(false).Role(ui.RoleButton).Label(s.nativeCardLabel(item)).Cursor(ui.CursorPointer).Clip()
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
			s.nativeCardContent(c, card, index, item, analysis, fg, meta, p)
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
	if clipmodel.IsSensitiveType(item.Type) {
		return s.nativeTypeLabel(item.Type) + " · " + s.sourceAppName(item)
	}
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
	ui.Box(c).Size(nativeSourceIconSize, nativeSourceIconSize).Shrink(0).Description(s.sourceAppName(item)).Children(func() {
		if bitmap != nil {
			ui.Image(c, bitmap).Size(nativeSourceIconSize, nativeSourceIconSize).Label(s.tr("Source: %s", s.sourceAppName(item)))
		} else {
			nativeGlyph(c, "ClipboardText", nativeSourceIconSize-2).TextColor(p.icon).Label(s.tr("Source: %s", s.sourceAppName(item)))
		}
		if len(item.AllowedApps) > 0 {
			color := p.icon
			if !s.native.Enabled() {
				color = p.warning
			}
			ui.Box(c).Absolute().Right(-5).Top(-5).Size(nativeBadgeSize, nativeBadgeSize).Radius(8).Background(p.link).Border(1, p.edge).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				nativeGlyph(c, "LockSimple", nativeBadgeIconSize).TextColor(color).Label(s.scopeLabel(item.AllowedApps))
			})
		}
	})
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
		key, str, value = ui.Hex("#be9bef"), ui.Hex("#a0dfca"), ui.Hex("#ffd374")
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
