// Package linkmetadata extracts copied webpage metadata and fetches bounded
// public link previews. Extract itself remains local and never follows resources.
package linkmetadata

import (
	"bytes"
	"encoding/base64"
	"html"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"copiggy/internal/clipmodel"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	maxHTMLBytes   = 768 << 10
	maxCoverBytes  = 384 << 10
	maxCoverPixels = 1_048_576
)

type Metadata struct {
	Title, Description string
	Cover              image.Image
}

// Extract associates metadata with the exact copied HTTP(S) URL. A source URL,
// canonical URL or og:url may establish this association; an unrelated page's
// title never labels a copied link from inside that page. A matching anchor's
// own text can supply a title without page metadata. Embedded data images are
// the only permitted cover source. Nothing is persisted or sent off-device.
func Extract(rawURL string, copiedHTML []byte) Metadata {
	if len(copiedHTML) == 0 || len(copiedHTML) > maxHTMLBytes {
		return Metadata{}
	}
	target := normalizedURL(rawURL)
	if target == "" {
		return Metadata{}
	}
	source := string(copiedHTML)
	associated := matchingSourceURL(source, target)
	var title, ogTitle, description, coverURL string
	var titleText, anchorText strings.Builder
	inTitle, inAnchor := false, false
	var anchorTitle string
	clipmodel.ScanHTML(source, func(token clipmodel.HTMLToken) bool {
		if token.Name == "" {
			if inTitle && titleText.Len() < 2048 {
				titleText.WriteString(token.Text)
			}
			if inAnchor && anchorText.Len() < 2048 {
				anchorText.WriteString(token.Text)
			}
			return true
		}
		switch token.Name {
		case "title":
			inTitle = !token.Closing
			if token.Closing && title == "" {
				title = cleanText(html.UnescapeString(titleText.String()), 180)
			}
		case "a":
			if token.Closing {
				if inAnchor && anchorTitle == "" {
					anchorTitle = cleanText(html.UnescapeString(anchorText.String()), 180)
				}
				inAnchor = false
			} else {
				inAnchor = normalizedURL(token.Attrs["href"]) == target
				anchorText.Reset()
			}
		case "link":
			if !token.Closing && slices.Contains(strings.Fields(strings.ToLower(token.Attrs["rel"])), "canonical") && normalizedURL(token.Attrs["href"]) == target {
				associated = true
			}
		case "meta":
			if token.Closing {
				break
			}
			key := strings.ToLower(token.Attrs["property"])
			if key == "" {
				key = strings.ToLower(token.Attrs["name"])
			}
			content := token.Attrs["content"]
			switch key {
			case "og:url":
				if normalizedURL(content) == target {
					associated = true
				}
			case "og:title":
				if ogTitle == "" {
					ogTitle = cleanText(content, 180)
				}
			case "description", "og:description":
				if description == "" {
					description = cleanText(content, 360)
				}
			case "og:image", "twitter:image":
				if coverURL == "" && strings.HasPrefix(strings.ToLower(content), "data:image/") {
					coverURL = content
				}
			}
		}
		return true
	})
	if !associated {
		return Metadata{Title: anchorTitle}
	}
	if ogTitle != "" {
		title = ogTitle
	}
	if title == "" {
		title = anchorTitle
	}
	return Metadata{Title: title, Description: description, Cover: decodeCover(coverURL)}
}

func normalizedURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return ""
	}
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.Fragment, parsed.RawFragment = "", ""
	return parsed.String()
}

func matchingSourceURL(source, target string) bool {
	// CF_HTML's SourceURL is a header field, never an arbitrary body string.
	if !strings.HasPrefix(source, "Version:") {
		return false
	}
	if opening := strings.IndexByte(source, '<'); opening >= 0 {
		source = source[:opening]
	}
	if len(source) > 4096 {
		return false
	}
	for _, line := range strings.Split(source, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "SourceURL:"); ok && normalizedURL(value) == target {
			return true
		}
	}
	return false
}

func cleanText(value string, limit int) string {
	value = strings.Map(func(ch rune) rune {
		if unicode.IsControl(ch) && !unicode.IsSpace(ch) {
			return -1
		}
		return ch
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit-1]) + "…"
	}
	return value
}

func decodeCover(value string) image.Image {
	header, payload, ok := strings.Cut(value, ",")
	if !ok || len(payload) > base64.StdEncoding.EncodedLen(maxCoverBytes) {
		return nil
	}
	switch strings.ToLower(header) {
	case "data:image/png;base64", "data:image/jpeg;base64", "data:image/webp;base64":
	default:
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(data) == 0 || len(data) > maxCoverBytes {
		return nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 2 || config.Height < 2 || int64(config.Width)*int64(config.Height) > maxCoverPixels {
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
