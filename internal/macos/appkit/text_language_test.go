package appkit

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextLanguageUnambiguousScript(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"", "und"},
		{"hello@example.com", "und"},
		{"🐷📋✨", "und"},
		{"東京", "und"},
		{"日本語のクリップボード", "ja"},
		{"カタカナ", "ja"},
		{"ｸﾘｯﾌﾟﾎﾞｰﾄﾞ", "ja"},
	} {
		if got := TextLanguage(test.text); got != test.want {
			t.Errorf("TextLanguage(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestTextLanguageSampleIsBounded(t *testing.T) {
	sample, hint, han := textLanguageSample(strings.Repeat("漢", textLanguageSampleLimit) + "かな")
	if utf8.RuneCountInString(sample) != textLanguageSampleLimit || hint != "" || han != textLanguageSampleLimit {
		t.Fatalf("unexpected bounded sample: runes=%d hint=%q Han=%d", utf8.RuneCountInString(sample), hint, han)
	}
}

func TestSupportedTextLanguageRequiresConfidence(t *testing.T) {
	for _, test := range []struct {
		tag        string
		confidence float64
		want       string
	}{
		{"zh-Hans", .95, "zh-Hans"},
		{"zh-Hant", .9, "zh-Hant"},
		{"ja", .99, "ja"},
		{"zh-Hant", .7, "und"},
		{"en", 1, "und"},
		{"", 0, "und"},
	} {
		if got := supportedTextLanguage(test.tag, test.confidence); got != test.want {
			t.Errorf("supportedTextLanguage(%q, %v) = %q, want %q", test.tag, test.confidence, got, test.want)
		}
	}
}
