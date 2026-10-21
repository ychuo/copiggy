package nativeui

import "copiggy/internal/clipmodel"

const maxDecodedImages = 8

// Only UI caches live here. The history service owns count, bytes and age.
func (s *application) pruneHistory() {
	if s.store == nil {
		return
	}
	live, sources := map[string]bool{}, map[string]bool{}
	for _, item := range s.store.Items() {
		live[item.ID] = true
		sources[item.SourceAppID] = true
	}
	for id := range s.itemSources {
		if !live[id] {
			delete(s.itemSources, id)
		}
	}
	for source := range s.appIcons {
		if !sources[source] {
			delete(s.appIcons, source)
		}
	}
	if s.viewState != nil {
		for id := range s.viewState.WebsiteIcons {
			if !live[id] {
				delete(s.viewState.WebsiteIcons, id)
			}
		}
		for id := range s.viewState.WebsiteHTML {
			if !live[id] {
				delete(s.viewState.WebsiteHTML, id)
			}
		}
		for id := range s.viewState.CardBounds {
			if !live[id] {
				delete(s.viewState.CardBounds, id)
			}
		}
		for source := range s.viewState.Icons {
			if !sources[source] {
				delete(s.viewState.Icons, source)
			}
		}
	}
	s.pruneImageCache("")
}

// pruneImageCache bounds decoded bitmaps independently of compressed history.
// Call it after caching a decoded image, passing that image's ID. The new image
// survives even when it is older than the other cached records; remaining slots
// favor recent history. This method never mutates clipboard history.
func (s *application) pruneImageCache(keepID string) {
	if s.store == nil || s.viewState == nil {
		return
	}
	items := s.store.Items()
	keep := make(map[string]bool, maxDecodedImages)
	if _, cached := s.viewState.Images[keepID]; keepID != "" && cached {
		for _, item := range items {
			if item.ID == keepID && item.Type == clipmodel.TypeImage {
				keep[keepID] = true
				break
			}
		}
	}
	for _, item := range items {
		if len(keep) >= maxDecodedImages {
			break
		}
		if _, cached := s.viewState.Images[item.ID]; cached && item.Type == clipmodel.TypeImage {
			keep[item.ID] = true
		}
	}
	for id := range s.viewState.Images {
		if !keep[id] {
			delete(s.viewState.Images, id)
		}
	}
}
