package clipmodel

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// LinkValue holds a destination derived from the complete saved text. URL is
// for an explicit open or QR action, never a replacement clipboard value.
// Inferred destinations and app links must not trigger background HTTP work.
type LinkValue struct {
	URL, Scheme, Host, Title string
	Inferred, App            bool
}

// ParseLink recognizes one complete webpage or application destination without
// disk, network, Markdown-document parsing or system application lookup.
func ParseLink(raw string) *LinkValue {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsFunc(value, func(ch rune) bool {
		return unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch)
	}) {
		return nil
	}
	title := ""
	if strings.HasPrefix(value, "[") {
		var ok bool
		title, value, ok = standaloneMarkdownLink(value)
		if !ok {
			return nil
		}
	}
	if strings.ContainsAny(value, "\\<>") || strings.ContainsFunc(value, unicode.IsSpace) {
		return nil
	}
	if separator := strings.Index(value, "://"); separator >= 0 {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Opaque != "" || !validLinkPort(parsed) {
			return nil
		}
		scheme := strings.ToLower(parsed.Scheme)
		switch scheme {
		case "http", "https":
			if parsed.Hostname() == "" {
				return nil
			}
			return &LinkValue{URL: value, Scheme: scheme, Host: parsed.Hostname(), Title: title}
		case "javascript", "data", "vbscript", "mailto", "tel", "file":
			return nil
		default:
			// Custom schemes are explicit, user-invoked app destinations. A
			// hostless path is valid (for example app:///route), but a bare
			// scheme, credentials or query/fragment alone are not a route.
			if parsed.User != nil || parsed.Host == "" && strings.Trim(parsed.Path, "/") == "" {
				return nil
			}
			return &LinkValue{URL: value, Scheme: scheme, Host: parsed.Hostname(), Title: title, App: true}
		}
	}
	parsed, err := url.Parse("https://" + value)
	if err != nil || parsed.User != nil || parsed.Opaque != "" || !validLinkPort(parsed) {
		return nil
	}
	host, err := idna.Lookup.ToASCII(parsed.Hostname())
	if err != nil {
		return nil
	}
	host = strings.ToLower(host)
	if !isBareLinkDomain(host) {
		return nil
	}
	port := parsed.Port()
	parsed.Host = host
	if port != "" {
		parsed.Host += ":" + port
	}
	return &LinkValue{URL: parsed.String(), Scheme: "https", Host: host, Title: title, Inferred: true}
}

func validLinkPort(parsed *url.URL) bool {
	if strings.HasSuffix(parsed.Host, ":") {
		return false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		return err == nil && number != 0
	}
	return true
}

func isBareLinkDomain(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	// Some real country/new TLDs are also common source/archive extensions.
	// Require an explicit HTTP(S) URL for those ambiguous bare values.
	switch labels[len(labels)-1] {
	case "md", "zip", "mov", "sh", "rs":
		return false
	}
	suffix, icann := publicsuffix.PublicSuffix(host)
	if !icann && !strings.Contains(suffix, ".") {
		return false
	}
	_, err := publicsuffix.EffectiveTLDPlusOne(host)
	return err == nil
}

// A standalone Markdown link is a link card; larger Markdown documents remain
// intact. Deliberately reject nested/escaped labels and optional title syntax
// here so this recognizer never extracts a destination from surrounding prose.
func standaloneMarkdownLink(value string) (title, target string, ok bool) {
	labelEnd := strings.Index(value, "](")
	if labelEnd <= 1 || !strings.HasSuffix(value, ")") {
		return "", "", false
	}
	title = strings.TrimSpace(value[1:labelEnd])
	if title == "" || strings.ContainsAny(title, "[]\\") || len(title) > 1024 {
		return "", "", false
	}
	target = value[labelEnd+2 : len(value)-1]
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
		target = target[1 : len(target)-1]
	}
	depth := 0
	for _, ch := range target {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return "", "", false
			}
		}
	}
	return title, target, depth == 0 && target != ""
}
