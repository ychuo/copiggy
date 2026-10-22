package clipmodel

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math"
	"net"
	"net/mail"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"copiggy/resources"
)

// Analysis contains values derived from captured text. It is never persisted or
// accepted from UI as clipboard authority. CopyText recomputes explicit format
// choices from the trusted saved item; ordinary copies use exact saved data.
// A single matching field is populated; unrecognized content remains TypeText.
type Analysis struct {
	Type        ContentType
	Email       *EmailValue
	Phone       *PhoneValue
	Coordinates *CoordinatesValue
	Date        *DateValue
	Timestamp   *TimestampValue
	Color       *ColorValue
	FilePath    *FilePathValue
	Secret      *SecretValue
}

type EmailValue struct{ Address, Local, Domain string }
type PhoneValue struct {
	Display, Number string
	International   bool
}
type CoordinatesValue struct{ Latitude, Longitude float64 }
type DateValue struct {
	// Without HasZone, Time holds civil fields in UTC solely to avoid local
	// daylight-saving normalization. It does not identify a UTC instant.
	Time             time.Time
	HasTime, HasZone bool
}
type TimestampValue struct {
	Time                  time.Time
	Seconds, Milliseconds int64
	Unit                  string
}
type FilePathValue struct {
	Path, Name, Dir, Ext string
	HomeRelative         bool
}
type SecretValue struct {
	Kind, Value, Masked string
	Length              int
}

func IsSensitiveType(kind ContentType) bool {
	return kind == TypeOTP || kind == TypeToken || kind == TypePrivateKey
}

// Analyze recognizes whole values, never matching an email or credential inside
// arbitrary prose. Parsing is bounded and does not access disk or the network.
func Analyze(text string) Analysis {
	value := strings.TrimSpace(text)
	plain := Analysis{Type: TypeText}
	if value == "" {
		return plain
	}
	if (value[0] == '{' || value[0] == '[') && json.Valid([]byte(value)) {
		return Analysis{Type: TypeJSON}
	}
	// Long prose and code should not spend time in semantic recognizers. PEM
	// blocks get a separate bound; real keys are typically only a few KiB.
	if len(value) <= 64<<10 {
		if secret := analyzePrivateKey(value); secret != nil {
			return Analysis{Type: TypePrivateKey, Secret: secret}
		}
	}
	if len(value) > 8192 || strings.ContainsAny(value, "\x00\r\n") {
		return plain
	}
	if secret := analyzeToken(value); secret != nil {
		return Analysis{Type: TypeToken, Secret: secret}
	}
	if secret := analyzeOTP(value); secret != nil {
		return Analysis{Type: TypeOTP, Secret: secret}
	}
	if local := analyzeFilePath(value); local != nil {
		return Analysis{Type: TypeFilePath, FilePath: local}
	}
	if !strings.ContainsAny(value, "\t ") {
		if parsed, err := url.Parse(value); err == nil &&
			(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" {
			return Analysis{Type: TypeLink}
		}
	}
	if email := analyzeEmail(value); email != nil {
		return Analysis{Type: TypeEmail, Email: email}
	}
	if color := analyzeColor(value); color != nil {
		return Analysis{Type: TypeColor, Color: color}
	}
	if coordinate := analyzeCoordinates(value); coordinate != nil {
		return Analysis{Type: TypeCoordinates, Coordinates: coordinate}
	}
	if date := analyzeDate(value); date != nil {
		return Analysis{Type: TypeDate, Date: date}
	}
	if timestamp := analyzeTimestamp(value); timestamp != nil {
		return Analysis{Type: TypeTimestamp, Timestamp: timestamp}
	}
	if phone := analyzePhone(value); phone != nil {
		return Analysis{Type: TypePhone, Phone: phone}
	}
	return plain
}

func analyzeEmail(value string) *EmailValue {
	if len(value) > 254 || strings.ContainsAny(value, "<>\"(),; \t") {
		return nil
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Name != "" || address.Address != value {
		return nil
	}
	local, domain, found := strings.Cut(value, "@")
	if !found || len(local) > 64 || !strings.Contains(domain, ".") {
		return nil
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return nil
		}
		for _, ch := range label {
			if !(unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-') {
				return nil
			}
		}
	}
	return &EmailValue{Address: value, Local: local, Domain: strings.ToLower(domain)}
}

var coordinatePattern = regexp.MustCompile(`^([+-]?[0-9]{1,3}(?:\.[0-9]+)?)\s*,\s*([+-]?[0-9]{1,3}(?:\.[0-9]+)?)$`)

func analyzeCoordinates(value string) *CoordinatesValue {
	parts := coordinatePattern.FindStringSubmatch(value)
	if parts == nil {
		return nil
	}
	lat, errLat := strconv.ParseFloat(parts[1], 64)
	lon, errLon := strconv.ParseFloat(parts[2], 64)
	if errLat != nil || errLon != nil || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return nil
	}
	// Requiring decimal precision avoids turning ordinary pairs/counts into maps.
	if !strings.Contains(parts[1], ".") || !strings.Contains(parts[2], ".") {
		return nil
	}
	return &CoordinatesValue{Latitude: lat, Longitude: lon}
}

func analyzeDate(value string) *DateValue {
	if len(value) < 10 || len(value) > 40 || value[4] != '-' || value[7] != '-' {
		return nil
	}
	if len(value) >= 22 && (value[len(value)-6] == '+' || value[len(value)-6] == '-') {
		offset := value[len(value)-5:]
		hours, hErr := strconv.Atoi(offset[:2])
		minutes, mErr := strconv.Atoi(offset[3:])
		if offset[2] != ':' || hErr != nil || mErr != nil || hours > 23 || minutes > 59 {
			return nil
		}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil && parsed.Year() >= 1900 && parsed.Year() <= 2199 {
		return &DateValue{Time: parsed, HasTime: true, HasZone: true}
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		// A zone-less value describes the fields the user copied, including
		// times in a local DST gap. Do not infer a location and change them.
		parsed, err := time.Parse(layout, value)
		if err == nil && parsed.Year() >= 1900 && parsed.Year() <= 2199 {
			return &DateValue{Time: parsed, HasTime: len(value) > 10}
		}
	}
	return nil
}

func analyzeTimestamp(value string) *TimestampValue {
	if (len(value) != 10 && len(value) != 13) || !asciiDigits(value) {
		return nil
	}
	integer, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	unit, milliseconds := "s", integer*1000
	if len(value) == 13 {
		unit, milliseconds = "ms", integer
	}
	instant := time.UnixMilli(milliseconds).UTC()
	// A fixed practical range avoids treating arbitrary IDs as dates. It never
	// depends on today's date, so restoring history does not change its type.
	if instant.Year() < 2000 || instant.Year() >= 2100 {
		return nil
	}
	return &TimestampValue{Time: instant, Seconds: instant.Unix(), Milliseconds: milliseconds, Unit: unit}
}

func analyzePhone(value string) *PhoneValue {
	if len(value) < 7 || len(value) > 32 || net.ParseIP(value) != nil {
		return nil
	}
	if last := value[len(value)-1]; last < '0' || last > '9' {
		return nil
	}
	var digits strings.Builder
	groups, inGroup, openParen := 0, false, false
	international := value[0] == '+'
	for index, ch := range value {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
			if !inGroup {
				groups++
				inGroup = true
			}
			continue
		}
		inGroup = false
		switch ch {
		case '+':
			if index != 0 {
				return nil
			}
		case '(':
			if openParen {
				return nil
			}
			openParen = true
		case ')':
			if !openParen {
				return nil
			}
			openParen = false
		case ' ', '-', '.':
		default:
			return nil
		}
	}
	number := digits.String()
	localGrouping := groups >= 3 || groups >= 2 && strings.Contains(value, "(")
	if openParen || len(number) < 7 || len(number) > 15 || (!international && (!localGrouping || len(number) < 10)) {
		return nil
	}
	if international {
		if number[0] == '0' {
			return nil
		}
		number = "+" + number
	}
	return &PhoneValue{Display: value, Number: number, International: international}
}

func analyzeFilePath(value string) *FilePathValue {
	homeRelative := strings.HasPrefix(value, "~/")
	if strings.HasPrefix(value, "file:") {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "file" || parsed.User != nil || parsed.Opaque != "" ||
			(parsed.Host != "" && parsed.Host != "localhost") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil
		}
		value = parsed.Path
	}
	if (!homeRelative && !strings.HasPrefix(value, "/")) || strings.HasPrefix(value, "//") ||
		strings.ContainsAny(value, "\x00\r\n\t") || strings.Contains(value, "/../") || strings.HasSuffix(value, "/..") {
		return nil
	}
	clean := path.Clean(value)
	if clean == "/" || clean == "~" || clean == "." {
		return nil
	}
	return &FilePathValue{Path: clean, Name: path.Base(clean), Dir: path.Dir(clean), Ext: path.Ext(clean), HomeRelative: homeRelative}
}

var otpPattern = func() *regexp.Regexp {
	var labels map[string]map[string][]string
	if err := json.Unmarshal(resources.ContentLabels, &labels); err != nil {
		panic("invalid embedded content labels: " + err.Error())
	}
	var prefixes []string
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		entries := labels["otp"][language]
		if len(entries) == 0 {
			panic("missing OTP content labels: " + language)
		}
		for _, label := range entries {
			if strings.TrimSpace(label) == "" {
				panic("empty OTP content label: " + language)
			}
			prefixes = append(prefixes, regexp.QuoteMeta(label))
		}
	}
	return regexp.MustCompile(`(?i)^(?:` + strings.Join(prefixes, "|") + `)(?:\s*[:：=]\s*|\s+)([0-9]{4,8}|[0-9]{3} [0-9]{3})$`)
}()
var tokenPattern = regexp.MustCompile(`^(?:gh[pousr]_[A-Za-z0-9]{30,255}|github_pat_[A-Za-z0-9_]{40,255}|sk-(?:proj-|ant-api[0-9]+-)?[A-Za-z0-9_-]{20,255}|xox[baprs]-[A-Za-z0-9-]{20,255}|AKIA[A-Z0-9]{16})$`)

func newSecret(kind, value string, length int) *SecretValue {
	return &SecretValue{Kind: kind, Value: value, Length: length, Masked: strings.Repeat("•", min(length, 12))}
}

func analyzeOTP(value string) *SecretValue {
	parts := otpPattern.FindStringSubmatch(value)
	if parts == nil {
		return nil
	}
	code := strings.ReplaceAll(parts[1], " ", "")
	return newSecret("OTP", code, len(code))
}

func analyzeToken(value string) *SecretValue {
	if parts := strings.Split(value, "."); len(parts) == 3 && len(parts[2]) >= 16 {
		header, headerErr := base64.RawURLEncoding.DecodeString(parts[0])
		payload, payloadErr := base64.RawURLEncoding.DecodeString(parts[1])
		_, signatureErr := base64.RawURLEncoding.DecodeString(parts[2])
		var fields map[string]json.RawMessage
		if headerErr == nil && payloadErr == nil && signatureErr == nil && json.Unmarshal(header, &fields) == nil &&
			fields["alg"] != nil && len(payload) > 1 && payload[0] == '{' && json.Valid(payload) {
			var algorithm string
			if json.Unmarshal(fields["alg"], &algorithm) == nil && algorithm != "" && algorithm != "none" {
				return newSecret("JWT", value, len(value))
			}
		}
	}
	if !tokenPattern.MatchString(value) {
		return nil
	}
	kind := "API key"
	switch {
	case strings.HasPrefix(value, "gh"):
		kind = "GitHub"
	case strings.HasPrefix(value, "xox"):
		kind = "Slack"
	case strings.HasPrefix(value, "AKIA"):
		kind = "AWS"
	}
	return newSecret(kind, value, len(value))
}

func analyzePrivateKey(value string) *SecretValue {
	if !strings.HasPrefix(value, "-----BEGIN ") {
		return nil
	}
	block, rest := pem.Decode([]byte(value))
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 || len(block.Bytes) < 32 {
		return nil
	}
	switch block.Type {
	case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY", "DSA PRIVATE KEY":
		// A complete PEM envelope with a decoded binary payload is enough for
		// presentation. This does not claim cryptographic validity or decrypt it.
		return newSecret("PEM", value, len(value))
	case "OPENSSH PRIVATE KEY":
		if strings.HasPrefix(string(block.Bytes), "openssh-key-v1\x00") {
			return newSecret("OpenSSH", value, len(value))
		}
	}
	return nil
}

func asciiDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
