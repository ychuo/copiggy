package nativeui

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
	"copiggy/internal/linkmetadata"
)

func contentAssetFixture(t *testing.T) (*application, clipmodel.Item) {
	t.Helper()
	s := &application{store: clipboard.NewHistory(nil, 30), viewState: &nativePanelState{}}
	item := nativeCaptureText(t, s, "31.2304, 121.4737", "Fixture")
	return s, item
}

func TestMapAssetLateCompletionCannotRestoreCancelledWork(t *testing.T) {
	s, item := contentAssetFixture(t)
	state := s.contentAssetState()
	ctx, cancel := context.WithCancel(context.Background())
	key := "map:" + item.ID + ":false:false:0"
	entry := &nativeContentAsset{pending: true, cancel: cancel}
	state.entries[key] = entry
	s.cancelNativeContentAssets()
	if ctx.Err() != context.Canceled || len(state.entries) != 0 {
		t.Fatal("closing the panel did not cancel and remove pending map work")
	}
	s.finishNativeMapAsset(state, key, entry, item, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil)
	if len(state.entries) != 0 || entry.bitmap != nil {
		t.Fatal("late snapshot recreated canceled cache state")
	}
}

func TestMapAssetCompletionRechecksSavedProtectionAndExistence(t *testing.T) {
	for _, change := range []string{"protect", "delete", "replace-cache", "quit"} {
		t.Run(change, func(t *testing.T) {
			s, item := contentAssetFixture(t)
			state := s.contentAssetState()
			entry := &nativeContentAsset{pending: true}
			key := "map:" + item.ID
			state.entries[key] = entry
			switch change {
			case "protect":
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.editor"}); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := s.store.Delete(context.Background(), item.ID); err != nil {
					t.Fatal(err)
				}
			case "replace-cache":
				s.contentAssets = &nativeContentAssetsState{entries: map[string]*nativeContentAsset{}}
			case "quit":
				s.quitting = true
			}
			s.finishNativeMapAsset(state, key, entry, item, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil)
			if entry.bitmap != nil {
				t.Fatal("obsolete or protected map result became visible")
			}
		})
	}
}

func TestMapAssetSuccessAndFailureAreBoundedPresentationOnly(t *testing.T) {
	s, item := contentAssetFixture(t)
	state := s.contentAssetState()
	key := "map:" + item.ID + ":false:false:0"
	entry := &nativeContentAsset{pending: true}
	state.entries[key] = entry
	s.finishNativeMapAsset(state, key, entry, item, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil)
	if entry.pending || entry.bitmap == nil || entry.bytes != 64*64*4 || s.nativeMapUnavailable(item, 0, false) {
		t.Fatal("successful snapshot did not become a bounded ready image")
	}
	s.cancelNativeContentAssets()
	if state.entries[key] != entry {
		t.Fatal("hiding discarded a completed thumbnail unnecessarily")
	}
	failed := &nativeContentAsset{pending: true}
	state.entries[key] = failed
	s.finishNativeMapAsset(state, key, failed, item, nil, errors.New("offline"))
	if failed.pending || !s.nativeMapUnavailable(item, 0, false) {
		t.Fatal("failed snapshot remained in loading state")
	}
	stored, _ := s.store.Record(item.ID)
	if stored.Item.Text != item.Text || len(stored.Item.AllowedApps) != 0 {
		t.Fatal("map presentation modified the saved coordinate or protection")
	}
}

func TestContentAssetCacheEvictsByBudgetAndCancelsOldWork(t *testing.T) {
	s, _ := contentAssetFixture(t)
	state := s.contentAssetState()
	canceled := 0
	for i := 0; i < 26; i++ {
		state.entries[fmt.Sprint(i)] = &nativeContentAsset{used: uint64(i), bytes: 2 << 20, cancel: func() { canceled++ }}
	}
	s.pruneNativeContentAssets("25")
	bytes := 0
	for _, entry := range state.entries {
		bytes += entry.bytes
	}
	if len(state.entries) > 24 || bytes > nativeContentAssetMaxBytes || canceled == 0 || state.entries["25"] == nil {
		t.Fatal("asset budget failed or most-recent visible item was evicted")
	}
}

func TestMapAssetInvalidationCancelsAllViewsAndRetainsCurrentArtwork(t *testing.T) {
	for _, change := range []string{"delete", "expire", "protect"} {
		t.Run(change, func(t *testing.T) {
			s, item := contentAssetFixture(t)
			other := nativeCaptureText(t, s, "37.7749, -122.4194", "Fixture")
			link := nativeCapture(t, s, clipmodel.TextData("https://example.com/article"), clipmodel.Source{Name: "Fixture"}, "com.example.editor")
			for _, saved := range []clipmodel.Item{other, link} {
				if err := s.store.SetPinned(context.Background(), saved.ID, true); err != nil {
					t.Fatal(err)
				}
			}
			state := s.contentAssetState()
			cardCtx, cancelCard := context.WithCancel(context.Background())
			defer cancelCard()
			previewCtx, cancelPreview := context.WithCancel(context.Background())
			defer cancelPreview()
			otherCtx, cancelOther := context.WithCancel(context.Background())
			defer cancelOther()
			cardKey, previewKey := nativeMapAssetKey(item.ID, 0, false), nativeMapAssetKey(item.ID, 1, true)
			card := &nativeContentAsset{recordID: item.ID, source: item.Text, pending: true, cancel: cancelCard}
			state.entries[cardKey] = card
			state.entries[previewKey] = &nativeContentAsset{recordID: item.ID, source: item.Text, pending: true, cancel: cancelPreview}
			state.entries[nativeMapAssetKey(item.ID, 0, true)] = &nativeContentAsset{recordID: item.ID, source: item.Text, failed: time.Now()}
			otherKey, linkKey := nativeMapAssetKey(other.ID, 0, false), "link:"+link.ID
			otherEntry := &nativeContentAsset{recordID: other.ID, source: other.Text, pending: true, cancel: cancelOther}
			// Protected copied HTML is local artwork and remains eligible.
			linkEntry := &nativeContentAsset{recordID: link.ID, source: link.Text, title: "Copied title"}
			state.entries[otherKey], state.entries[linkKey] = otherEntry, linkEntry
			switch change {
			case "delete":
				if err := s.store.Delete(context.Background(), item.ID); err != nil {
					t.Fatal(err)
				}
			case "expire":
				if expired, err := s.store.Expire(context.Background(), item.CreatedAt.Add(31*24*time.Hour)); err != nil || !expired {
					t.Fatalf("expire fixture: expired=%t, err=%v", expired, err)
				}
			case "protect":
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.editor"}); err != nil {
					t.Fatal(err)
				}
			}
			before := s.store.Records()
			s.pruneUnavailableNativeContentAssets()
			if cardCtx.Err() != context.Canceled || previewCtx.Err() != context.Canceled {
				t.Fatal("invalidated map work was not canceled immediately")
			}
			if len(state.entries) != 2 || state.entries[otherKey] != otherEntry || state.entries[linkKey] != linkEntry || otherCtx.Err() != nil {
				t.Fatal("invalidation retained obsolete map views or discarded unrelated artwork")
			}
			s.finishNativeMapAsset(state, cardKey, card, item, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil)
			if state.entries[cardKey] != nil || !reflect.DeepEqual(before, s.store.Records()) {
				t.Fatal("late completion restored invalidated artwork or changed history")
			}
		})
	}
}

func TestCopiedLinkMetadataRendersWithoutNativeRuntime(t *testing.T) {
	s := &application{store: clipboard.NewHistory(nil, 30)}
	rawURL := "https://example.com/guide"
	item := nativeCapture(t, s, clipmodel.Representations{{
		{Format: clipmodel.TextFormat, Bytes: []byte(rawURL)},
		{Format: clipmodel.HTMLFormat, Bytes: []byte(`<meta property="og:url" content="https://example.com/guide"><title>A copied guide</title>`)},
	}}, clipmodel.Source{Name: "Fixture"})
	title, cover := s.nativeLinkCardMetadata(item)
	if title != "A copied guide" || cover != nil || s.contentAssets.entries["link:"+item.ID].pending {
		t.Fatal("detached native scene did not resolve local metadata synchronously")
	}
}

func TestCopiedLinkMetadataQueuedWorkCancelsBeforeCapacityIsAvailable(t *testing.T) {
	for i := 0; i < cap(nativeLinkMetadataWorkers); i++ {
		nativeLinkMetadataWorkers <- struct{}{}
	}
	held := cap(nativeLinkMetadataWorkers)
	defer func() {
		for i := 0; i < held; i++ {
			<-nativeLinkMetadataWorkers
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		metadata, err := resolveNativeLinkMetadata(ctx, "https://example.com/", []byte(`<meta property="og:url" content="https://example.com/"><title>Queued title</title>`), nil)
		if metadata.Title != "" || metadata.Cover != nil {
			done <- errors.New("canceled metadata was returned")
			return
		}
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued metadata error = %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled queued metadata waited for a decoder slot")
	}
	if len(nativeLinkMetadataWorkers) != held {
		t.Fatal("canceling queued metadata released another decoder's slot")
	}
}

func TestCopiedLinkMetadataWorkerReturnsLocalResultAndReleasesSlot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	metadata, err := resolveNativeLinkMetadata(ctx, "https://example.com/", []byte(`<meta property="og:url" content="https://example.com/"><title>Local title</title>`), nil)
	if err != nil || metadata.Title != "Local title" || metadata.Cover != nil {
		t.Fatal("bounded metadata worker did not return the copied local title")
	}
	if len(nativeLinkMetadataWorkers) != 0 {
		t.Fatal("completed metadata extraction retained a decoder slot")
	}
}

func TestRemoteLinkMetadataEligibilityUsesSavedScope(t *testing.T) {
	item := clipmodel.Item{ID: "saved", Type: clipmodel.TypeLink, Text: "https://example.com/article?q=1#anchor"}
	if !nativeLinkMayFetch(item, item, true) {
		t.Fatal("visible unrestricted URL was rejected")
	}
	for _, change := range []string{"protected-store", "protected-ui", "hidden", "different-url", "different-id", "different-type", "private-url", "credentials"} {
		t.Run(change, func(t *testing.T) {
			displayed, saved, visible := item, item, true
			switch change {
			case "protected-store":
				saved.AllowedApps = []string{"com.example.editor"}
			case "protected-ui":
				displayed.AllowedApps = []string{"com.example.editor"}
			case "hidden":
				visible = false
			case "different-url":
				saved.Text = "https://example.com/another"
			case "different-id":
				saved.ID = "another"
			case "different-type":
				saved.Type = clipmodel.TypeText
			case "private-url":
				displayed.Text, saved.Text = "http://127.0.0.1/private", "http://127.0.0.1/private"
			case "credentials":
				displayed.Text, saved.Text = "https://user:secret@example.com/", "https://user:secret@example.com/"
			}
			if nativeLinkMayFetch(displayed, saved, visible) {
				t.Fatal("untrusted or hidden item permitted a request")
			}
		})
	}
}

func TestRemoteLinkMetadataMergesWithoutReplacingCapturedTitle(t *testing.T) {
	url := "https://example.com/article"
	markup := []byte(`<meta property="og:url" content="https://example.com/article"><title>Copied title</title>`)
	cover := image.NewNRGBA(image.Rect(0, 0, 16, 8))
	calls := 0
	fetch := func(_ context.Context, got string) (linkmetadata.Metadata, error) {
		calls++
		if got != url {
			t.Fatal("worker changed the saved URL")
		}
		return linkmetadata.Metadata{Title: "Website title", Cover: cover}, nil
	}
	got, err := resolveNativeLinkMetadata(context.Background(), url, markup, fetch)
	if err != nil || calls != 1 || got.Title != "Copied title" || got.Cover != cover {
		t.Fatal("remote cover did not augment copied metadata")
	}
	got, err = resolveNativeLinkMetadata(context.Background(), url, nil, fetch)
	if err != nil || calls != 2 || got.Title != "Website title" || got.Cover != cover {
		t.Fatal("plain URL did not resolve remote metadata")
	}
	failed := func(context.Context, string) (linkmetadata.Metadata, error) {
		return linkmetadata.Metadata{}, errors.New("offline")
	}
	got, err = resolveNativeLinkMetadata(context.Background(), url, markup, failed)
	if err == nil || got.Title != "Copied title" || got.Cover != nil {
		t.Fatal("failed lookup lost the local fallback")
	}
}

func TestEmbeddedLinkCoverDoesNotStartRemoteRequest(t *testing.T) {
	cover := image.NewNRGBA(image.Rect(0, 0, 16, 8))
	cover.SetNRGBA(0, 0, color.NRGBA{R: 123, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, cover); err != nil {
		t.Fatal(err)
	}
	markup := []byte(`<meta property="og:url" content="https://example.com/"><meta property="og:image" content="data:image/png;base64,` + base64.StdEncoding.EncodeToString(encoded.Bytes()) + `">`)
	got, err := resolveNativeLinkMetadata(context.Background(), "https://example.com/", markup, func(context.Context, string) (linkmetadata.Metadata, error) {
		t.Fatal("an embedded cover unexpectedly triggered a network lookup")
		return linkmetadata.Metadata{}, nil
	})
	if err != nil || got.Cover == nil {
		t.Fatal("embedded cover was lost")
	}
}

func TestRemoteLinkTimeoutKeepsCopiedTitleAndDiscardsLateCover(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	markup := []byte(`<meta property="og:url" content="https://example.com/"><title>Copied title</title>`)
	got, err := resolveNativeLinkMetadata(ctx, "https://example.com/", markup, func(ctx context.Context, _ string) (linkmetadata.Metadata, error) {
		<-ctx.Done()
		return linkmetadata.Metadata{Title: "Too late", Cover: image.NewNRGBA(image.Rect(0, 0, 16, 8))}, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || got.Title != "Copied title" || got.Cover != nil {
		t.Fatal("timeout lost local fallback or installed a late remote result")
	}
}

func TestRemoteLinkMetadataQueuedCancellationNeverFetches(t *testing.T) {
	for i := 0; i < cap(nativeLinkMetadataWorkers); i++ {
		nativeLinkMetadataWorkers <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(nativeLinkMetadataWorkers); i++ {
			<-nativeLinkMetadataWorkers
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := resolveNativeLinkMetadata(ctx, "https://example.com/", nil, func(context.Context, string) (linkmetadata.Metadata, error) {
			return linkmetadata.Metadata{}, errors.New("unexpected request")
		})
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued worker returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation waited for a request slot")
	}
}

func TestRemoteLinkAssetInvalidationRejectsLateResults(t *testing.T) {
	for _, change := range []string{"protect", "delete", "cancel", "replace-cache", "quit"} {
		t.Run(change, func(t *testing.T) {
			s, _ := contentAssetFixture(t)
			item := nativeCaptureText(t, s, "https://example.com/article", "Fixture")
			state, key := s.contentAssetState(), "link:"+item.ID
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entry := &nativeContentAsset{pending: true, remote: true, recordID: item.ID, source: item.Text, cancel: cancel}
			state.entries[key] = entry
			switch change {
			case "protect":
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.editor"}); err != nil {
					t.Fatal(err)
				}
				s.pruneUnavailableNativeContentAssets()
			case "delete":
				if err := s.store.Delete(context.Background(), item.ID); err != nil {
					t.Fatal(err)
				}
				s.pruneUnavailableNativeContentAssets()
			case "cancel":
				s.cancelNativeContentAssets()
			case "replace-cache":
				s.contentAssets = &nativeContentAssetsState{entries: map[string]*nativeContentAsset{}}
			case "quit":
				s.quitting = true
			}
			if (change == "protect" || change == "delete" || change == "cancel") && ctx.Err() != context.Canceled {
				t.Fatal("obsolete request was not canceled")
			}
			if s.nativeLinkAssetCurrent(state, key, entry, item) {
				t.Fatal("queued request still had authority to fetch")
			}
			s.finishNativeLinkAsset(state, key, entry, item, linkmetadata.Metadata{Title: "Late title", Cover: image.NewNRGBA(image.Rect(0, 0, 16, 8))}, nil)
			if entry.bitmap != nil || entry.title != "" || state.entries[key] != nil {
				t.Fatal("late completion recreated invalidated artwork")
			}
		})
	}
}

func TestRemoteLinkAssetCachesPresentationWithoutChangingHistory(t *testing.T) {
	s, _ := contentAssetFixture(t)
	item := nativeCaptureText(t, s, "https://example.com/article", "Fixture")
	before := s.store.Records()
	state, key := s.contentAssetState(), "link:"+item.ID
	entry := &nativeContentAsset{recordID: item.ID, source: item.Text, remote: true, pending: true}
	state.entries[key] = entry
	s.finishNativeLinkAsset(state, key, entry, item, linkmetadata.Metadata{Title: "Website title", Cover: image.NewNRGBA(image.Rect(0, 0, 16, 8))}, nil)
	if entry.pending || entry.title != "Website title" || entry.bitmap == nil || entry.bytes != 16*8*4 || time.Until(entry.expires) < 5*time.Hour {
		t.Fatal("successful cover was not cached")
	}
	s.cancelNativeContentAssets()
	if state.entries[key] != entry {
		t.Fatal("hiding discarded a completed cover")
	}
	failed := &nativeContentAsset{recordID: item.ID, source: item.Text, remote: true, pending: true}
	state.entries[key] = failed
	s.finishNativeLinkAsset(state, key, failed, item, linkmetadata.Metadata{Title: "Title without cover"}, errors.New("offline"))
	if failed.pending || failed.title == "" || failed.failed.IsZero() || time.Until(failed.expires) > 10*time.Minute || time.Until(failed.expires) < 9*time.Minute {
		t.Fatal("failure did not retain fallback and back off")
	}
	if !reflect.DeepEqual(before, s.store.Records()) {
		t.Fatal("metadata modified saved data, provenance or protection")
	}
}

func TestPlainLinkDetachedSceneNeverCreatesFetcher(t *testing.T) {
	s, _ := contentAssetFixture(t)
	item := nativeCaptureText(t, s, "https://example.com/article", "Fixture")
	title, cover := s.nativeLinkCardMetadata(item)
	if title != "" || cover != nil || s.contentAssets.links != nil || s.contentAssets.entries["link:"+item.ID].pending {
		t.Fatal("detached scene started remote work")
	}
}
