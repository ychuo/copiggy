package clipmodel

import (
	"slices"
	"testing"
)

func TestPhoneRecognitionUsesExplicitNumberingRegion(t *testing.T) {
	for _, tc := range []struct {
		text, region, number, actualRegion, extension string
		countryCode                                   int
		international                                 bool
	}{
		{"13812345678", "CN", "13812345678", "CN", "", 86, false},
		{"138-1234-5678", "cn", "13812345678", "CN", "", 86, false},
		{"１３８　１２３４–５６７８", "CN", "13812345678", "CN", "", 86, false},
		{"+86 138 1234 5678", "US", "+8613812345678", "CN", "", 86, true},
		{"＋８６ １３８ １２３４−５６７８", "", "+8613812345678", "CN", "", 86, true},
		{"0086 138 1234 5678", "CN", "+8613812345678", "CN", "", 86, true},
		{"011 86 138 1234 5678", "US", "+8613812345678", "CN", "", 86, true},
		{"(202) 555-0148", "US", "2025550148", "US", "", 1, false},
		{"6502530000", "US", "6502530000", "US", "", 1, false},
		{"02079460958", "GB", "02079460958", "GB", "", 44, false},
		{"09012345678", "JP", "09012345678", "JP", "", 81, false},
		{"91234567", "HK", "91234567", "HK", "", 852, false},
		{"66123456", "MO", "66123456", "MO", "", 853, false},
		{"0912345678", "TW", "0912345678", "TW", "", 886, false},
		{"0612345678", "FR", "0612345678", "FR", "", 33, false},
		{"030123456", "DE", "030123456", "DE", "", 49, false},
		{"+39 02 1234 5678", "", "+390212345678", "IT", "", 39, true},
		{"+1 (202) 555-0148 ext. 123", "CN", "+12025550148", "US", "123", 1, true},
		{"020 7946 0958 x42", "GB", "02079460958", "GB", "42", 44, false},
		{"tel:+86-138-1234-5678;ext=９９", "", "+8613812345678", "CN", "99", 86, true},
		{"tel:13812345678", "CN", "13812345678", "CN", "", 86, false},
		{"tel:5550148;phone-context=+1-202;ext=12", "", "+12025550148", "US", "12", 1, true},
	} {
		t.Run(tc.text+"/"+tc.region, func(t *testing.T) {
			analysis := AnalyzeInRegion(tc.text, tc.region)
			phone := analysis.Phone
			if analysis.Type != TypePhone || phone == nil {
				t.Fatalf("got %q, want phone", analysis.Type)
			}
			if phone.Display != tc.text || phone.Number != tc.number || phone.Region != tc.actualRegion || phone.CountryCode != tc.countryCode || phone.Extension != tc.extension || phone.International != tc.international {
				t.Fatalf("unexpected phone: %+v", phone)
			}
		})
	}
}

func TestPhoneRecognitionRejectsAmbiguousOrInvalidWholeValues(t *testing.T) {
	for _, tc := range []struct{ text, region string }{
		{"13812345678", ""}, {"13812345678", "ZZ"}, {"13812345678", "US"},
		{"6502530000", ""}, {"91234567", ""},
		{"2025550148", "US"}, {"２０２５５５０１４８", "US"},
		{"123456", "CN"}, {"12345678901", "CN"}, {"11111111111", "CN"},
		{"-13812345678", "CN"}, {".13812345678", "CN"},
		{"+99912345678", "CN"}, {"+86123", "CN"}, {"+0123456789", "CN"},
		{"call +86 13812345678", "CN"}, {"13812345678 then lunch", "CN"},
		{"138\u20281234\u20285678", "CN"},
		{"+1 800 FLOWERS", "US"}, {"255.255.255.255", "US"},
		{"+1 (202 555 0148", "US"}, {"+1 ()202 555 0148", "US"}, {"+12025550148---", "US"},
		{"tel:+12025550148;ext=abc", "US"}, {"tel:+12025550148;ext=1;ext=2", "US"},
		{"tel:+12025550148?send=1", "US"}, {"tel:+12025550148;isub=123", "US"},
		{"tel:5550148;phone-context=example.com", "US"}, {"tel:+12025550148;phone-context=+1", "US"},
		{"tel:5550148;phone-context=+1-202;phone-context=+44", "US"},
		{"+12025550148 ext 12345678901", "US"},
		{"02079460958;ext=", "GB"}, {"02079460958;ext=abc", "GB"},
		{"02079460958;ext=12345678901", "GB"}, {"02079460958;ext=1;ext=2", "GB"},
		{"02079460958;phone-context=+44", "GB"}, {"02079460958;isub=123", "GB"},
	} {
		t.Run(tc.text+"/"+tc.region, func(t *testing.T) {
			if got := AnalyzeInRegion(tc.text, tc.region).Type; got != TypeText {
				t.Fatalf("got %q, want plain text", got)
			}
		})
	}
	if got := AnalyzeInRegion("1792645800", "CN").Type; got != TypeTimestamp {
		t.Fatalf("unambiguous epoch became %q", got)
	}
	if got := AnalyzeInRegion("1792645800123", "CN").Type; got != TypeTimestamp {
		t.Fatalf("millisecond epoch became %q", got)
	}
	if got := Analyze("2025550148").Type; got != TypeTimestamp {
		t.Fatalf("region-free classification became %q", got)
	}
}

func TestPhoneDialingAndE164RetainNationalPrefixAndExtension(t *testing.T) {
	phone := AnalyzeInRegion("020 7946 0958 ext. 123", "GB").Phone
	if phone == nil || phone.Number != "02079460958" || phone.E164 != "+442079460958" || phone.DialingNumber() != "02079460958;ext=123" {
		t.Fatalf("national number or extension changed: %+v", phone)
	}
	phone = AnalyzeInRegion("+39 02 1234 5678", "GB").Phone
	if phone == nil || phone.E164 != "+390212345678" || phone.DialingNumber() != phone.Number {
		t.Fatalf("significant leading zero changed: %+v", phone)
	}
}

func TestPhoneDialingWithExtensionRoundTrips(t *testing.T) {
	for _, tc := range []struct{ text, region string }{
		{"020 7946 0958 ext. 123", "GB"},
		{"+86 138 1234 5678 x99", "CN"},
		{"tel:+1-202-555-0148;ext=0012", ""},
	} {
		t.Run(tc.text, func(t *testing.T) {
			original := AnalyzeInRegion(tc.text, tc.region).Phone
			if original == nil {
				t.Fatal("fixture was not recognized")
			}
			variant := original.DialingNumber()
			reparsed := AnalyzeInRegion(variant, tc.region)
			if reparsed.Type != TypePhone || reparsed.Phone == nil || reparsed.Phone.E164 != original.E164 || reparsed.Phone.Extension != original.Extension || reparsed.Phone.DialingNumber() != variant {
				t.Fatalf("dialing variant lost phone semantics: %q => %+v", variant, reparsed.Phone)
			}
		})
	}
}

func TestPhoneRecognitionKeepsDeterministicLegacyFormatting(t *testing.T) {
	for _, text := range []string{"+1 (202) 555-0148", "(202) 555-0148", "020 7946 0958", "+8613812345678"} {
		if got := Analyze(text).Type; got != TypePhone {
			t.Errorf("%q became %q", text, got)
		}
	}
}

func TestPhoneRegionPreferencesUseSupportedIndependentValues(t *testing.T) {
	for input, want := range map[string]string{"cn": "CN", " GB ": "GB", "HK": "HK", "": "", "ZZ": "", "zh-CN": "", "UTC": ""} {
		if got := NormalizePhoneRegion(input); got != want {
			t.Errorf("NormalizePhoneRegion(%q) = %q, want %q", input, got, want)
		}
	}
	regions := SupportedPhoneRegions()
	if !slices.IsSorted(regions) || !slices.Contains(regions, "CN") || !slices.Contains(regions, "US") || !slices.Contains(regions, "JP") {
		t.Fatalf("missing or unsorted regions: %v", regions)
	}
	regions[0] = "changed"
	if slices.Contains(SupportedPhoneRegions(), "changed") {
		t.Fatal("caller modified shared supported regions")
	}
}
