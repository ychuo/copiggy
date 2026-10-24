package contentactions

import (
	"errors"
	"testing"

	"copiggy/internal/clipmodel"
)

func TestLinkTargetDerivesOnlyCompleteRecognizedDestination(t *testing.T) {
	for _, tc := range []struct {
		raw, target string
		kind        clipmodel.ContentType
	}{
		{"example.com/a?q=one#part", "https://example.com/a?q=one#part", clipmodel.TypeLink},
		{"https://example.com/a?q=one#part", "https://example.com/a?q=one#part", clipmodel.TypeLink},
		{"aweme://video/123", "aweme://video/123", clipmodel.TypeAppLink},
		{"sslocal://route?url=https%3A%2F%2Fexample.com", "sslocal://route?url=https%3A%2F%2Fexample.com", clipmodel.TypeAppLink},
		{"[Example](https://example.com)", "https://example.com", clipmodel.TypeLink},
		{"[Video](aweme://video/123?q=one#part)", "aweme://video/123?q=one#part", clipmodel.TypeAppLink},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			item := clipmodel.Item{Type: tc.kind, Text: tc.raw, AllowedApps: []string{"editor.app"}}
			target, err := OpenItemTarget(item)
			if err != nil || target.URL != tc.target || target.Path != "" || target.Calendar != nil || item.Text != tc.raw {
				t.Fatalf("target = %+v, %v", target, err)
			}
			item.Type = clipmodel.TypeText
			if _, err := OpenItemTarget(item); !errors.Is(err, ErrUnsupported) {
				t.Fatal("mismatched saved content type accepted")
			}
			item.Type = clipmodel.TypeAppLink
			if tc.kind == clipmodel.TypeAppLink {
				item.Type = clipmodel.TypeLink
			}
			if _, err := OpenItemTarget(item); !errors.Is(err, ErrUnsupported) {
				t.Fatal("web and application link types were interchangeable")
			}
		})
	}
	for _, raw := range []string{"javascript://example.com", "data://example.com", "README.md", "example.com next", "aweme://", "[Go](javascript:alert(1))"} {
		for _, kind := range []clipmodel.ContentType{clipmodel.TypeLink, clipmodel.TypeAppLink} {
			if _, err := OpenItemTarget(clipmodel.Item{Type: kind, Text: raw}); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("unexpected action for %q: %v", raw, err)
			}
		}
	}
}
