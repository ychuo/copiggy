package linkqr

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func TestQRRoundTripsAtPhonePreviewSize(t *testing.T) {
	for _, target := range []string{
		"https://example.com/",
		"https://example.com/article?a=1&b=two#section",
		"https://example.com/路径?q=测试&emoji=🐷",
		"aweme://aweme/detail/123456789?source=copy",
		"sslocal://search?keyword=%E6%B5%8B%E8%AF%95",
		"https://example.com/" + strings.Repeat("a", 1600),
	} {
		t.Run(target[:min(len(target), 60)], func(t *testing.T) {
			code, err := Encode(target)
			if err != nil {
				t.Fatal(err)
			}
			scale := 320 / code.ModuleCount()
			if scale < 2 {
				t.Fatalf("symbol cannot fit at two pixels per module: %d", code.ModuleCount())
			}
			pixels := scaleQR(code.Image(), scale)
			binary, err := gozxing.NewBinaryBitmapFromImage(pixels)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := qrcode.NewQRCodeReader().Decode(binary, nil)
			if err != nil {
				t.Fatalf("independent decoder failed at %dpx: %v", pixels.Bounds().Dx(), err)
			}
			if got := decoded.GetText(); got != target {
				t.Fatalf("QR target changed: got %q, want %q", got, target)
			}
		})
	}
}

func TestQRHasFourModuleQuietZoneAndOnlyOpaqueBlackWhite(t *testing.T) {
	code, err := Encode("https://example.com/a?b=1#c")
	if err != nil {
		t.Fatal(err)
	}
	pixels, n := code.Image(), code.ModuleCount()
	black := false
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			r, g, b, a := pixels.At(x, y).RGBA()
			if a != 65535 || r != g || r != b || (r != 0 && r != 65535) {
				t.Fatalf("non-binary or translucent module at %d,%d", x, y)
			}
			border := x < 4 || y < 4 || x >= n-4 || y >= n-4
			if border && r != 65535 {
				t.Fatalf("quiet zone contains a dark module at %d,%d", x, y)
			}
			black = black || r == 0
		}
	}
	if !black {
		t.Fatal("empty QR symbol")
	}
	if _, mutable := pixels.(*image.Gray); mutable {
		t.Fatal("cached module image exposes a mutable pixel buffer")
	}
}

func TestQRRejectsOversizedAndInvalidTargets(t *testing.T) {
	for _, target := range []string{
		"https://example.com/" + strings.Repeat("a", MaxContentBytes),
		"https://example.com/" + strings.Repeat("a", 1800),
	} {
		if _, err := Encode(target); !errors.Is(err, ErrTooLong) {
			t.Fatalf("oversized target returned %v", err)
		}
	}
	for _, target := range []string{"", "https://example.com/\x00", "aweme://x\nnext", "https://example.com/\xff"} {
		if _, err := Encode(target); !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("invalid target returned %v", err)
		}
	}
}

func scaleQR(source image.Image, scale int) *image.Gray {
	n := source.Bounds().Dx()
	dest := image.NewGray(image.Rect(0, 0, n*scale, n*scale))
	for y := 0; y < dest.Bounds().Dy(); y++ {
		for x := 0; x < dest.Bounds().Dx(); x++ {
			dest.SetGray(x, y, color.GrayModel.Convert(source.At(x/scale, y/scale)).(color.Gray))
		}
	}
	return dest
}
