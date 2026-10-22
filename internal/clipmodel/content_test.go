package clipmodel

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPNGHistoryRetainsIndependentMetadataAndScope(t *testing.T) {
	store := NewStore(2)
	metadata := &ImageMetadata{MIME: "image/png", Width: 640, Height: 480, ByteSize: 2048}
	content := Content{Type: TypeImage, Image: metadata}
	protected, recorded := store.CaptureProtectedContent(content, "com.apple.Preview", []string{"com.apple.Preview"})
	if !recorded || protected.Type != TypeImage || protected.Text != "" || protected.Image == nil {
		t.Fatalf("PNG without text was not captured: %+v", protected)
	}
	metadata.Width = 1
	protected.Image.Height = 1
	protected.AllowedApps[0] = "com.other.app"
	current, ok := store.Current()
	if !ok || current.Image.Width != 640 || current.Image.Height != 480 ||
		!current.Allows("com.apple.Preview") || current.Allows("com.other.app") {
		t.Fatalf("caller mutated the protected PNG snapshot: %+v", current)
	}
	current.Image.ByteSize = 1
	items := store.Items()
	items[0].Image.MIME = "image/jpeg"
	echo, recorded := store.CaptureContent(Content{}, "Copiggy", true)
	if recorded || echo.ID != protected.ID || echo.Image.MIME != "image/png" || echo.Image.ByteSize != 2048 ||
		len(store.Items()) != 1 {
		t.Fatal("own-write echo changed PNG history or exposed mutable metadata")
	}

	ordinary, recorded := store.CaptureContent(Content{Type: TypeImage, Image: echo.Image}, "com.apple.Preview", false)
	if !recorded || ordinary.ID == protected.ID || len(ordinary.AllowedApps) != 0 {
		t.Fatalf("ordinary image copy inherited a stale restriction: %+v", ordinary)
	}
	assertCurrent(t, store, ordinary.ID)
	selected, err := store.Select(protected.ID)
	if err != nil || selected.Image.Width != 640 || selected.Allows("com.other.app") {
		t.Fatalf("selecting PNG failed to restore content metadata and scope: %+v, %v", selected, err)
	}
	selected.Image.Width = 1
	current, _ = store.Current()
	if current.Image.Width != 640 {
		t.Fatal("select returned mutable internal PNG metadata")
	}
}

func TestInvalidPNGCopyDoesNotApplyOldRestrictionsToNewClipboard(t *testing.T) {
	for _, content := range []Content{
		{Type: TypeImage},
		{Type: TypeImage, Image: &ImageMetadata{MIME: "image/jpeg", Width: 1, Height: 1, ByteSize: 8}},
		{Type: TypeImage, Image: &ImageMetadata{MIME: "image/png", Width: 0, Height: 1, ByteSize: 8}},
		{Type: TypeImage, Image: &ImageMetadata{MIME: "image/png", Width: 1, Height: -1, ByteSize: 8}},
		{Type: TypeImage, Image: &ImageMetadata{MIME: "image/png", Width: 1, Height: 1, ByteSize: 0}},
		{Type: ContentType("unknown"), Text: "not a supported capture"},
	} {
		store := NewStore(2)
		protected, _ := store.CaptureProtected("existing value", "com.editor", []string{"com.editor"})
		before := store.Items()
		if _, recorded := store.CaptureProtectedContent(content, "com.editor", []string{"com.editor"}); recorded {
			t.Fatalf("invalid protected content was recorded: %+v", content)
		}
		assertCurrent(t, store, protected.ID)
		if _, recorded := store.CaptureContent(content, "com.other", false); recorded {
			t.Fatalf("invalid external content was recorded: %+v", content)
		}
		if _, ok := store.Current(); ok {
			t.Fatal("unsupported external copy retained the previous restricted current item")
		}
		if !reflect.DeepEqual(before, store.Items()) {
			t.Fatal("invalid external capture changed existing history")
		}
	}
}

func TestContentClassificationAndTypeSearch(t *testing.T) {
	store := NewStore(20)
	for _, example := range []struct {
		text string
		kind ContentType
	}{
		{`{"value":9007199254740993}`, TypeJSON},
		{" [1,true] \n", TypeJSON},
		{"{invalid}", TypeText},
		{"123", TypeText},
		{"https://example.com/path?q=test", TypeLink},
		{"hello https://example.com", TypeText},
		{"file:///private/path", TypeFilePath},
		{"https://", TypeText},
	} {
		item, ok := store.Capture(example.text, "Some Editor", false)
		if !ok || item.Type != example.kind || item.Text != example.text {
			t.Fatalf("capture changed content or misclassified %q: %+v", example.text, item)
		}
	}
	image, _ := store.CaptureContent(Content{
		Type:  TypeImage,
		Image: &ImageMetadata{MIME: "image/png", Width: 20, Height: 10, ByteSize: 100},
	}, "com.apple.Preview", false)
	before := store.Items()
	for _, query := range []string{"IMAGE", " PNG ", "图片", "圖片", "画像", "COM.APPLE.PREVIEW"} {
		results := store.Search(query)
		if len(results) != 1 || results[0].ID != image.ID {
			t.Fatalf("image type/source search %q: %+v", query, results)
		}
	}
	if got := len(store.Search("json")); got != 2 {
		t.Fatalf("JSON type search returned %d records", got)
	}
	for _, query := range []string{"link", "链接", "連結", "リンク"} {
		if got := len(store.Search(query)); got != 1 {
			t.Fatalf("link type search %q returned %d records", query, got)
		}
	}
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("classification search modified history")
	}
	assertCurrent(t, store, image.ID)
}

func TestImageEnhancementIsRejectedWithoutCreatingTextDraft(t *testing.T) {
	store := NewStore(10)
	image, _ := store.CaptureProtectedContent(Content{
		Type:  TypeImage,
		Image: &ImageMetadata{MIME: "image/png", Width: 20, Height: 10, ByteSize: 100},
	}, "com.preview", []string{"com.preview"})
	before := store.Items()
	for _, kind := range []EnhancementKind{CleanWhitespace, FormatJSON} {
		if _, err := store.Enhance(context.Background(), image.ID, kind); !errors.Is(err, ErrUnsupportedContent) {
			t.Fatalf("image enhancement did not reject incompatible content: %v", err)
		}
	}
	if len(store.drafts) != 0 || !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("rejected image enhancement created a draft or changed history")
	}
	assertCurrent(t, store, image.ID)
}

func TestJSONEnhancementPreservesClassificationAndOriginalImage(t *testing.T) {
	store := NewStore(10)
	source, _ := store.CaptureProtected(`{"value":1}`, "com.editor", []string{"com.editor"})
	image, _ := store.CaptureContent(Content{
		Type:  TypeImage,
		Image: &ImageMetadata{MIME: "image/png", Width: 20, Height: 10, ByteSize: 100},
	}, "com.preview", false)
	draft, err := store.Enhance(context.Background(), source.ID, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrent(t, store, image.ID)
	validated, err := store.ValidateDraft(draft.ID)
	if err != nil || Analyze(validated.Text).Type != TypeJSON ||
		validated.ParentID != source.ID || !reflect.DeepEqual(validated.AllowedApps, []string{"com.editor"}) {
		t.Fatalf("validated JSON lost classification or scope: %+v, %v", validated, err)
	}
	items := store.Items()
	if len(items) != 2 || items[0].ID != image.ID || items[0].Image.Width != 20 || items[0].Type != TypeImage {
		t.Fatal("text enhancement changed the unrelated PNG history")
	}
}
