//go:build darwin

package appkit

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
	"unsafe"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// These tests validate input/decode rules and in-memory image representations.
// They never initialize MapKit, request location, run a native app, inspect
// desktop/clipboard state, or contact a map service.
func TestMapSnapshotRequestBounds(t *testing.T) {
	base := MapSnapshotRequest{Latitude: 31.2304, Longitude: 121.4737, LatitudeSpan: .02, LongitudeSpan: .02, Width: 188, Height: 188}
	if !validMapSnapshotRequest(base) {
		t.Fatal("valid map request rejected")
	}
	for _, change := range []func(*MapSnapshotRequest){
		func(r *MapSnapshotRequest) { r.Latitude = math.NaN() },
		func(r *MapSnapshotRequest) { r.Longitude = math.Inf(1) },
		func(r *MapSnapshotRequest) { r.Latitude = 90.1 },
		func(r *MapSnapshotRequest) { r.Longitude = -180.1 },
		func(r *MapSnapshotRequest) { r.LatitudeSpan = 0 },
		func(r *MapSnapshotRequest) { r.LongitudeSpan = 361 },
		func(r *MapSnapshotRequest) { r.Width = 0 },
		func(r *MapSnapshotRequest) { r.Width, r.Height = 1200, 1200 },
	} {
		request := base
		change(&request)
		if validMapSnapshotRequest(request) {
			t.Fatalf("invalid map request accepted: %#v", request)
		}
	}
}

func TestMapSnapshotEncodingKeepsRetinaRepresentation(t *testing.T) {
	if err := initAppKit(); err != nil {
		t.Fatal(err)
	}
	var setSize func(uintptr, objc.SEL, glassSize)
	purego.RegisterFunc(&setSize, nativeobjc.MessageSend())
	nativeobjc.WithPool(func() {
		img := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NSImage")), objc.RegisterName("alloc")), objc.RegisterName("init"))
		defer nativeobjc.Send(img, objc.RegisterName("release"))
		for _, pixels := range []int{376, 188} {
			bitmap := image.NewRGBA(image.Rect(0, 0, pixels, pixels))
			for y := range pixels {
				for x := range pixels {
					// Fine alternating columns distinguish a native 2x rep from
					// an enlarged 1x rep even if the output dimensions are correct.
					ink := color.RGBA{255, 255, 255, 255}
					if pixels == 376 && x%2 == 0 {
						ink = color.RGBA{0, 0, 0, 255}
					}
					bitmap.SetRGBA(x, y, ink)
				}
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, bitmap); err != nil {
				t.Fatal(err)
			}
			b := encoded.Bytes()
			data := nativeobjc.Send(uintptr(objc.GetClass("NSData")), objc.RegisterName("dataWithBytes:length:"), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
			rep := nativeobjc.Send(uintptr(objc.GetClass("NSBitmapImageRep")), objc.RegisterName("imageRepWithData:"), data)
			setSize(rep, objc.RegisterName("setSize:"), glassSize{188, 188})
			nativeobjc.Send(img, objc.RegisterName("addRepresentation:"), rep)
		}
		setSize(img, objc.RegisterName("setSize:"), glassSize{188, 188})
		encoded, err := nativeMapImagePNG(img, 188, 188)
		if err != nil {
			t.Fatal(err)
		}
		bitmap, err := decodeMapSnapshot(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if bitmap.Bounds() != image.Rect(0, 0, 376, 376) {
			t.Fatalf("Retina snapshot reduced to %v; want 376×376 pixels at 188×188 points", bitmap.Bounds())
		}
		black, _, _, _ := bitmap.At(0, 0).RGBA()
		white, _, _, _ := bitmap.At(1, 0).RGBA()
		if black != 0 || white != 0xffff {
			t.Fatalf("native 2x detail was resampled: neighboring columns = %x, %x", black, white)
		}
	})
}

func TestMapSnapshotEncodingRequestsRetinaRasterization(t *testing.T) {
	if err := initAppKit(); err != nil {
		t.Fatal(err)
	}
	var makeImage func(uintptr, objc.SEL, glassSize, bool, uintptr) uintptr
	var fillRect func(glassRect)
	purego.RegisterFunc(&makeImage, nativeobjc.MessageSend())
	purego.RegisterLibFunc(&fillRect, purego.RTLD_DEFAULT, "NSRectFill")
	for _, size := range []struct {
		name          string
		width, height int
	}{{"card", 188, 188}, {"preview", 504, 176}} {
		t.Run(size.name, func(t *testing.T) {
			nativeobjc.WithPool(func() {
				block := objc.NewBlock(func(_ objc.Block, rect glassRect) bool {
					for x := 0; x < size.width*2; x++ {
						name := "whiteColor"
						if x%2 == 0 {
							name = "blackColor"
						}
						color := nativeobjc.Send(uintptr(objc.GetClass("NSColor")), objc.RegisterName(name))
						nativeobjc.Send(color, objc.RegisterName("setFill"))
						fillRect(glassRect{Origin: glassPoint{float64(x) / 2, 0}, Size: glassSize{.5, float64(size.height)}})
					}
					return true
				})
				defer block.Release()
				img := makeImage(uintptr(objc.GetClass("NSImage")), objc.RegisterName("imageWithSize:flipped:drawingHandler:"), glassSize{float64(size.width), float64(size.height)}, false, uintptr(block))
				encoded, err := nativeMapImagePNG(img, size.width, size.height)
				if err != nil {
					t.Fatal(err)
				}
				bitmap, err := decodeMapSnapshot(encoded)
				if err != nil {
					t.Fatal(err)
				}
				if bitmap.Bounds() != image.Rect(0, 0, size.width*2, size.height*2) {
					t.Fatalf("scalable snapshot rasterized at %v; want %d×%d pixels at %d×%d points", bitmap.Bounds(), size.width*2, size.height*2, size.width, size.height)
				}
				black, _, _, _ := bitmap.At(0, 0).RGBA()
				white, _, _, _ := bitmap.At(1, 0).RGBA()
				if black != 0 || white != 0xffff {
					t.Fatalf("2x raster detail was lost: neighboring columns = %x, %x", black, white)
				}
			})
		})
	}
}

func TestMapSnapshotRejectsInvalidOrCancelledBeforeNativeInitialization(t *testing.T) {
	called := false
	completion := func(image.Image, error) { called = true }
	if err := SnapshotMap(context.Background(), MapSnapshotRequest{}, completion); err == nil || called {
		t.Fatal("invalid request reached completion")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := MapSnapshotRequest{LatitudeSpan: .02, LongitudeSpan: .02, Width: 188, Height: 188}
	if err := SnapshotMap(ctx, request, completion); err != context.Canceled || called {
		t.Fatal("cancelled request reached native setup")
	}
}

func TestMapSnapshotDecodeBounds(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	if got, err := decodeMapSnapshot(encoded.Bytes()); err != nil || got.Bounds().Dx() != 64 {
		t.Fatalf("valid snapshot unavailable: %v", err)
	}
	for _, data := range [][]byte{nil, []byte("not an image"), make([]byte, 8<<20+1)} {
		if _, err := decodeMapSnapshot(data); err == nil {
			t.Fatal("invalid map output accepted")
		}
	}
}
