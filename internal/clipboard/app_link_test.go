package clipboard

import (
	"context"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
)

func TestLegacyAppLinkHistoryReclassifiesWithoutAuthorityAndCopiesExactSavedRepresentations(t *testing.T) {
	for _, raw := range []string{"  aweme://video/123?q=one#part \n", "[Open route](sslocal://route?url=https%3A%2F%2Fexample.com)"} {
		t.Run(raw, func(t *testing.T) {
			ctx := context.Background()
			data := clipmodel.Representations{{
				{Format: clipmodel.TextFormat, Bytes: []byte(raw)},
				{Format: clipmodel.HTMLFormat, Bytes: []byte("<b>Original copied content</b>")},
			}}
			old := historyRecord(t, "item-17", raw, time.Now().Add(-time.Hour), true, []string{"editor.app"}, data)
			old.Item.Type = clipmodel.TypeLink
			repo := &memoryRepository{records: []clipmodel.Record{old}, authoritative: true}
			history := loadedHistory(t, repo, 0)
			if _, current := history.Current(); current {
				t.Fatal("reclassification acquired clipboard authority")
			}
			saved, ok := history.Record(old.Item.ID)
			if !ok || saved.Item.Type != clipmodel.TypeAppLink || saved.Item.Text != raw || !reflect.DeepEqual(saved.Data, data) {
				t.Fatalf("old app link was not reclassified intact: %+v", saved.Item)
			}
			before := history.Records()
			saved.Item.Text = "aweme://forged"
			saved.Item.AllowedApps = nil
			saved.Data[0][0].Bytes[0] = 'X'
			writer := &fakeWriter{}
			item, revision, err := (Copy{History: history, Port: writer}).Item(ctx, old.Item.ID)
			if err != nil || revision != 42 || item.Type != clipmodel.TypeAppLink || !reflect.DeepEqual(writer.data, data) || !reflect.DeepEqual(writer.scope, old.Item.AllowedApps) || !reflect.DeepEqual(before, history.Records()) {
				t.Fatalf("application copy lost saved authority or exact source: %+v, %v", item, err)
			}
			repeated, recorded, err := history.Capture(ctx, data, clipmodel.Source{Name: "Other", BundleID: "other.app"}, old.Item.AllowedApps)
			if err != nil || !recorded || repeated.ID != old.Item.ID || repeated.Type != clipmodel.TypeAppLink || repeated.SourceApp != old.Item.SourceApp || repeated.SourceAppID != old.Item.SourceAppID || len(history.Items()) != 1 {
				t.Fatalf("recapture duplicated migrated application link or changed provenance: %+v, %v", repeated, err)
			}
			ordinary, _, err := history.Capture(ctx, data, clipmodel.Source{Name: "Other"}, nil)
			if err != nil || ordinary.ID == repeated.ID || ordinary.Type != clipmodel.TypeAppLink || len(history.Items()) != 2 {
				t.Fatal("application links with different protection scopes merged")
			}
		})
	}
}
