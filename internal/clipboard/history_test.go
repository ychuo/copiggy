package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/storage"
)

type memoryRepository struct {
	records          []clipmodel.Record
	authoritative    bool
	loadErr, saveErr error
	saves            int
	beforeSave       func()
}

func (r *memoryRepository) Load(context.Context) ([]clipmodel.Record, bool, error) {
	return clipmodel.CloneRecords(r.records), r.authoritative, r.loadErr
}
func (r *memoryRepository) Save(_ context.Context, records []clipmodel.Record) error {
	if r.beforeSave != nil {
		r.beforeSave()
	}
	r.saves++
	if r.saveErr != nil {
		return r.saveErr
	}
	r.records = clipmodel.CloneRecords(records)
	r.authoritative = true
	return nil
}
func historyRecord(t *testing.T, id, text string, at time.Time, pinned bool, scope []string, data clipmodel.Representations) clipmodel.Record {
	t.Helper()
	if data == nil {
		data = clipmodel.TextData(text)
	}
	content, err := clipmodel.ContentFromRepresentations(data)
	if err != nil {
		t.Fatal(err)
	}
	content.SourceAppID = "com.example.editor"
	s := clipmodel.NewStore(100)
	var item clipmodel.Item
	if len(scope) > 0 {
		item, _ = s.CaptureProtectedContent(content, "Original Editor", scope)
	} else {
		item, _ = s.CaptureContent(content, "Original Editor", false)
	}
	item.ID, item.CreatedAt, item.Pinned = id, at, pinned
	return clipmodel.Record{Item: item, Data: data}
}
func loadedHistory(t *testing.T, repo HistoryRepository, days int) *History {
	t.Helper()
	h := NewHistory(repo, days)
	if err := h.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHistoryRoundTripExactRepresentationsAndScopeWithoutAuthority(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "Copiggy", "history.json")
	repo := storage.NewHistoryRepository(path, "")
	defer repo.Close()
	h := loadedHistory(t, repo, 0)
	data := testData(testItem(testRep(clipmodel.TextFormat, []byte("hello")), testRep(clipmodel.HTMLFormat, []byte("<b>hello</b>")), testRep("com.vendor.rich", []byte{0, 1, 255})), testItem(testRep(clipmodel.TextFormat, []byte("second"))))
	item, ok, err := h.Capture(ctx, data, clipmodel.Source{Name: "Editor", BundleID: "editor.app"}, []string{"editor.app"})
	if err != nil || !ok {
		t.Fatalf("capture: %v", err)
	}
	if err = h.SetPinned(ctx, item.ID, true); err != nil {
		t.Fatal(err)
	}
	restored := loadedHistory(t, repo, 0)
	record, ok := restored.Record(item.ID)
	if !ok || !reflect.DeepEqual(record.Data, data) || !record.Item.Pinned || record.Item.Allows("other.app") {
		t.Fatalf("incorrect restored record: %+v", record.Item)
	}
	if _, ok = restored.Current(); ok {
		t.Fatal("restoration restored clipboard authority")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("history permissions: %v %v", info, err)
	}
	data[0][0].Bytes[0] = 'X'
	record.Data[0][0].Bytes[0] = 'Y'
	again, _ := restored.Record(item.ID)
	if string(again.Data[0][0].Bytes) != "hello" {
		t.Fatal("caller mutated saved bytes")
	}
}

func TestHistoryRecopyPreservesIdentityProvenanceAndDistinctScope(t *testing.T) {
	ctx := context.Background()
	old := historyRecord(t, "item-1", "same", time.Now().AddDate(0, 0, -40), true, []string{"editor.app"}, nil)
	repo := &memoryRepository{records: []clipmodel.Record{old}, authoritative: true}
	h := loadedHistory(t, repo, 30)
	repeated, ok, err := h.Capture(ctx, old.Data, clipmodel.Source{Name: "Other", BundleID: "other.app"}, []string{"editor.app"})
	if err != nil || !ok || repeated.ID != old.Item.ID || repeated.SourceApp != old.Item.SourceApp || repeated.SourceAppID != old.Item.SourceAppID || !repeated.CreatedAt.After(old.Item.CreatedAt) {
		t.Fatalf("dedup provenance: %+v %v", repeated, err)
	}
	ordinary, _, err := h.Capture(ctx, old.Data, clipmodel.Source{Name: "Other"}, nil)
	if err != nil || ordinary.ID == repeated.ID || len(h.Items()) != 2 {
		t.Fatal("different scopes merged")
	}
	before := repo.saves
	if err = h.Persist(ctx); err != nil || repo.saves != before {
		t.Fatal("unchanged snapshot rewrote history")
	}
}

func TestHistoryExpiryAndUnpinUseOnePolicyAndInvalidateDraft(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	old := historyRecord(t, "item-1", `{"x":1}`, now.AddDate(0, 0, -31), true, nil, nil)
	fresh := historyRecord(t, "item-2", "fresh", now, false, nil, nil)
	repo := &memoryRepository{records: []clipmodel.Record{fresh, old}, authoritative: true}
	h := loadedHistory(t, repo, 30)
	draft, err := h.Enhance(ctx, old.Item.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.SetPinned(ctx, old.Item.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(h.Items()) != 1 {
		t.Fatal("unpin retained an expired ordinary record")
	}
	if _, ok := h.Record(old.Item.ID); ok {
		t.Fatal("evicted record retained representations")
	}
	if _, err = h.ValidateDraft(draft.ID); !errors.Is(err, clipmodel.ErrDraftStale) {
		t.Fatalf("deleted source draft remained valid: %v", err)
	}
	restored := loadedHistory(t, repo, 30)
	if len(restored.Items()) != 1 {
		t.Fatal("unpin expiration did not persist")
	}
}

func TestHistoryWriteFailureDoesNotExpireDeleteOrUnpin(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	old := historyRecord(t, "item-1", `{"x":1}`, now.AddDate(0, 0, -40), false, nil, nil)
	pin := historyRecord(t, "item-2", "pin", now.AddDate(0, 0, -60), true, []string{"editor.app"}, nil)
	repo := &memoryRepository{records: []clipmodel.Record{old, pin}, authoritative: true}
	h := loadedHistory(t, repo, 0)
	_, _ = h.Select(old.Item.ID)
	draft, err := h.Enhance(ctx, old.Item.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	before := h.Records()
	failure := errors.New("disk full")
	repo.saveErr = failure
	h.SetRetention(1)
	if changed, err := h.Expire(ctx, now); changed || !errors.Is(err, failure) {
		t.Fatal("failed expiration committed")
	}
	if !errors.Is(h.Delete(ctx, old.Item.ID), failure) {
		t.Fatal("failed deletion accepted")
	}
	if !errors.Is(h.SetPinned(ctx, pin.Item.ID, false), failure) {
		t.Fatal("failed unpin accepted")
	}
	if !reflect.DeepEqual(before, h.Records()) {
		t.Fatal("failed disk transaction changed complete records")
	}
	if _, err = h.ValidateDraft(draft.ID); err != nil {
		t.Fatal("failed transaction invalidated draft")
	}
	current, ok := h.Current()
	if !ok || current.ID != old.Item.ID {
		t.Fatal("failed transaction changed current")
	}
}

func TestHistoryStartupExpiryFailurePreservesReadableSnapshotAndRetries(t *testing.T) {
	ctx := context.Background()
	old := historyRecord(t, "item-1", "old", time.Now().AddDate(0, 0, -60), false, nil, nil)
	failure := errors.New("disk full")
	repo := &memoryRepository{records: []clipmodel.Record{old}, authoritative: true, saveErr: failure}
	h := NewHistory(repo, 30)
	if err := h.Load(ctx); !errors.Is(err, ErrHistorySave) || !errors.Is(err, failure) {
		t.Fatalf("load result: %v", err)
	}
	if len(h.Items()) != 1 {
		t.Fatal("failed cleanup hid readable history")
	}
	if _, ok := h.Current(); ok {
		t.Fatal("load made current")
	}
	repo.saveErr = nil
	if changed, err := h.Expire(ctx, time.Now()); err != nil || !changed || len(h.Items()) != 0 {
		t.Fatalf("retry: %v %v", changed, err)
	}
}

func TestLegacyMigrationCommitsOnceAndNeverResurrectsDeletedPin(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path, legacy := filepath.Join(dir, "history.json"), filepath.Join(dir, "pinned.json")
	pin := historyRecord(t, "item-19", "legacy", time.Now().AddDate(0, 0, -100), true, []string{"editor.app"}, nil)
	encoded, err := storage.EncodeHistory([]clipmodel.Record{pin})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(legacy, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	repo := storage.NewHistoryRepository(path, legacy)
	defer repo.Close()
	failure := errors.New("disk full")
	repo.Write = func(string, []byte) error { return failure }
	h := NewHistory(repo, 30)
	if err = h.Load(ctx); !errors.Is(err, ErrHistorySave) || len(h.Pinned()) != 1 {
		t.Fatalf("migration failure discarded pins: %v", err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed migration produced authoritative file")
	}
	repo.Write = nil
	if err = h.Persist(ctx); err != nil {
		t.Fatal(err)
	}
	if err = h.Delete(ctx, pin.Item.ID); err != nil {
		t.Fatal(err)
	}
	if len(loadedHistory(t, repo, 30).Items()) != 0 {
		t.Fatal("legacy pin resurrected")
	}
	after, _ := os.ReadFile(legacy)
	if !bytes.Equal(after, encoded) {
		t.Fatal("migration modified legacy backup")
	}
}

func TestCorruptHistoryBlocksOverwriteAndLegacyFallback(t *testing.T) {
	for _, corruptNew := range []bool{false, true} {
		t.Run(fmt.Sprint(corruptNew), func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			path, legacy := filepath.Join(dir, "history.json"), filepath.Join(dir, "pinned.json")
			badPath := legacy
			if corruptNew {
				badPath = path
			}
			bad := []byte(`{"version":999,"records":[]}`)
			if err := os.WriteFile(badPath, bad, 0600); err != nil {
				t.Fatal(err)
			}
			repo := storage.NewHistoryRepository(path, legacy)
			defer repo.Close()
			h := NewHistory(repo, 30)
			if err := h.Load(ctx); err == nil {
				t.Fatal("corrupt file loaded")
			}
			if _, _, err := h.Capture(ctx, clipmodel.TextData("new"), clipmodel.Source{}, nil); err == nil {
				t.Fatal("blocked history was overwritten")
			}
			after, _ := os.ReadFile(badPath)
			if !bytes.Equal(after, bad) {
				t.Fatal("corrupt file changed")
			}
			if !corruptNew {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("corrupt legacy created authoritative history")
				}
			}
		})
	}
}

func TestCaptureSaveFailureRetainsCompleteRecordAndRetries(t *testing.T) {
	repo := &memoryRepository{authoritative: true}
	h := loadedHistory(t, repo, 30)
	failure := errors.New("disk full")
	repo.saveErr = failure
	item, recorded, err := h.Capture(context.Background(), clipmodel.TextData("unsaved"), clipmodel.Source{}, nil)
	if !recorded || !errors.Is(err, failure) {
		t.Fatalf("capture: %v %v", recorded, err)
	}
	if record, ok := h.Record(item.ID); !ok || record.Item.Text != "unsaved" {
		t.Fatal("unsaved complete record lost")
	}
	repo.saveErr = nil
	if err = h.Persist(context.Background()); err != nil || len(repo.records) != 1 {
		t.Fatal("capture save did not retry")
	}
}

func TestQueriesDoNotWaitForPersistenceAndTransactionsPublishAfterAck(t *testing.T) {
	ctx := context.Background()
	repo := &memoryRepository{authoritative: true}
	h := loadedHistory(t, repo, 30)
	item, _, err := h.Capture(ctx, clipmodel.TextData("saved"), clipmodel.Source{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	repo.beforeSave = func() { close(entered); <-release }
	done := make(chan error, 1)
	go func() { done <- h.SetPinned(ctx, item.ID, true) }()
	<-entered
	queried := make(chan []clipmodel.Item, 1)
	go func() { queried <- h.Items() }()
	select {
	case items := <-queried:
		if items[0].Pinned {
			t.Fatal("pin published before persistence ack")
		}
	case <-time.After(time.Second):
		t.Fatal("history query waited for disk")
	}
	close(release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if !h.Items()[0].Pinned {
		t.Fatal("acknowledged pin not published")
	}
}

func TestUnifiedCountLimitsAndPins(t *testing.T) {
	ctx := context.Background()
	h := NewHistory(nil, 0)
	first, _, _ := h.Capture(ctx, clipmodel.TextData("pinned"), clipmodel.Source{}, nil)
	if err := h.SetPinned(ctx, first.ID, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 110; i++ {
		if _, _, err := h.Capture(ctx, clipmodel.TextData(fmt.Sprint(i)), clipmodel.Source{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(h.Items()) != 101 || len(h.Pinned()) != 1 {
		t.Fatal("count limit discarded pins or retained too many ordinary items")
	}
	if err := h.SetPinned(ctx, first.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(h.Items()) != 100 {
		t.Fatal("unpin did not use ordinary count limit")
	}
	if _, ok := h.Record(first.ID); ok {
		t.Fatal("evicted source retained data")
	}
}
