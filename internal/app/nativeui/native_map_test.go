package nativeui

import (
	"context"
	"image"
	"image/color"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestMapPreviewZoomKeepsKeyboardFocusAndOriginal(t *testing.T) {
	s, item := semanticPreviewFixture(t, "31.2304123, 121.4737123")
	before := s.store.Records()
	tt := ui.NewTester(s.nativePopoverView, nativeMapPreviewWidth, nativeMapPreviewHeight)
	mapRect, ok := tt.Find("Map preview")
	if !ok || mapRect.W != nativeMapImageWidth || mapRect.H != nativeMapImageHeight {
		t.Fatalf("map area does not match its uncropped snapshot: %+v", mapRect)
	}
	for _, label := range []string{"Zoom in", "Zoom out"} {
		r, ok := tt.Find(label)
		if !ok || r.X < mapRect.X+mapRect.W-50 || r.Y+r.H > mapRect.Y+mapRect.H-nativeMapAttributionInset {
			t.Fatalf("%s covers map attribution or escaped the map: %+v", label, r)
		}
	}
	copy, ok := tt.Find("Copy")
	if !ok || copy.Y < mapRect.Y+mapRect.H || !tt.HasText(item.Text) || !tt.HasText("WGS 84") {
		t.Fatal("original coordinates or copy control missing from map metadata")
	}
	for _, step := range []struct {
		label       string
		count, want int
	}{{"Zoom in", 5, 3}, {"Zoom out", 7, -2}} {
		for i := 0; i < step.count; i++ {
			if err := tt.Click(step.label); err != nil {
				t.Fatal(err)
			}
		}
		if s.semanticPreview.MapZoom != step.want || !tt.Focused("Content preview") {
			t.Fatalf("zoom did not retain bounds or navigation focus: %+v", s.semanticPreview)
		}
	}
	if !reflect.DeepEqual(before, s.store.Records()) || s.contentAssets != nil {
		t.Fatal("detached map preview changed history or started native map work")
	}
	assertNativeCurrent(t, s, item.ID)
	tt.Key(0, ui.KeyEscape)
	if s.preview || s.semanticPreview.MapZoom != 0 {
		t.Fatal("closing retained map zoom session")
	}
}

func TestMapProtectedAndFailedStatesStayLocalAcrossAppearance(t *testing.T) {
	for _, protected := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			s, item := semanticPreviewFixture(t, "31.2304, 121.4737")
			s.viewState.Dark = dark
			want := "Map unavailable"
			if protected {
				if err := s.store.SetAllowedApps(context.Background(), item.ID, []string{"com.example.Editor"}); err != nil {
					t.Fatal(err)
				}
				s.filter()
				want = "Protected location"
			} else {
				key := nativeMapAssetKey(item.ID, 0, true)
				s.contentAssetState().entries[key] = &nativeContentAsset{failed: time.Now()}
			}
			tt := ui.NewTester(s.nativePopoverView, nativeMapPreviewWidth, nativeMapPreviewHeight)
			if !tt.HasText(want) || !tt.HasText(item.Text) {
				t.Fatalf("map state %q or original coordinates missing, dark=%t", want, dark)
			}
			if protected {
				if _, exists := tt.Find("Zoom in"); exists || s.contentAssets != nil {
					t.Fatal("protected location exposed map controls or started map work")
				}
			}
			assertNativeCurrent(t, s, item.ID)
		}
	}
}

func TestMapImageKeepsRoundedMarginAndAttributionPixels(t *testing.T) {
	// A local fixture represents the attribution strip. No MapKit request or
	// desktop capture: the complete bitmap must remain visible inside the
	// rounded reading surface, including its bottom center.
	fixture := image.NewRGBA(image.Rect(0, 0, nativeMapImageWidth, nativeMapImageHeight))
	for y := 0; y < nativeMapImageHeight; y++ {
		for x := 0; x < nativeMapImageWidth; x++ {
			fixture.SetRGBA(x, y, color.RGBA{R: 220, G: 232, B: 213, A: 255})
		}
	}
	for x := 30; x < nativeMapImageWidth-30; x++ {
		fixture.SetRGBA(x, nativeMapImageHeight-1, color.RGBA{R: 32, G: 42, B: 51, A: 255})
	}
	bitmap := ui.NewBitmap(fixture)
	tt := ui.NewTester(func(c *ui.Context) {
		setNativeTheme(c, nativeMapColors(), false)
		ui.Box(c).Fill().Padding(8).Children(func() {
			ui.Box(c).Fill().Radius(nativePreviewPaperRadius).Clip().Children(func() { nativeMapImage(c, bitmap, 1.3) })
		})
	}, nativeMapImageWidth+16, nativeMapImageHeight+16)
	pixels := tt.Image()
	for _, point := range []image.Point{{8, 8}, {511, 8}, {8, 183}, {511, 183}} {
		if got := pixels.RGBAAt(point.X, point.Y); got.A != 0 {
			t.Fatalf("map filled rounded glass corner %v: %v", point, got)
		}
	}
	if got := pixels.RGBAAt(100, 183); got.R > 40 || got.A != 255 {
		t.Fatalf("map attribution strip was cropped: %v", got)
	}
}

func TestMapTypeGlyphHasOpenCenterAtControlSizes(t *testing.T) {
	for _, size := range []int{nativeIconSize, nativeToolbarIconSize} {
		tt := ui.NewTester(func(c *ui.Context) {
			setNativeTheme(c, nativeMapColors(), false)
			nativeGlyph(c, "MapPin", float32(size)).TextColor(ui.Hex("#78818b"))
		}, size, size)
		pixels := tt.Image()
		center := pixels.RGBAAt(size/2, int(float32(size)*9.7/24))
		if center.A > 30 {
			t.Fatalf("%d-point location glyph lost its center hole: %v", size, center)
		}
	}
}

func TestMapMarkerRetainsWhiteDetails(t *testing.T) {
	for _, scale := range []float32{1, 1.3} {
		tt := ui.NewTester(func(c *ui.Context) {
			setNativeTheme(c, nativeMapColors(), false)
			ui.Box(c).Fill().Draw(func(p *ui.Painter, bounds ui.Rect) {
				nativeMapMarker(p, bounds, scale)
			})
		}, 100, 100)
		pixels := tt.Image()
		center := pixels.RGBAAt(50, int(50-22*scale))
		if center.R < 245 || center.G < 245 || center.B < 245 || center.A < 245 {
			t.Fatalf("marker white center became a monochrome silhouette at scale %g: %v", scale, center)
		}
		white := 0
		for y := int(50 - 34*scale); y < 50; y++ {
			for x := int(50 - 14*scale); x < int(50+14*scale); x++ {
				pixel := pixels.RGBAAt(x, y)
				if pixel.R > 240 && pixel.G > 240 && pixel.B > 240 && pixel.A > 200 {
					white++
				}
			}
		}
		if white < int(75*scale*scale) {
			t.Fatalf("marker is missing its white outline: %d white pixels", white)
		}
	}
}
