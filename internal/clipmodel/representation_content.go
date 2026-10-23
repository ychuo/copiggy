package clipmodel

import (
	"bytes"
	"html"
	"image"
	"image/png"
	"strings"
)

// Capture errors carry a catalog key; raw decoding/platform failures stay in logs.
type ContentError string

func (err ContentError) Error() string { return string(err) }

// Materialized representations are bounded before they enter the history.
func ContentFromRepresentations(data Representations) (Content, error) {
	fingerprint, err := data.Fingerprint()
	if err != nil {
		return Content{}, err
	}
	files, err := FilesFromRepresentations(data)
	if err != nil {
		return Content{}, err
	}
	if len(files) > 0 {
		text, err := FilePathsText(files)
		if err != nil {
			return Content{}, err
		}
		if len(text) > 1<<20 {
			return Content{}, ContentError("Text exceeds 1 MiB · Not recorded")
		}
		return Content{Type: Analyze(text).Type, Text: text, Fingerprint: fingerprint}, nil
	}
	if text, err := data.Read(TextFormat); err == nil && len(text) > 0 {
		if len(text) > 1<<20 {
			return Content{}, ContentError("Text exceeds 1 MiB · Not recorded")
		}
		return Content{Text: string(text), Fingerprint: fingerprint}, nil
	}
	// Some applications publish only HTML alongside a transparent or 1×1
	// placeholder PNG. Choose useful text for the preview, while retaining every
	// original representation (and its fingerprint) for a later explicit use.
	var htmlText string
	if encoded, err := data.Read(HTMLFormat); err == nil && len(encoded) > 0 {
		htmlText = clipboardHTMLText(string(encoded))
	}
	textFallback := func() (Content, error) {
		if len(htmlText) > 1<<20 {
			return Content{}, ContentError("Text exceeds 1 MiB · Not recorded")
		}
		return Content{Text: htmlText, Fingerprint: fingerprint}, nil
	}
	if encoded, err := data.Read(PNGFormat); err == nil && len(encoded) > 0 {
		config, err := png.DecodeConfig(bytes.NewReader(encoded))
		if err != nil {
			return Content{}, ContentError("Image unavailable")
		}
		if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24_000_000 {
			return Content{}, ContentError("Image exceeds 24 MP · Not recorded")
		}
		img, err := png.Decode(bytes.NewReader(encoded))
		if err != nil {
			return Content{}, ContentError("Image data is incomplete")
		}
		visible := clipboardImageHasVisiblePixels(img)
		if htmlText != "" && (!visible || config.Width == 1 && config.Height == 1) {
			return textFallback()
		}
		if !visible {
			return Content{}, ContentError("Empty image ignored")
		}
		return Content{Type: TypeImage, Image: &ImageMetadata{MIME: "image/png", Width: config.Width, Height: config.Height, ByteSize: len(encoded)}, Fingerprint: fingerprint}, nil
	}
	if htmlText != "" {
		return textFallback()
	}
	return Content{}, ContentError("Unsupported clipboard content")
}

func clipboardImageHasVisiblePixels(img image.Image) bool {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha != 0 {
				return true
			}
		}
	}
	return false
}

// This deliberately small tokenizer reads clipboard HTML and <link> metadata;
// it never executes markup, follows embedded resources or constructs a DOM.
// Quoted '>' characters, comments, boolean attributes and raw script/style
// bodies are handled so they cannot masquerade as visible text or icon tags.
type HTMLToken struct {
	Name, Text string
	Attrs      map[string]string
	Closing    bool
}

func ScanHTML(source string, visit func(HTMLToken) bool) {
	for len(source) > 0 {
		start := strings.IndexByte(source, '<')
		if start < 0 {
			visit(HTMLToken{Text: source})
			return
		}
		if start > 0 && !visit(HTMLToken{Text: source[:start]}) {
			return
		}
		source = source[start:]
		if strings.HasPrefix(source, "<!--") {
			end := strings.Index(source[4:], "-->")
			if end < 0 {
				return
			}
			source = source[end+7:]
			continue
		}
		end, quote := 1, byte(0)
		for ; end < len(source); end++ {
			ch := source[end]
			if quote != 0 {
				if ch == quote {
					quote = 0
				}
			} else if ch == '\'' || ch == '"' {
				quote = ch
			} else if ch == '>' {
				break
			}
		}
		if end == len(source) {
			return
		}
		inside := strings.TrimSpace(source[1:end])
		source = source[end+1:]
		if inside == "" || inside[0] == '!' || inside[0] == '?' {
			continue
		}
		token := HTMLToken{Attrs: make(map[string]string), Closing: inside[0] == '/'}
		if token.Closing {
			inside = strings.TrimSpace(inside[1:])
		}
		i := 0
		for i < len(inside) && htmlNameByte(inside[i]) {
			i++
		}
		if i == 0 {
			continue
		}
		token.Name = strings.ToLower(inside[:i])
		inside = inside[i:]
		for len(inside) > 0 {
			inside = strings.TrimSpace(inside)
			i = 0
			for i < len(inside) && htmlNameByte(inside[i]) {
				i++
			}
			if i == 0 {
				break
			}
			name := strings.ToLower(inside[:i])
			inside = strings.TrimSpace(inside[i:])
			value := ""
			if len(inside) > 0 && inside[0] == '=' {
				inside = strings.TrimSpace(inside[1:])
				if len(inside) > 0 && (inside[0] == '\'' || inside[0] == '"') {
					quote := inside[0]
					inside = inside[1:]
					i = strings.IndexByte(inside, quote)
					if i < 0 {
						i = len(inside)
					}
					value, inside = inside[:i], inside[min(i+1, len(inside)):]
				} else {
					i = 0
					for i < len(inside) && !strings.ContainsRune(" \t\r\n\f", rune(inside[i])) {
						i++
					}
					value, inside = inside[:i], inside[i:]
				}
			}
			if _, exists := token.Attrs[name]; !exists {
				token.Attrs[name] = html.UnescapeString(value)
			}
		}
		if !visit(token) {
			return
		}
		if !token.Closing && (token.Name == "script" || token.Name == "style") {
			closing := "</" + token.Name
			i := strings.Index(strings.ToLower(source), closing)
			if i < 0 {
				return
			}
			source = source[i:]
		}
	}
}

func htmlNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '_' || b == ':'
}

func clipboardHTMLText(source string) string {
	var out strings.Builder
	suppressed := make(map[string]int)
	depth := 0
	ScanHTML(source, func(token HTMLToken) bool {
		switch token.Name {
		case "head", "script", "style", "template", "noscript":
			if token.Closing {
				if suppressed[token.Name] > 0 {
					suppressed[token.Name]--
					depth--
				}
			} else {
				suppressed[token.Name]++
				depth++
			}
		}
		if depth > 0 {
			return true
		}
		if token.Name == "" {
			out.WriteString(html.UnescapeString(token.Text))
		}
		switch token.Name {
		case "p", "div", "br", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre":
			out.WriteByte('\n')
		case "td", "th":
			out.WriteByte(' ')
		}
		return true
	})
	lines := strings.Split(out.String(), "\n")
	clean := lines[:0]
	for _, line := range lines {
		if value := strings.Join(strings.Fields(line), " "); value != "" {
			clean = append(clean, value)
		}
	}
	return strings.Join(clean, "\n")
}
