package nativeui

import (
	"cmp"
	"slices"

	"copiggy/internal/clipmodel"
	"github.com/egoist/mygo"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func (s *application) nativePhoneRegionMenu() []*mygo.MenuItem {
	regionLabel := func(code string) string {
		return nativeRegionName(code, s.currentLanguage())
	}
	systemLabel := s.tr("System region")
	if region := clipmodel.NormalizePhoneRegion(s.config.SystemRegion); region != "" {
		systemLabel += " · " + regionLabel(region)
	}
	choice := func(region, label string) *mygo.MenuItem {
		return &mygo.MenuItem{
			ID: "phone-region-" + region, Label: label, Type: mygo.MenuItemRadio, Checked: s.phoneRegion == region,
			Click: func(*mygo.MenuItem, *mygo.Window) { s.setPhoneRegion(region) },
		}
	}
	items := []*mygo.MenuItem{choice("", systemLabel), mygo.Separator()}
	// Country names come from the same offline CLDR catalog in both sections;
	// these codes only control menu order, never number recognition.
	common := []string{"CN", "HK", "MO", "TW", "JP", "US", "CA", "GB", "FR", "DE", "IN", "AU"}
	seen := map[string]bool{}
	for _, code := range common {
		items = append(items, choice(code, regionLabel(code)))
		seen[code] = true
	}
	var more []*mygo.MenuItem
	for _, code := range clipmodel.SupportedPhoneRegions() {
		if seen[code] {
			continue
		}
		more = append(more, choice(code, regionLabel(code)))
	}
	slices.SortStableFunc(more, func(a, b *mygo.MenuItem) int { return cmp.Compare(a.Label, b.Label) })
	items = append(items, &mygo.MenuItem{ID: "phone-region-more", Label: s.tr("More regions"), Submenu: more})
	return items
}

// Menus and recognized-number metadata share CLDR's offline region names.
// The display language changes only this label, never the numbering region.
func nativeRegionName(code, locale string) string {
	region, err := language.ParseRegion(code)
	if err != nil {
		return code
	}
	return display.Regions(language.Make(locale)).Name(region)
}

func (s *application) setPhoneRegion(region string) {
	if s.quitting || region == s.phoneRegion || (region != "" && clipmodel.NormalizePhoneRegion(region) == "") {
		return
	}
	previous := s.phoneRegion
	s.phoneRegion = region
	s.preferenceGeneration++
	generation := s.preferenceGeneration
	p := s.preferenceSnapshot()
	s.perform(func() error {
		if s.config.SavePreferences != nil {
			return s.config.SavePreferences(p)
		}
		return nil
	}, func(err error) {
		if err != nil {
			if s.preferenceGeneration == generation && s.phoneRegion == region {
				s.phoneRegion = previous
			}
			s.reportNativeError("save phone region", err, "Could not save phone region")
		}
		s.publish()
	})
}
