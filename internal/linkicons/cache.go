// Package linkicons fetches bounded, public-origin artwork without a UI dependency.
package linkicons

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"copiggy/internal/clipmodel"
	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
)

const (
	linkIconMaxEntries = 64
	linkHTMLMaxBytes   = 256 << 10
	linkIconMaxBytes   = 1 << 20
	linkIconMaxPixels  = 1_048_576
)

type linkIconEntry struct {
	bitmap  image.Image
	pending bool
	expires time.Time
	used    uint64
	cancel  context.CancelFunc
}

// Cache is bounded, asynchronous and safe to read from native builds.
// A nil result means loading/unsupported; the caller keeps its generic icon.
// onReady runs on a worker: dispatch UI invalidation onto MyGo's main thread.
// Lookup rejects restricted clipboard records before any cache or network work.
// Returned images are shared cached values and must be treated as immutable.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*linkIconEntry
	nextUse uint64
	client  *http.Client
	onReady func()
	workers chan struct{}
}

func New(onReady func()) *Cache {
	return newCacheWithClient(newPublicIconClient(), onReady)
}

func newCacheWithClient(client *http.Client, onReady func()) *Cache {
	return &Cache{entries: make(map[string]*linkIconEntry), client: client, onReady: onReady, workers: make(chan struct{}, 4)}
}

func (cache *Cache) Lookup(item clipmodel.Item, copiedHTML []byte) image.Image {
	if item.Type != clipmodel.TypeLink || len(item.AllowedApps) > 0 {
		return nil
	}
	origin, err := linkIconOrigin(item.Text)
	if err != nil {
		return nil
	}
	cache.mu.Lock()
	cache.nextUse++
	if entry := cache.entries[origin]; entry != nil {
		entry.used = cache.nextUse
		if entry.pending || time.Now().Before(entry.expires) {
			result := entry.bitmap
			cache.mu.Unlock()
			return result
		}
		entry.cancel()
		delete(cache.entries, origin)
	}
	if len(cache.entries) >= linkIconMaxEntries {
		oldestKey, oldestUse := "", ^uint64(0)
		for key, entry := range cache.entries {
			if entry.used < oldestUse {
				oldestKey, oldestUse = key, entry.used
			}
		}
		cache.entries[oldestKey].cancel()
		delete(cache.entries, oldestKey)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	entry := &linkIconEntry{pending: true, used: cache.nextUse, cancel: cancel}
	cache.entries[origin] = entry
	var markup []byte
	if len(copiedHTML) <= linkHTMLMaxBytes {
		markup = bytes.Clone(copiedHTML)
	}
	cache.mu.Unlock()
	go func() {
		defer cancel()
		select {
		case cache.workers <- struct{}{}:
		case <-ctx.Done():
			cache.finish(origin, entry, nil)
			return
		}
		bitmap := fetchLinkIcon(ctx, origin, markup, cache.client)
		<-cache.workers
		cache.finish(origin, entry, bitmap)
	}()
	return nil
}

func (cache *Cache) finish(origin string, entry *linkIconEntry, bitmap image.Image) {
	cache.mu.Lock()
	if cache.entries[origin] != entry {
		cache.mu.Unlock()
		return
	}
	entry.bitmap, entry.pending = bitmap, false
	lifetime := 10 * time.Minute
	if bitmap != nil {
		lifetime = 6 * time.Hour
	}
	entry.expires = time.Now().Add(lifetime)
	callback := cache.onReady
	cache.mu.Unlock()
	if callback != nil {
		callback()
	}
}

func (cache *Cache) Clear() {
	cache.mu.Lock()
	for _, entry := range cache.entries {
		entry.cancel()
	}
	clear(cache.entries)
	cache.mu.Unlock()
}

// Only the origin leaves the app. Never request the copied private path, query,
// fragment, username or password, nor send cookies, referrers or auth headers.
func linkIconOrigin(rawURL string) (string, error) {
	link := clipmodel.ParseLink(rawURL)
	if link == nil || link.Inferred || link.App {
		return "", errors.New("unsupported icon origin")
	}
	parsed, err := url.Parse(link.URL)
	if err != nil || !validIconURL(parsed) {
		return "", errors.New("unsupported icon origin")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = "/", "", "", ""
	parsed.ForceQuery = false
	return parsed.String(), nil
}

func validIconURL(parsed *url.URL) bool {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Hostname() == "" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}
	if port := parsed.Port(); port != "" && port != "80" && port != "443" {
		return false
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return publicIconAddress(address)
	}
	return strings.Contains(host, ".")
}

func publicIconAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, reserved := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/32", "2002::/16"} {
		if netip.MustParsePrefix(reserved).Contains(address) {
			return false
		}
	}
	return true
}

func newPublicIconClient() *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		DialContext: func(ctx context.Context, network, target string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(target)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(addresses) == 0 {
				return nil, errors.New("icon host unavailable")
			}
			// Validate all answers, then dial the validated literal to prevent DNS
			// rebinding between policy checking and connection establishment.
			for _, address := range addresses {
				if !publicIconAddress(address) {
					return nil, errors.New("private icon host")
				}
			}
			var lastErr error
			for _, address := range addresses {
				connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
				if err == nil {
					return connection, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	return &http.Client{Transport: transport, Timeout: 6 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 4 || !validIconURL(req.URL) {
			return errors.New("unsupported icon redirect")
		}
		req.URL.RawQuery, req.URL.Fragment = "", ""
		req.Header.Del("Referer")
		return nil
	}}
}

func fetchLinkIcon(ctx context.Context, origin string, copiedHTML []byte, client *http.Client) image.Image {
	candidates := iconLocations(origin, copiedHTML)
	// Clipboard HTML can contain icons, but never interpret an arbitrary copied
	// fragment's <base> as authority to fetch a non-origin page.
	if len(candidates) == 0 {
		if markup, err := readIconResource(ctx, client, origin, linkHTMLMaxBytes, "text/html,application/xhtml+xml"); err == nil {
			candidates = iconLocations(origin, markup)
		}
	}
	fallback, _ := url.Parse(origin)
	fallback.Path = "/favicon.ico"
	candidates = append(candidates, fallback.String())
	seen := make(map[string]bool)
	for _, location := range candidates {
		if seen[location] {
			continue
		}
		seen[location] = true
		data, err := readIconResource(ctx, client, location, linkIconMaxBytes, "image/*")
		if err != nil {
			continue
		}
		if bitmap, err := decodeLinkIcon(data); err == nil {
			return bitmap
		}
	}
	return nil
}

func readIconResource(ctx context.Context, client *http.Client, location string, limit int64, accept string) ([]byte, error) {
	parsed, err := url.Parse(location)
	if err != nil || !validIconURL(parsed) {
		return nil, errors.New("unsupported icon URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "Copiggy/1.0")
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > limit {
		return nil, errors.New("icon resource unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("icon resource exceeds limit")
	}
	return data, nil
}

func iconLocations(origin string, source []byte) []string {
	if len(source) > linkHTMLMaxBytes {
		return nil
	}
	base, err := url.Parse(origin)
	if err != nil {
		return nil
	}
	type candidate struct {
		location string
		priority int
	}
	var found []candidate
	clipmodel.ScanHTML(string(source), func(token clipmodel.HTMLToken) bool {
		if token.Closing || token.Name != "link" || len(found) >= 8 {
			return true
		}
		rel := strings.Fields(strings.ToLower(token.Attrs["rel"]))
		priority := 0
		if slices.Contains(rel, "icon") {
			priority = 2
		}
		if slices.Contains(rel, "apple-touch-icon") || slices.Contains(rel, "apple-touch-icon-precomposed") {
			priority = 1
		}
		if priority == 0 {
			return true
		}
		reference, err := url.Parse(strings.TrimSpace(token.Attrs["href"]))
		if err != nil || reference.String() == "" {
			return true
		}
		location := base.ResolveReference(reference)
		location.RawQuery, location.Fragment, location.ForceQuery = "", "", false
		if validIconURL(location) {
			found = append(found, candidate{location.String(), priority})
		}
		return true
	})
	slices.SortStableFunc(found, func(a, b candidate) int { return b.priority - a.priority })
	result := make([]string, 0, len(found))
	for _, candidate := range found {
		result = append(result, candidate.location)
	}
	return result
}

func decodeLinkIcon(data []byte) (image.Image, error) {
	var decoded image.Image
	var err error
	if len(data) >= 6 && bytes.Equal(data[:4], []byte{0, 0, 1, 0}) {
		decoded, err = decodeIconDirectory(data)
	} else {
		decoded, err = decodeBoundedIconImage(data)
	}
	if err != nil {
		return nil, err
	}
	if !imageHasVisiblePixels(decoded) {
		return nil, errors.New("empty icon")
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > 64 || height > 64 {
		if width >= height {
			height, width = max(1, height*64/width), 64
		} else {
			width, height = max(1, width*64/height), 64
		}
		scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), decoded, bounds, draw.Src, nil)
		decoded = scaled
	}
	return decoded, nil
}

func decodeBoundedIconImage(data []byte) (image.Image, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > linkIconMaxPixels {
		return nil, errors.New("unsupported icon dimensions")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	return decoded, err
}

// ICO commonly contains either PNG or a DIB with a separate one-bit alpha mask.
// Decode each bounded frame and prefer the largest <=256px supported image.
func decodeIconDirectory(data []byte) (image.Image, error) {
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 || count > 64 || len(data) < 6+16*count {
		return nil, errors.New("invalid icon directory")
	}
	var best image.Image
	for i := 0; i < count; i++ {
		entry := data[6+i*16 : 6+(i+1)*16]
		length, offset := uint64(binary.LittleEndian.Uint32(entry[8:12])), uint64(binary.LittleEndian.Uint32(entry[12:16]))
		if length == 0 || offset < uint64(6+16*count) || offset+length > uint64(len(data)) {
			continue
		}
		frame := data[int(offset):int(offset+length)]
		decoded, err := decodeBoundedIconImage(frame)
		if err != nil {
			decoded, err = decodeIconDIB(frame)
		}
		if err == nil && (best == nil || decoded.Bounds().Dx() > best.Bounds().Dx()) {
			best = decoded
		}
	}
	if best == nil {
		return nil, errors.New("no supported icon frame")
	}
	return best, nil
}

func decodeIconDIB(data []byte) (image.Image, error) {
	if len(data) < 40 {
		return nil, errors.New("short icon DIB")
	}
	header := int(binary.LittleEndian.Uint32(data[:4]))
	width, doubledHeight := int(int32(binary.LittleEndian.Uint32(data[4:8]))), int(int32(binary.LittleEndian.Uint32(data[8:12])))
	bits := int(binary.LittleEndian.Uint16(data[14:16]))
	if header != 40 || width <= 0 || width > 256 || doubledHeight <= 0 || doubledHeight%2 != 0 || doubledHeight > 512 || (bits != 8 && bits != 24 && bits != 32) || binary.LittleEndian.Uint32(data[16:20]) != 0 {
		return nil, errors.New("unsupported icon DIB")
	}
	height := doubledHeight / 2
	colors := int(binary.LittleEndian.Uint32(data[32:36]))
	if bits == 8 && colors == 0 {
		colors = 256
	}
	if colors < 0 || colors > 256 {
		return nil, errors.New("invalid icon palette")
	}
	offset := header + colors*4
	stride := ((width*bits + 31) / 32) * 4
	pixelEnd := offset + stride*height
	if pixelEnd > len(data) {
		return nil, errors.New("truncated icon pixels")
	}
	bitmap := make([]byte, 14+pixelEnd)
	copy(bitmap[:2], "BM")
	binary.LittleEndian.PutUint32(bitmap[2:6], uint32(len(bitmap)))
	binary.LittleEndian.PutUint32(bitmap[10:14], uint32(14+offset))
	copy(bitmap[14:], data[:pixelEnd])
	binary.LittleEndian.PutUint32(bitmap[22:26], uint32(height))
	decoded, err := bmp.Decode(bytes.NewReader(bitmap))
	if err != nil {
		return nil, err
	}
	rgba := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	hasAlpha := false
	if bits == 32 {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if data[offset+y*stride+x*4+3] != 0 {
					hasAlpha = true
				}
			}
		}
	}
	maskStride := ((width + 31) / 32) * 4
	hasMask := pixelEnd+maskStride*height <= len(data)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := rgba.NRGBAAt(x, y)
			sourceY := height - 1 - y
			if hasAlpha {
				value.A = data[offset+sourceY*stride+x*4+3]
			}
			if hasMask && data[pixelEnd+sourceY*maskStride+x/8]&(1<<uint(7-x%8)) != 0 {
				value.A = 0
			}
			rgba.SetNRGBA(x, y, color.NRGBA{R: value.R, G: value.G, B: value.B, A: value.A})
		}
	}
	return rgba, nil
}

func imageHasVisiblePixels(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha != 0 {
				return true
			}
		}
	}
	return false
}
