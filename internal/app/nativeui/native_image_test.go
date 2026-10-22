package nativeui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo/ui"
)

func nativeImagePreviewFixture(t *testing.T, width, height int, ink color.RGBA) (*application, clipmodel.Item) {
	t.Helper()
	s := nativeViewFixture(t)
	s.language = "en"
	item := nativeCaptureImage(t, s, encodedNativePNG(t, width, height), "Fixture image editor")
	s.filter()
	s.selectNative(0, true)
	s.preview = true
	pixels := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixels.SetRGBA(x, y, ink)
		}
	}
	s.viewState.Images[item.ID] = ui.NewBitmap(pixels)
	return s, item
}

func TestImageDisplayPreservesCompleteAspectRatioAtEveryZoom(t *testing.T) {
	for _, dimensions := range [][2]int{{1600, 1200}, {1200, 1600}, {1, 1}, {8192, 1}, {1, 8192}} {
		for zoom := 0; zoom <= 4; zoom++ {
			w, h := nativeImageDisplaySize(dimensions[0], dimensions[1], 504, 332, zoom)
			if w <= 0 || h <= 0 || math.IsInf(float64(w), 0) || math.IsInf(float64(h), 0) {
				t.Fatalf("%v at zoom %d produced invalid size %g × %g", dimensions, zoom, w, h)
			}
			wantRatio := float64(dimensions[0]) / float64(dimensions[1])
			if math.Abs(float64(w)/float64(h)/wantRatio-1) > .00001 {
				t.Fatalf("%v at zoom %d changed aspect ratio: %g × %g", dimensions, zoom, w, h)
			}
			if zoom == 0 && (w > 504.001 || h > 332.001) {
				t.Fatalf("Fit crops %v to %g × %g", dimensions, w, h)
			}
		}
	}
	for _, dimensions := range [][2]int{{0, 1}, {1, 0}, {-1, 1}} {
		if w, h := nativeImageDisplaySize(dimensions[0], dimensions[1], 504, 332, 0); w != 0 || h != 0 {
			t.Fatal("invalid image dimensions must not reach image layout")
		}
	}
	if w, h := nativeImageDisplaySize(100, 100, 504, 0, 0); w != 0 || h != 0 {
		t.Fatal("an exhausted viewport must not overflow the fixed toolbar/footer")
	}
}

func TestImagePreviewKeepsRoundedGlassMarginAndOriginalPixels(t *testing.T) {
	ink := color.RGBA{R: 32, G: 96, B: 192, A: 255}
	for _, dark := range []bool{false, true} {
		s, _ := nativeImagePreviewFixture(t, 504, 332, ink)
		s.viewState.Dark = dark
		tt := ui.NewTester(s.nativePopoverView, 520, nativeImagePreviewHeight)
		pixels := tt.Image()
		for _, point := range []image.Point{{7, 180}, {9, 51}, {510, 51}, {9, 380}, {510, 380}} {
			if got := pixels.RGBAAt(point.X, point.Y); got.A != 0 {
				t.Fatalf("dark=%t image fills glass/corner %v: %v", dark, point, got)
			}
		}
		for _, point := range []image.Point{{260, 80}, {260, 200}, {260, 370}} {
			if got := pixels.RGBAAt(point.X, point.Y); got != ink {
				t.Fatalf("dark=%t image was tinted or covered at %v: %v", dark, point, got)
			}
		}
		caption, ok := tt.Find("504 × 332 · PNG")
		if !ok || caption.X != 22 || caption.Y < 390 || caption.Y+caption.H > 412 {
			t.Fatalf("metadata is not aligned below the image: %+v", caption)
		}
	}
}

func TestImagePreviewContainsPortraitAndPreservesTransparency(t *testing.T) {
	ink := color.RGBA{R: 24, G: 48, B: 64, A: 128}
	s, _ := nativeImagePreviewFixture(t, 80, 160, ink)
	tt := ui.NewTester(s.nativePopoverView, 520, nativeImagePreviewHeight)
	imageRect, ok := tt.Find("Copied image")
	if !ok || math.Abs(float64(imageRect.W/imageRect.H)-.5) > .001 || imageRect.X < 175 || imageRect.X+imageRect.W > 345 {
		t.Fatalf("portrait image was cropped or stretched: %+v", imageRect)
	}
	pixels := tt.Image()
	if outside := pixels.RGBAAt(20, 180); outside.A != 0 {
		t.Fatalf("letterbox is not transparent glass: %v", outside)
	}
	if center := pixels.RGBAAt(260, 180); center.A < 126 || center.A > 130 || center.R > 26 || center.G > 50 || center.B > 66 {
		t.Fatalf("alpha image acquired an opaque background or altered ink: %v", center)
	}
}

func TestImagePreviewZoomRetainsFocusOriginalAndReachableImage(t *testing.T) {
	s, item := nativeImagePreviewFixture(t, 504, 332, color.RGBA{R: 32, G: 96, B: 192, A: 255})
	gradient := image.NewRGBA(image.Rect(0, 0, 504, 332))
	for y := 0; y < 332; y++ {
		for x := 0; x < 504; x++ {
			gradient.SetRGBA(x, y, color.RGBA{R: uint8(x / 2), G: uint8(y / 2), B: 90, A: 255})
		}
	}
	s.viewState.Images[item.ID] = ui.NewBitmap(gradient)
	before := s.store.Records()
	copyCalls := 0
	s.config.Copy = func(context.Context, string, bool) (clipmodel.Item, error) {
		copyCalls++
		return clipmodel.Item{}, nil
	}
	tt := ui.NewTester(s.nativePopoverView, 520, nativeImagePreviewHeight)
	for i := 0; i < 5; i++ {
		if err := tt.Click("Zoom in"); err != nil {
			t.Fatal(err)
		}
	}
	if s.semanticPreview.ImageZoom != 4 || !tt.Focused("Content preview") {
		t.Fatal("image zoom escaped bounds or stole preview focus")
	}
	beforeScroll := tt.Image().RGBAAt(260, 200)
	tt.Scroll(260, 200, 400, 0)
	afterHorizontal := tt.Image().RGBAAt(260, 200)
	if afterHorizontal.R == beforeScroll.R || afterHorizontal.G != beforeScroll.G {
		t.Fatalf("magnified image does not pan horizontally: before=%v after=%v", beforeScroll, afterHorizontal)
	}
	tt.Scroll(260, 200, 0, 400)
	afterVertical := tt.Image().RGBAAt(260, 200)
	if afterVertical.G == afterHorizontal.G || afterVertical.R != afterHorizontal.R {
		t.Fatalf("magnified image does not pan vertically: before=%v after=%v", afterHorizontal, afterVertical)
	}
	if err := tt.Click("Fit"); err != nil {
		t.Fatal(err)
	}
	fitted, _ := tt.Find("Copied image")
	if s.semanticPreview.ImageZoom != 0 || fitted.X < 8 || fitted.Y < 50 || fitted.X+fitted.W > 512 || fitted.Y+fitted.H > 382 {
		t.Fatalf("Fit did not restore the complete image: %+v", fitted)
	}
	if !tt.Focused("Content preview") || copyCalls != 0 || !reflect.DeepEqual(before, s.store.Records()) {
		t.Fatal("image zoom/fit changed focus, clipboard, or retained record")
	}
	assertNativeCurrent(t, s, item.ID)
}

func TestImagePreviewToolbarAndMetadataFitEveryLanguageAndReducedHeight(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		for _, dark := range []bool{false, true} {
			for _, height := range []int{nativeImagePreviewHeight, 220} {
				t.Run(fmt.Sprintf("%s/dark=%t/height=%d", language, dark, height), func(t *testing.T) {
					s, item := nativeImagePreviewFixture(t, 64, 32, color.RGBA{R: 32, G: 96, B: 192, A: 255})
					s.language, s.viewState.Dark = language, dark
					if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
						t.Fatal(err)
					}
					s.filter()
					tt := ui.NewTester(s.nativePopoverView, 520, height)
					lastRight := float32(0)
					for _, label := range []string{"Zoom out", "Zoom in", "Fit", "Pin", "Delete item · ⌘⌫", "Close preview"} {
						r, ok := tt.Find(s.tr(label))
						if !ok || r.X < lastRight || r.X+r.W > 498 || r.Y < 12 || r.Y+r.H > 42 {
							t.Fatalf("toolbar action %q escaped/overlapped: %+v", label, r)
						}
						lastRight = r.X + r.W
					}
					caption, ok := tt.Find("64 × 32 · PNG")
					if !ok || caption.Y < float32(height-62) || caption.Y+caption.H > float32(height-38) {
						t.Fatalf("scope or image displaced fixed metadata: %+v", caption)
					}
					image, ok := tt.Find(s.tr("Copied image"))
					if !ok || image.Y < 50 || image.Y+image.H > caption.Y-5 {
						t.Fatalf("image overlaps fixed chrome: %+v metadata=%+v", image, caption)
					}
				})
			}
		}
	}
}

func TestImagePreviewLoadingAndFailureKeepMetadata(t *testing.T) {
	for _, loading := range []bool{false, true} {
		s, item := nativeImagePreviewFixture(t, 64, 32, color.RGBA{A: 255})
		s.viewState.ImageLoading = map[string]bool{item.ID: loading}
		tt := ui.NewTester(func(c *ui.Context) {
			setNativeTheme(c, panelColors(false), false)
			s.nativeImagePreviewWithBitmap(c, item, panelColors(false), nil)
		}, 520, nativeImagePreviewHeight)
		if tt.HasText("Image unavailable") == loading || !tt.HasText("64 × 32 · PNG") {
			t.Fatalf("pending/failed image state lost metadata or misreported an error, loading=%t", loading)
		}
	}
}
