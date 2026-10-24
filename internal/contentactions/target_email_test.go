package contentactions

import (
	"net/url"
	"testing"
)

func TestWrappedEmailActionsUseOnlyCanonicalRecipient(t *testing.T) {
	for _, tc := range []struct{ input, recipient string }{
		{"Name <hello@example.com>", "hello@example.com"},
		{"<hello@example.com>", "hello@example.com"},
		{"mailto:alex+tag@example.com", "alex+tag@example.com"},
		{"mailto:alex%2Btag%40example.com", "alex+tag@example.com"},
		{"Name <a?subject=secret@example.com>", "a?subject=secret@example.com"},
		{"Name <a#fragment@example.com>", "a#fragment@example.com"},
		{"Name <a%0d%0abcc=other@example.com>", "a%0d%0abcc=other@example.com"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			target, err := OpenTarget(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(target.URL)
			if err != nil || parsed.Scheme != "mailto" || parsed.RawQuery != "" || parsed.Fragment != "" || target.Path != "" || target.Calendar != nil {
				t.Fatalf("email wrapper introduced headers or another target: %+v", target)
			}
			recipient, err := url.PathUnescape(parsed.Opaque)
			if err != nil || recipient != tc.recipient {
				t.Fatalf("recipient = %q, %v; want %q", recipient, err, tc.recipient)
			}
		})
	}
}
