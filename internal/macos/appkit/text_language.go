package appkit

import (
	"strings"
	"unicode"
)

const textLanguageSampleLimit = 1024

// textLanguageSample bounds the local detector's work and extracts unambiguous
// script hints. Shared Han characters alone do not identify a regional form.
func textLanguageSample(text string) (sample, hint string, han int) {
	var bounded strings.Builder
	count := 0
	for _, r := range text {
		if count == textLanguageSampleLimit {
			break
		}
		count++
		bounded.WriteRune(r)
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			hint = "ja"
		}
		if unicode.Is(unicode.Han, r) {
			han++
		}
	}
	return bounded.String(), hint, han
}

func supportedTextLanguage(language string, confidence float64) string {
	if confidence >= .8 {
		switch language {
		case "ja", "zh-Hans", "zh-Hant":
			return language
		}
	}
	return "und"
}
