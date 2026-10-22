package contentactions

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestOpenTargetEmailCannotIntroduceHeaders(t *testing.T) {
	for _, address := range []string{"hello@example.com", "alex+tag@example.com", "a?subject=secret@example.com", "a#fragment@example.com", "a%0d%0abcc=other@example.com"} {
		target, err := OpenTarget(address)
		if err != nil {
			t.Fatalf("email %q: %v", address, err)
		}
		parsed, err := url.Parse(target.URL)
		if err != nil || parsed.Scheme != "mailto" || parsed.RawQuery != "" || parsed.Fragment != "" || target.Path != "" || target.Calendar != nil {
			t.Fatalf("unexpected mail target for %q: %+v", address, target)
		}
		decoded, err := url.PathUnescape(parsed.Opaque)
		if err != nil || decoded != address {
			t.Fatalf("recipient changed: %q", target.URL)
		}
	}
	for _, value := range []string{"hello@example.com\r\nBcc:other@example.com", "hello@example.com?subject=hello", "mailto:hello@example.com?subject=hello", "Name <hello@example.com>"} {
		if _, err := OpenTarget(value); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("unsupported mail value accepted: %q", value)
		}
	}
}

func TestOpenTargetOnlyAllowsRecognizedActions(t *testing.T) {
	phone, err := OpenTarget("+1 (202) 555-0148")
	if err != nil || phone.URL != "tel:+12025550148" {
		t.Fatalf("phone target: %+v %v", phone, err)
	}
	mapTarget, err := OpenTarget("31.2304,121.4737")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(mapTarget.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "maps.apple.com" || parsed.Query().Get("ll") != "31.2304,121.4737" || len(parsed.Query()) != 1 {
		t.Fatalf("map target: %q", mapTarget.URL)
	}
	for _, value := range []string{"91.0,121.0", "31.0,181.0", "31.0,121.0&secret=value", "NaN,121.0", "javascript:alert(1)", "https://example.com", "OTP: 123456", "1792645800123", "ordinary text"} {
		if _, err := OpenTarget(value); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("unsupported target accepted: %q", value)
		}
	}
	pathTarget, err := OpenTarget("~/Documents/Brief.pdf")
	if err != nil || pathTarget.Path != "~/Documents/Brief.pdf" || pathTarget.URL != "" {
		t.Fatal("OpenTarget accessed or expanded the home directory")
	}
}

func TestResolvePathOnlyResolvesLocalPaths(t *testing.T) {
	for _, tc := range []struct{ input, home, want string }{
		{"~/Documents/Brief.pdf", "/Users/alex", "/Users/alex/Documents/Brief.pdf"},
		{"~/Documents/Brief.pdf", "/Users/alex/", "/Users/alex/Documents/Brief.pdf"},
		{"file:///Users/alex/My%20File.pdf", "", "/Users/alex/My File.pdf"},
		{"file://localhost/Users/alex/Brief.pdf", "", "/Users/alex/Brief.pdf"},
		{"/Users/alex/./Brief.pdf", "", "/Users/alex/Brief.pdf"},
		{"/Users/alex/file$(touch demo).txt", "", "/Users/alex/file$(touch demo).txt"},
	} {
		got, err := ResolvePath(tc.input, tc.home)
		if err != nil || got != tc.want {
			t.Errorf("%q => %q, %v", tc.input, got, err)
		}
	}
	for _, tc := range []struct{ input, home string }{
		{"~/a", ""}, {"~/a", "relative-home"}, {"~/a", "/Users/alex\x00"}, {"../a", "/Users/alex"},
		{"file://remote.example/a", "/Users/alex"}, {"file://user@localhost/a", "/Users/alex"},
		{"file:///tmp/a?download=1", "/Users/alex"}, {"file:///tmp/a%0Ab", "/Users/alex"}, {"~/../a", "/Users/alex"},
	} {
		if _, err := ResolvePath(tc.input, tc.home); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("invalid path accepted: %q", tc.input)
		}
	}
}

func TestOpenTargetDateOnlyReturnsCalendarBytes(t *testing.T) {
	for _, input := range []string{"2026-10-22", "2026-10-22 15:30", "2026-10-22T15:30:00+08:00"} {
		target, err := OpenTarget(input)
		if err != nil || len(target.Calendar) == 0 || target.Path != "" || target.URL != "" || !strings.HasPrefix(string(target.Calendar), "BEGIN:VCALENDAR\r\n") {
			t.Errorf("date target invalid: %q %v", input, err)
		}
	}
}
