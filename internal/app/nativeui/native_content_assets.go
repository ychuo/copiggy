package nativeui

import (
	"context"
	"image"
	"math"
	"strings"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/linkmetadata"
	macos "copiggy/internal/macos/appkit"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const nativeContentAssetMaxBytes = 24 << 20

// Canceled entries leave the cache immediately, but native cancellation still
// passes through AppKit. Keep a separate worker bound until each request has
// actually completed/canceled, including image decoding.
var nativeMapWorkers = make(chan struct{}, 3)

// Evicting an entry cannot interrupt an image decoder already in progress.
// Keep its slot until decoding finishes, and cancel queued work before it starts.
var nativeLinkMetadataWorkers = make(chan struct{}, 3)

type nativeContentAsset struct {
	bitmap  *ui.Bitmap
	title   string
	pending bool
	bytes   int
	used    uint64
	cancel  context.CancelFunc
	failed  time.Time
	// Artwork is bound to the saved record and source text. Remote link work
	// additionally requires unrestricted scope, as all map work does.
	recordID string
	source   string
	remote   bool
	expires  time.Time
}

// All cache state belongs to the main UI thread. Workers only produce immutable
// image data and hand it back through RunOnMain. Cache eviction invalidates the
// entry identity, so a late map/HTML completion can never recreate old state.
type nativeContentAssetsState struct {
	entries map[string]*nativeContentAsset
	use     uint64
	links   *linkmetadata.Fetcher
}

func (s *application) contentAssetState() *nativeContentAssetsState {
	if s.contentAssets == nil {
		s.contentAssets = &nativeContentAssetsState{entries: map[string]*nativeContentAsset{}}
	}
	return s.contentAssets
}

func (s *application) nativeMapBitmap(item clipmodel.Item) *ui.Bitmap {
	return s.nativeMapBitmapAt(item, 0, false)
}

func (s *application) nativeMapPreviewBitmap(item clipmodel.Item, zoom int) *ui.Bitmap {
	return s.nativeMapBitmapAt(item, zoom, true)
}

func (s *application) nativeMapUnavailable(item clipmodel.Item, zoom int, preview bool) bool {
	if s.contentAssets == nil {
		return false
	}
	key := nativeMapAssetKey(item.ID, zoom, preview)
	entry := s.contentAssets.entries[key]
	return entry != nil && !entry.pending && entry.bitmap == nil && !entry.failed.IsZero()
}

func (s *application) nativeMapBitmapAt(item clipmodel.Item, zoom int, preview bool) *ui.Bitmap {
	if s.quitting || s.panel == nil || s.store == nil || item.Type != clipmodel.TypeCoordinates || len(item.AllowedApps) != 0 || !s.panel.IsVisible() {
		return nil
	}
	// Re-read trusted capture state before allowing any network-backed map work.
	record, ok := s.store.Record(item.ID)
	if !ok || len(record.Item.AllowedApps) != 0 || record.Item.Text != item.Text {
		return nil
	}
	coordinate := clipmodel.AnalyzeItem(record.Item).Coordinates
	if coordinate == nil {
		return nil
	}
	zoom = max(-2, min(3, zoom))
	width, height := 188, 188
	if preview {
		width, height = nativeMapImageWidth, nativeMapImageHeight
	}
	key := nativeMapAssetKey(item.ID, zoom, preview)
	state := s.contentAssetState()
	state.use++
	if entry := state.entries[key]; entry != nil {
		entry.used = state.use
		if entry.pending || entry.bitmap != nil || time.Since(entry.failed) < time.Minute {
			return entry.bitmap
		}
		delete(state.entries, key)
	}
	// MapKit work is limited to three outstanding visible card/preview requests.
	pending := 0
	for key, entry := range state.entries {
		if strings.HasPrefix(key, "map:") && entry.pending {
			pending++
		}
	}
	if pending >= 3 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	entry := &nativeContentAsset{pending: true, used: state.use, cancel: cancel, recordID: item.ID, source: record.Item.Text}
	state.entries[key] = entry
	s.pruneNativeContentAssets(key)
	latitudeSpan := 0.02 / math.Pow(2, float64(zoom))
	longitudeSpan := latitudeSpan * float64(width) / float64(height) / math.Max(0.1, math.Cos(coordinate.Latitude*math.Pi/180))
	request := macos.MapSnapshotRequest{
		Latitude: coordinate.Latitude, Longitude: coordinate.Longitude,
		LatitudeSpan: latitudeSpan, LongitudeSpan: longitudeSpan,
		Width: width, Height: height, Dark: false,
	}
	go func() {
		finish := func(bitmap image.Image, err error) {
			cancel()
			mygo.RunOnMain(func() {
				s.finishNativeMapAsset(state, key, entry, item, bitmap, err)
			})
		}
		select {
		case nativeMapWorkers <- struct{}{}:
		case <-ctx.Done():
			finish(nil, ctx.Err())
			return
		}
		complete := func(bitmap image.Image, err error) {
			<-nativeMapWorkers
			finish(bitmap, err)
		}
		startErr := error(context.Canceled)
		mygo.RunOnMain(func() {
			if ctx.Err() != nil || s.quitting || s.contentAssets != state || state.entries[key] != entry || !s.panel.IsVisible() {
				return
			}
			// A request may have waited behind previous cancellation/decoding.
			// Check saved authority again immediately before entering MapKit.
			current, exists := s.store.Record(item.ID)
			if !exists || len(current.Item.AllowedApps) != 0 || current.Item.Text != item.Text {
				return
			}
			startErr = macos.SnapshotMap(ctx, request, complete)
		})
		if startErr != nil {
			complete(nil, startErr)
		}
	}()
	return nil
}

func (s *application) finishNativeMapAsset(state *nativeContentAssetsState, key string, entry *nativeContentAsset, item clipmodel.Item, bitmap image.Image, err error) {
	if s.quitting || s.contentAssets != state || state.entries[key] != entry {
		return
	}
	current, exists := s.store.Record(item.ID)
	if !exists || len(current.Item.AllowedApps) != 0 || current.Item.Text != item.Text {
		delete(state.entries, key)
		return
	}
	entry.pending, entry.cancel = false, nil
	if err == nil && bitmap != nil {
		entry.bitmap = ui.NewBitmap(bitmap)
		entry.bytes = bitmap.Bounds().Dx() * bitmap.Bounds().Dy() * 4
	} else {
		entry.failed = time.Now()
	}
	s.pruneNativeContentAssets(key)
	s.publish()
}

func (s *application) nativeLinkCardMetadata(item clipmodel.Item) (string, *ui.Bitmap) {
	if s.quitting || s.store == nil || item.Type != clipmodel.TypeLink {
		return "", nil
	}
	// Detached scenes only extract copied HTML. Hidden windows and history
	// capture never start page requests; only nearby, rendered cards call here.
	if s.panel != nil && !s.panel.IsVisible() {
		return "", nil
	}
	record, ok := s.store.Record(item.ID)
	if !ok || record.Item.Type != clipmodel.TypeLink || record.Item.Text != item.Text {
		return "", nil
	}
	remote := nativeLinkMayFetch(item, record.Item, s.panel != nil)
	state := s.contentAssetState()
	state.use++
	key := "link:" + item.ID
	if entry := state.entries[key]; entry != nil {
		valid := entry.source == record.Item.Text && (!entry.remote || remote)
		upgrade := remote && !entry.remote && entry.bitmap == nil
		fresh := entry.pending || entry.expires.IsZero() || time.Now().Before(entry.expires)
		if valid && !upgrade && fresh {
			entry.used = state.use
			return entry.title, entry.bitmap
		}
		if entry.cancel != nil {
			entry.cancel()
		}
		delete(state.entries, key)
	}
	markup, _ := record.Data.Read(clipmodel.HTMLFormat)
	entry := &nativeContentAsset{pending: len(markup) != 0 || remote, used: state.use, recordID: item.ID, source: record.Item.Text, remote: remote}
	state.entries[key] = entry
	s.pruneNativeContentAssets(key)
	if !entry.pending {
		return "", nil
	}
	if s.panel == nil {
		// Detached Native UI scenes are used by the in-memory layout tests.
		// Extraction itself is local and bounded; don't initialize AppKit just
		// to dispatch a result for a scene that owns no application window.
		metadata := linkmetadata.Extract(record.Item.Text, markup)
		entry.pending, entry.title = false, metadata.Title
		if metadata.Cover != nil {
			entry.bitmap = ui.NewBitmap(metadata.Cover)
			entry.bytes = metadata.Cover.Bounds().Dx() * metadata.Cover.Bounds().Dy() * 4
		}
		s.pruneNativeContentAssets(key)
		return entry.title, entry.bitmap
	}
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	entry.cancel = cancel
	var fetch nativeLinkMetadataFetch
	if remote {
		if state.links == nil {
			state.links = linkmetadata.NewFetcher()
		}
		fetcher := state.links
		fetch = func(ctx context.Context, rawURL string) (linkmetadata.Metadata, error) {
			// A worker can wait behind another image decode. Revalidate saved
			// authority and visibility immediately before starting HTTP work.
			allowed := false
			mygo.RunOnMain(func() {
				allowed = ctx.Err() == nil && s.panel.IsVisible() && s.nativeLinkAssetCurrent(state, key, entry, item)
			})
			if !allowed {
				return linkmetadata.Metadata{}, context.Canceled
			}
			return fetcher.Fetch(ctx, rawURL)
		}
	}
	go func() {
		defer cancel()
		metadata, err := resolveNativeLinkMetadata(ctx, record.Item.Text, markup, fetch)
		mygo.RunOnMain(func() {
			s.finishNativeLinkAsset(state, key, entry, item, metadata, err)
		})
	}()
	return "", nil
}

type nativeLinkMetadataFetch func(context.Context, string) (linkmetadata.Metadata, error)

func nativeLinkMayFetch(displayed, saved clipmodel.Item, visible bool) bool {
	return visible && displayed.ID == saved.ID && displayed.Type == clipmodel.TypeLink && saved.Type == clipmodel.TypeLink && displayed.Text == saved.Text && len(displayed.AllowedApps) == 0 && len(saved.AllowedApps) == 0 && linkmetadata.CanFetch(saved.Text)
}

// One slot covers extraction, HTTP and image decoding. Cancellation before the
// slot is acquired never starts a request; decoding holds its slot to completion.
func resolveNativeLinkMetadata(ctx context.Context, rawURL string, markup []byte, fetch nativeLinkMetadataFetch) (linkmetadata.Metadata, error) {
	select {
	case nativeLinkMetadataWorkers <- struct{}{}:
	case <-ctx.Done():
		return linkmetadata.Metadata{}, ctx.Err()
	}
	defer func() { <-nativeLinkMetadataWorkers }()
	if err := ctx.Err(); err != nil {
		return linkmetadata.Metadata{}, err
	}
	metadata := linkmetadata.Extract(rawURL, markup)
	var fetchErr error
	if fetch != nil && metadata.Cover == nil && ctx.Err() == nil {
		var remote linkmetadata.Metadata
		remote, fetchErr = fetch(ctx, rawURL)
		if err := ctx.Err(); err != nil {
			// Keep the already copied title on timeout, but never install an
			// image returned after cancellation. Hidden/deleted entries are also
			// rejected by their identity and saved-scope checks at completion.
			return metadata, err
		}
		if metadata.Title == "" {
			metadata.Title = remote.Title
		}
		if metadata.Description == "" {
			metadata.Description = remote.Description
		}
		metadata.Cover = remote.Cover
	}
	if err := ctx.Err(); err != nil {
		return linkmetadata.Metadata{}, err
	}
	return metadata, fetchErr
}

func (s *application) nativeLinkAssetCurrent(state *nativeContentAssetsState, key string, entry *nativeContentAsset, item clipmodel.Item) bool {
	if s.quitting || s.store == nil || s.contentAssets != state || state.entries[key] != entry {
		return false
	}
	record, ok := s.store.Record(item.ID)
	return ok && record.Item.Type == clipmodel.TypeLink && record.Item.Text == entry.source && record.Item.Text == item.Text && (!entry.remote || len(record.Item.AllowedApps) == 0)
}

func (s *application) finishNativeLinkAsset(state *nativeContentAssetsState, key string, entry *nativeContentAsset, item clipmodel.Item, metadata linkmetadata.Metadata, err error) {
	if !s.nativeLinkAssetCurrent(state, key, entry, item) {
		if state.entries[key] == entry {
			delete(state.entries, key)
		}
		return
	}
	entry.pending, entry.title, entry.cancel = false, metadata.Title, nil
	entry.expires = time.Now().Add(10 * time.Minute)
	if metadata.Cover != nil {
		entry.bitmap = ui.NewBitmap(metadata.Cover)
		entry.bytes = metadata.Cover.Bounds().Dx() * metadata.Cover.Bounds().Dy() * 4
		entry.expires = time.Now().Add(6 * time.Hour)
	}
	if err != nil {
		entry.failed = time.Now()
	}
	s.pruneNativeContentAssets(key)
	s.publish()
}

// Capture, deletion and protection changes invalidate pending and ready map/link
// artwork without selecting, copying, or mutating the underlying record.
func (s *application) pruneUnavailableNativeContentAssets() {
	if s.contentAssets == nil || s.store == nil {
		return
	}
	for key, entry := range s.contentAssets.entries {
		kind := clipmodel.TypeLink
		if strings.HasPrefix(key, "map:") {
			kind = clipmodel.TypeCoordinates
		} else if !strings.HasPrefix(key, "link:") {
			continue
		}
		record, ok := s.store.Record(entry.recordID)
		needsUnrestricted := kind == clipmodel.TypeCoordinates || entry.remote
		if ok && record.Item.Type == kind && record.Item.Text == entry.source && (!needsUnrestricted || len(record.Item.AllowedApps) == 0) {
			continue
		}
		if entry.cancel != nil {
			entry.cancel()
		}
		delete(s.contentAssets.entries, key)
	}
}

func (s *application) pruneNativeContentAssets(keep string) {
	state := s.contentAssets
	if state == nil {
		return
	}
	for {
		bytes, oldestKey, oldestUse := 0, "", ^uint64(0)
		for key, entry := range state.entries {
			bytes += entry.bytes
			if key != keep && entry.used < oldestUse {
				oldestKey, oldestUse = key, entry.used
			}
		}
		if oldestKey == "" || len(state.entries) <= 24 && bytes <= nativeContentAssetMaxBytes {
			return
		}
		if cancel := state.entries[oldestKey].cancel; cancel != nil {
			cancel()
		}
		delete(state.entries, oldestKey)
	}
}

// Hide/quit cancels pending maps and link requests. Completed thumbnails may stay
// in this bounded in-memory cache across panel sessions; none are persisted.
func (s *application) cancelNativeContentAssets() {
	if s.contentAssets == nil {
		return
	}
	for key, entry := range s.contentAssets.entries {
		if entry.cancel != nil {
			entry.cancel()
		}
		if entry.pending {
			delete(s.contentAssets.entries, key)
		}
	}
}
