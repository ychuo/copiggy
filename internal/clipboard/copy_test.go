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
	after func(clipmodel.Representations, []string)
}

func (w *fakeWriter) Write(_ context.Context, data clipmodel.Representations, scope []string) (int64, error) {
	w.calls++
	w.data = data.Clone()
	w.scope = append([]string(nil), scope...)
	if w.after != nil {
		w.after(data, scope)
	}
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
func TestDraftCopyRejectsStaleSourceAndOnlyConsumesAfterWrite(t *testing.T) {
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
	if accepted.ID != "" || !reflect.DeepEqual(before, h.Items()) {
		t.Fatal("copying a draft added or changed history")
	}
	if _, err = h.ValidateDraft(draft.ID); !errors.Is(err, clipmodel.ErrDraftNotFound) {
		t.Fatal("successful draft copy did not consume its draft")
	}
	if _, ok := h.Current(); ok {
		t.Fatal("formatted clipboard was incorrectly identified as its unformatted source")
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

func TestConvertedCopiesPreserveSavedHistoryWithoutPersistence(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		format clipmodel.CopyFormat
		want   string
	}{
		{"#f00", clipmodel.ColorHEX, "#FF0000"},
		{"#FF000080", clipmodel.ColorRGB, "rgba(255, 0, 0, 0.502)"},
		{"#FF0000", clipmodel.ColorHSL, "hsl(0, 100%, 50%)"},
		{"1792645800123", clipmodel.TimestampSeconds, "1792645800"},
		{"1792645800", clipmodel.TimestampMilliseconds, "1792645800000"},
		{"+1 (202) 555-0148", clipmodel.PhoneDialing, "+12025550148"},
		{`{"x":1}`, "", "{\n  \"x\": 1\n}"},
	} {
		t.Run(string(tc.format)+tc.raw, func(t *testing.T) {
			ctx := context.Background()
			repo := &memoryRepository{authoritative: true}
			h := loadedHistory(t, repo, 30)
			original := testData(testItem(testRep(clipmodel.TextFormat, []byte(tc.raw)), testRep(clipmodel.HTMLFormat, []byte("<b>"+tc.raw+"</b>"))))
			source, _, err := h.Capture(ctx, original, clipmodel.Source{Name: "Editor", BundleID: "editor.app"}, []string{"editor.app"})
			if err != nil {
				t.Fatal(err)
			}
			if err = h.SetPinned(ctx, source.ID, true); err != nil {
				t.Fatal(err)
			}
			before, saves := h.Records(), repo.saves
			repo.saveErr = errors.New("saving should not be attempted")
			w := &fakeWriter{after: func(data clipmodel.Representations, scope []string) {
				data[0][0].Bytes[0] = 'X'
				scope[0] = "other.app"
			}}
			copier := Copy{History: h, Port: w}
			var ack clipmodel.Item
			var revision int64
			if tc.format == "" {
				draft, draftErr := h.Enhance(ctx, source.ID, clipmodel.FormatJSON)
				if draftErr != nil {
					t.Fatal(draftErr)
				}
				ack, revision, err = copier.Draft(ctx, draft.ID)
			} else {
				ack, revision, err = copier.Variant(ctx, source.ID, tc.format)
			}
			if err != nil || revision != 42 || ack.ID != "" || ack.ParentID != source.ID || ack.Text != tc.want || ack.SourceAppID != "editor.app" {
				t.Fatalf("incorrect conversion acknowledgement: %+v, revision=%d err=%v", ack, revision, err)
			}
			if !reflect.DeepEqual(w.data, clipmodel.TextData(tc.want)) || !reflect.DeepEqual(w.scope, []string{"editor.app"}) || !reflect.DeepEqual(ack.AllowedApps, w.scope) {
				t.Fatal("conversion wrote altered data or used an untrusted scope")
			}
			if !reflect.DeepEqual(before, h.Records()) || repo.saves != saves {
				t.Fatal("conversion changed saved bytes, history metadata, order or persistence")
			}
			if _, ok := h.Current(); ok {
				t.Fatal("derived text with no exact saved representation remained current")
			}
		})
	}
}

func TestVariantCopyRejectsMissingInvalidCanceledAndFailedWrites(t *testing.T) {
	ctx := context.Background()
	h := NewHistory(nil, 30)
	source, _, _ := h.Capture(ctx, clipmodel.TextData("#f00"), clipmodel.Source{}, []string{"editor.app"})
	w := &fakeWriter{}
	copier := Copy{History: h, Port: w}
	before := h.Records()
	if _, _, err := copier.Variant(ctx, "missing", clipmodel.ColorRGB); !errors.Is(err, clipmodel.ErrItemNotFound) {
		t.Fatal("missing source accepted")
	}
	for _, format := range []clipmodel.CopyFormat{"unknown", clipmodel.PhoneDialing} {
		if _, _, err := copier.Variant(ctx, source.ID, format); !errors.Is(err, clipmodel.ErrInvalidCopyFormat) {
			t.Fatal("unsupported format accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := copier.Variant(canceled, source.ID, clipmodel.ColorRGB); !errors.Is(err, context.Canceled) || w.calls != 0 {
		t.Fatal("rejected or canceled conversion wrote clipboard")
	}
	w.err = errors.New("write failed")
	if _, _, err := copier.Variant(ctx, source.ID, clipmodel.ColorRGB); !errors.Is(err, w.err) || w.calls != 1 {
		t.Fatal("failed write did not propagate")
	}
	current, ok := h.Current()
	if !ok || current.ID != source.ID || !reflect.DeepEqual(before, h.Records()) {
		t.Fatal("failed conversion changed history or current")
	}
}

func TestConvertedCopyCurrentRequiresExactRepresentationsAndPolicy(t *testing.T) {
	for _, rich := range []bool{false, true} {
		for _, sameScope := range []bool{false, true} {
			ctx := context.Background()
			h := NewHistory(nil, 30)
			data := clipmodel.TextData("#FF0000")
			if rich {
				data[0] = append(data[0], testRep(clipmodel.HTMLFormat, []byte("<b>#FF0000</b>")))
			}
			scope := []string{"second.app", "editor.app"}
			if !sameScope {
				scope = nil
			}
			match, _, _ := h.Capture(ctx, data, clipmodel.Source{}, scope)
			source, _, _ := h.Capture(ctx, clipmodel.TextData("#f00"), clipmodel.Source{}, []string{"editor.app", "second.app"})
			before := h.Records()
			ack, _, err := (Copy{History: h, Port: &fakeWriter{}}).Variant(ctx, source.ID, clipmodel.ColorHEX)
			current, ok := h.Current()
			wantCurrent := !rich && sameScope
			if err != nil || ok != wantCurrent || wantCurrent && (current.ID != match.ID || ack.ID != match.ID) {
				t.Fatalf("incorrect current matching rich=%t scope=%t ack=%+v current=%+v err=%v", rich, sameScope, ack, current, err)
			}
			if !reflect.DeepEqual(before, h.Records()) {
				t.Fatal("selecting an exact copied representation altered history")
			}
		}
	}
}

func TestSuccessfulConvertedWriteAcknowledgesRevisionAfterCancellation(t *testing.T) {
	for _, draftCopy := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		h := NewHistory(nil, 30)
		raw := "#f00"
		if draftCopy {
			raw = `{"x":1}`
		}
		source, _, _ := h.Capture(ctx, clipmodel.TextData(raw), clipmodel.Source{}, nil)
		w := &fakeWriter{after: func(clipmodel.Representations, []string) { cancel() }}
		copier := Copy{History: h, Port: w}
		var revision int64
		var err error
		if draftCopy {
			draft, draftErr := h.Enhance(ctx, source.ID, clipmodel.FormatJSON)
			if draftErr != nil {
				t.Fatal(draftErr)
			}
			_, revision, err = copier.Draft(ctx, draft.ID)
			if _, validateErr := h.ValidateDraft(draft.ID); !errors.Is(validateErr, clipmodel.ErrDraftNotFound) {
				t.Fatal("successful write left its draft pending")
			}
		} else {
			_, revision, err = copier.Variant(ctx, source.ID, clipmodel.ColorHEX)
		}
		if err != nil || revision != 42 {
			t.Fatalf("successful write revision was lost to cancellation: %d, %v", revision, err)
		}
	}
}

func TestDraftCopyRejectsChangedPolicyAndCancellationBeforeWrite(t *testing.T) {
	for _, changeScope := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		h := NewHistory(nil, 30)
		source, _, _ := h.Capture(ctx, clipmodel.TextData(`{"x":1}`), clipmodel.Source{}, []string{"editor.app"})
		draft, err := h.Enhance(ctx, source.ID, clipmodel.FormatJSON)
		if err != nil {
			t.Fatal(err)
		}
		wantErr := context.Canceled
		if changeScope {
			if err = h.SetAllowedApps(ctx, source.ID, []string{"other.app"}); err != nil {
				t.Fatal(err)
			}
			wantErr = clipmodel.ErrDraftStale
		} else {
			cancel()
		}
		before := h.Records()
		w := &fakeWriter{}
		if _, _, err = (Copy{History: h, Port: w}).Draft(ctx, draft.ID); !errors.Is(err, wantErr) || w.calls != 0 {
			t.Fatalf("invalid draft reached native writer: %v, calls=%d", err, w.calls)
		}
		_, draftErr := h.ValidateDraft(draft.ID)
		if errors.Is(draftErr, clipmodel.ErrDraftNotFound) || !changeScope && draftErr != nil {
			t.Fatal("rejected write consumed its draft")
		}
		current, ok := h.Current()
		if !ok || current.ID != source.ID || !reflect.DeepEqual(before, h.Records()) {
			t.Fatal("rejected draft changed history or current")
		}
		cancel()
	}
}
