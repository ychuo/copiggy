// Package contentactions derives explicit user-action destinations from saved
// clipboard text. It performs no I/O and never executes or copies content.
package contentactions

import (
	"errors"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"copiggy/internal/clipmodel"
)

var (
	ErrUnsupported = errors.New("content has no external action")
	ErrInvalidPath = errors.New("invalid local file path")
)

// Target contains one destination. URL is restricted to mailto, tel or Apple
// Maps HTTPS. Path is for a reveal action, never shell execution. Calendar is an
// iCalendar document for the host to open after an explicit user request.
type Target struct {
	URL      string
	Path     string
	Calendar []byte
}

func OpenTarget(text string) (Target, error) {
	value := clipmodel.Analyze(text)
	switch value.Type {
	case clipmodel.TypeEmail:
		// Escape the complete recipient as a path component so literal '?' or
		// '#' in a valid email local part cannot introduce headers or fragments.
		return Target{URL: "mailto:" + url.PathEscape(value.Email.Address)}, nil
	case clipmodel.TypePhone:
		return Target{URL: "tel:" + value.Phone.Number}, nil
	case clipmodel.TypeCoordinates:
		coordinates := value.Coordinates
		point := strconv.FormatFloat(coordinates.Latitude, 'f', -1, 64) + "," + strconv.FormatFloat(coordinates.Longitude, 'f', -1, 64)
		target := url.URL{Scheme: "https", Host: "maps.apple.com", Path: "/", RawQuery: url.Values{"ll": {point}}.Encode()}
		return Target{URL: target.String()}, nil
	case clipmodel.TypeFilePath:
		return Target{Path: value.FilePath.Path}, nil
	case clipmodel.TypeDate:
		document, err := CalendarDocument(*value.Date)
		return Target{Calendar: document}, err
	default:
		return Target{}, ErrUnsupported
	}
}

// ResolvePath resolves a recognized local path against an explicitly supplied
// home directory. It never expands environment variables, follows symlinks,
// stats files, or permits remote file URLs. The caller must reveal, not execute.
func ResolvePath(text, home string) (string, error) {
	value := clipmodel.Analyze(text)
	if value.Type != clipmodel.TypeFilePath || value.FilePath == nil {
		return "", ErrInvalidPath
	}
	local := value.FilePath.Path
	if value.FilePath.HomeRelative {
		if !filepath.IsAbs(home) || strings.ContainsAny(home, "\x00\r\n") {
			return "", ErrInvalidPath
		}
		local = filepath.Join(filepath.Clean(home), strings.TrimPrefix(local, "~/"))
	}
	if !filepath.IsAbs(local) || strings.ContainsAny(local, "\x00\r\n") {
		return "", ErrInvalidPath
	}
	return filepath.Clean(local), nil
}
