package nativeui

import (
	"context"
	"errors"
	"testing"

	"copiggy/internal/clipboard"
	"copiggy/internal/clipmodel"
)

type nativeQueuedCommand struct {
	work func() error
	done func(error)
}

// Separating worker execution from completion models dispatch without starting
// an AppKit loop or touching the native clipboard.
type nativeCommandQueue struct{ commands []nativeQueuedCommand }

func (q *nativeCommandQueue) dispatch(work func() error, done func(error)) {
	q.commands = append(q.commands, nativeQueuedCommand{work, done})
}

func (q *nativeCommandQueue) runNext(t *testing.T) func() {
	t.Helper()
	if len(q.commands) == 0 {
		t.Fatal("expected a queued command")
	}
	command := q.commands[0]
	q.commands = q.commands[1:]
	err := command.work()
	return func() { command.done(err) }
}

func TestDeferredPinPreservesNewSelectionAndPreview(t *testing.T) {
	s := nativeViewFixture(t)
	first, second := s.results[0], s.results[1]
	current, _ := s.store.Current()
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	s.setNativePinned(first.ID, true)
	if record, _ := s.store.Record(first.ID); record.Item.Pinned {
		t.Fatal("pin was exposed before its command ran")
	}
	s.selectNative(1, true)
	finish := queue.runNext(t)
	finish()
	selected, ok := s.selected()
	if !ok || selected.ID != second.ID || !s.preview {
		t.Fatal("old pin completion replaced a newly selected preview")
	}
	if record, _ := s.store.Record(first.ID); !record.Item.Pinned {
		t.Fatal("pin command was discarded after navigation")
	}
	assertNativeCurrent(t, s, current.ID)
}

func TestDeferredUnpinDoesNotCloseAnotherPinnedPreview(t *testing.T) {
	s := nativeViewFixture(t)
	first, second := s.results[0], s.results[1]
	for _, id := range []string{first.ID, second.ID} {
		if err := s.store.SetPinned(context.Background(), id, true); err != nil {
			t.Fatal(err)
		}
	}
	s.viewState.PinnedOnly = true
	s.filter()
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	s.setNativePinned(first.ID, false)
	s.selectNative(1, true)
	queue.runNext(t)()
	selected, ok := s.selected()
	if !ok || selected.ID != second.ID || !s.preview || len(s.results) != 1 {
		t.Fatal("unpinning an old selection closed or replaced the current pinned preview")
	}
}

func TestDeferredDeleteKeepsAdjacentSelectionWithinSameSession(t *testing.T) {
	s := nativeViewFixture(t)
	s.selectNative(1, true)
	removed, _ := s.selected()
	next := s.results[2]
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	if !s.deleteNativeItem(removed.ID) {
		t.Fatal("existing item was not queued for deletion")
	}
	queue.runNext(t)()
	selected, ok := s.selected()
	if !ok || selected.ID != next.ID || s.chosen != 1 {
		t.Fatalf("deleting the second item selected %q at %d instead of its neighbor %q", selected.ID, s.chosen, next.ID)
	}
	if s.preview {
		t.Fatal("preview remained bound to the deleted record")
	}
}

func TestDeferredDeleteDoesNotMoveNewSelection(t *testing.T) {
	s := nativeViewFixture(t)
	s.selectNative(1, true)
	removed, _ := s.selected()
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	s.deleteNativeItem(removed.ID)
	finish := queue.runNext(t)
	// The command may have persisted while the main thread is still displaying
	// its previous snapshot; the user's later selection must win at completion.
	s.selectNative(3, true)
	newSelection, _ := s.selected()
	finish()
	selected, ok := s.selected()
	if !ok || selected.ID != newSelection.ID || !s.preview {
		t.Fatal("old deletion completion moved the new selection or closed its preview")
	}
	if _, exists := s.store.Record(removed.ID); exists {
		t.Fatal("successful deletion was not committed")
	}
}

type nativeFailingRepository struct {
	records []clipmodel.Record
	err     error
}

func (r *nativeFailingRepository) Load(context.Context) ([]clipmodel.Record, bool, error) {
	return clipmodel.CloneRecords(r.records), true, nil
}
func (r *nativeFailingRepository) Save(context.Context, []clipmodel.Record) error { return r.err }

func TestDeferredMutationFailureKeepsHistoryAndNewPreview(t *testing.T) {
	for _, operation := range []string{"pin", "delete"} {
		t.Run(operation, func(t *testing.T) {
			s := nativeViewFixture(t)
			s.language = "en"
			repository := &nativeFailingRepository{records: s.store.Records(), err: errors.New("disk unavailable")}
			s.store = clipboard.NewHistory(repository, 30)
			if err := s.store.Load(context.Background()); err != nil {
				t.Fatal(err)
			}
			s.filter()
			first := s.results[0]
			var queue nativeCommandQueue
			s.config.Dispatch = queue.dispatch
			if operation == "pin" {
				s.setNativePinned(first.ID, true)
			} else {
				s.deleteNativeItem(first.ID)
			}
			s.selectNative(1, true)
			before, _ := s.selected()
			queue.runNext(t)()
			selected, ok := s.selected()
			if !ok || selected.ID != before.ID || !s.preview {
				t.Fatal("failed old command disturbed the new preview")
			}
			record, exists := s.store.Record(first.ID)
			if !exists || record.Item.Pinned || len(s.store.Items()) != len(repository.records) {
				t.Fatal("failed persistence changed history")
			}
			if s.trayStatus.event == "" {
				t.Fatal("failed persistence did not report an actionable error")
			}
		})
	}
}

func TestDeferredCopyCancellationPreservesNewPreview(t *testing.T) {
	s := nativeViewFixture(t)
	first := s.results[0]
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	var requested string
	var copyContext context.Context
	s.config.Copy = func(ctx context.Context, id string, draft bool) (clipmodel.Item, error) {
		requested, copyContext = id, ctx
		if draft {
			t.Fatal("plain history item became an enhancement draft")
		}
		return clipmodel.Item{}, ctx.Err()
	}
	s.choose()
	if !s.commandBusy {
		t.Fatal("queued copy was not marked busy")
	}
	s.selectNative(1, true)
	selected, _ := s.selected()
	s.message = "Current preview"
	queue.runNext(t)()
	if requested != first.ID || !errors.Is(copyContext.Err(), context.Canceled) {
		t.Fatal("navigation did not cancel the original copy request")
	}
	after, ok := s.selected()
	if !ok || after.ID != selected.ID || !s.preview || s.message != "Current preview" || s.commandBusy {
		t.Fatal("cancelled copy completion changed the newer preview")
	}
}

func TestCompletedCopyDoesNotCloseAReopenedPreview(t *testing.T) {
	s := nativeViewFixture(t)
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) {
		return s.results[0], nil
	}
	s.preview = true
	s.choose()
	finish := queue.runNext(t)
	// Completion can already be waiting on the main thread when the user
	// dismisses and reopens the same preview. Its identity alone is insufficient.
	s.closeNativePreview()
	s.preview = true
	finish()
	if !s.preview || s.commandBusy {
		t.Fatal("completed old copy closed the reopened preview")
	}
}

func TestDeferredEnhancementDiscardsResultAfterNavigation(t *testing.T) {
	s := nativeViewFixture(t)
	var queue nativeCommandQueue
	s.config.Dispatch = queue.dispatch
	s.enhance(clipmodel.FormatJSON)
	finish := queue.runNext(t)
	s.selectNative(1, true)
	selected, _ := s.selected()
	finish()
	if s.draft != nil || s.busy || !s.preview {
		t.Fatal("old enhancement appeared in the new preview")
	}
	after, _ := s.selected()
	if after.ID != selected.ID {
		t.Fatal("enhancement completion changed selection")
	}
	if len(queue.commands) != 1 {
		t.Fatal("stale enhancement result was not queued for cancellation")
	}
	queue.runNext(t)()
}
