package nativeui

import (
	"copiggy/internal/clipmodel"
)

func nativeLinkItemType(kind clipmodel.ContentType) bool {
	return kind == clipmodel.TypeLink || kind == clipmodel.TypeAppLink
}

// QR is a presentation of the saved destination, never the page's current URL
// or a new copy format. Retrieve the record again at the explicit action.
func (s *application) toggleNativeLinkQR(item clipmodel.Item) {
	selected, ok := s.selected()
	if !ok || selected.ID != item.ID {
		return
	}
	record, exists := s.store.Record(item.ID)
	if !exists || !nativeLinkItemType(record.Item.Type) {
		return
	}
	link := clipmodel.ParseLink(record.Item.Text)
	if link == nil {
		return
	}
	s.prepareNativeLinkQR(record.Item)
	state := s.semanticState(record.Item)
	state.LinkQR = !state.LinkQR
	state.LinkModeInitialized = true
	if state.LinkQR {
		s.prepareNativeLinkQR(record.Item)
		// The QR frame omits ui.WebView, which hides the existing page while
		// retaining its navigation, document and scroll position. Session
		// reconciliation still releases it on record, host or lifetime changes.
	}
	s.publish()
}

// App links start in QR mode, while web links enter it through the toolbar.
// In either case, only an open preview may encode the trusted saved target.
func (s *application) prepareNativeLinkQR(item clipmodel.Item) {
	if !s.preview || !nativeLinkItemType(item.Type) {
		return
	}
	selected, ok := s.selected()
	if !ok || selected.ID != item.ID {
		return
	}
	state := s.semanticState(selected)
	// Saved content is immutable for an ID. Closing, navigation and record
	// invalidation clear this session; pointer/paint frames can reuse its QR
	// without cloning all captured HTML/image representations from history.
	if state.LinkModeInitialized && (!state.LinkQR || state.QRCode != nil) {
		return
	}
	record, exists := s.store.Record(item.ID)
	if !exists || !nativeLinkItemType(record.Item.Type) {
		return
	}
	link := clipmodel.ParseLink(record.Item.Text)
	if link == nil {
		return
	}
	if !state.LinkModeInitialized {
		state.LinkQR, state.LinkModeInitialized = link.App, true
	}
	if !state.LinkQR {
		return
	}
	if state.QRCode == nil || state.QRTarget != link.URL {
		state.QRCode = newNativeLinkQRCode(link.URL)
		state.QRTarget = link.URL
	}
}
