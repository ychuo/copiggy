package clipmodel

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLinkRecognizesExplicitInferredAndAppDestinations(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  LinkValue
	}{
		{"https://example.com/a?q=one#part", LinkValue{URL: "https://example.com/a?q=one#part", Scheme: "https", Host: "example.com"}},
		{" HTTP://Example.COM:8080/a%20b?next=two#part \n", LinkValue{URL: "HTTP://Example.COM:8080/a%20b?next=two#part", Scheme: "http", Host: "Example.COM"}},
		{"http://localhost:3000/", LinkValue{URL: "http://localhost:3000/", Scheme: "http", Host: "localhost"}},
		{"https://[::1]/", LinkValue{URL: "https://[::1]/", Scheme: "https", Host: "::1"}},
		{"https://user:pass@example.com/", LinkValue{URL: "https://user:pass@example.com/", Scheme: "https", Host: "example.com"}},
		{"example.com", LinkValue{URL: "https://example.com", Scheme: "https", Host: "example.com", Inferred: true}},
		{"www.example.com/path?a=1#part", LinkValue{URL: "https://www.example.com/path?a=1#part", Scheme: "https", Host: "www.example.com", Inferred: true}},
		{"EXAMPLE.CO.UK:8443/a%20b", LinkValue{URL: "https://example.co.uk:8443/a%20b", Scheme: "https", Host: "example.co.uk", Inferred: true}},
		{"bücher.de/lesen", LinkValue{URL: "https://xn--bcher-kva.de/lesen", Scheme: "https", Host: "xn--bcher-kva.de", Inferred: true}},
		{"例子.中国", LinkValue{URL: "https://xn--fsqu00a.xn--fiqs8s", Scheme: "https", Host: "xn--fsqu00a.xn--fiqs8s", Inferred: true}},
		{"alex.github.io/blog", LinkValue{URL: "https://alex.github.io/blog", Scheme: "https", Host: "alex.github.io", Inferred: true}},
		{"aweme://aweme/detail/123?refer=web#video", LinkValue{URL: "aweme://aweme/detail/123?refer=web#video", Scheme: "aweme", Host: "aweme", App: true}},
		{"SSLOCAL://route?url=https%3A%2F%2Fexample.com", LinkValue{URL: "SSLOCAL://route?url=https%3A%2F%2Fexample.com", Scheme: "sslocal", Host: "route", App: true}},
		{"my-app+v2:///route", LinkValue{URL: "my-app+v2:///route", Scheme: "my-app+v2", App: true}},
		{"[Example](https://example.com/a_(b))", LinkValue{URL: "https://example.com/a_(b)", Scheme: "https", Host: "example.com", Title: "Example"}},
		{"[Read this](<https://example.com/page>)", LinkValue{URL: "https://example.com/page", Scheme: "https", Host: "example.com", Title: "Read this"}},
		{"[Example](example.com)", LinkValue{URL: "https://example.com", Scheme: "https", Host: "example.com", Title: "Example", Inferred: true}},
		{"[Video](aweme://video/123)", LinkValue{URL: "aweme://video/123", Scheme: "aweme", Host: "video", Title: "Video", App: true}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseLink(tc.input)
			if got == nil || *got != tc.want {
				t.Fatalf("ParseLink(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
			analysis := Analyze(tc.input)
			kind := TypeLink
			if tc.want.App {
				kind = TypeAppLink
			}
			if analysis.Type != kind || analysis.Link == nil || *analysis.Link != tc.want {
				t.Fatalf("link analysis = %+v", analysis)
			}
		})
	}
}

func TestLinkRejectsAmbiguousTextUnsafeSchemesAndMalformedDestinations(t *testing.T) {
	for _, raw := range []string{
		"", "example", "www.example", "example.notarealtld", "localhost:3000", "192.168.0.1", "1.2.3", "v1.2.3",
		"README.md", "package.json", "archive.zip", "movie.mov", "setup.sh", "main.rs", "report.pdf",
		"com", "co.uk", "github.io", "example.com.", "-bad.com", "bad-.com", "bad..com", "bad_domain.com", strings.Repeat("a", 64) + ".com",
		"user@example.com", "user:pass@example.com", "example.com:0", "example.com:65536", "example.com:port", "example.com:",
		"see example.com", "example.com and example.org", "example.com/a\u200bvalue",
		"https://", "https:///page", "https://example.com/a b", "https://example.com/\tpath", "https://example.com/\x00", "https://example.com/\\file",
		"javascript:alert(1)", "javascript://example.com", "data://example.com", "vbscript://example.com", "file://localhost/tmp/a", "mailto://alex@example.com", "tel://123456",
		"aweme://", "aweme://?id=1", "aweme://#id", "aweme:route", "aweme://user:password@route", "aweme://route/\nother", "aweme://route/%zz",
		"[Example](https://example.com) then text", "text [Example](https://example.com)", "[Example](https://example.com) [Other](https://example.org)",
		"[](https://example.com)", "[Nested [label]](https://example.com)", "[Example](https://example.com \"title\")", "[Example](https://example.com/a_(b)",
		"https://example.com/" + strings.Repeat("a", 8192),
	} {
		t.Run(raw, func(t *testing.T) {
			if got := ParseLink(raw); got != nil {
				t.Fatalf("unexpected link: %+v", got)
			}
		})
	}
	if got := ParseLink("example.com\u00a0"); got == nil || !got.Inferred {
		t.Fatal("trailing Unicode whitespace should not affect recognition")
	}
	if got := ParseLink("example.com/a\u00a0b"); got != nil {
		t.Fatal("internal Unicode whitespace accepted")
	}
}

func TestLinkRestoreRetainsExactSourceRepresentationsAndProtection(t *testing.T) {
	for _, raw := range []string{"  example.com/path?q=one#part \n", "[Example](https://example.com/path)"} {
		t.Run(raw, func(t *testing.T) {
			data := TextData(raw)
			before := data.Clone()
			saved := Item{ID: "item-7", Type: TypeText, Text: raw, SourceApp: "Editor", SourceAppID: "editor.app", AllowedApps: []string{"editor.app"}, CreatedAt: time.Now()}
			store, err := RestoreRecords([]Record{{Item: saved, Data: data}})
			if err != nil {
				t.Fatal(err)
			}
			item := store.Items()[0]
			if item.Type != TypeLink || item.Text != raw || item.ID != saved.ID || !reflect.DeepEqual(item.AllowedApps, saved.AllowedApps) || !reflect.DeepEqual(data, before) {
				t.Fatal("link recognition rewrote saved source or protection")
			}
			if _, selected := store.Current(); selected {
				t.Fatal("link reclassification acquired clipboard authority")
			}
		})
	}
}
