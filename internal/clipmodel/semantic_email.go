package clipmodel

import (
	"net/mail"
	"net/url"
	"strings"
	"unicode"
)

func analyzeEmail(value string) *EmailValue {
	if len(value) > 1024 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return nil
	}
	mailbox := value
	switch {
	case len(value) >= 7 && strings.EqualFold(value[:7], "mailto:"):
		// Only a single recipient is recognized. Even empty query/fragment
		// delimiters are excluded, and decoded text must still be a mailbox.
		if strings.ContainsAny(value, "?#") {
			return nil
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Opaque == "" || parsed.Host != "" || parsed.Path != "" || parsed.User != nil {
			return nil
		}
		mailbox, err = url.PathUnescape(parsed.Opaque)
		if err != nil || strings.ContainsAny(mailbox, "?#") {
			return nil
		}
	case strings.ContainsAny(value, "<>"):
		if strings.Count(value, "<") != 1 || strings.Count(value, ">") != 1 || !strings.HasSuffix(value, ">") {
			return nil
		}
		address, err := mail.ParseAddress(value)
		if err != nil {
			return nil
		}
		mailbox = strings.TrimSpace(value[strings.IndexByte(value, '<')+1 : len(value)-1])
		if address.Address != mailbox {
			return nil
		}
	}
	return analyzeEmailMailbox(mailbox)
}

func analyzeEmailMailbox(value string) *EmailValue {
	if len(value) > 254 || strings.ContainsAny(value, "<>\"(),; \t") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return nil
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return nil
	}
	local, domain, found := strings.Cut(value, "@")
	if !found || len(local) > 64 || !strings.Contains(domain, ".") {
		return nil
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return nil
		}
		for _, ch := range label {
			if !(unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-') {
				return nil
			}
		}
	}
	return &EmailValue{Address: value, Local: local, Domain: strings.ToLower(domain)}
}
