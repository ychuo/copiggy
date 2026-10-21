package nativeui

import (
	"fmt"
	"unicode/utf8"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

// nativePopoverView is the content of an AppKit NSPopover. Its system container
// supplies the material, continuous outline, arrow and screen-edge placement.
func (s *application) nativePopoverView(c *ui.Context) {
	state := s.nativeState()
	state.PopoverHint = ""
	if s.draft != nil {
		if current, ok := s.store.Current(); !ok || current.ID != state.DraftCurrent {
			state.PopoverHint = s.tr("Clipboard changed")
		}
	}
	defer func() {
		if s.preview {
			s.reportNativeHint(state.PopoverHint)
		}
	}()
	p := panelColors(state.Dark)
	setNativeTheme(c, p, state.Dark)
	item, ok := s.selected()
	if !ok {
		return
	}
	// Keep this focus target stable across selection, draft comparison and zoom.
	// Only this native window binds PreviewFocus; the tray owns Focus separately.
	root := ui.Column(c).Key("native-preview-surface").Fill().Focusable().AutoFocus().Bind(&state.PreviewFocus).Role(ui.RoleDialog).Label(s.tr("Content preview")).TextColor(p.text)
	root.Children(func() {
		draftID := ""
		if s.draft != nil {
			draftID = s.draft.ID
		}
		contentKey := struct {
			ID, Draft            string
			Language             string
			Dark, Original, Zoom bool
		}{item.ID, draftID, s.currentLanguage(), state.Dark, state.Original, state.Zoom}
		ui.Column(c).Key(contentKey).Fill().Children(func() {
			if state.Zoom {
				s.nativeZoom(c, item, p)
			} else {
				s.nativePreviewContent(c, item, p)
			}
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
		if state.Zoom {
			state.Zoom = false
			state.PreviewFocus.Focus(c)
			s.publish()
		} else {
			s.closeNativePreview()
		}
		return
	}
	if state.Zoom || !state.PreviewFocus.Focused(c) {
		return
	}
	if c.Shortcut(0, ui.KeyEnter) && !s.busy {
		s.chooseNative(state.Original)
	}
	if c.Shortcut(0, ui.KeySpace) {
		s.closeNativePreview()
	}
	if c.Shortcut(0, ui.KeyLeft) {
		s.selectNative(max(0, s.chosen-1), true)
	}
	if c.Shortcut(0, ui.KeyRight) {
		s.selectNative(min(len(s.results)-1, s.chosen+1), true)
	}
}

func (s *application) nativePreviewContent(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.nativeState()
	text := item.Text
	if s.draft != nil && !state.Original {
		text = s.draft.Text
	}
	_, height := c.Size()
	ui.Column(c).Fill().Padding(12, 22, 18).Gap(14).Children(func() {
		ui.Row(c).Height(30).Gap(8).AlignItems(ui.Center).Children(func() {
			if item.Type == clipmodel.TypeJSON {
				s.nativeJSONActions(c, p)
			} else if item.Type == clipmodel.TypeImage {
				if s.nativePreviewButton(c, "ArrowsOut", s.tr("Enlarge image"), p).Clicked() {
					state.Zoom = true
					state.PreviewFocus.Focus(c)
					s.publish()
				}
			} else if item.Type == clipmodel.TypeLink {
				s.nativeLinkActions(c, item, p)
			}
			ui.Spacer(c)
			pinLabel := s.tr("Pin")
			if item.Pinned {
				pinLabel = s.tr("Unpin")
			}
			if s.nativePreviewButton(c, "PushPin", pinLabel, p).Clicked() {
				s.setNativePinned(item.ID, !item.Pinned)
			}
			if s.nativePreviewButton(c, "Trash", s.tr("Delete item · ⌘⌫"), p).Clicked() {
				s.deleteNativeItem(item.ID)
			}
			if s.nativePreviewButton(c, "X", s.tr("Close preview"), p).Clicked() {
				s.closeNativePreview()
			}
		})
		if item.Type == clipmodel.TypeLink {
			s.nativeLinkContent(c, item, p)
		} else {
			ui.Scroll(c).Grow(1).MinHeight(50).FillWidth().Children(func() {
				if item.Type == clipmodel.TypeImage {
					if bitmap := s.nativeImage(item); bitmap != nil {
						image := ui.Image(c, bitmap).FillWidth().Height(max(150, height-140)).Fit(ui.Contain).Radius(10).Label(s.tr("Copied image")).Cursor(ui.CursorPointer)
						if image.Clicked() {
							state.Zoom = true
							state.PreviewFocus.Focus(c)
							s.publish()
						}
					} else {
						ui.Text(c, s.tr("Image unavailable")).FontSize(14).TextColor(p.muted).Role(ui.RoleStatus)
					}
				} else if item.Type == clipmodel.TypeJSON {
					nativeJSONText(c, text, state.Dark).Font(nativeCodeFont).FontSize(14).FontWeight(400).LineHeight(1.6).Wrap()
				} else {
					ui.Text(c, text).Font(nativeContentFont).FontSize(17).FontWeight(400).LineHeight(1.6).Wrap()
				}
			})
		}
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(5).Grow(1).MinWidth(0).Children(func() {
				info := s.tr("%d characters", utf8.RuneCountInString(text))
				if item.Type == clipmodel.TypeImage && item.Image != nil {
					info = fmt.Sprintf("%d × %d · PNG", item.Image.Width, item.Image.Height)
				} else if item.Type == clipmodel.TypeLink {
					info = s.nativeLinkLocation(item)
				}
				if s.draft != nil {
					prefix := s.tr("Formatted draft")
					if state.Original {
						prefix = s.tr("Original")
					}
					info = prefix + " · " + info
				}
				ui.Text(c, info).FontSize(11).FontFeatures("tnum").TextColor(p.muted)
				if len(item.AllowedApps) > 0 {
					color := p.muted
					scope := s.scopeLabel(item.AllowedApps)
					if !s.native.Enabled() {
						scope += " · " + s.tr("Inactive")
						color = p.warning
					}
					ui.Row(c).Gap(5).AlignItems(ui.Center).TextColor(color).Children(func() {
						nativeGlyph(c, "LockSimple", 13)
						ui.Text(c, scope).FontSize(12).SingleLine()
					})
				}
			})
		})
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
		if s.nativePreviewButton(c, "BracketsCurly", s.tr("Format JSON"), p).Clicked() {
			if current, ok := s.store.Current(); ok {
				state.DraftCurrent = current.ID
			} else {
				state.DraftCurrent = ""
			}
			s.enhance(clipmodel.FormatJSON)
		}
	} else {
		if s.nativePreviewButton(c, "ArrowsLeftRight", s.tr("Original / result"), p).Clicked() {
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
	button := ui.ButtonBase(c).Label(label).Description(label).Size(30, 30).Radius(6).Padding(0).AlignItems(ui.Center).Justify(ui.Center).TextColor(p.icon).Cursor(ui.CursorPointer).Shrink(0).KeepFocus()
	if button.Hovered() || button.Pressed() {
		button.Background(p.hover)
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() { nativeGlyph(c, name, 19) })
	collectNativeHint(button, label, &s.nativeState().PopoverHint)
	return button
}

func (s *application) nativeZoom(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.nativeState()
	ui.Column(c).Fill().Padding(12, 22, 18).Gap(14).Children(func() {
		ui.Row(c).Height(30).Justify(ui.End).Children(func() {
			if s.nativePreviewButton(c, "X", s.tr("Close image preview"), p).Clicked() {
				state.Zoom = false
				state.PreviewFocus.Focus(c)
				s.publish()
			}
		})
		if bitmap := s.nativeImage(item); bitmap != nil {
			ui.Image(c, bitmap).Fill().Fit(ui.Contain).Label(s.tr("Full image"))
		} else {
			ui.Text(c, s.tr("Image unavailable")).Grow(1).FontSize(14).TextColor(p.muted).Role(ui.RoleStatus)
		}
	})
}
