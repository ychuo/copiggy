package clipmodel

import (
	"strings"

	"github.com/nyaruka/phonenumbers/v2"
	"github.com/nyaruka/phonenumbers/v2/geocoding"
)

// PhoneLocation returns the numbering-prefix area (city, province or state),
// never the subscriber's current location. It reads only embedded metadata and
// does not change recognition, the saved region, or any clipboard content.
//
// A translated area is preferred; unavailable translations fall back to the
// original English area name. Country-only results are omitted because the UI
// already displays the number's country. Only the four UI locales are accepted,
// bounding the geocoder's lazy per-language metadata cache. Call this on demand
// for a preview, not during capture or card rendering.
func PhoneLocation(phone PhoneValue, locale string) string {
	var metadataLanguage string
	switch locale {
	case "en", "ja":
		metadataLanguage = locale
	case "zh-Hans":
		metadataLanguage = "zh"
	case "zh-Hant":
		metadataLanguage = "zh_Hant"
	default:
		return ""
	}
	// Derive everything from the canonical validated number, not display text,
	// a caller-provided country name, or the current system region.
	if len(phone.E164) < 2 || len(phone.E164) > 16 || phone.E164[0] != '+' || !asciiDigits(phone.E164[1:]) {
		return ""
	}
	number, err := phonenumbers.Parse(phone.E164, "ZZ")
	if err != nil || !phonenumbers.IsValidNumber(number) || phonenumbers.Format(number, phonenumbers.E164) != phone.E164 || !phonenumbers.IsNumberGeographical(number) {
		return ""
	}
	if location := phoneLocationInLanguage(number, metadataLanguage); location != "" {
		return location
	}
	if metadataLanguage != "en" {
		return phoneLocationInLanguage(number, "en")
	}
	return ""
}

func phoneLocationInLanguage(number *phonenumbers.PhoneNumber, lang string) string {
	location, err := geocoding.GetDescriptionForNumber(number, lang)
	if err != nil || strings.TrimSpace(location) == "" {
		return ""
	}
	// An unknown user region asks the upstream geocoder for its country-level
	// result. Comparing both descriptions avoids country-name translations or
	// territory-specific exceptions in our recognition rules.
	country, err := geocoding.GetDescriptionForNumberForUserRegion(number, lang, "ZZ")
	if err != nil || country == "" || location == country {
		return ""
	}
	return location
}
