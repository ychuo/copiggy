package clipboard

import (
	"context"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/contentactions"
)

func TestCapturedPhoneContextSurvivesPreferenceChangesAndCopy(t *testing.T) {
	ctx := context.Background()
	repo := &memoryRepository{authoritative: true}
	h := loadedHistory(t, repo, 0)
	h.SetRecognitionRegion("CN")
	original := clipmodel.TextData("１３８　１２３４–５６７８")
	source := clipmodel.Source{Name: "Editor", BundleID: "editor.app"}
	item, _, err := h.Capture(ctx, original, source, []string{"editor.app"})
	if err != nil || item.Type != clipmodel.TypePhone || item.RecognitionRegion != "CN" {
		t.Fatalf("national phone capture: %+v %v", item, err)
	}
	h.SetRecognitionRegion("US")
	repeated, _, err := h.Capture(ctx, original, clipmodel.Source{Name: "Other"}, []string{"editor.app"})
	if err != nil || repeated.ID != item.ID || repeated.Type != item.Type || repeated.RecognitionRegion != "CN" || repeated.SourceApp != source.Name || len(h.Items()) != 1 {
		t.Fatalf("region change split or reinterpreted exact duplicate: %+v %v", repeated, err)
	}
	w := &fakeWriter{}
	copy := Copy{History: h, Port: w}
	if _, _, err := copy.Item(ctx, item.ID); err != nil || !reflect.DeepEqual(w.data, original) || !reflect.DeepEqual(w.scope, []string{"editor.app"}) {
		t.Fatalf("original bytes or policy changed: %v", err)
	}
	if _, _, err := copy.Variant(ctx, item.ID, clipmodel.PhoneDialing); err != nil {
		t.Fatal(err)
	}
	text, _ := w.data.Read(clipmodel.TextFormat)
	if string(text) != "13812345678" || !reflect.DeepEqual(w.scope, []string{"editor.app"}) || len(h.Items()) != 1 {
		t.Fatal("dialing variant lost context, scope, or created history")
	}
	restored := NewHistory(repo, 0)
	restored.SetRecognitionRegion("JP")
	if err := restored.Load(ctx); err != nil {
		t.Fatal(err)
	}
	record, _ := restored.Record(item.ID)
	target, err := contentactions.OpenItemTarget(record.Item)
	if err != nil || target.URL != "tel:+8613812345678" || record.Item.RecognitionRegion != "CN" || !reflect.DeepEqual(record.Data, original) {
		t.Fatalf("restore changed captured phone interpretation: %+v %v", target, err)
	}
	if _, ok := restored.Current(); ok {
		t.Fatal("restore selected a record")
	}
	// A different scope is still a separate record, using the current region.
	other, _, err := h.Capture(ctx, original, source, nil)
	if err != nil || other.ID == item.ID || other.RecognitionRegion != "US" || other.Type != clipmodel.TypeText {
		t.Fatalf("different protection scope merged: %+v %v", other, err)
	}
}

func TestLegacyHistoryGainsOnePersistedRecognitionContext(t *testing.T) {
	ctx := context.Background()
	legacy := historyRecord(t, "item-9", "13812345678", time.Now(), true, []string{"editor.app"}, nil)
	repo := &memoryRepository{authoritative: true, records: []clipmodel.Record{legacy}}
	h := NewHistory(repo, 0)
	h.SetRecognitionRegion("CN")
	if err := h.Load(ctx); err != nil {
		t.Fatal(err)
	}
	record, _ := h.Record(legacy.Item.ID)
	if record.Item.Type != clipmodel.TypePhone || record.Item.RecognitionRegion != "CN" || record.Item.CreatedAt != legacy.Item.CreatedAt || !record.Item.Pinned || !reflect.DeepEqual(record.Data, legacy.Data) || repo.saves != 1 {
		t.Fatalf("legacy context migration changed provenance or was not saved: %+v", record.Item)
	}
	h = NewHistory(repo, 0)
	h.SetRecognitionRegion("US")
	if err := h.Load(ctx); err != nil {
		t.Fatal(err)
	}
	record, _ = h.Record(legacy.Item.ID)
	if record.Item.Type != clipmodel.TypePhone || record.Item.RecognitionRegion != "CN" || repo.saves != 1 {
		t.Fatal("new default reinterpreted or remigrated saved history")
	}
}

func TestLegacyGroupedPhonesKeepTheirExistingInterpretation(t *testing.T) {
	for _, tc := range []struct{ text, region string }{
		{"(202) 555-0148", "US"}, {"020 7946 0958", "GB"},
		{"00 1 202 555 0148", "GB"}, {"011 44 20 7946 0958", "US"},
	} {
		legacy := historyRecord(t, "item-1", tc.text, time.Now(), false, nil, nil)
		repo := &memoryRepository{authoritative: true, records: []clipmodel.Record{legacy}}
		h := NewHistory(repo, 0)
		h.SetRecognitionRegion("CN")
		if err := h.Load(context.Background()); err != nil {
			t.Fatal(err)
		}
		item := h.Items()[0]
		if item.Type != clipmodel.TypePhone || item.RecognitionRegion != tc.region {
			t.Fatalf("legacy phone reinterpreted with system region: %+v", item)
		}
		if got, want := clipmodel.AnalyzeItem(item).Phone, clipmodel.Analyze(tc.text).Phone; got == nil || got.E164 != want.E164 {
			t.Fatalf("legacy phone changed destination: %+v", got)
		}
	}
}

func TestPhoneVariantDoesNotAdoptAnotherRecordsRegion(t *testing.T) {
	ctx := context.Background()
	h := NewHistory(nil, 0)
	h.SetRecognitionRegion("US")
	_, _, _ = h.Capture(ctx, clipmodel.TextData("13812345678"), clipmodel.Source{}, nil)
	h.SetRecognitionRegion("CN")
	source, _, err := h.Capture(ctx, clipmodel.TextData("138 1234 5678"), clipmodel.Source{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	copy := Copy{History: h, Port: &fakeWriter{}}
	ack, _, err := copy.Variant(ctx, source.ID, clipmodel.PhoneDialing)
	if err != nil || ack.ID != "" || ack.RecognitionRegion != "CN" || ack.Type != clipmodel.TypePhone || len(h.Items()) != 2 {
		t.Fatalf("derived result adopted a different region: %+v %v", ack, err)
	}
}

func TestLegacyTimestampKeepsItsInterpretationInPhoneRegion(t *testing.T) {
	legacy := historyRecord(t, "item-1", "2025550148", time.Now(), false, nil, nil)
	repo := &memoryRepository{authoritative: true, records: []clipmodel.Record{legacy}}
	h := NewHistory(repo, 0)
	h.SetRecognitionRegion("US")
	if err := h.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	item := h.Items()[0]
	if item.Type != clipmodel.TypeTimestamp || item.RecognitionRegion != "ZZ" || clipmodel.AnalyzeItem(item).Timestamp == nil {
		t.Fatalf("legacy timestamp lost its interpretation: %+v", item)
	}
	// The same new value is ambiguous with US phone metadata, but migration
	// must not remove the old record's established timestamp presentation.
	if clipmodel.AnalyzeInRegion(item.Text, "US").Type != clipmodel.TypeText {
		t.Fatal("fixture is no longer ambiguous in the current numbering plan")
	}
}
