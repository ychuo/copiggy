package clipmodel

import (
	"net/url"
	"path"
	"strings"
	"unicode"
)

func analyzeFilePath(value string) *FilePathValue {
	// A single enclosing quote pair is a presentation wrapper, never shell
	// syntax. Keep interior whitespace, backslashes and other filename characters
	// literal; another matching quote makes the wrapper ambiguous.
	if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
		quote := value[0]
		if len(value) < 2 || value[len(value)-1] != quote {
			return nil
		}
		value = value[1 : len(value)-1]
		if strings.IndexByte(value, quote) >= 0 {
			return nil
		}
	}
	homeRelative := strings.HasPrefix(value, "~/")
	if len(value) >= 5 && strings.EqualFold(value[:5], "file:") {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "file" || parsed.User != nil || parsed.Opaque != "" ||
			(parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost")) || parsed.RawQuery != "" || parsed.ForceQuery ||
			strings.ContainsRune(value, '#') {
			return nil
		}
		value = parsed.Path
	}
	if (!homeRelative && !strings.HasPrefix(value, "/")) || strings.HasPrefix(value, "//") ||
		strings.ContainsFunc(value, unicode.IsControl) || strings.Contains(value, "/../") || strings.HasSuffix(value, "/..") {
		return nil
	}
	clean := path.Clean(value)
	if clean == "/" || clean == "~" || clean == "." {
		return nil
	}
	return &FilePathValue{Path: clean, Name: path.Base(clean), Dir: path.Dir(clean), Ext: path.Ext(clean), HomeRelative: homeRelative}
}
