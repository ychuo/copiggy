// Package settings defines user preferences independently of their UI and file format.
package settings

const DefaultRetentionDays = 30

// Preferences is the complete persisted preference snapshot. Nil optional values
// preserve legacy defaults. Guard enablement expresses intent, never native authority.
type Preferences struct {
	Language          string `json:"language"`
	Theme             string `json:"theme,omitempty"`
	RecordingEnabled  *bool  `json:"recordingEnabled,omitempty"`
	PasteGuardEnabled *bool  `json:"pasteGuardEnabled,omitempty"`
	RetentionDays     *int   `json:"retentionDays,omitempty"`
}

func ValidRetentionDays(days int) bool { return days == 0 || days == 1 || days == 7 || days == 30 }
func Enabled(value *bool) bool         { return value == nil || *value }
func Retention(value *int) int {
	if value == nil || !ValidRetentionDays(*value) {
		return DefaultRetentionDays
	}
	return *value
}
func Default() Preferences { return Preferences{Language: "en"} }
func Normalize(p Preferences) Preferences {
	switch p.Language {
	case "en", "zh-Hans", "zh-Hant", "ja":
	default:
		p.Language = "en"
	}
	if p.Theme != "light" && p.Theme != "dark" {
		p.Theme = ""
	}
	return p.Clone()
}
func (p Preferences) Clone() Preferences {
	if p.RecordingEnabled != nil {
		v := *p.RecordingEnabled
		p.RecordingEnabled = &v
	}
	if p.PasteGuardEnabled != nil {
		v := *p.PasteGuardEnabled
		p.PasteGuardEnabled = &v
	}
	if p.RetentionDays != nil {
		v := *p.RetentionDays
		p.RetentionDays = &v
	}
	return p
}
func (p Preferences) Dark(system bool) bool {
	if p.Theme == "dark" {
		return true
	}
	if p.Theme == "light" {
		return false
	}
	return system
}
