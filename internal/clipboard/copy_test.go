package clipboard

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"copiggy/internal/clipmodel"
)

type fakeWriter struct {
	data  clipmodel.Representations
	scope []string
	calls int
	err   error
}

func (w *fakeWriter) Write(_ context.Context, data clipmodel.Representations, scope []string) (int64, error) {
	w.calls++
	w.data = data.Clone()
	w.scope = append([]string(nil), scope...)
	return 42, w.err
}
func TestCopyUsesTrustedStoredBytesAndScope(t *testing.T) {
	ctx := context.Background()
	h := NewHistory(nil, 0)
	original := testData(testItem(testRep(clipmodel.TextFormat, []byte("original")), testRep(clipmodel.HTMLFormat, []byte("<b>original</b>"))))
	item, _, err := h.Capture(ctx, original, clipmodel.Source{}, []string{"editor.app"})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := h.Record(item.ID)
	r.Data[0][0].Bytes[0] = 'X'
	r.Item.AllowedApps = nil
	w := &fakeWriter{}
	copier := Copy{History: h, Port: w}
	got, rev, err := copier.Item(ctx, item.ID)
	if err != nil || rev != 42 || got.ID != item.ID || !reflect.DeepEqual(w.data, original) || !reflect.DeepEqual(w.scope, []string{"editor.app"}) {
		t.Fatalf("untrusted write: %+v %v", got, err)
	}
	if _, _, err = copier.Item(ctx, "missing"); !errors.Is(err, clipmodel.ErrItemNotFound) || w.calls != 1 {
		t.Fatal("missing item wrote clipboard")
	}
}
func TestDraftCopyRejectsStaleSourceAndOnlyAcceptsAfterWrite(t *testing.T) {
	ctx := context.Background()
	h := NewHistory(nil, 0)
	source, _, _ := h.Capture(ctx, clipmodel.TextData(`{"x":1}`), clipmodel.Source{Name: "Editor"}, []string{"editor.app"})
	draft, err := h.Enhance(ctx, source.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	draft.Text = "UI substituted text"
	draft.AllowedApps = nil
	w := &fakeWriter{err: errors.New("write failed")}
	copier := Copy{History: h, Port: w}
	before := h.Items()
	if _, _, err = copier.Draft(ctx, draft.ID); err == nil || !reflect.DeepEqual(before, h.Items()) {
		t.Fatal("failed write accepted draft")
	}
	if _, err = h.ValidateDraft(draft.ID); err != nil {
		t.Fatal("failed write consumed draft")
	}
	w.err = nil
	accepted, _, err := copier.Draft(ctx, draft.ID)
	if err != nil || accepted.ParentID != source.ID || accepted.Text == draft.Text || !reflect.DeepEqual(w.scope, []string{"editor.app"}) {
		t.Fatalf("trusted draft not used: %+v %v", accepted, err)
	}
	stale, err := h.Enhance(ctx, source.ID, clipmodel.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Delete(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	calls := w.calls
	if _, _, err = copier.Draft(ctx, stale.ID); !errors.Is(err, clipmodel.ErrDraftStale) || w.calls != calls {
		t.Fatal("stale draft wrote clipboard")
	}
}
