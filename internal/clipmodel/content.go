package clipmodel

import (
	"crypto/sha256"
	"encoding/json"
	"strings"

	"copiggy/resources"
)

// ContentType describes the presentation of a captured value. Text-based
// classifications are derived from the text rather than trusted from a caller.
type ContentType string

const (
	TypeText  ContentType = "text"
	TypeJSON  ContentType = "json"
	TypeLink  ContentType = "link"
	TypeImage ContentType = "image"
	// TypeFiles is retained only to decode and migrate older history records.
	TypeFiles       ContentType = "files"
	TypeEmail       ContentType = "email"
	TypePhone       ContentType = "phone"
	TypeCoordinates ContentType = "coordinates"
	TypeDate        ContentType = "date"
	TypeTimestamp   ContentType = "timestamp"
	TypeColor       ContentType = "color"
	TypeFilePath    ContentType = "file-path"
	TypeOTP         ContentType = "otp"
	TypeToken       ContentType = "token"
	TypePrivateKey  ContentType = "private-key"
)

// ImageMetadata describes a validated PNG. Record keeps its exact original
// representations alongside these small, independently copied display fields.
type ImageMetadata struct {
	MIME     string `json:"mime"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	ByteSize int    `json:"byteSize"`
}

// Content is the value supplied by a native clipboard adapter. An omitted Type
// means text; all text-based content is classified from Text. Image content
// must explicitly use TypeImage and supply valid PNG metadata.
type Content struct {
	Type              ContentType
	Text              string
	Image             *ImageMetadata
	RecognitionRegion string
	SourceAppID       string
	// Fingerprint is the native adapter's SHA-256 of every retained item and
	// representation. Zero means plain text only, or unknown image bytes. Image
	// metadata alone is never sufficient to identify a duplicate.
	Fingerprint [sha256.Size]byte
}

func normalizeContent(content Content) (Content, bool) {
	if content.RecognitionRegion != "" && content.RecognitionRegion != "ZZ" &&
		NormalizePhoneRegion(content.RecognitionRegion) != content.RecognitionRegion {
		return Content{}, false
	}
	switch content.Type {
	case "", TypeText, TypeJSON, TypeLink, TypeEmail, TypePhone, TypeCoordinates,
		TypeDate, TypeTimestamp, TypeColor, TypeFilePath, TypeOTP, TypeToken, TypePrivateKey:
		if content.Text == "" {
			return Content{}, false
		}
		content.Type = AnalyzeInRegion(content.Text, content.RecognitionRegion).Type
		content.Image = nil
		if content.Fingerprint == ([sha256.Size]byte{}) {
			content.Fingerprint = textFingerprint(content.Text)
		}
		return content, true
	case TypeImage:
		if content.Image == nil || content.Image.MIME != "image/png" ||
			content.Image.Width <= 0 || content.Image.Height <= 0 || content.Image.ByteSize <= 0 {
			return Content{}, false
		}
		metadata := *content.Image
		content.Text = ""
		content.Image = &metadata
		return content, true
	default:
		return Content{}, false
	}
}

func textFingerprint(text string) [sha256.Size]byte {
	return sha256.Sum256([]byte("copiggy/plain-text/v1\x00" + text))
}

func classifyText(text string) ContentType {
	return Analyze(text).Type
}

var localizedTypeSearchTerms = func() map[ContentType]string {
	var catalog map[ContentType]map[string]string
	if err := json.Unmarshal(resources.SearchTerms, &catalog); err != nil {
		panic("invalid embedded search terms: " + err.Error())
	}
	terms := make(map[ContentType]string, len(catalog))
	for kind, translations := range catalog {
		var words []string
		for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			words = append(words, translations[language])
		}
		terms[kind] = strings.Join(words, " ")
	}
	return terms
}()

func typeSearchTerms(kind ContentType) string {
	return localizedTypeSearchTerms[kind]
}
