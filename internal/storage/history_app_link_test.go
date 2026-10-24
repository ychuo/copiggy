package storage

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
)

func TestVersionOneAppLinksRestoreAsIndependentTypeWithoutChangingSavedContent(t *testing.T) {
	for _, tc := range []struct {
		name, raw, text string
		format          string
	}{
		{"plain", "  aweme://video/123?q=one#part \n", "  aweme://video/123?q=one#part \n", clipmodel.TextFormat},
		{"markdown", "[Open video](aweme://video/123?q=one#part)", "[Open video](aweme://video/123?q=one#part)", clipmodel.TextFormat},
		{"URI only", "# Saved route\r\nsslocal://route?url=https%3A%2F%2Fexample.com\r\n", "sslocal://route?url=https%3A%2F%2Fexample.com", clipmodel.URIListFormat},
	} {
		for _, oldType := range []clipmodel.ContentType{clipmodel.TypeText, clipmodel.TypeLink, clipmodel.TypeAppLink} {
			t.Run(tc.name+"/"+string(oldType), func(t *testing.T) {
				original := clipmodel.Record{
					Item: clipmodel.Item{
						ID: "item-7", Type: oldType, Text: tc.text,
						SourceApp: "Original Editor", SourceAppID: "editor.app",
						AllowedApps: []string{"editor.app", "reader.app"},
						CreatedAt:   time.Date(2026, 10, 23, 1, 2, 3, 0, time.UTC), Pinned: true,
					},
					Data: clipmodel.Representations{{{Format: tc.format, Bytes: []byte(tc.raw)}, {Format: "com.example.original", Bytes: []byte{0, 1, 255}}}},
				}
				encoded, err := json.Marshal(savedHistory{Version: 1, Records: []clipmodel.Record{original}})
				if err != nil {
					t.Fatal(err)
				}
				for _, pinsOnly := range []bool{false, true} {
					restored, err := DecodeHistory(encoded, pinsOnly)
					if err != nil || len(restored) != 1 {
						t.Fatalf("version-one restore: %v", err)
					}
					expected := original.Clone()
					expected.Item.Type = clipmodel.TypeAppLink
					want, _ := json.Marshal(expected)
					got, _ := json.Marshal(restored[0])
					if string(got) != string(want) {
						t.Fatalf("migration changed original metadata or representations:\n%s\nwant:\n%s", got, want)
					}
					reencoded, err := EncodeHistory(restored)
					if err != nil {
						t.Fatal(err)
					}
					again, err := DecodeHistory(reencoded, pinsOnly)
					if err != nil || !reflect.DeepEqual(again, restored) {
						t.Fatalf("new application type did not round trip: %v", err)
					}
				}
			})
		}
	}
}
