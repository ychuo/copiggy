package clipmodel

import (
	"reflect"
	"testing"
)

func TestAppLinkNormalizationDerivesTypeFromOriginalText(t *testing.T) {
	for _, tc := range []struct {
		raw         string
		given, want ContentType
	}{
		{" aweme://video/123?q=one#part \n", TypeLink, TypeAppLink},
		{"sslocal://route?url=https%3A%2F%2Fexample.com", TypeText, TypeAppLink},
		{"[Video](aweme://video/123)", TypeAppLink, TypeAppLink},
		{"https://example.com/", TypeAppLink, TypeLink},
		{"example.com", TypeAppLink, TypeLink},
		{"javascript://example.com", TypeAppLink, TypeText},
		{"# Route\n\n[Video](aweme://video/123)", TypeAppLink, TypeMarkdown},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			store := NewStore(1)
			item, ok := store.CaptureProtectedContent(Content{Type: tc.given, Text: tc.raw}, "Fixture", []string{"editor.app"})
			if !ok || item.Type != tc.want || item.Text != tc.raw || !reflect.DeepEqual(item.AllowedApps, []string{"editor.app"}) {
				t.Fatalf("normalization = %+v, %v; want %s and unchanged original/scope", item, ok, tc.want)
			}
		})
	}
}

func TestAppLinkLocalizedTypeSearchPreservesLinkQueries(t *testing.T) {
	store := NewStore(3)
	app, _ := store.Capture("aweme://video/123", "Fixture", false)
	_, _ = store.Capture("https://example.com/", "Fixture", false)
	_, _ = store.Capture("# Article\n\nBody.", "Fixture", false)
	before := store.Items()
	for _, query := range []string{"app-link", "app link", "scheme", "deep link", "应用链接", "應用連結", "アプリリンク"} {
		items := store.Search(query)
		if len(items) != 1 || items[0].ID != app.ID || items[0].Type != TypeAppLink {
			t.Fatalf("application query %q = %+v", query, items)
		}
	}
	for _, query := range []string{"link", "url", "链接", "連結", "リンク"} {
		if got := len(store.Search(query)); got != 2 {
			t.Fatalf("general link query %q returned %d items", query, got)
		}
	}
	if !reflect.DeepEqual(before, store.Items()) {
		t.Fatal("type search mutated history")
	}
}
