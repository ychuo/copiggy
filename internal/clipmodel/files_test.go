package clipmodel

import (
	"reflect"
	"strings"
	"testing"
)

func fileTestData() Representations {
	return Representations{
		{{Format: TextFormat, Bytes: []byte("报告 #1.txt")}, {Format: FileListFormat, Bytes: []byte("file:///Users/test/%E6%8A%A5%E5%91%8A%20%231.txt")}, {Format: NativeFileURLFormat, Bytes: []byte("file:///.file/id=123.456/")}},
		{{Format: TextFormat, Bytes: []byte("写真😀.png")}, {Format: FileListFormat, Bytes: []byte("file:///Users/test/%E5%86%99%E7%9C%9F%F0%9F%98%80.png")}},
	}
}

func TestNativeFilesUseTheSameCompletePathTextAsPlainCopies(t *testing.T) {
	for _, tc := range []struct {
		name string
		data Representations
		text string
		kind ContentType
	}{
		{"single", fileTestData()[:1], "/Users/test/报告 #1.txt", TypeFilePath},
		{"multiple", fileTestData(), "/Users/test/报告 #1.txt\n/Users/test/写真😀.png", TypeText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.data.Clone()
			content, err := ContentFromRepresentations(tc.data)
			if err != nil || content.Type != tc.kind || content.Text != tc.text {
				t.Fatalf("native files did not become complete paths: %+v, %v", content, err)
			}
			native, ok := NewStore(100).CaptureContent(content, "Finder", false)
			if !ok {
				t.Fatal("native file capture rejected")
			}
			direct, ok := NewStore(100).Capture(tc.text, "TextEdit", false)
			if !ok || native.Type != direct.Type || native.Text != direct.Text || len(native.Files) != 0 {
				t.Fatalf("native and direct path copies differ: %+v / %+v", native, direct)
			}
			if !reflect.DeepEqual(tc.data, before) {
				t.Fatal("recognition changed raw bytes or item boundaries")
			}
		})
	}
	for _, original := range []string{"报告 #1.txt", "/Users/test/报告 #1.txt", "file:///Users/test/a.txt"} {
		content, err := ContentFromRepresentations(TextData(original))
		if err != nil || content.Text != original || content.Type == TypeFiles {
			t.Fatalf("ordinary text was rewritten: %q %+v %v", original, content, err)
		}
	}
}

func TestFileListsAreBoundedAndNeverKeepOnlyAValidSubset(t *testing.T) {
	for _, raw := range []string{
		"", "file://remote.example/share/a", "https://example.com/a", "file://user@localhost/a", "file:///tmp/a?x=1", "file:///tmp/a#fragment", "file:///tmp/a%00b", "file:///.file/id=123.456/", "file:relative", "file:///tmp/a\r\nhttps://example.com/b",
		strings.Repeat("file:///tmp/a\r\n", MaxFileReferences+1),
	} {
		data := Representations{{{Format: FileListFormat, Bytes: []byte(raw)}, {Format: TextFormat, Bytes: []byte("innocent filename")}}}
		if _, err := ContentFromRepresentations(data); err == nil {
			t.Errorf("invalid native file payload accepted: %q", raw)
		}
	}
	mixed := Representations{{{Format: URIListFormat, Bytes: []byte("file:///tmp/a\r\nhttps://example.com/b")}, {Format: TextFormat, Bytes: []byte("mixed URLs")}}}
	if content, err := ContentFromRepresentations(mixed); err != nil || content.Text != "mixed URLs" {
		t.Fatalf("mixed URI list was partially converted: %+v %v", content, err)
	}
	data := Representations{{{Format: URIListFormat, Bytes: []byte("# local references\r\nfile:///tmp/a%0Ab\r\nfile://localhost/tmp/c%25d\r\n")}}}
	content, err := ContentFromRepresentations(data)
	if err != nil || content.Type != TypeText || content.Text != "/tmp/a\nb\n/tmp/c%d" {
		t.Fatalf("valid encoded filenames rejected or rewritten: %+v %v", content, err)
	}
}

func TestLegacyFileHistoryMigratesValidatedMetadataToPaths(t *testing.T) {
	data := fileTestData()
	content, err := ContentFromRepresentations(data)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(100)
	item, ok := store.CaptureProtectedContent(content, "Finder", []string{"com.apple.finder"})
	if !ok {
		t.Fatal("file capture rejected")
	}
	if err := store.SetPinned(item.ID, true); err != nil {
		t.Fatal(err)
	}
	item = store.Items()[0]
	files, err := FilesFromRepresentations(data)
	if err != nil {
		t.Fatal(err)
	}
	legacy := item
	legacy.Type, legacy.Text, legacy.Files = TypeFiles, fileNames(files), files
	record := Record{Item: legacy, Data: data}
	original := record.Clone()
	restored, err := RestoreRecords([]Record{record})
	if err != nil {
		t.Fatal(err)
	}
	got := restored.Items()[0]
	if !reflect.DeepEqual(got, item) || len(got.Files) != 0 {
		t.Fatalf("legacy file history did not become ordinary paths: %+v", got)
	}
	if _, current := restored.Current(); current {
		t.Fatal("restoring granted current authority")
	}
	if _, err := RestoreRecords([]Record{{Item: got, Data: data}}); err != nil {
		t.Fatalf("migrated record cannot be restored again: %v", err)
	}
	if !reflect.DeepEqual(record, original) {
		t.Fatal("migration mutated caller-owned records")
	}
	for _, forge := range []func(*Record){
		func(r *Record) { r.Item.Files[0].Path = "/unrelated" },
		func(r *Record) { r.Item.Text = "unrelated.txt" },
		func(r *Record) { r.Data = TextData("/unrelated") },
	} {
		forged := record.Clone()
		forge(&forged)
		if _, err := RestoreRecords([]Record{forged}); err == nil {
			t.Fatal("forged legacy metadata was accepted")
		}
	}
	duplicate, ok := restored.CaptureProtectedContent(content, "Another app", []string{"com.apple.finder"})
	if !ok || duplicate.ID != item.ID || duplicate.SourceApp != item.SourceApp {
		t.Fatal("migration changed exact duplicate identity or provenance")
	}
}

func TestFileDuplicatesRequireSamePayloadAndProtection(t *testing.T) {
	data := fileTestData()
	capture := func(store *Store, value Representations, scope []string) Item {
		content, err := ContentFromRepresentations(value)
		if err != nil {
			t.Fatal(err)
		}
		if len(scope) > 0 {
			item, _ := store.CaptureProtectedContent(content, "Finder", scope)
			return item
		}
		item, _ := store.CaptureContent(content, "Finder", false)
		return item
	}
	store := NewStore(100)
	first := capture(store, data, nil)
	if duplicate := capture(store, data.Clone(), nil); duplicate.ID != first.ID {
		t.Fatal("exact file duplicate was not refreshed")
	}
	if protected := capture(store, data, []string{"com.apple.finder"}); protected.ID == first.ID {
		t.Fatal("file records merged across protection scopes")
	}
	if plain := capture(store, TextData(first.Text), nil); plain.ID == first.ID {
		t.Fatal("different raw representations were merged")
	}
	other := data.Clone()
	other[0][1].Bytes = []byte("file:///Users/other/%E6%8A%A5%E5%91%8A%20%231.txt")
	if distinct := capture(store, other, nil); distinct.ID == first.ID {
		t.Fatal("same filename merged different file references")
	}
	reordered := Representations{data[1], data[0]}
	if distinct := capture(store, reordered, nil); distinct.ID == first.ID {
		t.Fatal("file order was discarded")
	}
	if _, err := CopyText(first, CopyFormat("file-paths")); err != ErrInvalidCopyFormat {
		t.Fatal("removed file-specific variant is still active")
	}
}
