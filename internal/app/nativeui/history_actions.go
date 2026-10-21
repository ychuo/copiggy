package nativeui

import (
	"context"
	"copiggy/internal/clipmodel"
	"errors"
)

func backgroundContext() context.Context { return context.Background() }
func (s *application) setNativePinned(id string, pinned bool) {
	if s.quitting {
		return
	}
	selected, hadSelection := s.selected()
	generation := s.uiGeneration
	s.perform(func() error { return s.store.SetPinned(context.Background(), id, pinned) }, func(err error) {
		if err != nil {
			if errors.Is(err, clipmodel.ErrPinLimit) {
				s.reportNativeStatus(s.tr("Pin limit reached"))
			} else {
				s.reportNativeError("save pinned history", err, "Could not save pin")
			}
			s.publish()
			return
		}
		s.Changed()
		if generation == s.uiGeneration && hadSelection && selected.ID == id {
			if after, ok := s.selected(); !ok || after.ID != id {
				s.closeNativePreview()
				if s.viewState != nil {
					s.viewState.Focus.Focus()
				}
			}
		}
		s.publish()
	})
}

// Deletion affects only saved history. The native clipboard and its policy live
// in a separate adapter and are never called by this command.
func (s *application) deleteNativeItem(id string) bool {
	if s.quitting {
		return false
	}
	if _, exists := s.store.Record(id); !exists {
		return false
	}
	previousIndex := s.chosen
	generation := s.uiGeneration
	selected, wasSelected := s.selected()
	wasSelected = wasSelected && selected.ID == id
	s.perform(func() error { return s.store.Delete(context.Background(), id) }, func(err error) {
		if err != nil {
			s.reportNativeError("delete history record", err, "Could not delete item")
			s.publish()
			return
		}
		// Changed closes the removed selection and advances uiGeneration itself.
		// Capture the user's session before that cleanup so its own cancellation
		// does not suppress the intended adjacent selection.
		sameSession := generation == s.uiGeneration
		s.Changed()
		if sameSession && wasSelected {
			s.chosen = max(0, min(previousIndex, len(s.results)-1))
			if s.viewState != nil {
				s.viewState.Focus.Focus()
			}
		}
		s.publish()
	})
	return true
}
