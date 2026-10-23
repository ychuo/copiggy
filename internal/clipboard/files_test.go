package clipboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"copiggy/internal/clipmodel"
	"copiggy/internal/storage"
)

func clipboardFileFixture() clipmodel.Representations {
	return clipmodel.Representations{
		{{Format: clipmodel.FileListFormat, Bytes: []byte("file:///tmp/%E6%8A%A5%E5%91%8A.txt")}, {Format: clipmodel.NativeFileURLFormat, Bytes: []byte("file:///.file/id=123.456/")}, {Format: clipmodel.TextFormat, Bytes: []byte("报告.txt")}},
		{{Format: clipmodel.FileListFormat, Bytes: []byte("file:///tmp/b%20c.pdf")}},
	}
}

func TestFileCopyDefaultsToTrustedPathTextAndKeepsProtection(t *testing.T) {
	ctx := context.Background()
	data := clipboardFileFixture()
	repo := &memoryRepository{}
	history := loadedHistory(t, repo, 0)
	item, ok, err := history.Capture(ctx, data, clipmodel.Source{Name: "Finder", BundleID: "com.apple.finder"}, []string{"com.apple.finder"})
	if err != nil || !ok || item.Type != clipmodel.TypeText || item.Text != "/tmp/报告.txt\n/tmp/b c.pdf" {
		t.Fatalf("capture: %+v %v", item, err)
	}
	if err := history.SetPinned(ctx, item.ID, true); err != nil {
		t.Fatal(err)
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
		t.Fatal("restore selected clipboard authority")
	}
	saved, ok := restored.Record(item.ID)
	if !ok || !saved.Item.Pinned || !reflect.DeepEqual(saved.Data, data) {
		t.Fatal("persistence changed original file payload")
	}
	saved.Item.Text = "/injected"
	saved.Item.AllowedApps = nil
	before := restored.Items()
	writer := &fakeWriter{}
	ack, revision, err := (Copy{History: restored, Port: writer}).Item(ctx, item.ID)
	if err != nil || revision != 42 {
		t.Fatalf("copy: %v", err)
	}
	if !reflect.DeepEqual(writer.data, clipmodel.TextData("/tmp/报告.txt\n/tmp/b c.pdf")) || !reflect.DeepEqual(writer.scope, []string{"com.apple.finder"}) {
		t.Fatal("default copy did not derive complete paths with saved protection")
	}
	if ack.ID != "" || ack.ParentID != item.ID || ack.Text != item.Text || ack.Type != item.Type {
		t.Fatalf("copy acknowledgement misidentified clipboard contents: %+v", ack)
	}
	if _, current := restored.Current(); current {
		t.Fatal("raw file record became current for a plain-text clipboard")
	}
	after, _ := restored.Record(item.ID)
	if !reflect.DeepEqual(before, restored.Items()) || !reflect.DeepEqual(after.Data, data) {
		t.Fatal("default path copy changed saved history")
	}
}

func TestFileCopyMatchesOnlyExistingPathTextWithTheSameScope(t *testing.T) {
	for _, sameScope := range []bool{true, false} {
		t.Run(map[bool]string{true: "same scope", false: "different scope"}[sameScope], func(t *testing.T) {
			ctx := context.Background()
			history := loadedHistory(t, &memoryRepository{}, 0)
			scope := []string{"com.apple.finder"}
			file, ok, err := history.Capture(ctx, clipboardFileFixture()[:1], clipmodel.Source{Name: "Finder"}, scope)
			if err != nil || !ok {
				t.Fatal(err)
			}
			textScope := scope
			if !sameScope {
				textScope = nil
			}
			plain, ok, err := history.Capture(ctx, clipmodel.TextData(file.Text), clipmodel.Source{Name: "TextEdit"}, textScope)
			if err != nil || !ok {
				t.Fatal(err)
			}
			before := history.Items()
			writer := &fakeWriter{}
			ack, _, err := (Copy{History: history, Port: writer}).Item(ctx, file.ID)
			if err != nil {
				t.Fatal(err)
			}
			current, selected := history.Current()
			if sameScope && (ack.ID != plain.ID || !selected || current.ID != plain.ID) {
				t.Fatal("exact path text with the same scope was not acknowledged")
			}
			if !sameScope && (ack.ID != "" || selected) {
				t.Fatal("copy acknowledged history with a different protection scope")
			}
			if !reflect.DeepEqual(writer.data, clipmodel.TextData(file.Text)) || !reflect.DeepEqual(writer.scope, scope) || !reflect.DeepEqual(history.Items(), before) {
				t.Fatal("copy changed saved history, output or protection")
			}
		})
	}
}

func TestFailedFilePathCopyLeavesSelectionAndRecordsUntouched(t *testing.T) {
	ctx := context.Background()
	history := loadedHistory(t, &memoryRepository{}, 0)
	item, ok, err := history.Capture(ctx, clipboardFileFixture()[:1], clipmodel.Source{Name: "Finder"}, []string{"com.apple.finder"})
	if err != nil || !ok {
		t.Fatal(err)
	}
	before := history.Items()
	writer := &fakeWriter{err: errors.New("write failed")}
	if _, _, err := (Copy{History: history, Port: writer}).Item(ctx, item.ID); err == nil {
		t.Fatal("failed write succeeded")
	}
	current, selected := history.Current()
	if !selected || current.ID != item.ID || !reflect.DeepEqual(before, history.Items()) {
		t.Fatal("failed copy changed selection or history")
	}
}

func TestLegacyFilesJSONMigratesAndCanBeSavedAgain(t *testing.T) {
	ctx := context.Background()
	data := clipboardFileFixture()
	history := loadedHistory(t, &memoryRepository{}, 0)
	item, ok, err := history.Capture(ctx, data, clipmodel.Source{Name: "Finder"}, []string{"com.apple.finder"})
	if err != nil || !ok {
		t.Fatal(err)
	}
	item.Pinned = true
	item.Files, err = clipmodel.FilesFromRepresentations(data)
	if err != nil {
		t.Fatal(err)
	}
	item.Type, item.Text = clipmodel.TypeFiles, "报告.txt\nb c.pdf"
	legacy, err := json.Marshal(map[string]any{"version": 1, "records": []clipmodel.Record{{Item: item, Data: data}}})
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := storage.DecodeHistory(legacy, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 1 || migrated[0].Item.Type != clipmodel.TypeText || migrated[0].Item.Text != "/tmp/报告.txt\n/tmp/b c.pdf" || len(migrated[0].Item.Files) != 0 || !migrated[0].Item.Pinned || migrated[0].Item.ID != item.ID || !reflect.DeepEqual(migrated[0].Data, data) {
		t.Fatalf("old saved file record did not migrate: %+v", migrated)
	}
	encoded, err := storage.EncodeHistory(migrated)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := storage.DecodeHistory(encoded, false)
	if err != nil || !reflect.DeepEqual(migrated, reloaded) {
		t.Fatalf("resaved migration did not reload: %+v %v", reloaded, err)
	}
}

func TestInvalidEncodedFilePathsCannotCorruptSavedHistory(t *testing.T) {
	for _, raw := range []string{"file:///tmp/%FF", "file:///tmp/%C0%AF", "file:///tmp/%ED%A0%80"} {
		t.Run(raw, func(t *testing.T) {
			ctx := context.Background()
			repo := &memoryRepository{}
			history := loadedHistory(t, repo, 0)
			if _, _, err := history.Capture(ctx, clipmodel.TextData("Keep existing history"), clipmodel.Source{Name: "Editor"}, []string{"editor.app"}); err != nil {
				t.Fatal(err)
			}
			before, saves := history.Records(), repo.saves
			data := clipmodel.Representations{{
				{Format: clipmodel.FileListFormat, Bytes: []byte(raw)},
				{Format: clipmodel.TextFormat, Bytes: []byte("filename")},
			}}
			if _, captured, err := history.Capture(ctx, data, clipmodel.Source{Name: "Finder"}, nil); err == nil || captured {
				t.Fatal("invalid decoded path was captured and can become lossy JSON text")
			}
			if repo.saves != saves || !reflect.DeepEqual(history.Records(), before) {
				t.Fatal("invalid path changed existing history or its persisted snapshot")
			}
			encoded, err := storage.EncodeHistory(repo.records)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := storage.DecodeHistory(encoded, false)
			if err != nil {
				t.Fatalf("existing history no longer loads: %v", err)
			}
			roundTrip, err := storage.EncodeHistory(restored)
			if err != nil || !bytes.Equal(roundTrip, encoded) {
				t.Fatalf("existing history no longer round-trips: %v", err)
			}
		})
	}
}
