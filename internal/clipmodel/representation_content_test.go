package clipmodel

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func encodedTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 32, G: 96, B: 192, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func pngClipboardData(encoded []byte) Representations {
	return testRepresentations(testRepresentationItem(testRepresentation(PNGFormat, encoded)))
}

// A valid PNG header advertising oversized dimensions must be rejected before
// image allocation. Its IDAT intentionally remains the original small image.
func withPNGDimensions(encoded []byte, width, height uint32) []byte {
	changed := bytes.Clone(encoded)
	binary.BigEndian.PutUint32(changed[16:20], width)
	binary.BigEndian.PutUint32(changed[20:24], height)
	binary.BigEndian.PutUint32(changed[29:33], crc32.ChecksumIEEE(changed[12:29]))
	return changed
}

func TestClipboardContentPurePNGMetadataAndStore(t *testing.T) {
	encoded := encodedTestPNG(t, 37, 19)
	data := pngClipboardData(encoded)
	content, err := ContentFromRepresentations(data)
	if err != nil {
		t.Fatal(err)
	}
	if content.Type != TypeImage || content.Text != "" || content.Image == nil ||
		content.Image.MIME != "image/png" || content.Image.Width != 37 || content.Image.Height != 19 ||
		content.Image.ByteSize != len(encoded) {
		t.Fatalf("incorrect pure PNG content: %+v", content)
	}
	store := NewStore(10)
	item, recorded := store.CaptureContent(content, "Preview", false)
	if !recorded || item.Type != TypeImage || item.Image.Width != 37 {
		t.Fatalf("valid PNG was not captured in history: %+v", item)
	}
	original, err := data.Read(PNGFormat)
	if err != nil || !bytes.Equal(original, encoded) {
		t.Fatal("reading PNG metadata changed its original representation")
	}
}

func TestClipboardContentRejectsFullyTransparentImage(t *testing.T) {
	for _, size := range []int{1, 12} {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, size, size))); err != nil {
			t.Fatal(err)
		}
		content, err := ContentFromRepresentations(pngClipboardData(encoded.Bytes()))
		if err == nil || content.Image != nil {
			t.Fatalf("fully transparent %d×%d PNG becomes an empty image card: %+v, %v", size, size, content, err)
		}
	}
}

func TestClipboardContentPlaceholderPrefersHTMLTextAndRetainsData(t *testing.T) {
	for _, opaque := range []bool{false, true} {
		img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		if opaque {
			img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			t.Fatal(err)
		}
		html := []byte(`<html><head><title>Do not copy this title</title></head><body><p>Copied &amp; useful</p><script>not content</script><div>second line</div></body></html>`)
		data := testRepresentations(testRepresentationItem(testRepresentation(PNGFormat, encoded.Bytes()), testRepresentation(HTMLFormat, html)))
		content, err := ContentFromRepresentations(data)
		if err != nil || content.Text != "Copied & useful\nsecond line" || content.Image != nil {
			t.Fatalf("HTML + 1×1 placeholder should produce useful text: %+v, %v", content, err)
		}
		retained, err := data.Read(PNGFormat)
		if err != nil || !bytes.Equal(retained, encoded.Bytes()) {
			t.Fatal("choosing a useful preview changed the saved PNG representation")
		}
		if got, err := data.Fingerprint(); err != nil || got != content.Fingerprint {
			t.Fatal("choosing HTML changed the original representations' fingerprint")
		}
	}
}

func TestClipboardContentKeepsIntentionalOpaqueSinglePixel(t *testing.T) {
	content, err := ContentFromRepresentations(pngClipboardData(encodedTestPNG(t, 1, 1)))
	if err != nil || content.Image == nil || content.Image.Width != 1 || content.Image.Height != 1 {
		t.Fatalf("an intentional visible 1×1 image must remain supported: %+v, %v", content, err)
	}
}

func TestClipboardContentTextThenStoreClassifiesJSON(t *testing.T) {
	for _, example := range []struct {
		text string
		kind ContentType
	}{
		{"a plain text note", TypeText},
		{`{"n":9007199254740993,"enabled":true}`, TypeJSON},
		{"https://example.com/copied", TypeLink},
	} {
		t.Run(string(example.kind), func(t *testing.T) {
			content, err := ContentFromRepresentations(TextData(example.text))
			if err != nil || content.Text != example.text {
				t.Fatalf("text changed while materializing: %+v, %v", content, err)
			}
			item, recorded := NewStore(10).CaptureContent(content, "Editor", false)
			if !recorded || item.Type != example.kind || item.Text != example.text {
				t.Fatalf("store classified the content incorrectly: %+v", item)
			}
		})
	}
}

func TestClipboardContentBoundsAllRepresentationsAndJoinedText(t *testing.T) {
	t.Run("text exact limit", func(t *testing.T) {
		text := strings.Repeat("a", 1<<20)
		content, err := ContentFromRepresentations(TextData(text))
		if err != nil || content.Text != text {
			t.Fatalf("text at its exact byte limit was rejected: %v", err)
		}
	})
	t.Run("joined text includes separators", func(t *testing.T) {
		part := bytes.Repeat([]byte("a"), 1<<19)
		data := testRepresentations(
			testRepresentationItem(testRepresentation(TextFormat, part)),
			testRepresentationItem(testRepresentation(TextFormat, part)),
		)
		if _, err := ContentFromRepresentations(data); err == nil {
			t.Fatal("joined text exceeded 1 MiB after its inserted newline")
		}
	})
	for _, extra := range []int{0, 1} {
		t.Run(map[bool]string{true: "all representations exact limit", false: "all representations over limit"}[extra == 0], func(t *testing.T) {
			data := testRepresentations(
				testRepresentationItem(testRepresentation(TextFormat, []byte("a"))),
				testRepresentationItem(testRepresentation(HTMLFormat, make([]byte, int(MaxClipboardBytes)-1+extra))),
			)
			content, err := ContentFromRepresentations(data)
			if extra == 0 {
				if err != nil || content.Text != "a" {
					t.Fatalf("all representations at their limit should be accepted: %v", err)
				}
			} else if err == nil {
				t.Fatal("oversized alternate representation was ignored")
			}
		})
	}
	t.Run("oversized PNG", func(t *testing.T) {
		if _, err := ContentFromRepresentations(pngClipboardData(make([]byte, int(MaxClipboardBytes)+1))); err == nil {
			t.Fatal("PNG exceeding the raw data limit was accepted")
		}
	})
}

func TestClipboardContentRejectsMalformedAndOversizedPNG(t *testing.T) {
	encoded := encodedTestPNG(t, 8, 6)
	tiffBytes := []byte{0x49, 0x49, 0x2a, 0x00, 0x08, 0x00, 0x00, 0x00}
	for _, example := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"invalid signature", []byte("not an image")},
		{"header without image data", encoded[:33]},
		{"truncated final chunk", encoded[:len(encoded)-8]},
		{"TIFF mislabeled as PNG", tiffBytes},
		{"too many pixels", withPNGDimensions(encoded, 6001, 4000)},
		{"huge dimensions", withPNGDimensions(encoded, 2147483647, 2147483647)},
	} {
		t.Run(example.name, func(t *testing.T) {
			if _, err := ContentFromRepresentations(pngClipboardData(example.data)); err == nil {
				t.Fatal("invalid or oversized PNG was accepted")
			}
		})
	}
}

func TestClipboardDuplicatePNGUsesOriginalBytes(t *testing.T) {
	encoded := encodedTestPNG(t, 32, 20)
	firstContent, err := ContentFromRepresentations(pngClipboardData(encoded))
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(10)
	first, _ := store.CaptureContent(firstContent, "Preview", false)
	copyContent, err := ContentFromRepresentations(pngClipboardData(bytes.Clone(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	copy, _ := store.CaptureContent(copyContent, "Browser", false)
	if copy.ID != first.ID || len(store.Items()) != 1 || copy.SourceApp != "Preview" {
		t.Fatalf("exact image recopy did not retain its original record: %+v", copy)
	}
	otherImage := image.NewNRGBA(image.Rect(0, 0, 32, 20))
	otherImage.SetNRGBA(0, 0, color.NRGBA{R: 192, G: 96, B: 32, A: 255})
	var otherBytes bytes.Buffer
	if err := png.Encode(&otherBytes, otherImage); err != nil {
		t.Fatal(err)
	}
	otherContent, err := ContentFromRepresentations(pngClipboardData(otherBytes.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := store.CaptureContent(otherContent, "Preview", false)
	if other.ID == first.ID || len(store.Items()) != 2 {
		t.Fatal("two images with equal dimensions but different pixels were merged")
	}
}

func TestClipboardDuplicateIncludesAlternateRepresentations(t *testing.T) {
	text := testRepresentation(TextFormat, []byte("hello"))
	html := testRepresentation(HTMLFormat, []byte("<b>hello</b>"))
	store := NewStore(10)
	capture := func(data Representations) Item {
		t.Helper()
		content, err := ContentFromRepresentations(data)
		if err != nil {
			t.Fatal(err)
		}
		item, recorded := store.CaptureContent(content, "Editor", false)
		if !recorded {
			t.Fatal("valid in-memory content was not captured")
		}
		return item
	}
	first := capture(testRepresentations(testRepresentationItem(text, html)))
	copy := capture(testRepresentations(testRepresentationItem(html, text)))
	if copy.ID != first.ID || len(store.Items()) != 1 {
		t.Fatal("representation enumeration order incorrectly prevented deduplication")
	}
	otherHTML := capture(testRepresentations(testRepresentationItem(text, testRepresentation(HTMLFormat, []byte("<i>hello</i>")))))
	plain := capture(TextData("hello"))
	if otherHTML.ID == first.ID || plain.ID == first.ID || len(store.Items()) != 3 {
		t.Fatal("matching preview text merged different retained representations")
	}
	plainDirect, _ := store.Capture("hello", "Another App", false)
	if plainDirect.ID != plain.ID {
		t.Fatal("adapter and direct plain text did not share content identity")
	}
	imageA := testRepresentation(PNGFormat, encodedTestPNG(t, 10, 12))
	imageB := testRepresentation(PNGFormat, encodedTestPNG(t, 12, 10))
	withImageA := capture(testRepresentations(testRepresentationItem(text, imageA)))
	withImageB := capture(testRepresentations(testRepresentationItem(text, imageB)))
	if withImageA.ID == withImageB.ID || withImageA.ID == plain.ID {
		t.Fatal("a non-preview image representation was omitted from duplicate identity")
	}
}

func TestClipboardDuplicatePreservesItemBoundariesAndOrder(t *testing.T) {
	item := func(value string) []Representation {
		return testRepresentationItem(testRepresentation(TextFormat, []byte(value)))
	}
	store := NewStore(10)
	for _, data := range []Representations{
		testRepresentations(item("a"), item("b\nc")),
		testRepresentations(item("a\nb"), item("c")),
		TextData("a\nb\nc"),
		testRepresentations(item("b\nc"), item("a")),
	} {
		content, err := ContentFromRepresentations(data)
		if err != nil {
			t.Fatal(err)
		}
		store.CaptureContent(content, "Editor", false)
	}
	if len(store.Items()) != 4 {
		t.Fatal("multi-item copy identity lost the original item boundaries or order")
	}
	content, err := ContentFromRepresentations(testRepresentations(item("a"), item("b\nc")))
	if err != nil {
		t.Fatal(err)
	}
	oldest := store.Items()[3]
	copy, _ := store.CaptureContent(content, "Browser", false)
	if copy.ID != oldest.ID || len(store.Items()) != 4 {
		t.Fatal("identical multi-item recopy failed to refresh the original")
	}
}

func testRepresentation(format string, data []byte) Representation {
	return Representation{Format: format, Bytes: data}
}
func testRepresentationItem(reps ...Representation) []Representation { return reps }
func testRepresentations(items ...[]Representation) Representations  { return items }
