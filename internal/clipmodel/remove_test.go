package clipmodel

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRemoveEvictsWithoutReorderingOrSelectingAnotherRecord(t *testing.T) {
	store := NewStore(10)
	first, _ := store.Capture("first", "Editor", false)
	second, _ := store.Capture("second", "Editor", false)
	third, _ := store.Capture("third", "Editor", false)
	if err := store.Remove(second.ID); err != nil {
		t.Fatal(err)
	}
	items := store.Items()
	if len(items) != 2 || items[0].ID != third.ID || items[1].ID != first.ID {
		t.Fatalf("removing a record reordered survivors: %+v", items)
	}
	assertCurrent(t, store, third.ID)
	before := store.Items()
	if err := store.Remove("missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("removing a missing record should fail: %v", err)
	}
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("missing removal changed history")
	}
	assertCurrent(t, store, third.ID)
	if err := store.Remove(third.ID); err != nil {
		t.Fatal(err)
	}
	if _, exists := store.Current(); exists {
		t.Fatal("removing current should clear it without selecting another item")
	}
	if items := store.Items(); len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("removing current also removed unrelated history: %+v", items)
	}
	if _, err := store.Select(third.ID); !errors.Is(err, ErrItemNotFound) {
		t.Fatal("removed record could still be selected")
	}
}

func TestRemoveDraftSourceMakesDraftStaleUntilCanceled(t *testing.T) {
	store := NewStore(10)
	source, _ := store.CaptureProtected(`{"v":1}`, "Editor", []string{"com.editor"})
	draft, err := store.Enhance(context.Background(), source.ID, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := store.Capture("unrelated current", "Browser", false)
	if err := store.Remove(source.ID); err != nil {
		t.Fatal(err)
	}
	before := store.Items()
	if _, err := store.ValidateDraft(draft.ID); !errors.Is(err, ErrDraftStale) {
		t.Fatalf("removed source was still valid before a native write: %v", err)
	}
	assertCurrent(t, store, current.ID)
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("validating stale draft changed history")
	}
	store.Cancel(draft.ID)
	if _, err := store.ValidateDraft(draft.ID); !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("stale draft could not be canceled: %v", err)
	}
}
