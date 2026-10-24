package clipmodel

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"regexp"
	"strings"
	"time"

	"copiggy/resources"
)

// Analysis contains values derived from captured text. It is never persisted or
// accepted from UI as clipboard authority. CopyText recomputes explicit format
// choices from the trusted saved item; ordinary copies use exact saved data.
// A single matching field is populated; unrecognized content remains TypeText.
type Analysis struct {
	Type        ContentType
	Markdown    *MarkdownValue
	Link        *LinkValue
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
	E164            string
	International   bool
	Region          string
	CountryCode     int
	Extension       string
}

// DialingNumber includes a validated extension without rewriting the saved text.
func (phone PhoneValue) DialingNumber() string {
	if phone.Extension != "" {
		return phone.Number + ";ext=" + phone.Extension
	}
	return phone.Number
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
	return AnalyzeInRegion(text, "")
}

// AnalyzeInRegion uses an explicit telephone numbering region. The region is
// independent of UI language and time zone; no system or global state is read.
func AnalyzeInRegion(text, region string) Analysis {
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
		if markdown := AnalyzeMarkdown(value); markdown != nil {
			return Analysis{Type: TypeMarkdown, Markdown: markdown}
		}
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
	if link := ParseLink(value); link != nil {
		if link.App {
			return Analysis{Type: TypeAppLink, Link: link}
		}
		return Analysis{Type: TypeLink, Link: link}
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
	timestamp := analyzeTimestamp(value)
	phone := analyzePhoneInRegion(value, region)
	// A bare number can be both a valid local phone number and an epoch. Neither
	// interpretation has stronger evidence, so do not silently choose an action.
	if phone != nil && (timestamp != nil || analyzeTimestamp(normalizePhoneCharacters(value)) != nil) {
		return plain
	}
	if timestamp != nil {
		return Analysis{Type: TypeTimestamp, Timestamp: timestamp}
	}
	if phone != nil {
		return Analysis{Type: TypePhone, Phone: phone}
	}
	if markdown := AnalyzeMarkdown(value); markdown != nil {
		return Analysis{Type: TypeMarkdown, Markdown: markdown}
	}
	return plain
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
	return regexp.MustCompile(`(?i)^(?:` + strings.Join(prefixes, "|") + `)(?:\s*[:：=]\s*|\s+)([0-9]{4,8}|[0-9]{2,4}[ -][0-9]{2,4})$`)
}()
var tokenPattern = regexp.MustCompile(`^(?:gh[pousr]_[A-Za-z0-9]{30,255}|github_pat_[A-Za-z0-9_]{40,255}|sk-(?:proj-|ant-api[0-9]+-)?[A-Za-z0-9_-]{20,255}|xox[baprs]-[A-Za-z0-9-]{20,255}|AKIA[A-Z0-9]{16})$`)
var bearerTokenPattern = regexp.MustCompile(`(?i)^(?:Authorization:[ \t]*)?Bearer[ \t]+([^ \t]+)$`)

func newSecret(kind, value string, length int) *SecretValue {
	return &SecretValue{Kind: kind, Value: value, Length: length, Masked: strings.Repeat("•", min(length, 12))}
}

func analyzeOTP(value string) *SecretValue {
	// Normalize only explicit code typography, never the saved representation
	// or arbitrary prose. A recognized label is still required before the digits.
	normalized := strings.Map(func(ch rune) rune {
		switch {
		case ch >= '\uff10' && ch <= '\uff19':
			return '0' + ch - '\uff10'
		}
		switch ch {
		case '\u00a0', '\u202f', '\u3000':
			return ' '
		case '\uff1d':
			return '='
		case '\uff0d', '\u2010', '\u2011':
			return '-'
		default:
			return ch
		}
	}, value)
	parts := otpPattern.FindStringSubmatch(normalized)
	if parts == nil {
		return nil
	}
	code := strings.NewReplacer(" ", "", "-", "").Replace(parts[1])
	return newSecret("OTP", code, len(code))
}

func analyzeToken(value string) *SecretValue {
	// A complete Bearer field is only a wrapper around an already recognized
	// credential, not evidence that arbitrary strings are secrets. Preserve the
	// wrapper in item.Text; use the parsed token only for masked presentation.
	if parts := bearerTokenPattern.FindStringSubmatch(value); parts != nil {
		value = parts[1]
	}
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
	// pem.Decode can skip malformed leading blocks. Require a single pair of
	// envelope lines so it cannot extract a later key from a larger value.
	// Only count line-start markers; ordinary PEM headers remain supported.
	if !strings.HasPrefix(value, "-----BEGIN ") || strings.Contains(value, "\n-----BEGIN ") ||
		strings.Count(value, "\n-----END ") != 1 {
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
