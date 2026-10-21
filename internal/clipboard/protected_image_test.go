package clipboard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"copiggy/internal/clipmodel"
)

func TestCompleteProtectedPNGIsRecordedOnlyAfterConfirmation(t *testing.T) {
	store := newFixtureHistory()
	old, _ := store.Capture("previous clipboard", "Editor", false)
	before := store.Items()
	bridge := newFakeProtectedCopyBridge()
	bridge.onConfirm = func() {
		assertCopyStoreUnchanged(t, store, before, old.ID)
	}
	encoded := encodedTestPNG(t, 29, 17)
	captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		return pngClipboardData(encoded), nil
	}, store)
	if !ok || captured.Item.Type != clipmodel.TypeImage || captured.Item.Text != "" ||
		captured.Item.Image == nil || captured.Item.Image.Width != 29 || captured.Item.Image.Height != 17 ||
		captured.Item.Image.ByteSize != len(encoded) || captured.Source != bridge.request.Source ||
		captured.Revision != bridge.revision {
		t.Fatalf("pure PNG protected copy was not captured correctly: %+v, %v", captured, ok)
	}
	if len(bridge.confirmed) != 1 || len(bridge.canceled) != 0 ||
		!reflect.DeepEqual(captured.Item.AllowedApps, []string{bridge.request.Source.BundleID}) {
		t.Fatal("PNG confirmation or initial source restriction was incorrect")
	}
	original, err := captured.Data.Read(clipmodel.PNGFormat)
	if err != nil || !bytes.Equal(original, encoded) {
		t.Fatal("protected PNG capture changed or lost original image bytes")
	}
	current, exists := store.Current()
	if !exists || current.ID != captured.Item.ID || current.Allows("com.other.app") {
		t.Fatalf("protected PNG did not become a restricted current item: %+v", current)
	}
}

func TestCompleteProtectedPNGFailureDoesNotEnterHistory(t *testing.T) {
	validPNG := encodedTestPNG(t, 8, 6)
	for _, scenario := range []struct {
		name       string
		data       []byte
		readErr    error
		confirmErr error
		reject     bool
		race       bool
		confirm    bool
	}{
		{name: "read failed", data: validPNG, readErr: errors.New("clipboard unavailable")},
		{name: "invalid PNG", data: []byte("not a PNG")},
		{name: "truncated PNG", data: validPNG[:33]},
		{name: "oversized PNG dimensions", data: withPNGDimensions(validPNG, 6001, 4000)},
		{name: "clipboard changed during read", data: validPNG, race: true},
		{name: "native confirmation rejected", data: validPNG, reject: true, confirm: true},
		{name: "native confirmation failed", data: validPNG, confirmErr: errors.New("confirmation unavailable"), confirm: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := newFixtureHistory()
			old, _ := store.Capture("previous clipboard", "Editor", false)
			before := store.Items()
			bridge := newFakeProtectedCopyBridge()
			bridge.confirmResult = !scenario.reject
			bridge.confirmErr = scenario.confirmErr
			if _, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
				if scenario.race {
					bridge.revision++
				}
				return pngClipboardData(scenario.data), scenario.readErr
			}, store); ok {
				t.Fatal("failed PNG protected copy was recorded")
			}
			if got := len(bridge.confirmed) != 0; got != scenario.confirm {
				t.Fatalf("native confirmation attempted=%v; want %v", got, scenario.confirm)
			}
			assertCopyStoreUnchanged(t, store, before, old.ID)
		})
	}
}

func TestCompleteProtectedPNGReplacementRetainsHistoryButClearsCurrent(t *testing.T) {
	store := newFixtureHistory()
	bridge := newFakeProtectedCopyBridge()
	bridge.onConfirm = func() { bridge.revision++ }
	encoded := encodedTestPNG(t, 21, 13)
	captured, ok := completeProtectedCopy(bridge, func() (clipmodel.Representations, error) {
		return pngClipboardData(encoded), nil
	}, store)
	if !ok {
		t.Fatal("confirmed PNG history was discarded after a later external copy")
	}
	if _, exists := store.Current(); exists {
		t.Fatal("superseded PNG was still marked current")
	}
	history := store.Items()
	if len(history) != 1 || history[0].ID != captured.Item.ID ||
		history[0].Type != clipmodel.TypeImage || history[0].Image.Width != 21 ||
		history[0].Allows("com.other.app") {
		t.Fatalf("confirmed PNG lost its metadata or source restriction: %+v", history)
	}
}

func encodedTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 32, G: 96, B: 192, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func pngClipboardData(encoded []byte) clipmodel.Representations {
	return testData(testItem(testRep(clipmodel.PNGFormat, encoded)))
}

// A valid PNG header advertising oversized dimensions must be rejected before
// image allocation. Its IDAT intentionally remains the original small image.
func withPNGDimensions(encoded []byte, width, height uint32) []byte {
	changed := bytes.Clone(encoded)
	binary.BigEndian.PutUint32(changed[16:20], width)
	binary.BigEndian.PutUint32(changed[20:24], height)
	binary.BigEndian.PutUint32(changed[29:33], crc32.ChecksumIEEE(changed[12:29]))
	return changed
}
