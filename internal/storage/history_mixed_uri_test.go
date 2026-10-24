package storage

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
)

func legacyMixedURIRecord(t *testing.T, fileFormat string, oldFileType bool) clipmodel.Record {
	t.Helper()
	data := clipmodel.Representations{
		{{Format: clipmodel.URIListFormat, Bytes: []byte("# Original web destination\r\nhttps://example.com/a\r\n")}},
		{{Format: fileFormat, Bytes: []byte("file:///tmp/b")}},
	}
	item := clipmodel.Item{
		ID: "item-7", Type: clipmodel.TypeFilePath, Text: "/tmp/b",
		SourceApp: "Original Editor", SourceAppID: "editor.app",
		AllowedApps: []string{"editor.app", "reader.app"},
		CreatedAt:   time.Date(2026, 10, 23, 1, 2, 3, 0, time.UTC), Pinned: true,
	}
	if oldFileType {
		files, err := clipmodel.FilesFromRepresentations(data[1:])
		if err != nil {
			t.Fatal(err)
		}
		item.Type, item.Text, item.Files = clipmodel.TypeFiles, "b", files
	}
	return clipmodel.Record{Item: item, Data: data}
}

func TestLegacyMixedURIHistoryKeepsVerifiedAnnotationAndEveryRepresentation(t *testing.T) {
	for _, fileFormat := range []string{clipmodel.URIListFormat, clipmodel.FileListFormat} {
		for _, oldFileType := range []bool{false, true} {
			original := legacyMixedURIRecord(t, fileFormat, oldFileType)
			encoded, err := json.Marshal(savedHistory{Version: 1, Records: []clipmodel.Record{original}})
			if err != nil {
				t.Fatal(err)
			}
			for _, pinsOnly := range []bool{false, true} {
				restored, err := DecodeHistory(encoded, pinsOnly)
				if err != nil || len(restored) != 1 {
					t.Fatalf("legacy mixed history became unreadable: %v", err)
				}
				want := original.Clone()
				want.Item.Type, want.Item.Text, want.Item.Files = clipmodel.TypeFilePath, "/tmp/b", nil
				wantJSON, _ := json.Marshal(want)
				gotJSON, _ := json.Marshal(restored[0])
				if string(gotJSON) != string(wantJSON) {
					t.Fatalf("legacy mixed restore changed metadata or data:\n%s\nwant:\n%s", gotJSON, wantJSON)
				}
				reencoded, err := EncodeHistory(restored)
				if err != nil {
					t.Fatal(err)
				}
				again, err := DecodeHistory(reencoded, pinsOnly)
				if err != nil || !reflect.DeepEqual(again, restored) {
					t.Fatalf("mixed compatibility restore was not idempotent: %v", err)
				}
			}
			if _, err := clipmodel.ContentFromRepresentations(original.Data); err == nil {
				t.Fatal("history compatibility enabled new unsupported mixed captures")
			}
		}
	}
}

func TestLegacyMixedURICompatibilityRetainsNormalValidation(t *testing.T) {
	for _, corrupt := range []func(*clipmodel.Record){
		func(r *clipmodel.Record) { r.Item.Text = "/unrelated" },
		func(r *clipmodel.Record) { r.Item.ID = "not-an-item" },
		func(r *clipmodel.Record) { r.Item.CreatedAt = time.Time{} },
		func(r *clipmodel.Record) { r.Item.AllowedApps = []string{"editor.app", "editor.app"} },
		func(r *clipmodel.Record) {
			r.Item.Image = &clipmodel.ImageMetadata{MIME: "image/png", Width: 1, Height: 1, ByteSize: 1}
		},
		func(r *clipmodel.Record) { r.Data[1][0].Bytes = []byte("file://remote.example/b") },
		func(r *clipmodel.Record) {
			r.Item.Files = []clipmodel.FileReference{{URL: "file:///unrelated", Path: "/unrelated", Name: "unrelated"}}
		},
	} {
		original := legacyMixedURIRecord(t, clipmodel.URIListFormat, false)
		corrupt(&original)
		encoded, err := json.Marshal(savedHistory{Version: 1, Records: []clipmodel.Record{original}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeHistory(encoded, false); err == nil {
			t.Fatal("mixed legacy compatibility bypassed normal validation")
		}
	}
}

func TestLegacyMixedFileAnnotationStillMigratesWhenFallbackMatchesBasename(t *testing.T) {
	original := legacyMixedURIRecord(t, clipmodel.FileListFormat, true)
	original.Data[1] = append(original.Data[1], clipmodel.Representation{Format: clipmodel.TextFormat, Bytes: []byte(original.Item.Text)})
	encoded, err := json.Marshal(savedHistory{Version: 1, Records: []clipmodel.Record{original}})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeHistory(encoded, false)
	if err != nil || len(restored) != 1 {
		t.Fatalf("matching legacy filename fallback blocked migration: %v", err)
	}
	if restored[0].Item.Type != clipmodel.TypeFilePath || restored[0].Item.Text != "/tmp/b" || len(restored[0].Item.Files) != 0 || !reflect.DeepEqual(restored[0].Data, original.Data) {
		t.Fatal("legacy mixed filename fallback changed its verified paths or original data")
	}
}
