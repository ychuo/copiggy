package nativeui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeNavigationRetainsPendingCardAssets(t *testing.T) {
	s := nativeViewFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entry := &nativeContentAsset{pending: true, cancel: cancel}
	s.contentAssetState().entries["visible-card"] = entry
	draftCtx, cancelDraft := context.WithCancel(context.Background())
	s.cancel = cancelDraft

	s.selectNative(1, true)
	if draftCtx.Err() != context.Canceled {
		t.Fatal("navigation retained the previous preview operation")
	}
	if ctx.Err() != nil || s.contentAssets.entries["visible-card"] != entry {
		t.Fatal("navigation canceled a still-visible card's pending asset")
	}
	s.closeNativePreview()
	if ctx.Err() != nil || s.contentAssets.entries["visible-card"] != entry {
		t.Fatal("closing a preview canceled card assets while the panel remained open")
	}
}

func TestNativePanelLifecycleCancelsPendingCardAssets(t *testing.T) {
	for _, end := range []struct {
		name string
		run  func(*application)
	}{
		{"hide", (*application).hide},
		{"quit", (*application).BeginQuit},
	} {
		t.Run(end.name, func(t *testing.T) {
			s := &application{viewState: &nativePanelState{}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			state := s.contentAssetState()
			state.entries["pending"] = &nativeContentAsset{pending: true, cancel: cancel}
			completed := &nativeContentAsset{bitmap: &ui.Bitmap{}}
			state.entries["completed"] = completed
			end.run(s)
			if ctx.Err() != context.Canceled || state.entries["pending"] != nil {
				t.Fatal("ending the panel session retained pending asset work")
			}
			if state.entries["completed"] != completed {
				t.Fatal("ending the panel session unnecessarily discarded a completed thumbnail")
			}
		})
	}
}
