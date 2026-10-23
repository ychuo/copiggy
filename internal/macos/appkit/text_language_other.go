//go:build !darwin

package appkit

// TextLanguage returns an unambiguous Japanese script hint, or und when no
// local language detector is available. Callers select a stable fallback.
func TextLanguage(text string) string {
	_, hint, _ := textLanguageSample(text)
	if hint != "" {
		return hint
	}
	return "und"
}
