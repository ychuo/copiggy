package contentactions

import (
	"errors"
	"testing"

	"copiggy/internal/clipmodel"
)

func TestPhoneActionUsesSavedRegionAndExtension(t *testing.T) {
	item := clipmodel.Item{Type: clipmodel.TypePhone, Text: "020 7946 0958 ext. 0012", RecognitionRegion: "GB"}
	target, err := OpenItemTarget(item)
	if err != nil || target.URL != "tel:+442079460958;ext=0012" {
		t.Fatalf("phone action lost region or extension: %+v %v", target, err)
	}
	item.RecognitionRegion = "CN"
	target, err = OpenItemTarget(item)
	if err != nil || target.URL != "tel:+862079460958;ext=0012" {
		t.Fatalf("national digits ignored explicit region: %+v %v", target, err)
	}
	item.RecognitionRegion = "US"
	if _, err := OpenItemTarget(item); !errors.Is(err, ErrUnsupported) {
		t.Fatal("stale phone classification authorized a call")
	}
}
