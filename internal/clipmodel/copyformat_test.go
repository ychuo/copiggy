package clipmodel

import (
	"errors"
	"reflect"
	"testing"
)

func TestCopyTextConvertsRecognizedValuesWithoutChangingSource(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		format CopyFormat
		want   string
	}{
		{"#f008", ColorHEX, "#FF000088"},
		{"#FF000080", ColorRGB, "rgba(255, 0, 0, 0.502)"},
		{"#FF000080", ColorHSL, "hsla(0, 100%, 50%, 0.502)"},
		{"#010101", ColorHSL, "hsl(0, 0%, 0.39%)"},
		{"#FEFEFE", ColorHSL, "hsl(0, 0%, 99.61%)"},
		{"rgb(255, 0, 0)", ColorHEX, "#FF0000"},
		{"1792645800123", TimestampSeconds, "1792645800"},
		{"1792645800", TimestampMilliseconds, "1792645800000"},
		{"1792645800123", TimestampMilliseconds, "1792645800123"},
		{"+1 (202) 555-0148", PhoneDialing, "+12025550148"},
		{"020 7946 0958", PhoneDialing, "02079460958"},
	} {
		t.Run(tc.raw+"/"+string(tc.format), func(t *testing.T) {
			item := Item{ID: "source", Type: Analyze(tc.raw).Type, Text: tc.raw, SourceApp: "Editor", AllowedApps: []string{"editor.app"}, Pinned: true}
			before := cloneItem(item)
			got, err := CopyText(item, tc.format)
			if err != nil || got != tc.want {
				t.Fatalf("copy text = %q, %v; want %q", got, err, tc.want)
			}
			if !reflect.DeepEqual(item, before) {
				t.Fatal("conversion changed the original")
			}
		})
	}
}

func TestCopiedHSLPreservesEightBitChannels(t *testing.T) {
	check := func(r, g, b, a uint8) {
		t.Helper()
		original := describeColor(r, g, b, a)
		copied, err := CopyText(Item{Type: TypeColor, Text: original.HEX}, ColorHSL)
		parsed := Analyze(copied).Color
		if err != nil || parsed == nil || parsed.HEX != original.HEX {
			t.Fatalf("HSL conversion changed %s to %q: %+v, %v", original.HEX, copied, parsed, err)
		}
	}
	for level := 0; level < 256; level++ {
		check(uint8(level), uint8(level), uint8(level), 255)
	}
	channels := []uint8{0, 1, 2, 63, 64, 127, 128, 191, 192, 253, 254, 255}
	for _, r := range channels {
		for _, g := range channels {
			for _, b := range channels {
				for _, a := range []uint8{0, 1, 128, 255} {
					check(r, g, b, a)
				}
			}
		}
	}
}

func TestCopyTextRejectsUnknownOrMismatchedFormats(t *testing.T) {
	for _, tc := range []struct {
		kind   ContentType
		raw    string
		format CopyFormat
	}{
		{TypeColor, "#FF0000", "unknown"},
		{TypeColor, "#FF0000", PhoneDialing},
		{TypeText, "#FF0000", ColorHEX},
		{TypeColor, "invalid", ColorHEX},
		{TypeColor, "1792645800", ColorHEX},
		{TypeTimestamp, "2026-10-22", TimestampSeconds},
		{TypePhone, "OTP: 123456", PhoneDialing},
	} {
		got, err := CopyText(Item{Type: tc.kind, Text: tc.raw}, tc.format)
		if got != "" || !errors.Is(err, ErrInvalidCopyFormat) {
			t.Fatalf("invalid conversion accepted: %+v => %q, %v", tc, got, err)
		}
	}
}
