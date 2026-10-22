package linkmetadata

import (
	"bytes"
	"context"
	"errors"
	"html"
	"image"
	_ "image/gif"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"copiggy/internal/clipmodel"
	"golang.org/x/image/draw"
)

const (
	remoteCoverBytes  = 4 << 20
	remoteCoverPixels = 8_000_000
	metadataTimeout   = 12 * time.Second
	maxCoverLocations = 8
)

type lookupAddresses func(context.Context, string, string) ([]netip.Addr, error)
type dialAddress func(context.Context, string, string) (net.Conn, error)

// Fetcher retrieves presentation metadata for a public webpage. Callers decide
// whether a record is eligible for network access; protected records must never
// be passed here. It has no browser session, cookie jar, proxy or app bindings.
type Fetcher struct {
	client    *http.Client
	transport http.RoundTripper
	lookup    lookupAddresses
}

func NewFetcher() *Fetcher {
	f := &Fetcher{lookup: net.DefaultResolver.LookupNetIP}
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	f.transport = &http.Transport{
		Proxy:                  nil,
		TLSHandshakeTimeout:    3 * time.Second,
		ResponseHeaderTimeout:  4 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		MaxIdleConns:           12,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        30 * time.Second,
		DialContext: func(ctx context.Context, network, target string) (net.Conn, error) {
			return dialValidatedAddress(ctx, network, target, dialer.DialContext)
		},
	}
	f.client = &http.Client{
		Transport: metadataTransport{f},
		Timeout:   metadataTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 4 || !validMetadataURL(req.URL) {
				return errors.New("unsupported metadata redirect")
			}
			if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return errors.New("insecure metadata redirect")
			}
			req.URL.Fragment, req.URL.RawFragment = "", ""
			clearRequestCredentials(req)
			return nil
		},
	}
	return f
}

// CanFetch is a cheap eligibility check. Fetch also validates every DNS answer
// before any network request, including redirects and cross-origin cover URLs.
func CanFetch(rawURL string) bool {
	_, err := metadataURL(rawURL)
	return err == nil
}

// Fetch loads the exact public page path and query, without its fragment. A
// missing or unsuitable cover is not an error: the page title remains useful.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Metadata, error) {
	location, err := metadataURL(rawURL)
	if err != nil {
		return Metadata{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	source, finalURL, err := f.readResource(ctx, location, maxHTMLBytes, false)
	if err != nil {
		return Metadata{}, err
	}
	metadata, references := parseRemoteMetadata(source, finalURL)
	for _, reference := range references {
		if err := ctx.Err(); err != nil {
			return metadata, err
		}
		if strings.HasPrefix(strings.ToLower(reference), "data:") {
			metadata.Cover = decodeCover(reference)
		} else if coverURL, err := metadataURL(reference); err == nil {
			if data, _, err := f.readResource(ctx, coverURL, remoteCoverBytes, true); err == nil {
				metadata.Cover = decodeRemoteCover(data)
			}
		}
		if metadata.Cover != nil {
			return metadata, nil
		}
	}
	return metadata, ctx.Err()
}

func metadataURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !validMetadataURL(parsed) {
		return nil, errors.New("unsupported metadata URL")
	}
	parsed.Fragment, parsed.RawFragment = "", ""
	return parsed, nil
}

func validMetadataURL(parsed *url.URL) bool {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Opaque != "" {
		return false
	}
	port := parsed.Port()
	if port != "" && !((parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80")) {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if address, err := netip.ParseAddr(host); err == nil {
		return publicMetadataAddress(address)
	}
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".home", ".lan", ".invalid", ".test"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	allNumeric := true
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if ch >= 'a' && ch <= 'z' || ch == '-' {
				allNumeric = false
			} else if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return !allNumeric
}

var nonPublicMetadataRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func publicMetadataAddress(address netip.Addr) bool {
	if address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	if address.Is6() && !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, reserved := range nonPublicMetadataRanges {
		if reserved.Contains(address) {
			return false
		}
	}
	return true
}

type validatedMetadataTarget struct {
	host, port string
	addresses  []netip.Addr
}
type metadataTargetKey struct{}
type metadataTransport struct{ fetcher *Fetcher }

func (t metadataTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !validMetadataURL(req.URL) {
		return nil, errors.New("unsupported metadata request")
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	host := strings.TrimSuffix(strings.ToLower(req.URL.Hostname()), ".")
	port := req.URL.Port()
	if port == "" {
		port = "80"
		if req.URL.Scheme == "https" {
			port = "443"
		}
	}
	var addresses []netip.Addr
	if address, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{address}
	} else {
		var err error
		// An absolute DNS name prevents search domains from resolving a public-
		// looking name to an unrelated local service.
		addresses, err = t.fetcher.lookup(req.Context(), "ip", host+".")
		if err != nil {
			if err := req.Context().Err(); err != nil {
				return nil, err
			}
			return nil, errors.New("metadata host unavailable")
		}
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, errors.New("metadata host unavailable")
	}
	for _, address := range addresses {
		if !publicMetadataAddress(address) {
			return nil, errors.New("private metadata host")
		}
	}
	target := validatedMetadataTarget{host: host, port: port, addresses: addresses}
	request := req.Clone(context.WithValue(req.Context(), metadataTargetKey{}, target))
	request.URL.Fragment, request.URL.RawFragment = "", ""
	clearRequestCredentials(request)
	return t.fetcher.transport.RoundTrip(request)
}

func clearRequestCredentials(req *http.Request) {
	for _, header := range []string{"Cookie", "Authorization", "Proxy-Authorization", "Referer", "Origin"} {
		req.Header.Del(header)
	}
}

// The transport resolves each request before opening it and supplies its
// validated addresses here. Never ask the resolver again during dialing.
func dialValidatedAddress(ctx context.Context, network, target string, dial dialAddress) (net.Conn, error) {
	validated, ok := ctx.Value(metadataTargetKey{}).(validatedMetadataTarget)
	host, port, err := net.SplitHostPort(target)
	if !ok || err != nil || strings.TrimSuffix(strings.ToLower(host), ".") != validated.host || port != validated.port {
		return nil, errors.New("unvalidated metadata connection")
	}
	var lastError error
	for _, address := range validated.addresses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		connection, err := dial(ctx, network, net.JoinHostPort(address.Unmap().String(), port))
		if err == nil {
			return connection, nil
		}
		lastError = err
	}
	if lastError == nil {
		lastError = errors.New("metadata host unavailable")
	}
	return nil, lastError
}

func (f *Fetcher) readResource(ctx context.Context, location *url.URL, limit int64, isImage bool) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Copiggy/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	if isImage {
		req.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/gif")
	}
	response, err := f.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > limit {
		return nil, nil, errors.New("metadata resource unavailable")
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (!isImage && mediaType != "text/html" && mediaType != "application/xhtml+xml") || (isImage && !supportedCoverMIME(mediaType)) {
		return nil, nil, errors.New("unsupported metadata media type")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) > limit {
		return nil, nil, errors.New("metadata resource exceeds limit")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	finalURL := response.Request.URL
	return data, finalURL, nil
}

func supportedCoverMIME(value string) bool {
	switch value {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	}
	return false
}

func parseRemoteMetadata(source []byte, pageURL *url.URL) (Metadata, []string) {
	var result Metadata
	var title strings.Builder
	var ogTitle, ogDescription, baseRef string
	var ogImages, twitterImages []string
	inTitle := false
	clipmodel.ScanHTML(string(source), func(token clipmodel.HTMLToken) bool {
		if token.Name == "" {
			if inTitle && title.Len() < 4096 {
				title.WriteString(token.Text[:min(len(token.Text), 4096-title.Len())])
			}
			return true
		}
		switch token.Name {
		case "head":
			if token.Closing {
				return false
			}
		case "body":
			return false
		case "title":
			inTitle = !token.Closing
		case "base":
			if !token.Closing && baseRef == "" {
				baseRef = strings.TrimSpace(token.Attrs["href"])
			}
		case "meta":
			if token.Closing {
				return true
			}
			key := strings.ToLower(strings.TrimSpace(token.Attrs["property"]))
			if key == "" {
				key = strings.ToLower(strings.TrimSpace(token.Attrs["name"]))
			}
			content := token.Attrs["content"]
			switch key {
			case "og:title":
				if ogTitle == "" {
					ogTitle = cleanText(content, 180)
				}
			case "og:description":
				if ogDescription == "" {
					ogDescription = cleanText(content, 360)
				}
			case "description":
				if result.Description == "" {
					result.Description = cleanText(content, 360)
				}
			case "og:image", "og:image:url", "og:image:secure_url":
				if len(ogImages) < maxCoverLocations {
					ogImages = append(ogImages, content)
				}
			case "twitter:image", "twitter:image:src":
				if len(twitterImages) < maxCoverLocations {
					twitterImages = append(twitterImages, content)
				}
			}
		}
		return true
	})
	result.Title = ogTitle
	if result.Title == "" {
		result.Title = cleanText(html.UnescapeString(title.String()), 180)
	}
	if ogDescription != "" {
		result.Description = ogDescription
	}
	base := pageURL
	if ref, err := url.Parse(baseRef); err == nil && baseRef != "" {
		candidate := pageURL.ResolveReference(ref)
		// A base can only redefine paths on this page's origin. Images may still
		// explicitly use a public CDN, which gets its own DNS validation.
		if candidate.Scheme == pageURL.Scheme && strings.EqualFold(candidate.Host, pageURL.Host) && validMetadataURL(candidate) {
			base = candidate
		}
	}
	locations := make([]string, 0, maxCoverLocations)
	seen := make(map[string]bool)
	for _, value := range append(ogImages, twitterImages...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		if strings.HasPrefix(strings.ToLower(value), "data:image/") {
			locations = append(locations, value)
		} else if ref, err := url.Parse(value); err == nil {
			location := base.ResolveReference(ref)
			if validMetadataURL(location) && !(pageURL.Scheme == "https" && location.Scheme != "https") {
				location.Fragment, location.RawFragment = "", ""
				locations = append(locations, location.String())
			}
		}
		if len(locations) >= maxCoverLocations {
			break
		}
	}
	return result, locations
}

func decodeRemoteCover(data []byte) image.Image {
	if len(data) == 0 || len(data) > remoteCoverBytes {
		return nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 2 || config.Height < 2 || config.Width > remoteCoverPixels || config.Height > remoteCoverPixels || int64(config.Width)*int64(config.Height) > remoteCoverPixels {
		return nil
	}
	bitmap, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	visible := false
	for y := bitmap.Bounds().Min.Y; y < bitmap.Bounds().Max.Y && !visible; y++ {
		for x := bitmap.Bounds().Min.X; x < bitmap.Bounds().Max.X; x++ {
			if _, _, _, alpha := bitmap.At(x, y).RGBA(); alpha != 0 {
				visible = true
				break
			}
		}
	}
	if !visible {
		return nil
	}
	width, height := bitmap.Bounds().Dx(), bitmap.Bounds().Dy()
	if width > 640 || height > 640 {
		if width >= height {
			height, width = max(1, height*640/width), 640
		} else {
			width, height = max(1, width*640/height), 640
		}
		scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), bitmap, bitmap.Bounds(), draw.Src, nil)
		bitmap = scaled
	}
	return bitmap
}
