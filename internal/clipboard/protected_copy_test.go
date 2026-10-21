package clipboard

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"copiggy/internal/clipmodel"
)

type confirmedCopyCall struct {
	id       uint64
	revision int64
}

type canceledCopyCall struct {
	id      uint64
	message string
}

type fakeProtectedCopyBridge struct {
	request       CopyRequest
	pending       bool
	frontmost     SourceApp
	revision      int64
	confirmResult bool
	confirmErr    error
	onConfirm     func()
	confirmed     []confirmedCopyCall
	canceled      []canceledCopyCall
}

func newFakeProtectedCopyBridge() *fakeProtectedCopyBridge {
	source := SourceApp{BundleID: "com.source.editor", Name: "Source Editor", PID: 123}
	return &fakeProtectedCopyBridge{
		request: CopyRequest{
			ID: 9, Source: source, BaselineVersion: 41,
		},
		pending: true, frontmost: source, revision: 42, confirmResult: true,
	}
}

func (f *fakeProtectedCopyBridge) PendingCopy() (CopyRequest, bool) {
	return f.request, f.pending
}

func (f *fakeProtectedCopyBridge) Frontmost() SourceApp { return f.frontmost }
func (f *fakeProtectedCopyBridge) ChangeCount() int64   { return f.revision }

func (f *fakeProtectedCopyBridge) CancelCopy(id uint64, message string) bool {
	f.canceled = append(f.canceled, canceledCopyCall{id: id, message: message})
	if !f.pending || id != f.request.ID {
		return false
	}
	f.pending = false
	return true
}

func (f *fakeProtectedCopyBridge) ConfirmCopy(id uint64, revision int64) (bool, error) {
	f.confirmed = append(f.confirmed, confirmedCopyCall{id: id, revision: revision})
	if f.onConfirm != nil {
		f.onConfirm()
	}
	if f.confirmResult && f.confirmErr == nil {
		f.pending = false
	}
	return f.confirmResult, f.confirmErr
}

func protectedCopyText(text string) clipmodel.Representations {
	return testData(testItem(testRep(clipmodel.TextFormat, []byte(text))))
}

func assertCopyStoreUnchanged(t *testing.T, store *fixtureHistory, before []clipmodel.Item, currentID string) {
	t.Helper()
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("unsuccessful protected copy changed history")
	}
	current, ok := store.Current()
	if !ok || current.ID != currentID {
		t.Fatalf("unsuccessful protected copy changed current: %+v, exists=%v", current, ok)
	}
}

func TestCompleteProtectedCopyCapturesOnlyAfterNativeConfirmation(t *testing.T) {
	store := newFixtureHistory()
	old, _ := store.Capture("previous clipboard", "Previous App", false)
	before := store.Items()
	bridge := newFakeProtectedCopyBridge()
	bridge.onConfirm = func() {
		assertCopyStoreUnchanged(t, store, before, old.ID)
	}
	readCalls := 0
	captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		readCalls++
		return protectedCopyText("new protected content"), nil
	}, store)
	if !ok || readCalls != 1 {
		t.Fatalf("protected copy was not captured exactly once: ok=%v reads=%d", ok, readCalls)
	}
	wantConfirmation := []confirmedCopyCall{{id: bridge.request.ID, revision: 42}}
	if !reflect.DeepEqual(bridge.confirmed, wantConfirmation) || len(bridge.canceled) != 0 {
		t.Fatalf("unexpected native transaction: confirmed=%+v canceled=%+v", bridge.confirmed, bridge.canceled)
	}
	if captured.Source != bridge.request.Source || captured.Revision != 42 ||
		captured.Item.Text != "new protected content" || captured.Item.SourceApp != bridge.request.Source.Name ||
		captured.Item.SourceAppID != bridge.request.Source.BundleID ||
		!reflect.DeepEqual(captured.Item.AllowedApps, []string{bridge.request.Source.BundleID}) {
		t.Fatalf("captured source, content or policy is incorrect: %+v", captured)
	}
	current, exists := store.Current()
	if !exists || current.ID != captured.Item.ID || current.Allows("com.other.app") ||
		!current.Allows(bridge.request.Source.BundleID) {
		t.Fatalf("new current record was not protected from creation: %+v", current)
	}
	if items := store.Items(); len(items) != 2 || items[1].ID != old.ID || len(items[1].AllowedApps) != 0 {
		t.Fatalf("protected copy altered the previous record: %+v", items)
	}
}

func TestCompleteProtectedCopyDuplicateRefreshesOnlyAfterConfirmation(t *testing.T) {
	store := newFixtureHistory()
	bridge := newFakeProtectedCopyBridge()
	read := func() (clipmodel.Representations, error) { return protectedCopyText("protected duplicate"), nil }
	first, ok := completeProtectedCopy(bridge, read, store)
	if !ok {
		t.Fatal("initial protected copy was not recorded")
	}
	ordinary, _ := store.Capture(first.Item.Text, "Other App", false)
	before := store.Items()
	bridge = newFakeProtectedCopyBridge()
	bridge.request.ID++
	bridge.request.BaselineVersion = 43
	bridge.revision = 44
	bridge.onConfirm = func() {
		assertCopyStoreUnchanged(t, store, before, ordinary.ID)
	}
	repeated, ok := completeProtectedCopy(bridge, read, store)
	if !ok || repeated.Item.ID != first.Item.ID || repeated.Revision != 44 || len(store.Items()) != 2 {
		t.Fatalf("confirmed duplicate was not refreshed in place: %+v, %v", repeated, ok)
	}
	if repeated.Item.SourceAppID != first.Item.SourceAppID || repeated.Item.Allows("com.other.app") ||
		!repeated.Item.CreatedAt.After(first.Item.CreatedAt) {
		t.Fatalf("duplicate lost source/policy or kept its previous time: %+v", repeated.Item)
	}
	current, exists := store.Current()
	if !exists || current.ID != first.Item.ID || store.Items()[0].ID != first.Item.ID {
		t.Fatal("confirmed protected recopy did not become the newest current record")
	}
}

func TestCompleteProtectedCopyNoNewVersionDoesNotReadOrLockOldContent(t *testing.T) {
	store := newFixtureHistory()
	old, _ := store.Capture("old unrelated text", "Other App", false)
	before := store.Items()
	bridge := newFakeProtectedCopyBridge()
	bridge.revision = bridge.request.BaselineVersion
	readCalls := 0
	if _, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		readCalls++
		return protectedCopyText("must not be read"), nil
	}, store); ok {
		t.Fatal("unchanged clipboard was treated as a protected copy")
	}
	if readCalls != 0 || len(bridge.confirmed) != 0 || len(bridge.canceled) != 0 || !bridge.pending {
		t.Fatalf("unchanged copy should remain pending without read or confirmation: reads=%d bridge=%+v", readCalls, bridge)
	}
	assertCopyStoreUnchanged(t, store, before, old.ID)
}

func TestCompleteProtectedCopySourceChangeCancelsBeforeRead(t *testing.T) {
	for _, change := range []string{"bundle", "process"} {
		t.Run(change, func(t *testing.T) {
			store := newFixtureHistory()
			old, _ := store.Capture("old", "Other App", false)
			before := store.Items()
			bridge := newFakeProtectedCopyBridge()
			if change == "bundle" {
				bridge.frontmost.BundleID = "com.other.app"
			} else {
				bridge.frontmost.PID++
			}
			readCalls := 0
			if _, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
				readCalls++
				return protectedCopyText("new"), nil
			}, store); ok {
				t.Fatal("copy from a changed source was captured")
			}
			if readCalls != 0 || len(bridge.confirmed) != 0 || len(bridge.canceled) != 1 ||
				bridge.canceled[0].id != bridge.request.ID || bridge.canceled[0].message == "" || bridge.pending {
				t.Fatalf("changed source was not canceled before reading: reads=%d bridge=%+v", readCalls, bridge)
			}
			assertCopyStoreUnchanged(t, store, before, old.ID)
		})
	}
}

func TestCompleteProtectedCopyUnavailableDataCanRetry(t *testing.T) {
	for _, scenario := range []struct {
		name string
		data clipmodel.Representations
		err  error
	}{
		{name: "read error", err: errors.New("data not ready")},
		{name: "empty text", data: protectedCopyText("")},
		{name: "empty HTML", data: testData(testItem(testRep(clipmodel.HTMLFormat, []byte("<b></b>"))))},
		{name: "text provider error", err: errors.New("representation not ready")},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := newFixtureHistory()
			old, _ := store.Capture("old", "Other App", false)
			before := store.Items()
			bridge := newFakeProtectedCopyBridge()
			if _, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
				return scenario.data, scenario.err
			}, store); ok {
				t.Fatal("unavailable data was captured")
			}
			if len(bridge.confirmed) != 0 || len(bridge.canceled) != 0 || !bridge.pending {
				t.Fatalf("temporary unavailability consumed the pending copy: %+v", bridge)
			}
			assertCopyStoreUnchanged(t, store, before, old.ID)
			captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
				return protectedCopyText("ready on retry"), nil
			}, store)
			if !ok || captured.Item.Text != "ready on retry" || len(bridge.confirmed) != 1 {
				t.Fatalf("pending copy did not recover on retry: captured=%+v ok=%v", captured, ok)
			}
		})
	}
}

func TestCompleteProtectedHTMLKeepsOriginalDataAndScope(t *testing.T) {
	store := newFixtureHistory()
	bridge := newFakeProtectedCopyBridge()
	markup := "<b>Protected HTML</b>"
	data := testData(testItem(testRep(clipmodel.HTMLFormat, []byte(markup))))
	captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) { return data, nil }, store)
	if !ok || captured.Item.Text != "Protected HTML" || !captured.Item.Allows(bridge.request.Source.BundleID) || captured.Item.Allows("another.app") {
		t.Fatalf("HTML fallback lost its text or source restriction: %+v", captured)
	}
	got, err := captured.Data.Read(clipmodel.HTMLFormat)
	if err != nil || string(got) != markup || len(bridge.confirmed) != 1 {
		t.Fatal("protected HTML did not preserve its exact representation")
	}
}

func TestCompleteProtectedCopyRevisionChangesDuringReadAreNotConfirmed(t *testing.T) {
	store := newFixtureHistory()
	old, _ := store.Capture("old", "Other App", false)
	before := store.Items()
	bridge := newFakeProtectedCopyBridge()
	if _, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		bridge.revision++
		return protectedCopyText("raced content"), nil
	}, store); ok {
		t.Fatal("unstable clipboard version was captured")
	}
	if len(bridge.confirmed) != 0 || len(bridge.canceled) != 0 || !bridge.pending {
		t.Fatalf("unstable read was confirmed or consumed: %+v", bridge)
	}
	assertCopyStoreUnchanged(t, store, before, old.ID)
}

func TestCompleteProtectedCopyRejectedConfirmationDoesNotRecord(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		result bool
		err    error
	}{
		{name: "false result"},
		{name: "error", result: true, err: errors.New("confirmation failed")},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := newFixtureHistory()
			old, _ := store.Capture("old", "Other App", false)
			before := store.Items()
			bridge := newFakeProtectedCopyBridge()
			bridge.confirmResult, bridge.confirmErr = scenario.result, scenario.err
			if _, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
				return protectedCopyText("not confirmed"), nil
			}, store); ok {
				t.Fatal("rejected native confirmation still created a protected item")
			}
			if len(bridge.confirmed) != 1 {
				t.Fatalf("expected one attempted confirmation, got %d", len(bridge.confirmed))
			}
			assertCopyStoreUnchanged(t, store, before, old.ID)
		})
	}
}

func TestCompleteProtectedCopyTextSizeBoundary(t *testing.T) {
	for _, size := range []int{1 << 20, (1 << 20) + 1} {
		t.Run(map[bool]string{true: "exact limit", false: "over limit"}[size == 1<<20], func(t *testing.T) {
			store := newFixtureHistory()
			old, _ := store.Capture("old", "Other App", false)
			before := store.Items()
			bridge := newFakeProtectedCopyBridge()
			captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
				return protectedCopyText(strings.Repeat("a", size)), nil
			}, store)
			if size == 1<<20 {
				if !ok || len(captured.Item.Text) != size || len(bridge.confirmed) != 1 || len(bridge.canceled) != 0 {
					t.Fatal("text exactly at the limit should be captured")
				}
				return
			}
			if ok || len(bridge.confirmed) != 0 || len(bridge.canceled) != 1 ||
				bridge.canceled[0].id != bridge.request.ID || bridge.canceled[0].message == "" {
				t.Fatalf("oversized text was not rejected before confirmation: ok=%v bridge=%+v", ok, bridge)
			}
			assertCopyStoreUnchanged(t, store, before, old.ID)
		})
	}
}

func TestCompleteProtectedCopyLaterReplacementKeepsHistoryWithoutMarkingCurrent(t *testing.T) {
	store := newFixtureHistory()
	old, _ := store.Capture("old", "Other App", false)
	bridge := newFakeProtectedCopyBridge()
	confirmedVersion := bridge.revision
	bridge.onConfirm = func() { bridge.revision++ }
	captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		return protectedCopyText("protected before replacement"), nil
	}, store)
	if !ok || captured.Revision != confirmedVersion {
		t.Fatalf("successfully confirmed history was lost: captured=%+v ok=%v", captured, ok)
	}
	if _, exists := store.Current(); exists {
		t.Fatal("replaced protected value was incorrectly marked current")
	}
	items := store.Items()
	if len(items) != 2 || items[0].ID != captured.Item.ID || items[1].ID != old.ID ||
		items[0].Allows("com.other.app") || !items[0].Allows(bridge.request.Source.BundleID) {
		t.Fatalf("confirmed protected history was not retained with its scope: %+v", items)
	}
}

func TestCompleteProtectedCopyPreservesOriginalRichRepresentations(t *testing.T) {
	store := newFixtureHistory()
	bridge := newFakeProtectedCopyBridge()
	html := []byte(`<p style="font-weight: bold">Original rich content</p>`)
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	data := testData(testItem(
		testRep(clipmodel.TextFormat, []byte("Original rich content")),
		testRep(clipmodel.HTMLFormat, html),
		testRep(clipmodel.PNGFormat, png),
	))
	captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		return data, nil
	}, store)
	if !ok {
		t.Fatal("rich clipboard content was not captured")
	}
	for _, representation := range []struct {
		format string
		bytes  []byte
	}{
		{clipmodel.TextFormat, []byte("Original rich content")},
		{clipmodel.HTMLFormat, html},
		{clipmodel.PNGFormat, png},
	} {
		got, err := captured.Data.Read(representation.format)
		if err != nil || !bytes.Equal(got, representation.bytes) {
			t.Fatalf("representation %q changed: got=%q err=%v", representation.format, got, err)
		}
	}
	if captured.Item.Text != "Original rich content" {
		t.Fatalf("history display text is incorrect: %q", captured.Item.Text)
	}
}

func testRep(format string, data []byte) clipmodel.Representation {
	return clipmodel.Representation{Format: format, Bytes: data}
}
func testItem(reps ...clipmodel.Representation) []clipmodel.Representation   { return reps }
func testData(items ...[]clipmodel.Representation) clipmodel.Representations { return items }

type fixtureHistory struct{ *History }

func newFixtureHistory() *fixtureHistory { return &fixtureHistory{NewHistory(nil, 0)} }
func (f *fixtureHistory) Capture(text, source string, own bool) (clipmodel.Item, bool) {
	if own {
		item, _ := f.Current()
		return item, false
	}
	if text == "" {
		f.ClearCurrent()
		return clipmodel.Item{}, false
	}
	item, ok, err := f.History.Capture(context.Background(), clipmodel.TextData(text), clipmodel.Source{Name: source}, nil)
	if err != nil {
		panic(err)
	}
	return item, ok
}
func completeProtectedCopy(p ProtectedCopyPort, read func() (clipmodel.Representations, error), h *fixtureHistory) (CapturedCopy, bool) {
	return CompleteProtectedCopy(context.Background(), p, read, h.History)
}
