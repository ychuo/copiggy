package linkicons

import (
	"bytes"
	"context"
	"copiggy/internal/clipmodel"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type iconRoundTripFunc func(*http.Request) (*http.Response, error)

func (f iconRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// httptest handlers run entirely in memory. There are no sockets, DNS lookups
// or public requests, and the production private-address policy stays intact.
func localIconTestClient(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	return &http.Client{Transport: iconRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	}), Timeout: time.Second}
}

func TestLinkIconOriginNeverContainsCopiedPathOrCredentials(t *testing.T) {
	for _, raw := range []string{"https://Example.COM/private/document?token=secret#fragment", "https://example.com/"} {
		got, err := linkIconOrigin(raw)
		if err != nil || got != "https://example.com/" {
			t.Fatalf("origin %q = %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"file:///etc/hosts", "data:image/png;base64,a", "https://user:pass@example.com/", "http://127.0.0.1/", "http://[::ffff:127.0.0.1]/", "http://localhost/", "http://service.internal/", "http://machine.local/", "https://example.com:8080/"} {
		if got, err := linkIconOrigin(raw); err == nil {
			t.Fatalf("unsafe origin accepted: %q => %q", raw, got)
		}
	}
	for _, raw := range []string{"10.1.2.3", "172.16.2.3", "192.168.0.1", "169.254.169.254", "100.64.0.1", "::1", "fc00::1", "fe80::1", "2001:db8::1", "0.0.0.0"} {
		if publicIconAddress(netip.MustParseAddr(raw)) {
			t.Fatalf("unsafe address %s accepted", raw)
		}
	}
	if !publicIconAddress(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("public address rejected")
	}
}

func TestLinkIconsParseHTMLMetadataAndSkipExecutableMarkup(t *testing.T) {
	source := []byte(`<!-- <link rel=icon href=/comment.png> --><script>"<link rel=icon href=/script.png>"</script><base href="http://127.0.0.1"><link rel='apple-touch-icon' href='/apple.png'><LINK rel="shortcut ICON" disabled href="/images/icon.png?v=2&amp;a=3" data-label="a > b"><link rel=icon href=//cdn.example.com/favicon.png><link rel=icon href="javascript:alert(1)">`)
	got := iconLocations("https://example.com/", source)
	want := []string{"https://example.com/images/icon.png", "https://cdn.example.com/favicon.png", "https://example.com/apple.png"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("icon metadata = %v; want %v", got, want)
	}
}

func TestLinkIconFetchesOriginThenHTMLIconAndCaches(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	pngBytes := encodedTestPNG(t, 32, 32)
	client := localIconTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.RequestURI())
		mu.Unlock()
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Referer") != "" {
			t.Error("unexpected credentials on icon request")
		}
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<link rel="icon" href="/brand.png">`))
		case "/brand.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes)
		default:
			t.Errorf("unexpected URL %s", r.URL)
			http.NotFound(w, r)
		}
	})
	done := make(chan struct{}, 2)
	cache := newCacheWithClient(client, func() { done <- struct{}{} })
	t.Cleanup(cache.Clear)
	if cache.Lookup(clipmodel.Item{Type: clipmodel.TypeLink, Text: "https://example.com/private?token=secret#section"}, nil) != nil {
		t.Fatal("initial lookup must be asynchronous")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("icon lookup did not finish")
	}
	if cache.Lookup(clipmodel.Item{Type: clipmodel.TypeLink, Text: "https://example.com/another"}, nil) == nil {
		t.Fatal("origin cache not reused")
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(paths, "|") != "/|/brand.png" {
		t.Fatalf("requests = %v", paths)
	}
}

func TestLinkIconCapturedHTMLAndFaviconFallback(t *testing.T) {
	pngBytes := encodedTestPNG(t, 16, 16)
	var paths []string
	client := localIconTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/favicon.ico" {
			w.Write(pngBytes)
			return
		}
		http.NotFound(w, r)
	})
	bitmap := fetchLinkIcon(context.Background(), "https://example.com/", []byte(`<link rel=icon href=/missing.png>`), client)
	if bitmap == nil || strings.Join(paths, "|") != "/missing.png|/favicon.ico" {
		t.Fatalf("fallback failed: bitmap=%v paths=%v", bitmap, paths)
	}
}

func TestLinkIconRejectsOversizedResourcesAndPixels(t *testing.T) {
	client := localIconTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write(bytes.Repeat([]byte("a"), linkHTMLMaxBytes+1)) })
	if _, err := readIconResource(context.Background(), client, "https://example.com/", linkHTMLMaxBytes, "text/html"); err == nil {
		t.Fatal("oversized HTML accepted")
	}
	if _, err := decodeLinkIcon(withPNGDimensions(encodedTestPNG(t, 1, 1), 2000, 2000)); err == nil {
		t.Fatal("oversized decoded icon accepted")
	}
	if _, err := decodeLinkIcon([]byte(`<svg onload=alert(1)>`)); err == nil {
		t.Fatal("active/unsupported SVG accepted")
	}
}

func TestLinkIconDecodesPNGAndDIBICO(t *testing.T) {
	pngBytes := encodedTestPNG(t, 16, 16)
	wrap := func(frame []byte) []byte {
		ico := make([]byte, 22+len(frame))
		copy(ico[:6], []byte{0, 0, 1, 0, 1, 0})
		ico[6] = 16
		ico[7] = 16
		binary.LittleEndian.PutUint32(ico[14:18], uint32(len(frame)))
		binary.LittleEndian.PutUint32(ico[18:22], 22)
		copy(ico[22:], frame)
		return ico
	}
	if _, err := decodeLinkIcon(wrap(pngBytes)); err != nil {
		t.Fatalf("embedded PNG ICO: %v", err)
	}
	dib := make([]byte, 40+16*16*4+4*16)
	binary.LittleEndian.PutUint32(dib[:4], 40)
	binary.LittleEndian.PutUint32(dib[4:8], 16)
	binary.LittleEndian.PutUint32(dib[8:12], 32)
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	for i := 40; i < 40+16*16*4; i += 4 {
		dib[i+2] = 90
		dib[i+3] = 255
	}
	decoded, err := decodeIconDirectory(wrap(dib))
	if err != nil {
		t.Fatalf("DIB ICO: %v", err)
	}
	r, _, _, a := decoded.At(0, 0).RGBA()
	if r != 90*257 || a != 65535 {
		t.Fatalf("DIB color/alpha = %d/%d", r, a)
	}
}

func TestLinkIconCacheBoundAndCancel(t *testing.T) {
	client := localIconTestClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	cache := newCacheWithClient(client, nil)
	for i := 0; i < linkIconMaxEntries+10; i++ {
		cache.Lookup(clipmodel.Item{Type: clipmodel.TypeLink, Text: "https://site" + strconv.Itoa(i) + ".example.com/"}, nil)
	}
	cache.mu.Lock()
	count := len(cache.entries)
	cache.mu.Unlock()
	if count != linkIconMaxEntries {
		t.Fatalf("unbounded cache: %d", count)
	}
	cache.Clear()
	cache.mu.Lock()
	count = len(cache.entries)
	cache.mu.Unlock()
	if count != 0 {
		t.Fatal("cache not cleared")
	}
}

func TestLinkIconDownsizesButPreservesAspectAndRejectsTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 128, 64))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	png.Encode(&encoded, img)
	bitmap, err := decodeLinkIcon(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	w, h := bitmap.Bounds().Dx(), bitmap.Bounds().Dy()
	if w != 64 || h != 32 {
		t.Fatalf("size=%d×%d", w, h)
	}
	encoded.Reset()
	png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 16, 16)))
	if _, err := decodeLinkIcon(encoded.Bytes()); err == nil {
		t.Fatal("fully transparent icon accepted")
	}
}

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

func withPNGDimensions(encoded []byte, width, height uint32) []byte {
	changed := bytes.Clone(encoded)
	binary.BigEndian.PutUint32(changed[16:20], width)
	binary.BigEndian.PutUint32(changed[20:24], height)
	binary.BigEndian.PutUint32(changed[29:33], crc32.ChecksumIEEE(changed[12:29]))
	return changed
}

func TestLookupRejectsProtectedRecordsBeforeCacheOrNetwork(t *testing.T) {
	client := localIconTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("protected clipboard content caused a request")
	})
	cache := newCacheWithClient(client, nil)
	item := clipmodel.Item{Type: clipmodel.TypeLink, Text: "https://example.com/private", AllowedApps: []string{"source.app"}}
	if cache.Lookup(item, []byte(`<link rel=icon href=/private-icon.png>`)) != nil {
		t.Fatal("protected item returned artwork")
	}
	if len(cache.entries) != 0 {
		t.Fatal("protected item entered the asynchronous cache")
	}
	cache.entries["https://example.com/"] = &linkIconEntry{bitmap: image.NewNRGBA(image.Rect(0, 0, 1, 1)), expires: time.Now().Add(time.Hour)}
	if cache.Lookup(item, nil) != nil || cache.nextUse != 0 {
		t.Fatal("protected item consulted a warmed public cache")
	}
	item.AllowedApps = nil
	item.Type = clipmodel.TypeText
	if cache.Lookup(item, nil) != nil {
		t.Fatal("non-link content requested artwork")
	}
}
