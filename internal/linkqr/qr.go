// Package linkqr encodes explicit link previews locally. It never opens a link,
// persists the target, or accesses the clipboard or network.
package linkqr

import (
	"errors"
	"image"
	"image/color"
	"unicode"
	"unicode/utf8"

	"github.com/skip2/go-qrcode"
)

const (
	// Bound work before the encoder allocates. The symbol-size bound below can
	// reject shorter targets when their encoding needs too many modules.
	MaxContentBytes = 2048
	// Including the four-module quiet zone on each edge, version 33 is the
	// largest symbol that fits at two complete pixels per module in 320px.
	MaxModuleCount = 157
)

var (
	ErrTooLong        = errors.New("link is too long for a readable QR code")
	ErrInvalidContent = errors.New("invalid QR code content")
)

// Code is an immutable QR symbol. Neither its data nor a mutable image buffer
// is exposed, so cached previews cannot accidentally alter its quiet zone.
type Code struct {
	modules [][]bool
}

// Encode preserves the exact normalized target supplied by the caller. URL
// recognition and scheme policy belong to clipmodel, not this encoder.
func Encode(target string) (*Code, error) {
	if len(target) > MaxContentBytes {
		return nil, ErrTooLong
	}
	if target == "" || !utf8.ValidString(target) {
		return nil, ErrInvalidContent
	}
	for _, r := range target {
		if unicode.IsControl(r) {
			return nil, ErrInvalidContent
		}
	}
	symbol, err := qrcode.New(target, qrcode.Medium)
	if err != nil {
		return nil, ErrTooLong
	}
	// The encoder's default border is the required four light modules. Keep it
	// inside the symbol so every image consumer preserves the quiet zone.
	modules := symbol.Bitmap()
	if len(modules) > MaxModuleCount {
		return nil, ErrTooLong
	}
	return &Code{modules: modules}, nil
}

func (c *Code) ModuleCount() int {
	if c == nil {
		return 0
	}
	return len(c.modules)
}

// Image returns a read-only one-pixel-per-module image. Display at an integer
// scale with nearest-neighbor sampling; never interpolate or tint the symbol.
func (c *Code) Image() image.Image { return codeImage{code: c} }

type codeImage struct{ code *Code }

func (i codeImage) ColorModel() color.Model { return color.GrayModel }
func (i codeImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, i.code.ModuleCount(), i.code.ModuleCount())
}
func (i codeImage) At(x, y int) color.Color {
	if image.Pt(x, y).In(i.Bounds()) && i.code.modules[y][x] {
		return color.Gray{Y: 0}
	}
	return color.Gray{Y: 255}
}
