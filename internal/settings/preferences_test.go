package settings

import "testing"

func TestDefaultsAndValidationAreIndependentOfStorage(t *testing.T) {
	p := Default()
	if p.Language != "en" || p.Theme != "" || !Enabled(p.RecordingEnabled) || !Enabled(p.PasteGuardEnabled) || Retention(p.RetentionDays) != 30 {
		t.Fatal("defaults changed")
	}
	for _, days := range []int{0, 1, 7, 30} {
		if !ValidRetentionDays(days) || Retention(&days) != days {
			t.Fatalf("valid retention rejected: %d", days)
		}
	}
	for _, days := range []int{-1, 2, 365} {
		if ValidRetentionDays(days) || Retention(&days) != 30 {
			t.Fatalf("invalid retention accepted: %d", days)
		}
	}
}
func TestPreferenceSnapshotsOwnOptionalValues(t *testing.T) {
	flag, days := false, 7
	p := Preferences{Language: "ja", Theme: "dark", RecordingEnabled: &flag, PasteGuardEnabled: &flag, RetentionDays: &days}
	copy := p.Clone()
	*copy.RecordingEnabled = true
	*copy.RetentionDays = 30
	if *p.RecordingEnabled || *p.RetentionDays != 7 {
		t.Fatal("snapshot aliased caller settings")
	}
	if !p.Dark(false) {
		t.Fatal("explicit dark theme ignored")
	}
	p.Theme = "light"
	if p.Dark(true) {
		t.Fatal("explicit light theme ignored")
	}
}

func TestPhoneRegionPreferenceIsIndependentAndBounded(t *testing.T) {
	for input, want := range map[string]string{"cn": "CN", " GB ": "GB", "": "", "zh-CN": "", "UTC": "", "ZZ": ""} {
		p := Normalize(Preferences{Language: "ja", Theme: "dark", PhoneRegion: input})
		if p.PhoneRegion != want || p.Language != "ja" || p.Theme != "dark" {
			t.Fatalf("region normalization changed other preferences: %+v", p)
		}
	}
}
