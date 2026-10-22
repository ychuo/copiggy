package linkmetadata

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestCopiedMetadataRequiresMatchingPage(t *testing.T) {
	markup := []byte(`<title>Source page</title><meta property="og:title" content="Actual &amp; title"><meta property="og:url" content="https://example.com/article?q=1"><meta name="description" content="  Short description  ">`)
	metadata := Extract("https://example.com/article?q=1#section", markup)
	if metadata.Title != "Actual & title" || metadata.Description != "Short description" {
		t.Fatalf("associated metadata: %#v", metadata)
	}
	for _, other := range []string{"https://example.com/other?q=1", "https://example.com/article?q=2", "https://other.example/article?q=1"} {
		if got := Extract(other, markup); got.Title != "" || got.Description != "" || got.Cover != nil {
			t.Fatalf("unrelated title attached to %q: %#v", other, got)
		}
	}
}

func TestMatchingAnchorUsesItsOwnText(t *testing.T) {
	markup := []byte(`<title>Unrelated source page</title><a href="https://example.com/article">A <b>useful</b> guide</a>`)
	got := Extract("https://example.com/article", markup)
	if got.Title != "A useful guide" || got.Description != "" {
		t.Fatalf("anchor metadata: %#v", got)
	}
}

func TestMetadataEntitiesDecodeOnce(t *testing.T) {
	for _, markup := range []string{
		`<meta property="og:url" content="https://example.com/"><meta property="og:title" content="Writing &amp;amp; in HTML">`,
		`<meta property="og:url" content="https://example.com/"><title>Writing &amp;amp; in HTML</title>`,
		`<a href="https://example.com/">Writing &amp;amp; in HTML</a>`,
	} {
		if got := Extract("https://example.com", []byte(markup)); got.Title != "Writing &amp; in HTML" {
			t.Fatalf("entities decoded more than once: %q", got.Title)
		}
	}
}

func TestCanonicalAndClipboardHeaderAssociation(t *testing.T) {
	for _, markup := range []string{
		`<link rel="canonical" href="https://example.com/article"><title>Guide</title>`,
		"Version:1.0\r\nSourceURL:https://example.com/article\r\n<html><title>Guide</title></html>",
	} {
		if got := Extract("https://example.com/article", []byte(markup)); got.Title != "Guide" {
			t.Fatalf("missing associated title: %#v", got)
		}
	}
	for _, markup := range []string{
		`<script><meta property="og:url" content="https://example.com/article"></script><title>Wrong</title>`,
		"<p>Version:1.0\nSourceURL:https://example.com/article</p><title>Wrong</title>",
		`<!-- <meta property="og:url" content="https://example.com/article"> --><title>Wrong</title>`,
	} {
		if got := Extract("https://example.com/article", []byte(markup)); got.Title != "" {
			t.Fatalf("body or code established identity: %#v", got)
		}
	}
}

func TestEmbeddedCoverOnlyAndBounded(t *testing.T) {
	bitmap := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	bitmap.SetNRGBA(3, 2, color.NRGBA{R: 127, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, bitmap); err != nil {
		t.Fatal(err)
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	markup := `<meta property="og:url" content="https://example.com/article"><meta property="og:image" content="` + dataURI + `">`
	got := Extract("https://example.com/article", []byte(markup))
	if got.Cover == nil || got.Cover.Bounds().Dx() != 8 {
		t.Fatal("valid local cover unavailable")
	}
	if got := Extract("https://example.com/other", []byte(markup)); got.Cover != nil {
		t.Fatal("unrelated page cover attached")
	}
	for _, invalid := range []string{
		"https://example.com/private.jpg?token=secret",
		"file:///Users/alex/photo.png",
		"data:image/svg+xml;base64,PHN2Zy8+",
		"data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(maxCoverBytes)+1),
	} {
		if decodeCover(invalid) != nil {
			t.Fatalf("accepted unsupported cover %q", invalid[:min(50, len(invalid))])
		}
	}
}

func TestTransparentPlaceholderCoverIgnored(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	if got := decodeCover("data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())); got != nil {
		t.Fatal("transparent cover accepted")
	}
}

func TestMetadataInputBounds(t *testing.T) {
	if got := Extract("https://example.com", bytes.Repeat([]byte("x"), maxHTMLBytes+1)); got.Title != "" {
		t.Fatal("oversized HTML accepted")
	}
	if got := cleanText(" <guide>\x00\n"+strings.Repeat("界", 300), 180); len([]rune(got)) != 180 || strings.ContainsRune(got, 0) {
		t.Fatal("title not bounded or control characters retained")
	}
	if got := Extract("https://user:password@example.com", []byte(`<title>Private</title>`)); got.Title != "" {
		t.Fatal("credential URL metadata accepted")
	}
}
