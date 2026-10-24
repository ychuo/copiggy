package linkmetadata

import (
	"context"
	"net/http"
	"testing"
)

func TestDerivedLinksNeverFetchMetadataAutomatically(t *testing.T) {
	fetcher := fakeFetcher(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("inferred or app link triggered metadata request")
		return nil, nil
	})
	fetcher.lookup = nil // Any attempt to resolve a derived destination is a bug.
	for _, raw := range []string{"example.com/private?q=secret", "[Example](example.com/private)", "aweme://video/123", "sslocal://route", "[Video](aweme://video/123)"} {
		if CanFetch(raw) {
			t.Errorf("derived URL eligible for background fetch: %q", raw)
		}
		if _, err := fetcher.Fetch(context.Background(), raw); err == nil {
			t.Errorf("derived URL fetch accepted: %q", raw)
		}
	}
	for _, raw := range []string{"aweme://video/123", "[Video](aweme://video/123)"} {
		if got := Extract(raw, []byte(`<title>Unrelated website</title><meta property=og:image content=https://example.com/cover.png>`)); got.Title != "" || got.Cover != nil {
			t.Errorf("application link accepted website metadata: %+v", got)
		}
	}
}

func TestStandaloneMarkdownLinkMetadataUsesDerivedExplicitDestination(t *testing.T) {
	location, err := metadataURL("[Article](https://example.com/article?q=one#part)")
	if err != nil || location.String() != "https://example.com/article?q=one" {
		t.Fatalf("metadata URL = %v, %v", location, err)
	}
	markup := []byte(`<link rel=canonical href=https://example.com/article?q=one><title>Copied page title</title>`)
	for _, raw := range []string{"[Article](https://example.com/article?q=one#part)", "example.com/article?q=one"} {
		metadata := Extract(raw, markup)
		if metadata.Title != "Copied page title" {
			t.Fatalf("copied local metadata for %q = %+v", raw, metadata)
		}
	}
}
