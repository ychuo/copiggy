package nativeui

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/contentactions"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Format choices belong to one preview session. Explicit copy can use the
// selected format; changing it alone never writes or edits a saved record.
// Closing or moving away resets formats and clears revealed credentials.
type nativeSemanticPreviewState struct {
	LinkQR                               bool
	LinkModeInitialized                  bool
	QRCode                               *nativeLinkQRCode
	QRTarget                             string
	ItemID                               string
	CopyFormat                           clipmodel.CopyFormat
	Revealed, UTC, Milliseconds, Dialing bool
	MarkdownSource                       bool
	ColorFormat, MapZoom, ImageZoom      int
	PhoneLocation                        *nativePhoneLocationState
}

func (s *application) semanticState(item clipmodel.Item) *nativeSemanticPreviewState {
	if s.semanticPreview.ItemID != item.ID {
		s.semanticPreview = nativeSemanticPreviewState{ItemID: item.ID}
		if value := clipmodel.AnalyzeItem(item); value.Timestamp != nil {
			s.semanticPreview.Milliseconds = value.Timestamp.Unit == "ms"
		}
	}
	return &s.semanticPreview
}

func (s *application) nativeSemanticActions(c *ui.Context, item clipmodel.Item, p nativeColors) {
	state := s.semanticState(item)
	icon, action := "", ""
	switch item.Type {
	case clipmodel.TypeEmail:
		icon, action = "NotePencil", "Mail"
	case clipmodel.TypePhone:
		icon, action = "Phone", "Call"
	case clipmodel.TypeCoordinates:
		icon, action = "MapPin", "Maps"
	case clipmodel.TypeDate:
		icon, action = "Calendar", "Calendar"
	case clipmodel.TypeFilePath:
		icon, action = "FolderOpen", "Finder"
	}
	if action != "" && s.nativeActionButton(c, icon, s.tr(action), p).Clicked() {
		s.openNativeContent(item.ID)
	}
	switch item.Type {
	case clipmodel.TypePhone:
		nativeSegmentGroup(c, p).Children(func() {
			for index, option := range []struct{ title, label string }{{"Original", "Original number"}, {"Dialing", "Dialing number"}} {
				button := s.nativeFormatSegment(c, s.tr(option.title), state.Dialing == (index == 1), p).Label(s.tr(option.label)).Disabled(s.commandBusy)
				if button.Clicked() {
					state.Dialing = index == 1
					state.CopyFormat = ""
					if state.Dialing {
						state.CopyFormat = clipmodel.PhoneDialing
					}
					s.publish()
				}
			}
		})
	case clipmodel.TypeColor:
		s.nativeColorActions(c, item, p)
	case clipmodel.TypeTimestamp:
		s.nativeTimestampActions(c, item, p)
	case clipmodel.TypeOTP, clipmodel.TypeToken, clipmodel.TypePrivateKey:
		s.nativeSecretActions(c, item, p)
	}
}

func (s *application) nativeActionButton(c *ui.Context, icon, label string, p nativeColors) ui.Element {
	button := ui.ButtonBase(c).Label(label).Height(nativeControlSize).Padding(0, 8).Radius(nativeControlRadius).Gap(nativeInlineGap).AlignItems(ui.Center).TextColor(p.icon).Cursor(ui.CursorPointer).KeepFocus().Shrink(0)
	if button.Hovered() || button.Pressed() {
		button.Background(p.hover)
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		nativeGlyph(c, icon, nativeToolbarIconSize)
		ui.Text(c, label).FontSize(nativeTextControl).FontWeight(nativeWeightControl).TextColor(p.muted).SingleLine()
	})
	return button
}

func (s *application) nativeSegment(c *ui.Context, label string, selected bool, p nativeColors) ui.Element {
	button := ui.ButtonBase(c).Label(label).Height(nativeControlSize).Padding(0, 8).Radius(nativeControlRadius).KeepFocus().Shrink(0).AlignItems(ui.Center).Justify(ui.Center).TextColor(p.muted)
	if selected || button.Hovered() {
		button.Background(p.control).TextColor(p.text)
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() { ui.Text(c, label).FontSize(nativeTextControl).FontWeight(nativeWeightControl).SingleLine() })
	return button
}

// Format selections share one inset track. A selected format stays distinct
// from the hover state and keeps the preview's keyboard navigation target.
func nativeSegmentGroup(c *ui.Context, p nativeColors) ui.Element {
	return ui.Row(c).Height(nativeControlSize).Padding(2).Radius(nativeControlRadius + 2).Background(p.control).Shrink(0)
}

func (s *application) nativeFormatSegment(c *ui.Context, label string, selected bool, p nativeColors) ui.Element {
	button := s.nativeSegment(c, label, false, p).Height(nativeControlSize - 4).Cursor(ui.CursorPointer).Selected(selected)
	if selected {
		button.Background(p.hover).TextColor(p.text)
	}
	return button
}

// Destination and protection metadata always come from the trusted record,
// not text supplied by a UI element. This explicit action never copies/pastes.
func (s *application) openNativeContent(id string) {
	item, exists := s.store.Record(id)
	if !exists {
		return
	}
	target, err := contentactions.OpenItemTarget(item.Item)
	if err != nil {
		s.reportNativeError("derive content action", err, "Could not open content")
		return
	}
	s.perform(func() error {
		if target.Path != "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			path, err := contentactions.ResolvePath(item.Item.Text, home)
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); err != nil {
				return err
			}
			mygo.Shell.ShowItemInFolder(path)
			return nil
		}
		if len(target.Calendar) > 0 {
			dir, err := os.MkdirTemp("", "copiggy-calendar-")
			if err != nil {
				return err
			}
			// Calendar opens an import sheet. Give it time to read the private
			// file; no event is inserted without its own confirmation.
			time.AfterFunc(10*time.Minute, func() { _ = os.RemoveAll(dir) })
			path := filepath.Join(dir, "Event.ics")
			if err := os.WriteFile(path, target.Calendar, 0600); err != nil {
				_ = os.RemoveAll(dir)
				return err
			}
			return mygo.Shell.OpenPath(path)
		}
		return mygo.Shell.OpenExternal(target.URL)
	}, func(err error) {
		if err != nil {
			fallback := "Could not open content"
			if target.Path != "" {
				fallback = "File unavailable"
			}
			// Handler errors may echo a full mailto/tel/Maps URL or local
			// path. Keep copied values out of application diagnostics.
			s.reportNativeError("open content action", errors.New("system handler could not open the requested content"), fallback)
		}
	})
}
