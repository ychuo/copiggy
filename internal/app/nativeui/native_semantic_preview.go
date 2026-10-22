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
	ItemID                               string
	CopyFormat                           clipmodel.CopyFormat
	Revealed, UTC, Milliseconds, Dialing bool
	ColorFormat, MapZoom, ImageZoom      int
}

func (s *application) semanticState(item clipmodel.Item) *nativeSemanticPreviewState {
	if s.semanticPreview.ItemID != item.ID {
		s.semanticPreview = nativeSemanticPreviewState{ItemID: item.ID}
		if value := clipmodel.Analyze(item.Text); value.Timestamp != nil {
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
		label := s.tr("Dialing number")
		if state.Dialing {
			label = s.tr("Original number")
		}
		if s.nativePreviewButton(c, "TextAlignLeft", label, p).Disabled(s.commandBusy).Clicked() {
			state.Dialing = !state.Dialing
			state.CopyFormat = ""
			if state.Dialing {
				state.CopyFormat = clipmodel.PhoneDialing
			}
			s.publish()
		}
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

// Destination and protection metadata always come from the trusted record,
// not text supplied by a UI element. This explicit action never copies/pastes.
func (s *application) openNativeContent(id string) {
	item, exists := s.store.Record(id)
	if !exists {
		return
	}
	target, err := contentactions.OpenTarget(item.Item.Text)
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
