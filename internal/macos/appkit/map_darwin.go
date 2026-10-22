//go:build darwin

package appkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"sync"
	"unsafe"

	nativeobjc "copiggy/internal/macos/internal/objc"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// MapSnapshotRequest describes a system-rendered static map. It requests no
// device location. Callers must authorize the coordinate before invoking it;
// MapKit may contact Apple's map service to resolve the requested region.
type MapSnapshotRequest struct {
	Latitude, Longitude         float64
	LatitudeSpan, LongitudeSpan float64
	Width, Height               int
	Dark                        bool
}

type mapCoordinate struct{ Latitude, Longitude float64 }
type mapSpan struct{ LatitudeDelta, LongitudeDelta float64 }
type mapRegion struct {
	Center mapCoordinate
	Span   mapSpan
}

var mapNative struct {
	once      sync.Once
	err       error
	setRegion func(uintptr, objc.SEL, mapRegion)
	setSize   func(uintptr, objc.SEL, glassSize)
}

func initMapNative() error {
	if err := initGlassNative(); err != nil {
		return err
	}
	mapNative.once.Do(func() {
		defer func() {
			if failure := recover(); failure != nil {
				mapNative.err = fmt.Errorf("map runtime: %v", failure)
			}
		}()
		_, mapNative.err = purego.Dlopen("/System/Library/Frameworks/MapKit.framework/MapKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if mapNative.err != nil {
			return
		}
		purego.RegisterFunc(&mapNative.setRegion, nativeobjc.MessageSend())
		purego.RegisterFunc(&mapNative.setSize, nativeobjc.MessageSend())
	})
	return mapNative.err
}

func validMapSnapshotRequest(request MapSnapshotRequest) bool {
	finite := func(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
	return finite(request.Latitude) && finite(request.Longitude) &&
		math.Abs(request.Latitude) <= 90 && math.Abs(request.Longitude) <= 180 &&
		finite(request.LatitudeSpan) && finite(request.LongitudeSpan) &&
		request.LatitudeSpan > 0 && request.LatitudeSpan <= 180 &&
		request.LongitudeSpan > 0 && request.LongitudeSpan <= 360 &&
		request.Width >= 64 && request.Height >= 64 &&
		request.Width <= 1200 && request.Height <= 1200 && request.Width*request.Height <= 960_000
}

// SnapshotMap begins an asynchronous MKMapSnapshotter request. On a successful
// start, completion runs once on a worker goroutine, including cancellation.
// An immediate error means completion will not be called. Snapshot output
// retains MapKit attribution and contains no fabricated map tiles or markers.
// The caller draws its coordinate marker over the image and must not obscure
// system attribution. No UI, clipboard or permission state is inspected here.
func SnapshotMap(ctx context.Context, request MapSnapshotRequest, completion func(image.Image, error)) error {
	if ctx == nil || completion == nil || !validMapSnapshotRequest(request) {
		return errors.New("invalid map snapshot request")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := errors.New("map snapshot did not start")
	mygo.RunOnMain(func() {
		// Framework loading, appearance lookup, and every Objective-C message
		// belong to AppKit's main thread, including the first map request.
		if err = initMapNative(); err != nil {
			return
		}
		nativeobjc.WithPool(func() {
			if err = ctx.Err(); err != nil {
				return
			}
			options := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("MKMapSnapshotOptions")), objc.RegisterName("alloc")), objc.RegisterName("init"))
			if options == 0 {
				err = errors.New("map snapshot options unavailable")
				return
			}
			defer nativeobjc.Send(options, objc.RegisterName("release"))
			mapNative.setRegion(options, objc.RegisterName("setRegion:"), mapRegion{
				Center: mapCoordinate{request.Latitude, request.Longitude},
				Span:   mapSpan{request.LatitudeSpan, request.LongitudeSpan},
			})
			mapNative.setSize(options, objc.RegisterName("setSize:"), glassSize{float64(request.Width), float64(request.Height)})
			nativeobjc.Send(options, objc.RegisterName("setAppearance:"), glassAppearance(request.Dark))
			snapshotter := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("MKMapSnapshotter")), objc.RegisterName("alloc")), objc.RegisterName("initWithOptions:"), options)
			if snapshotter == 0 {
				err = errors.New("map snapshotter unavailable")
				return
			}
			// These values belong to AppKit's main thread. The context watcher
			// only schedules cancellation onto that same thread.
			finished := false
			done := make(chan struct{})
			finish := func(data []byte, failure error, cancel bool) {
				if finished {
					return
				}
				finished = true
				close(done)
				if cancel {
					nativeobjc.Send(snapshotter, objc.RegisterName("cancel"))
				}
				nativeobjc.Send(snapshotter, objc.RegisterName("release"))
				go func() {
					var bitmap image.Image
					if failure == nil {
						bitmap, failure = decodeMapSnapshot(data)
					}
					if ctx.Err() != nil {
						bitmap, failure = nil, ctx.Err()
					}
					completion(bitmap, failure)
				}()
			}
			block := objc.NewBlock(func(_ objc.Block, snapshot, failure objc.ID) {
				if finished {
					return
				}
				if failure != 0 || snapshot == 0 {
					finish(nil, errors.New("map snapshot unavailable"), false)
					return
				}
				nativeobjc.WithPool(func() {
					data, failure := nativeMapSnapshotPNG(uintptr(snapshot), request)
					finish(data, failure, false)
				})
			})
			// MapKit copies this block until completion or cancellation.
			nativeobjc.Send(snapshotter, objc.RegisterName("startWithCompletionHandler:"), uintptr(block))
			block.Release()
			err = nil
			go func() {
				select {
				case <-done:
				case <-ctx.Done():
					mygo.RunOnMain(func() { finish(nil, ctx.Err(), true) })
				}
			}()
		})
	})
	return err
}

func nativeMapSnapshotPNG(snapshot uintptr, request MapSnapshotRequest) ([]byte, error) {
	image := nativeobjc.Send(snapshot, objc.RegisterName("image"))
	return nativeMapImagePNG(image, request.Width, request.Height)
}

func nativeMapImagePNG(image uintptr, width, height int) ([]byte, error) {
	if image == 0 || width <= 0 || height <= 0 || width > 1200 || height > 1200 || width*height > 960_000 {
		return nil, errors.New("map image unavailable")
	}
	// macOS MapKit returns an NSImage with standard and high-resolution
	// representations. Its options.size is in points; unlike iOS there is no
	// options.scale. Select/rasterize the native 2x image explicitly, without
	// changing the geographic region, label size, or logical snapshot size.
	// TIFFRepresentation can rasterize a lazy NSImage at only 1x before its
	// representations reach NSBitmapImageRep, irreversibly losing fine detail.
	ctmAddress, err := purego.Dlsym(purego.RTLD_DEFAULT, "NSImageHintCTM")
	if err != nil {
		return nil, errors.New("map image scale unavailable")
	}
	ctmKey := **(**uintptr)(unsafe.Pointer(&ctmAddress))
	identity := nativeobjc.Send(uintptr(objc.GetClass("NSAffineTransform")), objc.RegisterName("transform"))
	hints := nativeobjc.Send(uintptr(objc.GetClass("NSDictionary")), objc.RegisterName("dictionaryWithObject:forKey:"), identity, ctmKey)
	// With the identity CTM hint, AppKit interprets this destination in pixels,
	// independent of the active display or ambient graphics context.
	pixels := glassRect{Size: glassSize{float64(width * 2), float64(height * 2)}}
	cgImage := nativeobjc.Send(image, objc.RegisterName("CGImageForProposedRect:context:hints:"), uintptr(unsafe.Pointer(&pixels)), 0, hints)
	if cgImage == 0 {
		return nil, errors.New("map image unavailable")
	}
	representation := nativeobjc.Send(nativeobjc.Send(uintptr(objc.GetClass("NSBitmapImageRep")), objc.RegisterName("alloc")), objc.RegisterName("initWithCGImage:"), cgImage)
	if representation == 0 {
		return nil, errors.New("map image unavailable")
	}
	defer nativeobjc.Send(representation, objc.RegisterName("release"))
	// AppKit treats the requested size as a hint. Keep the existing decoder
	// pixel bound before encoding too, including unexpected oversized reps.
	pixelWidth := nativeobjc.Send(representation, objc.RegisterName("pixelsWide"))
	pixelHeight := nativeobjc.Send(representation, objc.RegisterName("pixelsHigh"))
	if pixelWidth == 0 || pixelHeight == 0 || pixelWidth > 2400 || pixelHeight > 2400 || pixelWidth*pixelHeight > 3_840_000 {
		return nil, errors.New("invalid map image dimensions")
	}
	properties := nativeobjc.Send(uintptr(objc.GetClass("NSDictionary")), objc.RegisterName("dictionary"))
	// NSBitmapImageFileTypePNG = 4.
	data := nativeobjc.Send(representation, objc.RegisterName("representationUsingType:properties:"), 4, properties)
	length := nativeobjc.Send(data, objc.RegisterName("length"))
	address := nativeobjc.Send(data, objc.RegisterName("bytes"))
	if address == 0 || length == 0 || length > 8<<20 {
		return nil, errors.New("map image encoding unavailable")
	}
	return bytes.Clone(unsafe.Slice(*(**byte)(unsafe.Pointer(&address)), int(length))), nil
}

func decodeMapSnapshot(data []byte) (image.Image, error) {
	if len(data) == 0 || len(data) > 8<<20 {
		return nil, errors.New("invalid map image data")
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	// macOS may return a Retina image at twice the requested dimensions.
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 3_840_000 {
		return nil, errors.New("invalid map image dimensions")
	}
	return png.Decode(bytes.NewReader(data))
}
