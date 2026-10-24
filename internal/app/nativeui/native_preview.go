package nativeui

import (
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// nativePopoverView is the content of an AppKit NSPopover. Its system container
// supplies the material, continuous outline, arrow and screen-edge placement.
func (s *application) nativePopoverView(c *ui.Context) {
	state := s.nativeState()
	p := panelColors(state.Dark)
	setNativeTheme(c, p, state.Dark, s.currentLanguage())
	item, ok := s.selected()
	if !ok {
		return
	}
	if nativeLinkItemType(item.Type) {
		s.prepareNativeLinkQR(item)
	}
	semantic := *s.semanticState(item)
	// Async metadata updates do not change format controls or their identity.
	// In particular, allocating its first slot must not remount the toolbar
	// between pointer-down and the user's first format selection.
	semantic.PhoneLocation = nil
	semantic.QRCode = nil
	semantic.QRTarget = ""
	// Keep this focus target stable across selection, draft comparison and zoom.
	// Only this native window binds PreviewFocus; the tray owns Focus separately.
	root := ui.Column(c).Key("native-preview-surface").Fill().Focusable().AutoFocus().Bind(&state.PreviewFocus).Role(ui.RoleDialog).Label(s.tr("Content preview")).TextColor(p.text).Lang(s.currentLanguage())
	root.Children(func() {
		draftID := ""
		if s.draft != nil {
			draftID = s.draft.ID
		}
		contentKey := struct {
			ID, Draft      string
			Language       string
			Dark, Original bool
			Semantic       nativeSemanticPreviewState
		}{item.ID, draftID, s.currentLanguage(), state.Dark, state.Original, semantic}
		ui.Column(c).Key(contentKey).Fill().Children(func() {
			s.nativePreviewContent(c, item, p)
		})
	})
	if c.Shortcut(ui.Cmd, ui.KeyF) {
		s.closeNativePreview()
		state.SearchOpen = true
		// The input belongs to the parent surface, and may not be mounted yet.
		// Queue its handle request without binding it to this popover's Context.
		state.Focus.CancelFocus()
		s.search.Focus()
		if s.panel != nil {
			s.panel.Focus()
		}
		s.publish()
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyY) {
		s.closeNativePreview()
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyP) {
		s.setNativePinned(item.ID, !item.Pinned)
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyBackspace) {
		s.deleteNativeItem(item.ID)
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		s.closeNativePreview()
		return
	}
	if !state.PreviewFocus.Focused(c) {
		return
	}
	if c.Shortcut(0, ui.KeyEnter) && !s.busy {
		s.chooseNative(state.Original)
	}
	if c.Shortcut(0, ui.KeySpace) {
		s.closeNativePreview()
	}
	if c.Shortcut(ui.Cmd, ui.KeyLeft) {
		s.selectNativeEdge(false, true)
	}
	if c.Shortcut(ui.Cmd, ui.KeyRight) {
		s.selectNativeEdge(true, true)
	}
	if c.Shortcut(0, ui.KeyLeft) {
		s.selectNative(max(0, s.chosen-1), true)
	}
	if c.Shortcut(0, ui.KeyRight) {
		s.selectNative(min(len(s.results)-1, s.chosen+1), true)
	}
}

func (s *application) nativePreviewContent(c *ui.Context, item clipmodel.Item, p nativeColors) {
	switch item.Type {
	case clipmodel.TypeJSON:
		s.nativeJSONPreview(c, item, p)
		return
	case clipmodel.TypeMarkdown:
		s.nativeMarkdownPreview(c, item, p)
		return
	case clipmodel.TypeImage:
		s.nativeImagePreview(c, item, p)
		return
	case clipmodel.TypeAppLink:
		s.nativeAppLinkPreview(c, item, p)
		return
	case clipmodel.TypeLink:
		s.nativeLinkPreview(c, item, p)
		return
	case clipmodel.TypeFilePath:
		s.nativeFilePathPreview(c, item, p)
		return
	case clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey:
		s.nativeSecretPreview(c, item, p)
		return
	case clipmodel.TypeDate:
		s.nativeDatePreview(c, item, p)
		return
	case clipmodel.TypeTimestamp:
		s.nativeTimestampPreview(c, item, p)
		return
	case clipmodel.TypeColor:
		s.nativeColorPreview(c, item, p)
		return
	case clipmodel.TypeEmail:
		s.nativeEmailPreview(c, item, p)
		return
	case clipmodel.TypePhone:
		s.nativePhonePreview(c, item, p)
		return
	case clipmodel.TypeCoordinates:
		s.nativeMapPreview(c, item, p)
		return
	}
	s.nativeNotePreview(c, item, p)
}

func (s *application) nativePreviewScope(c *ui.Context, item clipmodel.Item, p nativeColors) {
	if len(item.AllowedApps) > 0 {
		color := p.muted
		scope := s.scopeLabel(item.AllowedApps)
		inactive := !s.native.Enabled()
		label := scope
		if inactive {
			label += " · " + s.tr("Inactive")
			color = p.warning
		}
		ui.Row(c).Label(label).Height(nativePreviewMetadataHeight).Shrink(0).FillWidth().Gap(nativeInlineGap).AlignItems(ui.Center).TextColor(color).Children(func() {
			nativeGlyph(c, "LockSimple", 13)
			ui.Text(c, scope).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Grow(1).MinWidth(0).SingleLine()
			if inactive {
				// Keep the live-listener warning visible even when application
				// names are long. The full scope remains accessible above.
				ui.Text(c, s.tr("Inactive")).FontSize(nativeTextMeta).FontWeight(nativeWeightBody).Shrink(0).SingleLine()
			}
		})
	}
}

// Metadata stays outside the scrolling body. Protection always has its own
// final row, using the ink of the containing paper or glass surface.
func nativePreviewFooterHeight(item clipmodel.Item, hasMetadata bool) float32 {
	height := float32(0)
	if hasMetadata {
		height = nativePreviewMetadataHeight
	}
	if len(item.AllowedApps) > 0 {
		if hasMetadata {
			height += nativePreviewScopeGap
		}
		height += nativePreviewMetadataHeight
	}
	return height
}

func (s *application) nativePreviewFooter(c *ui.Context, item clipmodel.Item, p nativeColors, content func()) {
	if content == nil && len(item.AllowedApps) == 0 {
		return
	}
	ui.Column(c).FillWidth().Shrink(0).Gap(nativePreviewScopeGap).TextColor(p.muted).Children(func() {
		if content != nil {
			ui.Row(c).Height(nativePreviewMetadataHeight).Shrink(0).FillWidth().Gap(nativeToolbarGap).AlignItems(ui.Center).Children(content)
		}
		s.nativePreviewScope(c, item, p)
	})
}

func (s *application) nativePreviewToolbar(c *ui.Context, item clipmodel.Item, p nativeColors) {
	ui.Row(c).Height(nativeControlSize).Gap(nativeToolbarGap).AlignItems(ui.Center).Children(func() {
		if item.Type == clipmodel.TypeJSON {
			s.nativeJSONActions(c, p)
		} else if item.Type == clipmodel.TypeMarkdown {
			s.nativeMarkdownActions(c, item, p)
		} else if item.Type == clipmodel.TypeImage {
			s.nativeImageActions(c, item, p)
		} else if nativeLinkItemType(item.Type) {
			s.nativeLinkActions(c, item, p)
		} else {
			s.nativeSemanticActions(c, item, p)
		}
		ui.Spacer(c)
		if nativeLinkItemType(item.Type) {
			s.nativeLinkOpenAction(c, item, p)
		}
		pinLabel, pinIcon := s.tr("Pin"), "PushPin"
		if item.Pinned {
			pinLabel, pinIcon = s.tr("Unpin"), "PushPinFill"
		}
		if s.nativePreviewButton(c, pinIcon, pinLabel, p).Clicked() {
			s.setNativePinned(item.ID, !item.Pinned)
		}
		if s.nativePreviewButton(c, "Trash", s.tr("Delete item · ⌘⌫"), p).Clicked() {
			s.deleteNativeItem(item.ID)
		}
		if s.nativePreviewButton(c, "X", s.tr("Close preview"), p).Clicked() {
			s.closeNativePreview()
		}
	})
}

func (s *application) nativeJSONActions(c *ui.Context, p nativeColors) {
	state := s.nativeState()
	if s.busy {
		ui.Text(c, "…").FontSize(20)
		if s.nativePreviewButton(c, "X", s.tr("Cancel processing"), p).Clicked() {
			s.cancelNativeDraft()
			state.PreviewFocus.Focus(c)
			s.publish()
		}
	} else if s.draft == nil {
		if s.nativeActionButton(c, "BracketsCurly", s.tr("Format"), p).Label(s.tr("Format JSON")).Disabled(s.commandBusy).Clicked() {
			s.enhance(clipmodel.FormatJSON)
		}
	} else {
		if s.nativePreviewButton(c, "ArrowsLeftRight", s.tr("Original / result"), p).Disabled(s.commandBusy).Clicked() {
			state.Original = !state.Original
			state.PreviewFocus.Focus(c)
			s.publish()
		}
		if s.nativePreviewButton(c, "ArrowCounterClockwise", s.tr("Discard result"), p).Clicked() {
			s.cancelNativeDraft()
			state.PreviewFocus.Focus(c)
			s.publish()
		}
		if s.draft != nil {
			label := s.tr("Copy result")
			if state.Original {
				label = s.tr("Copy original")
			}
			if s.nativePreviewButton(c, "CheckCircle", label, p).Clicked() {
				s.chooseNative(state.Original)
			}
		}
	}
}

// Toolbar controls stay visually quiet and retain the preview's keyboard focus.
func (s *application) nativePreviewButton(c *ui.Context, name, label string, p nativeColors) ui.Element {
	button := ui.ButtonBase(c).Label(label).Description(label).Size(nativeControlSize, nativeControlSize).Radius(nativeControlRadius).Padding(0).AlignItems(ui.Center).Justify(ui.Center).TextColor(p.icon).Cursor(ui.CursorPointer).Shrink(0).KeepFocus()
	if button.Hovered() || button.Pressed() {
		button.Background(p.hover)
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() { nativeGlyph(c, name, nativeToolbarIconSize) })
	return button
}

// Metadata copy controls share their glyph, target and focus behavior across
// content types. chooseNative resolves the original or explicit preview format.
func (s *application) nativeCopyButton(c *ui.Context, p nativeColors) ui.Element {
	button := ui.ButtonBase(c).Label(s.tr("Copy")).Size(nativeCompactControl, nativeCompactControl).Shrink(0).Radius(nativeControlRadius).KeepFocus().Disabled(s.busy || s.commandBusy).AlignItems(ui.Center).Justify(ui.Center).TextColor(p.icon)
	if button.Hovered() {
		button.Background(p.hover)
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() { nativeGlyph(c, "Copy", nativeIconSize) })
	return button
}
