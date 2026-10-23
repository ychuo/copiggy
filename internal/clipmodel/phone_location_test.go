package clipmodel

import "testing"

func TestPhoneLocationUsesOfflineGeocodingAndLanguageFallback(t *testing.T) {
	// Mainland and US examples are public fixtures from the upstream geocoding
	// tests. Taipei and Tokyo are examples from its numbering-plan metadata.
	for _, tc := range []struct {
		e164   string
		locale string
		want   string
	}{
		{"+8613702032331", "en", "Tianjin"},
		{"+8613702032331", "zh-Hans", "天津市"},
		{"+8613702032331", "zh-Hant", "Tianjin"},
		{"+8613702032331", "ja", "Tianjin"},
		{"+863197785050", "zh-Hans", "河北省邢台市"},
		{"+8613323241342", "en", "Baoding, Hebei"},
		{"+17047181840", "en", "North Carolina"},
		{"+17047181840", "zh-Hans", "North Carolina"},
		{"+12542462158", "en", "Texas"},
		{"+16193165996", "ja", "California"},
		{"+886221234567", "en", "Taipei"},
		{"+886221234567", "zh-Hans", "台北"},
		{"+886221234567", "zh-Hant", "臺北"},
		{"+886221234567", "ja", "Taipei"},
		{"+81312345678", "ja", "東京"},
		{"+81312345678", "zh-Hans", "Tokyo"},
	} {
		t.Run(tc.e164+"/"+tc.locale, func(t *testing.T) {
			phone := Analyze(tc.e164).Phone
			if phone == nil {
				t.Fatal("fixture was not recognized as a phone")
			}
			if got := PhoneLocation(*phone, tc.locale); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPhoneLocationOmitsCountryOnlyAndNonGeographicalResults(t *testing.T) {
	for _, e164 := range []string{
		"+85221234567",  // Hong Kong fixed line: no finer geographical area.
		"+85251234567",  // Hong Kong mobile.
		"+85328212345",  // Macao fixed line.
		"+886912345678", // Taiwan mobile: country only, unlike Taipei fixed line.
		"+447825602614", // UK mobile, upstream country-only fixture.
		"+819012345678", // Japan mobile.
		"+80012345678",  // Non-geographical international freephone.
	} {
		for _, locale := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
			t.Run(e164+"/"+locale, func(t *testing.T) {
				phone := Analyze(e164).Phone
				if phone == nil {
					t.Fatal("fixture was not recognized as a phone")
				}
				if got := PhoneLocation(*phone, locale); got != "" {
					t.Fatalf("country-only or non-geographical number returned %q", got)
				}
			})
		}
	}
}

func TestPhoneLocationRequiresCanonicalValidatedE164AndKnownUILocale(t *testing.T) {
	for _, e164 := range []string{"", "13812345678", "8613702032331", "tel:+8613702032331", "+86 13702032331", "+8613702032331;ext=12", "+99912345678", "+0123456789", "+86123", "+86137020323310000"} {
		if got := PhoneLocation(PhoneValue{E164: e164}, "en"); got != "" {
			t.Errorf("invalid E164 %q returned %q", e164, got)
		}
	}
	phone := PhoneValue{E164: "+8613702032331", Display: "untrusted display", Number: "other", Region: "US", CountryCode: 1, Extension: "12"}
	if got := PhoneLocation(phone, "en"); got != "Tianjin" {
		t.Fatalf("caller-supplied display or region affected location: %q", got)
	}
	for _, locale := range []string{"", "zh", "zh_Hant", "en-US", "fr", "../en"} {
		if got := PhoneLocation(phone, locale); got != "" {
			t.Errorf("unsupported UI locale %q returned %q", locale, got)
		}
	}
}
