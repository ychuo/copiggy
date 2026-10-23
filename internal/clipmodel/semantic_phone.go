package clipmodel

import (
	"net"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers/v2"
)

// NormalizePhoneRegion returns a supported numbering region, or an empty value
// when no region is known. It never guesses a country from language or time zone.
func NormalizePhoneRegion(region string) string {
	region = strings.ToUpper(strings.TrimSpace(region))
	if !phonenumbers.GetSupportedRegions()[region] {
		return ""
	}
	return region
}

// SupportedPhoneRegions returns an independent, sorted list for preferences.
func SupportedPhoneRegions() []string {
	regions := phonenumbers.GetSupportedRegions()
	result := make([]string, 0, len(regions))
	for region := range regions {
		result = append(result, region)
	}
	sort.Strings(result)
	return result
}

var phoneExtensionPattern = regexp.MustCompile(`(?i)\s*(?:ext\.?|extension|x|#)\s*([0-9]{1,10})$`)

func analyzePhoneInRegion(value, region string) *PhoneValue {
	if len(value) < 7 || len(value) > 256 {
		return nil
	}
	normalized := normalizePhoneCharacters(value)
	main, extension, ok := splitPhoneValue(normalized)
	if !ok || net.ParseIP(main) != nil {
		return nil
	}
	digits, groups, ok := phoneDigits(main)
	if !ok {
		return nil
	}
	region = NormalizePhoneRegion(region)
	if strings.HasPrefix(main, "+") || region != "" {
		return validatedPhone(value, main, digits, extension, region)
	}
	// Preserve the two explicitly formatted national conventions supported by
	// older records. Bare numbers never trigger a search across countries.
	if len(digits) < 10 || groups < 3 && !(groups >= 2 && strings.Contains(main, "(")) {
		return nil
	}
	var match *PhoneValue
	for _, fallback := range []string{"US", "GB"} {
		candidate := validatedPhone(value, main, digits, extension, fallback)
		if candidate == nil {
			continue
		}
		if match != nil {
			return nil
		}
		match = candidate
	}
	return match
}

func validatedPhone(original, main, digits, extension, region string) *PhoneValue {
	number, err := phonenumbers.ParseAndKeepRawInput(main, region)
	if err != nil || !phonenumbers.IsValidNumber(number) {
		return nil
	}
	source := number.GetCountryCodeSource()
	international := source == phonenumbers.PhoneNumber_FROM_NUMBER_WITH_PLUS_SIGN || source == phonenumbers.PhoneNumber_FROM_NUMBER_WITH_IDD
	e164 := phonenumbers.Format(number, phonenumbers.E164)
	dialing := digits
	if international {
		dialing = e164
	}
	return &PhoneValue{
		Display: original, Number: dialing, E164: e164, International: international,
		Region: phonenumbers.GetRegionCodeForNumber(number), CountryCode: int(number.GetCountryCode()), Extension: extension,
	}
}

// Normalize only characters with a clear phone-formatting meaning. This string
// is used for recognition; the saved text and all original formats are intact.
func normalizePhoneCharacters(value string) string {
	return strings.Map(func(ch rune) rune {
		// Separate lines are separate values, not phone-number separators.
		switch ch {
		case '\n', '\r', '\v', '\f', '\u0085', '\u2028', '\u2029':
			return ch
		}
		switch {
		case ch >= '\uff10' && ch <= '\uff19':
			return '0' + ch - '\uff10'
		case unicode.IsSpace(ch):
			return ' '
		}
		switch ch {
		case '\uff0b':
			return '+'
		case '\uff08':
			return '('
		case '\uff09':
			return ')'
		case '\uff0e':
			return '.'
		case '\uff0d', '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2212':
			return '-'
		default:
			return ch
		}
	}, value)
}

// Accept only an entire phone value, never the permissive library parser's
// embedded-number extraction or alphabetic vanity-number conversion.
func phoneDigits(value string) (string, int, bool) {
	if value == "" || value[len(value)-1] < '0' || value[len(value)-1] > '9' {
		return "", 0, false
	}
	if first := value[0]; (first < '0' || first > '9') && first != '+' && first != '(' {
		return "", 0, false
	}
	var digits strings.Builder
	groups, inGroup, openParen, parenDigits := 0, false, false, false
	for index, ch := range value {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
			if !inGroup {
				groups++
				inGroup = true
			}
			if openParen {
				parenDigits = true
			}
			continue
		}
		inGroup = false
		switch ch {
		case '+':
			if index != 0 {
				return "", 0, false
			}
		case '(':
			if openParen {
				return "", 0, false
			}
			openParen, parenDigits = true, false
		case ')':
			if !openParen || !parenDigits {
				return "", 0, false
			}
			openParen = false
		case ' ', '-', '.':
		default:
			return "", 0, false
		}
	}
	// The bound includes a possible three-digit international dialing prefix.
	if openParen || digits.Len() < 7 || digits.Len() > 18 {
		return "", 0, false
	}
	return digits.String(), groups, true
}

func splitPhoneValue(value string) (main, extension string, ok bool) {
	if len(value) >= 4 && strings.EqualFold(value[:4], "tel:") {
		parts := strings.Split(value[4:], ";")
		main = parts[0]
		context := ""
		seen := map[string]bool{}
		for _, parameter := range parts[1:] {
			key, data, found := strings.Cut(parameter, "=")
			key = strings.ToLower(key)
			if !found || seen[key] || data == "" {
				return "", "", false
			}
			seen[key] = true
			switch key {
			case "ext":
				if len(data) > 10 || !asciiDigits(data) {
					return "", "", false
				}
				extension = data
			case "phone-context":
				// Numeric contexts are self-contained. Domain contexts cannot be
				// resolved locally and must not cause a network lookup or guess.
				if !strings.HasPrefix(data, "+") || strings.HasPrefix(main, "+") {
					return "", "", false
				}
				context = data
			default:
				return "", "", false
			}
		}
		if context != "" {
			main = context + main
		}
		return main, extension, main != ""
	}
	// The explicit Dialing variant uses this unambiguous suffix even without a
	// tel scheme. Accept exactly that form so copying it retains the extension
	// when it is analyzed again; other bare URI parameters remain unsupported.
	if strings.Contains(value, ";") {
		main, suffix, _ := strings.Cut(value, ";")
		key, extension, found := strings.Cut(suffix, "=")
		if !found || !strings.EqualFold(key, "ext") || len(extension) > 10 || !asciiDigits(extension) {
			return "", "", false
		}
		return main, extension, main != ""
	}
	main = value
	if indices := phoneExtensionPattern.FindStringSubmatchIndex(value); indices != nil {
		main, extension = strings.TrimSpace(value[:indices[0]]), value[indices[2]:indices[3]]
	}
	return main, extension, main != ""
}
