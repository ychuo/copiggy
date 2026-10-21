package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"copiggy/internal/clipmodel"
)

const versionOneFixture = `{"version":1,"records":[{"item":{"id":"item-7","type":"text","text":"hello","sourceApp":"Editor","sourceAppId":"editor.app","allowedApps":["editor.app"],"createdAt":"2026-10-20T00:00:00Z","pinned":true},"data":[[{"format":"text/plain","bytes":"aGVsbG8="},{"format":"text/html","bytes":"PGI+aGVsbG88L2I+"}]]}]}`

func TestExistingVersionOneFixtureRoundTrip(t *testing.T) {
	records, err := DecodeHistory([]byte(versionOneFixture), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Item.ID != "item-7" || !records[0].Item.Pinned || records[0].Item.Allows("other.app") {
		t.Fatalf("legacy identity/scope: %+v", records)
	}
	if raw, err := records[0].Data.Read(clipmodel.HTMLFormat); err != nil || string(raw) != "<b>hello</b>" {
		t.Fatal("exact HTML changed")
	}
	encoded, err := EncodeHistory(records)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeHistory(encoded, false)
	if err != nil || !reflect.DeepEqual(records, again) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestSavedHistoryRejectsMalformedMetadataRepresentationsAndTrailingData(t *testing.T) {
	records, err := DecodeHistory([]byte(versionOneFixture), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func([]clipmodel.Record) []clipmodel.Record
	}{
		{"mismatched display text", func(r []clipmodel.Record) []clipmodel.Record { r[0].Item.Text = "forged"; return r }},
		{"empty scope entry", func(r []clipmodel.Record) []clipmodel.Record { r[0].Item.AllowedApps = []string{""}; return r }},
		{"duplicate scope entry", func(r []clipmodel.Record) []clipmodel.Record {
			r[0].Item.AllowedApps = []string{"editor.app", "editor.app"}
			return r
		}},
		{"duplicate record identity", func(r []clipmodel.Record) []clipmodel.Record { return append(r, r[0].Clone()) }},
		{"invalid id", func(r []clipmodel.Record) []clipmodel.Record { r[0].Item.ID = "not-an-item"; return r }},
		{"duplicate format", func(r []clipmodel.Record) []clipmodel.Record {
			r[0].Data[0] = append(r[0].Data[0], r[0].Data[0][0])
			return r
		}},
		{"invalid format", func(r []clipmodel.Record) []clipmodel.Record { r[0].Data[0][0].Format = "text/plain\n"; return r }},
		{"missing bytes", func(r []clipmodel.Record) []clipmodel.Record { r[0].Data = nil; return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := tc.change(clipmodel.CloneRecords(records))
			encoded, _ := json.Marshal(savedHistory{Version: 1, Records: changed})
			if _, err := DecodeHistory(encoded, false); err == nil {
				t.Fatal("invalid history accepted")
			}
			if _, err := EncodeHistory(changed); err == nil {
				t.Fatal("writer accepted unreadable history")
			}
		})
	}
	for _, raw := range []string{`{"version":2,"records":[]}`, versionOneFixture + `{}`, `{"version":1,"records":[],"unknown":true}`} {
		if _, err := DecodeHistory([]byte(raw), false); err == nil {
			t.Fatalf("invalid envelope accepted: %.80s", raw)
		}
	}
	records[0].Item.Pinned = false
	encoded, _ := json.Marshal(savedHistory{Version: 1, Records: records})
	if _, err := DecodeHistory(encoded, true); err == nil {
		t.Fatal("legacy migration accepted ordinary record")
	}
}

func TestRepositoryFailurePreservesPreviousFileAndPermissions(t *testing.T) {
	ctx := context.Background()
	records, err := DecodeHistory([]byte(versionOneFixture), false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "nested", "history.json")
	r := NewHistoryRepository(path, "")
	defer r.Close()
	if err = r.Save(ctx, records); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, _ := os.Stat(path)
	dir, _ := os.Stat(filepath.Dir(path))
	if file.Mode().Perm() != 0600 || dir.Mode().Perm() != 0700 {
		t.Fatal("private history permissions were not retained")
	}
	failure := errors.New("disk full")
	r.Write = func(string, []byte) error { return failure }
	if err = r.Save(ctx, nil); !errors.Is(err, failure) {
		t.Fatal("write failure was hidden")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("failed write replaced history")
	}
}

func TestHistoryRepositoryLoadsOnlyLegacyWhenAuthoritativeFileMissing(t *testing.T) {
	dir := t.TempDir()
	path, legacy := filepath.Join(dir, "history.json"), filepath.Join(dir, "pinned.json")
	if err := os.WriteFile(legacy, []byte(versionOneFixture), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewHistoryRepository(path, legacy)
	defer r.Close()
	records, authority, err := r.Load(context.Background())
	if err != nil || authority || len(records) != 1 {
		t.Fatalf("migration load: %v %v", authority, err)
	}
	if err = r.Save(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	records, authority, err = r.Load(context.Background())
	if err != nil || !authority || len(records) != 0 {
		t.Fatal("authoritative empty file fell back to pins")
	}
}
