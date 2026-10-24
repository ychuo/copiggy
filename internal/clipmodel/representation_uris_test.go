package clipmodel

import (
	"reflect"
	"strings"
	"testing"
)

func TestClipboardWebURIListsKeepWholeValuesAndOriginalBytes(t *testing.T) {
	for _, tc := range []struct {
		name, raw, text string
		kind            ContentType
	}{
		{"single URL", "https://example.com/a?q=1#section", "https://example.com/a?q=1#section", TypeLink},
		{"comments and CRLF", "# Saved address\r\n\r\nhttps://example.com/a%20b?q=1#part\r\n", "https://example.com/a%20b?q=1#part", TypeLink},
		{"multiple URLs", "https://example.com/one\r\n# Next\r\nhttp://example.org/two\r\n", "https://example.com/one\nhttp://example.org/two", TypeText},
		{"scheme casing", "HTTPS://EXAMPLE.COM/path", "HTTPS://EXAMPLE.COM/path", TypeLink},
		{"local URL", "http://localhost:8080/path", "http://localhost:8080/path", TypeLink},
		{"IPv6 URL", "http://[::1]:8080/path", "http://[::1]:8080/path", TypeLink},
		{"Unicode URL", "https://例子.中国/路径", "https://例子.中国/路径", TypeLink},
		{"application URL", "# Video\r\naweme://video/123?refer=web\r\n", "aweme://video/123?refer=web", TypeAppLink},
		{"application route", "sslocal://route?url=https%3A%2F%2Fexample.com", "sslocal://route?url=https%3A%2F%2Fexample.com", TypeAppLink},
		{"mixed web and application", "https://example.com/one\naweme://video/123", "https://example.com/one\naweme://video/123", TypeText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := Representations{{{Format: URIListFormat, Bytes: []byte(tc.raw)}}}
			before := data.Clone()
			content, err := ContentFromRepresentations(data)
			if err != nil || content.Type != tc.kind || content.Text != tc.text || content.Image != nil {
				t.Fatalf("URI list = %+v, %v; want %q/%q", content, err, tc.kind, tc.text)
			}
			fingerprint, err := data.Fingerprint()
			if err != nil || content.Fingerprint != fingerprint || !reflect.DeepEqual(before, data) {
				t.Fatal("URI presentation changed saved bytes or fingerprint")
			}
		})
	}
	data := Representations{
		{{Format: URIListFormat, Bytes: []byte("https://example.com/one")}},
		{{Format: URIListFormat, Bytes: []byte("https://example.com/two")}},
	}
	content, err := ContentFromRepresentations(data)
	if err != nil || content.Type != TypeText || content.Text != "https://example.com/one\nhttps://example.com/two" {
		t.Fatalf("URI item boundaries lost: %+v, %v", content, err)
	}
}

func TestClipboardWebURIListsRejectInvalidOrMixedLists(t *testing.T) {
	for _, raw := range []string{
		"", "# Only a comment\r\n", "https://", "https:example.com", "example.com", "//example.com/path",
		"https://user:password@example.com/", "https://user@example.com/", "javascript:alert(1)", "data:text/plain,hello",
		"https://example.com/one\r\nfile:///tmp/a", "https://example.com/one\njavascript://example.com/two",
		"aweme://video/123\nfile:///tmp/a", "aweme://user:pass@video/123", "aweme://", "[Video](aweme://video/123)",
		"https://example.com/one\nnot a URL", " https://example.com/", "https://example.com/path with space",
		"https://example.com/\t", "https://example.com/\x00", "https://example.com/\u00a0", "https://example.com/\u2028",
		"https://example.com/a\rhttps://example.com/b", "https://example.com/\xff", "https://example.com/%invalid",
		"https://example.com:" + "notaport", "https://example.com/" + strings.Repeat("a", 8192),
		strings.Repeat("https://example.com/\r\n", maxClipboardURIEntries+1),
		"#" + strings.Repeat("comment", 1<<18) + "\nhttps://example.com/",
	} {
		data := Representations{{{Format: URIListFormat, Bytes: []byte(raw)}}}
		before := data.Clone()
		if content, err := ContentFromRepresentations(data); err == nil {
			t.Errorf("invalid URI list accepted: %q => %+v", raw[:min(len(raw), 90)], content)
		}
		if !reflect.DeepEqual(data, before) {
			t.Fatal("rejecting a URI list mutated its data")
		}
	}
}

func TestClipboardWebURIListsKeepExistingRepresentationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		rep        Representation
		image      bool
	}{
		{"plain text", "A selected label", Representation{Format: TextFormat, Bytes: []byte("A selected label")}, false},
		{"HTML text", "A copied label", Representation{Format: HTMLFormat, Bytes: []byte("<p>A copied label</p>")}, false},
		{"image", "", Representation{Format: PNGFormat, Bytes: encodedTestPNG(t, 2, 3)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := Representations{{{Format: URIListFormat, Bytes: []byte("https://example.com/")}, tc.rep}}
			content, err := ContentFromRepresentations(data)
			if err != nil || content.Text != tc.text || (content.Image != nil) != tc.image {
				t.Fatalf("URI list displaced an existing representation: %+v, %v", content, err)
			}
		})
	}
	data := Representations{{
		{Format: URIListFormat, Bytes: []byte("https://example.com/one\nfile:///tmp/a")},
		{Format: TextFormat, Bytes: []byte("Original mixed selection")},
	}}
	content, err := ContentFromRepresentations(data)
	if err != nil || content.Text != "Original mixed selection" {
		t.Fatalf("invalid URI list discarded valid plain text: %+v, %v", content, err)
	}
	data[0][0].Format = FileListFormat
	if _, err := ContentFromRepresentations(data); err == nil {
		t.Fatal("malformed declared Finder data was accepted by URI fallback")
	}
}
