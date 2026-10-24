package clipboard

import (
	"context"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
	"copiggy/internal/storage"
)

func TestURIOnlyHistoryRestoresAndCopiesExactDataWithSavedProtection(t *testing.T) {
	for _, tc := range []struct {
		name, raw, text string
		kind            clipmodel.ContentType
	}{
		{"single", "# Browser URL\r\nhttps://example.com/a?q=1#section\r\n", "https://example.com/a?q=1#section", clipmodel.TypeLink},
		{"multiple", "https://example.com/a\r\nhttps://example.com/b\r\n", "https://example.com/a\nhttps://example.com/b", clipmodel.TypeText},
		{"application", "# Video URL\r\naweme://video/123?q=one\r\n", "aweme://video/123?q=one", clipmodel.TypeAppLink},
		{"web and application", "https://example.com/a\r\nsslocal://route?q=one\r\n", "https://example.com/a\nsslocal://route?q=one", clipmodel.TypeText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			data := clipmodel.Representations{{{Format: clipmodel.URIListFormat, Bytes: []byte(tc.raw)}}}
			repo := &memoryRepository{authoritative: true}
			history := loadedHistory(t, repo, 0)
			source := clipmodel.Source{Name: "Fixture Browser", BundleID: "browser.app"}
			item, ok, err := history.Capture(ctx, data, source, []string{"browser.app"})
			if err != nil || !ok || item.Type != tc.kind || item.Text != tc.text {
				t.Fatalf("URI capture: %+v, %v", item, err)
			}
			encoded, err := storage.EncodeHistory(repo.records)
			if err != nil {
				t.Fatal(err)
			}
			records, err := storage.DecodeHistory(encoded, false)
			if err != nil {
				t.Fatal(err)
			}
			restored := loadedHistory(t, &memoryRepository{records: records, authoritative: true}, 0)
			if _, current := restored.Current(); current {
				t.Fatal("URI restoration granted clipboard authority")
			}
			before := restored.Records()
			saved, ok := restored.Record(item.ID)
			if !ok || !reflect.DeepEqual(saved.Data, data) || saved.Item.Type != tc.kind || saved.Item.Text != tc.text {
				t.Fatal("URI persistence changed its content or representation")
			}
			saved.Item.Text = "https://injected.example/"
			saved.Item.AllowedApps = nil
			saved.Data[0][0].Bytes[0] = 'X'
			writer := &fakeWriter{}
			ack, revision, err := (Copy{History: restored, Port: writer}).Item(ctx, item.ID)
			if err != nil || revision != 42 || ack.ID != item.ID || !reflect.DeepEqual(writer.data, data) ||
				!reflect.DeepEqual(writer.scope, []string{"browser.app"}) || !reflect.DeepEqual(before, restored.Records()) {
				t.Fatalf("URI copy did not retain exact saved bytes and protection: %+v, %v", ack, err)
			}
		})
	}
}

func TestLegacyMixedURIRecordCopiesAllSavedDataWithOriginalProtection(t *testing.T) {
	for _, fileFormat := range []string{clipmodel.URIListFormat, clipmodel.FileListFormat} {
		data := clipmodel.Representations{
			{{Format: clipmodel.URIListFormat, Bytes: []byte("# App route\r\naweme://video/123\r\n")}},
			{{Format: fileFormat, Bytes: []byte("file:///tmp/b")}},
		}
		old := historyRecord(t, "item-17", "/tmp/b", time.Now().Add(-time.Hour), true, []string{"editor.app"}, clipmodel.TextData("/tmp/b"))
		old.Data = data
		history := loadedHistory(t, &memoryRepository{records: []clipmodel.Record{old}, authoritative: true}, 0)
		if _, current := history.Current(); current {
			t.Fatal("legacy mixed restore granted clipboard authority")
		}
		before := history.Records()
		snapshot, ok := history.Record(old.Item.ID)
		if !ok {
			t.Fatal("legacy mixed record was lost")
		}
		snapshot.Data[0][0].Bytes[0] = 'X'
		snapshot.Item.AllowedApps = nil
		writer := &fakeWriter{}
		ack, revision, err := (Copy{History: history, Port: writer}).Item(context.Background(), old.Item.ID)
		if err != nil || revision != 42 || ack.ID != old.Item.ID || !reflect.DeepEqual(writer.data, data) ||
			!reflect.DeepEqual(writer.scope, old.Item.AllowedApps) || !reflect.DeepEqual(history.Records(), before) {
			t.Fatalf("legacy mixed copy lost saved data or authority: %+v, %v", ack, err)
		}
	}
}

func TestMixedURIItemsKeepFallbackRepresentationsAndNeverCopyOnlyFiles(t *testing.T) {
	for _, destination := range []string{"https://example.com/a", "aweme://video/123"} {
		for _, fileFirst := range []bool{false, true} {
			ctx := context.Background()
			repo := &memoryRepository{authoritative: true}
			history := loadedHistory(t, repo, 0)
			if _, _, err := history.Capture(ctx, clipmodel.TextData("Keep existing history"), clipmodel.Source{Name: "Editor"}, nil); err != nil {
				t.Fatal(err)
			}
			data := clipmodel.Representations{
				{{Format: clipmodel.URIListFormat, Bytes: []byte(destination)}},
				{{Format: clipmodel.URIListFormat, Bytes: []byte("file:///tmp/b")}},
			}
			if fileFirst {
				data[0], data[1] = data[1], data[0]
			}
			before, saves := history.Records(), repo.saves
			if _, captured, err := history.Capture(ctx, data, clipmodel.Source{Name: "Editor"}, nil); err == nil || captured {
				t.Fatal("unsupported mixed URI items were partially captured")
			}
			if repo.saves != saves || !reflect.DeepEqual(history.Records(), before) {
				t.Fatal("rejected mixed selection changed history")
			}

			data[0] = append(data[0], clipmodel.Representation{Format: clipmodel.TextFormat, Bytes: []byte("Original mixed selection")})
			item, captured, err := history.Capture(ctx, data, clipmodel.Source{Name: "Editor"}, []string{"editor.app"})
			if err != nil || !captured || item.Type != clipmodel.TypeText || item.Text != "Original mixed selection" {
				t.Fatalf("mixed URI fallback capture: %+v, %v", item, err)
			}
			encoded, err := storage.EncodeHistory(repo.records)
			if err != nil {
				t.Fatal(err)
			}
			records, err := storage.DecodeHistory(encoded, false)
			if err != nil {
				t.Fatal(err)
			}
			restored := loadedHistory(t, &memoryRepository{records: records, authoritative: true}, 0)
			before = restored.Records()
			writer := &fakeWriter{}
			ack, revision, err := (Copy{History: restored, Port: writer}).Item(ctx, item.ID)
			if err != nil || revision != 42 || ack.ID != item.ID || !reflect.DeepEqual(writer.data, data) ||
				!reflect.DeepEqual(writer.scope, []string{"editor.app"}) || !reflect.DeepEqual(restored.Records(), before) {
				t.Fatalf("mixed URI copy lost representations or protection: %+v, %v", ack, err)
			}
		}
	}
}
