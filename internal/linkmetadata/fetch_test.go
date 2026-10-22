package linkmetadata

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type metadataRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn metadataRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func fakeFetcher(t *testing.T, roundTrip metadataRoundTripFunc) *Fetcher {
	t.Helper()
	f := NewFetcher()
	f.lookup = func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		if network != "ip" || !strings.HasSuffix(host, ".") {
			t.Fatalf("DNS lookup must use an absolute host: %s %s", network, host)
		}
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}
	f.transport = roundTrip
	return f
}

func metadataResponse(req *http.Request, status int, mimeType string, content []byte) *http.Response {
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {mimeType}},
		Body: io.NopCloser(bytes.NewReader(content)), ContentLength: int64(len(content)), Request: req,
	}
}

func metadataPNG(t *testing.T, width, height int, alpha uint8) []byte {
	t.Helper()
	bitmap := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			bitmap.SetNRGBA(x, y, color.NRGBA{R: 80, G: 130, B: 200, A: alpha})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, bitmap); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestCanFetchRestrictsPublicWebURLs(t *testing.T) {
	for _, location := range []string{
		"https://www.apple.com/newsroom/page?language=en#details", "http://example.com/", "https://example.com:443/", "http://example.com:80/",
		"https://93.184.216.34/", "https://[2606:4700:4700::1111]/", "https://EXAMPLE.com./", "https://xn--bcher-kva.de/",
	} {
		if !CanFetch(location) {
			t.Errorf("public URL rejected: %s", location)
		}
	}
	for _, location := range []string{
		"file:///etc/passwd", "ftp://example.com/image.png", "https://user:secret@example.com/", "https://example.com:8080/", "https://example.com:80/",
		"http://localhost/", "http://LOCALHOST./", "http://service.local/", "https://service.internal/", "https://device.lan/", "https://host.test/",
		"http://127.0.0.1/", "http://127.1/", "http://2130706433/", "http://0.0.0.0/", "http://10.0.0.1/", "http://169.254.169.254/",
		"http://100.64.0.1/", "http://192.168.0.1/", "http://192.0.2.1/", "http://198.18.1.1/", "http://198.51.100.1/", "http://203.0.113.2/",
		"http://224.0.0.1/", "http://240.0.0.1/", "http://[::1]/", "http://[::ffff:127.0.0.1]/", "http://[fe80::1%25en0]/", "http://[fc00::1]/",
		"http://[64:ff9b::7f00:1]/", "http://[2001:db8::1]/", "http://[2002:7f00:1::1]/", "http://[3fff::1]/", "https://a..com/", "https://-bad.com/",
	} {
		if CanFetch(location) {
			t.Errorf("nonpublic URL accepted: %s", location)
		}
	}
}

func TestFetchPreservesPageURLAndUsesOpenGraphCover(t *testing.T) {
	var seen []string
	bitmap := metadataPNG(t, 1200, 600, 255)
	f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.String())
		if req.Method != http.MethodGet || req.Header.Get("User-Agent") != "Copiggy/1.0" || req.URL.Fragment != "" {
			t.Fatalf("unexpected request: %s %s %v", req.Method, req.URL, req.Header)
		}
		for _, name := range []string{"Cookie", "Authorization", "Proxy-Authorization", "Referer", "Origin"} {
			if req.Header.Get(name) != "" {
				t.Errorf("request leaked %s", name)
			}
		}
		switch req.URL.Path {
		case "/news/a%20b":
			t.Fatal("Path should be decoded while RawPath is retained")
		case "/news/a b":
			if req.Header.Get("Accept") != "text/html,application/xhtml+xml" {
				t.Fatal("missing page accept header")
			}
			return metadataResponse(req, 200, "text/html; charset=utf-8", []byte(`<title>Fallback</title><meta name="description" content="Fallback description"><meta property="og:title" content="  Air &amp; light  "><meta property="og:description" content="A better laptop"><meta name="twitter:image" content="/twitter.png"><meta property="og:image" content="../cover.png?size=large#unused">`)), nil
		case "/cover.png":
			if !strings.Contains(req.Header.Get("Accept"), "image/png") {
				t.Fatal("missing image accept header")
			}
			return metadataResponse(req, 200, "image/png", bitmap), nil
		}
		t.Fatalf("unexpected request %s", req.URL)
		return nil, nil
	})
	got, err := f.Fetch(context.Background(), "https://example.com/news/a%20b?q=first%20last&lang=en#content")
	if err != nil || got.Title != "Air & light" || got.Description != "A better laptop" || got.Cover == nil {
		t.Fatalf("metadata = %#v, error = %v", got, err)
	}
	if got.Cover.Bounds().Dx() != 640 || got.Cover.Bounds().Dy() != 320 {
		t.Fatalf("cover wasn't proportionally bounded: %v", got.Cover.Bounds())
	}
	want := []string{"https://example.com/news/a%20b?q=first%20last&lang=en", "https://example.com/cover.png?size=large"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("requests = %v; want %v", seen, want)
	}
}

func TestFetchRedirectsResolveCoverAgainstFinalPage(t *testing.T) {
	var seen []string
	bitmap := metadataPNG(t, 8, 4, 255)
	f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.String())
		if req.URL.Host == "example.com" {
			r := metadataResponse(req, 302, "text/html", nil)
			r.Header.Set("Location", "https://www.example.com/final/article#part")
			r.Header.Set("Set-Cookie", "session=do-not-send")
			return r, nil
		}
		if req.Header.Get("Referer") != "" || req.Header.Get("Cookie") != "" {
			t.Fatal("redirect forwarded browser headers")
		}
		if req.URL.Path == "/final/article" {
			return metadataResponse(req, 200, "application/xhtml+xml", []byte(`<title>Final title</title><meta property="og:image" content="cover.png">`)), nil
		}
		return metadataResponse(req, 200, "image/png", bitmap), nil
	})
	got, err := f.Fetch(context.Background(), "https://example.com/start")
	if err != nil || got.Title != "Final title" || got.Cover == nil {
		t.Fatalf("metadata = %#v, error = %v", got, err)
	}
	want := []string{"https://example.com/start", "https://www.example.com/final/article", "https://www.example.com/final/cover.png"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("requests = %v; want %v", seen, want)
	}
}

func TestFetchCoversUsePublicCDNAndFallBackToTwitter(t *testing.T) {
	var seen []string
	bitmap := metadataPNG(t, 8, 4, 255)
	f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.String())
		if req.URL.Host == "example.com" {
			return metadataResponse(req, 200, "text/html", []byte(`<meta name="twitter:image" content="https://twitter-cdn.example.com/card.png"><meta property="og:title" content="Article"><meta property="og:image" content="http://old-cdn.example.com/insecure.png"><meta property="og:image:secure_url" content="https://images.example.com/cover.png">`)), nil
		}
		if req.URL.Host == "images.example.com" {
			return metadataResponse(req, 404, "image/png", nil), nil
		}
		return metadataResponse(req, 200, "image/png", bitmap), nil
	})
	got, err := f.Fetch(context.Background(), "https://example.com/article")
	if err != nil || got.Cover == nil {
		t.Fatalf("metadata = %#v, error = %v", got, err)
	}
	want := []string{"https://example.com/article", "https://images.example.com/cover.png", "https://twitter-cdn.example.com/card.png"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("requests = %v; want %v", seen, want)
	}
}

func TestParseRemoteMetadataBaseAndNoScriptResources(t *testing.T) {
	page, _ := url.Parse("https://example.com/posts/article")
	for _, test := range []struct {
		name, base, want string
	}{
		{"same origin", `<base href="/assets/">`, "https://example.com/assets/cover.png"},
		{"other origin ignored", `<base href="https://cdn.example.com/assets/">`, "https://example.com/posts/cover.png"},
		{"private base ignored", `<base href="http://127.0.0.1/">`, "https://example.com/posts/cover.png"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := test.base + `<title> A &amp; B </title><script><meta property="og:image" content="https://bad.example.com/script.png"></script><meta property="og:image" content="cover.png"><img src="https://bad.example.com/body.png">`
			got, refs := parseRemoteMetadata([]byte(source), page)
			if got.Title != "A & B" || !reflect.DeepEqual(refs, []string{test.want}) {
				t.Fatalf("metadata %#v references %v", got, refs)
			}
		})
	}
	t.Run("metadata stops at head", func(t *testing.T) {
		source := `<head><title>Real title</title></head><body><meta property="og:title" content="Body title"><meta property="og:image" content="/unrelated.png"></body>`
		got, refs := parseRemoteMetadata([]byte(source), page)
		if got.Title != "Real title" || len(refs) != 0 {
			t.Fatalf("body content treated as metadata: %#v %v", got, refs)
		}
	})
}

func TestFetchRejectsDNSBeforeTransportAndRechecksRedirects(t *testing.T) {
	for _, test := range []struct {
		name      string
		addresses []netip.Addr
	}{
		{"private", []netip.Addr{netip.MustParseAddr("10.1.2.3")}},
		{"mixed", []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("::ffff:127.0.0.1")}},
		{"empty", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
				requests++
				return metadataResponse(req, 200, "text/html", []byte("<title>Blocked</title>")), nil
			})
			f.lookup = func(context.Context, string, string) ([]netip.Addr, error) { return test.addresses, nil }
			if _, err := f.Fetch(context.Background(), "https://public-looking.example.com/page"); err == nil || requests != 0 {
				t.Fatalf("DNS policy bypass: requests %d, error %v", requests, err)
			}
		})
	}
	t.Run("same host rebind on redirect", func(t *testing.T) {
		requests, lookups := 0, 0
		f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
			requests++
			r := metadataResponse(req, 302, "text/html", nil)
			r.Header.Set("Location", "/redirected")
			return r, nil
		})
		f.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
			lookups++
			if lookups > 1 {
				return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
			}
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		}
		if _, err := f.Fetch(context.Background(), "https://example.com/start"); err == nil || requests != 1 || lookups != 2 {
			t.Fatalf("redirect not revalidated: requests %d, lookups %d, error %v", requests, lookups, err)
		}
	})
}

func TestFetchRejectsPrivateImagesAndUnsafeRedirects(t *testing.T) {
	for _, destination := range []string{"http://example.com/downgrade", "https://user:pass@example.com/a", "https://127.0.0.1/", "https://example.com:8443/", "file:///tmp/image.png"} {
		t.Run(destination, func(t *testing.T) {
			requests := 0
			f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
				requests++
				r := metadataResponse(req, 302, "text/html", nil)
				r.Header.Set("Location", destination)
				return r, nil
			})
			if _, err := f.Fetch(context.Background(), "https://example.com/page"); err == nil || requests != 1 {
				t.Fatalf("unsafe redirect dispatched: requests %d error %v", requests, err)
			}
		})
	}
	t.Run("private CDN DNS", func(t *testing.T) {
		requests := 0
		f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
			requests++
			return metadataResponse(req, 200, "text/html", []byte(`<title>Keep title</title><meta property="og:image" content="https://cdn.example.com/image.png">`)), nil
		})
		f.lookup = func(_ context.Context, _, host string) ([]netip.Addr, error) {
			address := "93.184.216.34"
			if host == "cdn.example.com." {
				address = "169.254.169.254"
			}
			return []netip.Addr{netip.MustParseAddr(address)}, nil
		}
		got, err := f.Fetch(context.Background(), "https://example.com/page")
		if err != nil || got.Title != "Keep title" || got.Cover != nil || requests != 1 {
			t.Fatalf("private image bypass: %#v requests %d error %v", got, requests, err)
		}
	})
	t.Run("redirect bound", func(t *testing.T) {
		requests := 0
		f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
			requests++
			r := metadataResponse(req, 302, "text/html", nil)
			r.Header.Set("Location", fmt.Sprintf("/redirect/%d", requests))
			return r, nil
		})
		if _, err := f.Fetch(context.Background(), "https://example.com/page"); err == nil || requests != 5 {
			t.Fatalf("redirect limit: requests %d error %v", requests, err)
		}
	})
}

func TestMetadataTransportDialsValidatedLiteralAndDisablesProxy(t *testing.T) {
	f := NewFetcher()
	transport := f.transport.(*http.Transport)
	if transport.Proxy != nil || f.client.Jar != nil || f.client.Timeout != metadataTimeout || transport.DialTLSContext != nil {
		t.Fatal("transport bypasses credential or dial policies")
	}
	lookups := 0
	f.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		lookups++
		return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("2606:4700:4700::1111")}, nil
	}
	var dialed []string
	f.transport = metadataRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		conn, err := dialValidatedAddress(req.Context(), "tcp", "example.com:443", func(_ context.Context, network, target string) (net.Conn, error) {
			dialed = append(dialed, target)
			if target == "93.184.216.34:443" {
				return nil, errors.New("try next address")
			}
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
		if _, err := dialValidatedAddress(req.Context(), "tcp", "unvalidated.example.com:443", func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("wrong host dialed")
			return nil, nil
		}); err == nil {
			t.Fatal("unvalidated target accepted")
		}
		return metadataResponse(req, 200, "text/html", []byte("<title>OK</title>")), nil
	})
	if _, err := f.Fetch(context.Background(), "https://example.com/page"); err != nil {
		t.Fatal(err)
	}
	if lookups != 1 || !reflect.DeepEqual(dialed, []string{"93.184.216.34:443", "[2606:4700:4700::1111]:443"}) {
		t.Fatalf("dial was not bound to validated DNS: lookups %d targets %v", lookups, dialed)
	}
}

func TestFetchRejectsOversizedAndUnsupportedPages(t *testing.T) {
	for _, test := range []struct {
		name, contentType string
		content           []byte
		contentLength     int64
		status            int
	}{
		{"unsupported MIME", "application/json", []byte(`{"title":"no"}`), -1, 200},
		{"missing MIME", "", []byte(`<title>No</title>`), -1, 200},
		{"too big declared", "text/html", nil, maxHTMLBytes + 1, 200},
		{"too big streamed", "text/html", bytes.Repeat([]byte("x"), maxHTMLBytes+1), -1, 200},
		{"not found", "text/html", []byte(`<title>404</title>`), -1, 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
				r := metadataResponse(req, test.status, test.contentType, test.content)
				r.ContentLength = test.contentLength
				return r, nil
			})
			if _, err := f.Fetch(context.Background(), "https://example.com/"); err == nil {
				t.Fatal("invalid page accepted")
			}
		})
	}
}

func TestFetchRetainsTitleWithoutSuitableCover(t *testing.T) {
	valid := metadataPNG(t, 4, 2, 255)
	oversizedPixels := bytes.Clone(valid)
	binary.BigEndian.PutUint32(oversizedPixels[16:20], 9000)
	binary.BigEndian.PutUint32(oversizedPixels[20:24], 1000)
	binary.BigEndian.PutUint32(oversizedPixels[29:33], crc32.ChecksumIEEE(oversizedPixels[12:29]))
	for _, test := range []struct {
		name, contentType string
		content           []byte
		contentLength     int64
	}{
		{"invalid", "image/png", []byte("not an image"), -1},
		{"placeholder", "image/png", metadataPNG(t, 1, 1, 255), -1},
		{"transparent", "image/png", metadataPNG(t, 8, 8, 0), -1},
		{"HTML as image", "text/html", valid, -1},
		{"SVG", "image/svg+xml", []byte(`<svg/>`), -1},
		{"too many pixels", "image/png", oversizedPixels, -1},
		{"too big declared", "image/png", nil, remoteCoverBytes + 1},
		{"too big streamed", "image/png", bytes.Repeat([]byte("x"), remoteCoverBytes+1), -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/" {
					return metadataResponse(req, 200, "text/html", []byte(`<title>Retained</title><meta property="og:image" content="/cover">`)), nil
				}
				r := metadataResponse(req, 200, test.contentType, test.content)
				r.ContentLength = test.contentLength
				return r, nil
			})
			got, err := f.Fetch(context.Background(), "https://example.com/")
			if err != nil || got.Title != "Retained" || got.Cover != nil {
				t.Fatalf("metadata = %#v, error = %v", got, err)
			}
		})
	}
}

func TestFetchEmbeddedCoverAndBoundedCandidates(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(metadataPNG(t, 8, 4, 255))
	requests := 0
	f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
		requests++
		return metadataResponse(req, 200, "text/html", []byte(`<title>Embedded</title><meta property="og:image" content="data:image/png;base64,`+encoded+`">`)), nil
	})
	got, err := f.Fetch(context.Background(), "https://example.com/")
	if err != nil || got.Cover == nil || requests != 1 {
		t.Fatalf("embedded cover %#v requests %d error %v", got, requests, err)
	}
	page, _ := url.Parse("https://example.com/")
	var source strings.Builder
	for i := range 100 {
		fmt.Fprintf(&source, `<meta property="og:image" content="/%d.png">`, i)
	}
	_, locations := parseRemoteMetadata([]byte(source.String()), page)
	if len(locations) != maxCoverLocations {
		t.Fatalf("unbounded cover candidate list: %d", len(locations))
	}
}

func TestFetchCancellationStopsPageOrCover(t *testing.T) {
	t.Run("before lookup", func(t *testing.T) {
		f := fakeFetcher(t, func(*http.Request) (*http.Response, error) { t.Fatal("canceled request dispatched"); return nil, nil })
		f.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
			t.Fatal("canceled DNS lookup")
			return nil, nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := f.Fetch(ctx, "https://example.com/"); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("during lookup", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := fakeFetcher(t, func(*http.Request) (*http.Response, error) {
			t.Fatal("canceled request dispatched")
			return nil, nil
		})
		f.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
			cancel()
			return nil, context.Canceled
		}
		if _, err := f.Fetch(ctx, "https://example.com/"); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("during image", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := fakeFetcher(t, func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/" {
				return metadataResponse(req, 200, "text/html", []byte(`<title>Retained</title><meta property="og:image" content="/cover.png">`)), nil
			}
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) > metadataTimeout {
				t.Fatal("request has no bounded deadline")
			}
			cancel()
			<-req.Context().Done()
			return nil, req.Context().Err()
		})
		got, err := f.Fetch(ctx, "https://example.com/")
		if !errors.Is(err, context.Canceled) || got.Title != "Retained" || got.Cover != nil {
			t.Fatalf("canceled fetch = %#v error %v", got, err)
		}
	})
}
