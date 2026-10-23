package nativeui

import (
	"strings"
	"sync"

	"copiggy/internal/clipmodel"
	"copiggy/resources"
	"github.com/egoist/mygo/ui"
)

const nativePhoneFlagWidth, nativePhoneFlagHeight = 24, 18

var nativePhoneFlags = struct {
	sync.Mutex
	images map[string]*ui.SVG
}{images: make(map[string]*ui.SVG)}

// Region is the numbering library's result for this exact saved number. The
// non-geographic "001" code and unknown regions have no national flag. Only
// supported two-letter codes can enter the cache, which bounds it by the finite
// embedded region set. Nothing is fetched or installed on the user's machine.
func nativePhoneFlagRegion(phone clipmodel.PhoneValue) string {
	region := clipmodel.NormalizePhoneRegion(phone.Region)
	if len(region) != 2 {
		return ""
	}
	return region
}

func nativePhoneFlag(region string) *ui.SVG {
	region = nativePhoneFlagRegion(clipmodel.PhoneValue{Region: region})
	if region == "" {
		return nil
	}
	nativePhoneFlags.Lock()
	defer nativePhoneFlags.Unlock()
	if image, exists := nativePhoneFlags.images[region]; exists {
		return image
	}
	var image *ui.SVG
	if encoded, err := resources.RegionFlags.ReadFile("flags/" + strings.ToLower(region) + ".svg"); err == nil {
		image, _ = ui.ParseSVG(encoded)
	}
	nativePhoneFlags.images[region] = image
	return image
}

func (s *application) nativePhoneRegionMetadata(c *ui.Context, phone clipmodel.PhoneValue) {
	region := nativePhoneFlagRegion(phone)
	label := s.tr("Phone")
	if phone.International {
		label = s.tr("International")
	}
	if region != "" {
		label = nativeRegionName(region, s.currentLanguage())
	}
	ui.Row(c).MaxWidth(400).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if flag := nativePhoneFlag(region); flag != nil {
			// Image preserves the asset's original colors. The adjacent country
			// name supplies the accessible meaning without reading it twice.
			ui.Image(c, flag).Key("phone-region-flag").Size(nativePhoneFlagWidth, nativePhoneFlagHeight).Shrink(0).Fit(ui.Contain).Role(ui.RoleNone)
		}
		ui.Text(c, label).FontSize(nativeTextBody).FontWeight(nativeWeightBody).MinWidth(0).Wrap().TextAlign(ui.Center)
	})
}
