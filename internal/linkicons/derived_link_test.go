package linkicons

import (
	"net/http"
	"testing"

	"copiggy/internal/clipmodel"
)

func TestDerivedLinksDoNotScheduleAutomaticIconRequests(t *testing.T) {
	cache := newCacheWithClient(localIconTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("inferred or app link triggered an icon request")
	}), nil)
	t.Cleanup(cache.Clear)
	for _, raw := range []string{"example.com/private?q=secret", "aweme://video/123", "sslocal://route", "[Example](example.com/private)", "[Video](aweme://video/123)"} {
		for _, kind := range []clipmodel.ContentType{clipmodel.TypeLink, clipmodel.TypeAppLink} {
			if got := cache.Lookup(clipmodel.Item{Type: kind, Text: raw}, []byte(`<link rel=icon href=https://example.com/icon.png>`)); got != nil {
				t.Fatalf("unexpected cached icon for %q/%q", kind, raw)
			}
		}
	}
	// Even an inconsistent caller snapshot cannot let an application card
	// acquire website assets before it has been validated and reclassified.
	if got := cache.Lookup(clipmodel.Item{Type: clipmodel.TypeAppLink, Text: "https://example.com/"}, nil); got != nil {
		t.Fatal("application card accepted website assets")
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) != 0 {
		t.Fatal("inferred or app link scheduled icon work")
	}
}

func TestStandaloneMarkdownLinkIconUsesOnlyExplicitOrigin(t *testing.T) {
	origin, err := linkIconOrigin("[Article](https://example.com/private?q=secret#part)")
	if err != nil || origin != "https://example.com/" {
		t.Fatalf("icon origin = %q, %v", origin, err)
	}
}
