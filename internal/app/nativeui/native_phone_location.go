package nativeui

import (
	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// One result belongs to the current preview only. Closing, navigating or
// changing language drops the slot, including an unavailable (empty) result.
type nativePhoneLocationState struct {
	e164, locale, value string
	started             bool
	needsFrame          bool
}

func (s *application) nativePhoneLocation(item clipmodel.Item, phone clipmodel.PhoneValue) string {
	if !s.preview || s.quitting || phone.E164 == "" {
		return ""
	}
	state := s.semanticState(item)
	locale := s.currentLanguage()
	entry := state.PhoneLocation
	if entry == nil || entry.e164 != phone.E164 || entry.locale != locale {
		entry = &nativePhoneLocationState{e164: phone.E164, locale: locale}
		state.PhoneLocation = entry
	}
	if !entry.started && s.panel != nil && s.panel.IsVisible() {
		entry.started = true
		// The embedded geocoder lazily decompresses each locale on first use.
		// Keep that work out of rendering and history capture; no network or
		// clipboard access is involved, including for protected records.
		go func() {
			location := clipmodel.PhoneLocation(phone, locale)
			mygo.RunOnMain(func() {
				if s.finishNativePhoneLocation(item.ID, entry, location) {
					s.publish()
				}
			})
		}()
	}
	// This getter is read while building the preview. A held surface frame
	// never gets here, so only an actual rebuild acknowledges the result.
	entry.needsFrame = false
	return entry.value
}

func (s *application) finishNativePhoneLocation(id string, entry *nativePhoneLocationState, value string) bool {
	if !s.nativePhoneLocationActive(id, entry) {
		return false
	}
	entry.value = value
	entry.needsFrame = value != ""
	return true
}

func (s *application) nativePhoneLocationActive(id string, entry *nativePhoneLocationState) bool {
	if entry == nil || s.quitting || s.hiding || !s.preview || s.semanticPreview.ItemID != id || s.semanticPreview.PhoneLocation != entry || s.currentLanguage() != entry.locale {
		return false
	}
	if s.panel != nil && !s.panel.IsVisible() {
		return false
	}
	selected, exists := s.selected()
	if !exists || selected.ID != id {
		return false
	}
	record, exists := s.store.Record(id)
	if !exists {
		return false
	}
	phone := clipmodel.AnalyzeItem(record.Item).Phone
	if phone == nil || phone.E164 != entry.e164 {
		return false
	}
	return true
}

// NSPopover can consume an invalidation while its temporary window is still
// occluded during presentation, without the ordinary window's SurfaceShown
// event to resume it. The existing popover tick retries only a completed result
// that no preview build has read yet; rendering, dismissal or invalidation of
// its record/locale stops the retries. Empty results need no additional frame.
func (s *application) syncNativePhoneLocation(invalidate func()) {
	entry := s.semanticPreview.PhoneLocation
	if entry != nil && entry.needsFrame && s.nativePhoneLocationActive(s.semanticPreview.ItemID, entry) {
		invalidate()
	}
}

func (s *application) nativePhoneLocationMetadata(c *ui.Context, location string) {
	if location == "" {
		return
	}
	nativeContentText(c, location).Key("phone-number-location").Label(s.tr("Number location: %s", location)).FontSize(nativeTextControl).FontWeight(nativeWeightBody).LineHeight(1.35).FillWidth().Wrap().TextAlign(ui.Center)
}
