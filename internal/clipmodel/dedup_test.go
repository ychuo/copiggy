package clipmodel

import (
	"context"
	"crypto/sha256"
	"reflect"
	"testing"
	"time"
)

func TestDuplicateCaptureRefreshesTimeAndOrderKeepingOriginalProvenance(t *testing.T) {
	store := NewStore(3)
	first, _ := store.CaptureContent(Content{Text: "copied text", SourceAppID: "com.original.editor"}, "Original Editor", false)
	second, _ := store.Capture("second", "Browser", false)
	third, _ := store.Capture("third", "Notes", false)
	beforeCopy := time.Now()
	refreshed, recorded := store.CaptureContent(Content{Text: first.Text, SourceAppID: "com.other.browser"}, "Other Browser", false)
	if !recorded || refreshed.ID != first.ID {
		t.Fatalf("duplicate must refresh the existing record: %+v, %v", refreshed, recorded)
	}
	if refreshed.SourceApp != first.SourceApp || refreshed.SourceAppID != first.SourceAppID || refreshed.ParentID != first.ParentID {
		t.Fatalf("duplicate rewrote its original provenance: %+v", refreshed)
	}
	if refreshed.CreatedAt.Before(beforeCopy) || !refreshed.CreatedAt.After(first.CreatedAt) {
		t.Fatal("duplicate kept the old copy time")
	}
	items := store.Items()
	if len(items) != 3 || items[0].ID != first.ID || items[1].ID != third.ID || items[2].ID != second.ID {
		t.Fatalf("duplicate did not move to front without evicting/reordering others: %+v", items)
	}
	if !reflect.DeepEqual(items[1], third) || !reflect.DeepEqual(items[2], second) {
		t.Fatal("refresh changed an unrelated history record")
	}
	assertCurrent(t, store, first.ID)

	// Recopying the already-first record is also a refresh, not a new ID.
	again, _ := store.Capture(first.Text, "Third App", false)
	if again.ID != first.ID || len(store.Items()) != 3 || again.SourceAppID != first.SourceAppID {
		t.Fatalf("repeating the newest copy created or relabeled a record: %+v", again)
	}
	fourth, _ := store.Capture("fourth", "Notes", false)
	items = store.Items()
	if items[0].ID != fourth.ID || items[1].ID != first.ID || items[2].ID != third.ID {
		t.Fatalf("capacity eviction ignored refreshed recency: %+v", items)
	}
}

func TestDuplicateCaptureKeepsUnknownOriginalSource(t *testing.T) {
	store := NewStore(10)
	first, _ := store.Capture("text", "未知应用", false)
	copy, _ := store.CaptureContent(Content{Text: first.Text, SourceAppID: "com.known.editor"}, "Editor", false)
	if copy.ID != first.ID || copy.SourceApp != first.SourceApp || copy.SourceAppID != "" {
		t.Fatalf("a repeated copy relabeled an unknown original source: %+v", copy)
	}
}

func TestDuplicateOwnWriteEchoDoesNotRefreshTimeOrOrder(t *testing.T) {
	store := NewStore(10)
	first, _ := store.Capture("first", "Editor", false)
	store.Capture("second", "Browser", false)
	if _, err := store.Select(first.ID); err != nil {
		t.Fatal(err)
	}
	before := store.Items()
	item, recorded := store.Capture(first.Text, "Copiggy", true)
	if recorded || item.ID != first.ID || !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("a verified own-write echo refreshed external copy history")
	}
}

func TestDuplicateCaptureRequiresExactText(t *testing.T) {
	store := NewStore(10)
	for _, text := range []string{"value", "Value", "value ", "value\n", `{"a":1}`, `{"a": 1}`} {
		store.Capture(text, "Editor", false)
	}
	if len(store.Items()) != 6 {
		t.Fatal("deduplication normalized text or JSON instead of preserving exact contents")
	}
}

func TestDuplicateCaptureRequiresSameAllowedAppSet(t *testing.T) {
	store := NewStore(10)
	content := Content{Text: "same contents", SourceAppID: "com.editor"}
	protected, _ := store.CaptureProtectedContent(content, "Editor", []string{"com.editor", "com.chat"})
	ordinary, _ := store.CaptureContent(content, "Editor", false)
	narrower, _ := store.CaptureProtectedContent(content, "Editor", []string{"com.editor"})
	otherScope, _ := store.CaptureProtectedContent(content, "Editor", []string{"com.browser", "com.chat"})
	if len(store.Items()) != 4 || ordinary.ID == protected.ID || narrower.ID == protected.ID || otherScope.ID == protected.ID {
		t.Fatal("copies with different protection policies were merged")
	}
	content.SourceAppID = "com.another.editor"
	refreshed, _ := store.CaptureProtectedContent(content, "Another Editor", []string{"com.chat", " com.editor ", "com.chat"})
	if refreshed.ID != protected.ID || len(store.Items()) != 4 {
		t.Fatalf("the same normalized app set did not merge: %+v", refreshed)
	}
	if refreshed.SourceAppID != protected.SourceAppID || refreshed.SourceApp != protected.SourceApp ||
		!reflect.DeepEqual(refreshed.AllowedApps, protected.AllowedApps) {
		t.Fatal("refresh changed the original source or policy ordering")
	}
	if refreshed.Allows("com.browser") || refreshed.Allows("") || !refreshed.Allows("com.editor") {
		t.Fatal("deduplication weakened a protected copy")
	}

	// An ordinary recopy selects its independent unrestricted record, leaving
	// the protected record in history with the original policy.
	copy, _ := store.CaptureContent(content, "Another Editor", false)
	if copy.ID != ordinary.ID || len(copy.AllowedApps) != 0 {
		t.Fatalf("ordinary duplicate selected a protected record: %+v", copy)
	}
	saved, err := store.Select(protected.ID)
	if err != nil || saved.Allows("com.browser") {
		t.Fatal("ordinary duplicate removed the original protected scope")
	}
}

func TestDuplicateEnhancedCopyKeepsLineageAndSource(t *testing.T) {
	store := NewStore(10)
	source, _ := store.CaptureProtectedContent(Content{Text: "hello   \n", SourceAppID: "com.editor"}, "Editor", []string{"com.editor"})
	draft, err := store.Enhance(context.Background(), source.ID, CleanWhitespace)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := store.Accept(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if draft.SourceAppID != source.SourceAppID || accepted.SourceAppID != source.SourceAppID {
		t.Fatal("enhancement lost the original source application identity")
	}
	store.Capture("another record", "Notes", false)
	copy, _ := store.CaptureProtectedContent(Content{Text: accepted.Text, SourceAppID: "com.another"}, "Another App", accepted.AllowedApps)
	if copy.ID != accepted.ID || copy.ParentID != source.ID || copy.SourceApp != source.SourceApp || copy.SourceAppID != source.SourceAppID {
		t.Fatalf("recopying enhanced content lost its original lineage: %+v", copy)
	}
	if len(store.Items()) != 3 || store.Items()[2].Text != source.Text {
		t.Fatal("recopying the enhanced result changed its original record")
	}
}

func TestDuplicateImageRequiresBytesFingerprintNotJustMetadata(t *testing.T) {
	store := NewStore(10)
	content := Content{Type: TypeImage, Image: &ImageMetadata{MIME: "image/png", Width: 30, Height: 20, ByteSize: 300}}
	first, _ := store.CaptureContent(content, "Preview", false)
	unknown, _ := store.CaptureContent(content, "Preview", false)
	if first.ID == unknown.ID {
		t.Fatal("image metadata alone incorrectly identified a duplicate")
	}
	content.Fingerprint = sha256.Sum256([]byte("image A"))
	imageA, _ := store.CaptureContent(content, "Preview", false)
	content.Fingerprint = sha256.Sum256([]byte("image B"))
	imageB, _ := store.CaptureContent(content, "Preview", false)
	if imageA.ID == imageB.ID {
		t.Fatal("different image bytes with equal dimensions/length were merged")
	}
	content.Fingerprint = sha256.Sum256([]byte("image A"))
	copy, _ := store.CaptureContent(content, "Browser", false)
	if copy.ID != imageA.ID || len(store.Items()) != 4 {
		t.Fatal("an exact repeated image was not refreshed")
	}
	copy.Image.Width = 999
	current, _ := store.Current()
	if current.Image.Width != 30 {
		t.Fatal("returned duplicate image metadata mutated the stored record")
	}
}
