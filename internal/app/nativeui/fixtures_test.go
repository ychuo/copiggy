package nativeui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"copiggy/internal/clipmodel"
)

// UI fixtures enter through the same history service as observed clipboard
// records. No native clipboard, window, or persistence directory is involved.
func nativeCapture(t *testing.T, s *application, data clipmodel.Representations, source clipmodel.Source, allowed ...string) clipmodel.Item {
	t.Helper()
	item, recorded, err := s.store.Capture(context.Background(), data, source, allowed)
	if err != nil || !recorded {
		t.Fatalf("capture native UI fixture: recorded=%v, err=%v", recorded, err)
	}
	return item
}

func nativeCaptureText(t *testing.T, s *application, text, source string) clipmodel.Item {
	t.Helper()
	return nativeCapture(t, s, clipmodel.TextData(text), clipmodel.Source{Name: source})
}

func nativeCaptureImage(t *testing.T, s *application, data []byte, source string) clipmodel.Item {
	t.Helper()
	return nativeCapture(t, s, clipmodel.Representations{{{Format: clipmodel.PNGFormat, Bytes: data}}}, clipmodel.Source{Name: source})
}

func encodedNativePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 32, G: 96, B: 192, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
