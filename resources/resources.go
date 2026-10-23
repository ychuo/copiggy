// Package resources embeds the native application's static assets.
package resources

import "embed"

// Icons contains the Phosphor SVGs used by the native UI.
//
//go:embed icons/*.svg
var Icons embed.FS

// RegionFlags contains local 4:3 SVG flags keyed by lowercase numbering region.
// The artwork keeps its original colors and never requires a network request.
//
//go:embed flags/*.svg
var RegionFlags embed.FS

// SearchTerms contains localized content-type keywords. Searching any supported
// language should work independently of the selected interface language.
//
//go:embed locales/search_terms.json
var SearchTerms []byte

// ContentLabels contains localized whole-value prefixes used by conservative
// content recognition, independently of the selected interface language.
//
//go:embed locales/content_labels.json
var ContentLabels []byte

// Brand contains Copiggy's application icon and native empty-state artwork.
//
//go:embed brand/*.png
var Brand embed.FS

// AppIcon is the PNG used when starting the executable outside its app bundle.
//
//go:embed brand/app-icon.png
var AppIcon []byte

// Fonts contains the full-glyph Regular and Medium faces used by native text.
// Registration is process-local; installation and network access are unnecessary.
//
//go:embed fonts/*/*.ttf
var Fonts embed.FS
