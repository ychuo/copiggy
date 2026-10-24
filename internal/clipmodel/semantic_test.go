package clipmodel

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"reflect"
	"strings"
	"testing"
	"time"

	"copiggy/resources"
)

func testJWT() string {
	part := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	return part(`{"alg":"HS256","typ":"JWT"}`) + "." + part(`{"sub":"fixture"}`) + "." + part(strings.Repeat("x", 32))
}

func TestSemanticRecognitionUsesWholeUnambiguousValues(t *testing.T) {
	pemValue := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte(strings.Repeat("fixture", 12))}))
	cases := []struct {
		value string
		want  ContentType
	}{
		{"hello@example.com", TypeEmail}, {" First.Last+tag@EXAMPLE.com \n", TypeEmail},
		{"+1 (202) 555-0148", TypePhone}, {"(202) 555-0148", TypePhone}, {"+8613812345678", TypePhone},
		{"31.2304, 121.4737", TypeCoordinates}, {"-90.0, +180.0", TypeCoordinates},
		{"2026-10-22", TypeDate}, {"2026-10-22 15:30", TypeDate}, {"2026-10-22T15:30:00+08:00", TypeDate},
		{"1792645800", TypeTimestamp}, {"1792645800123", TypeTimestamp},
		{"#D78379", TypeColor}, {"#abc8", TypeColor}, {"rgb(215, 131, 121)", TypeColor}, {"hsl(6.4, 54%, 66%)", TypeColor},
		{"/Users/alex/Documents/Brief.pdf", TypeFilePath}, {"~/Downloads/example.txt", TypeFilePath}, {"file:///Users/alex/My%20File.pdf", TypeFilePath},
		{"Verification code: 123456", TypeOTP}, {"OTP 123 456", TypeOTP}, {"验证码：123456", TypeOTP},
		{testJWT(), TypeToken}, {"ghp_" + strings.Repeat("a", 36), TypeToken}, {pemValue, TypePrivateKey},
		{`{"email":"hello@example.com"}`, TypeJSON}, {"https://example.com/#D78379", TypeLink},
	}
	for _, tc := range cases {
		t.Run(tc.value[:min(len(tc.value), 48)], func(t *testing.T) {
			analysis := Analyze(tc.value)
			if analysis.Type != tc.want {
				t.Fatalf("type = %q, want %q", analysis.Type, tc.want)
			}
			store := NewStore(1)
			item, ok := store.CaptureContent(Content{Type: TypeText, Text: tc.value}, "Fixture Editor", false)
			if !ok || item.Type != tc.want || item.Text != tc.value {
				t.Fatal("semantic recognition changed original text or failed capture")
			}
		})
	}
}

func TestAmbiguousAndMalformedValuesStayText(t *testing.T) {
	for _, value := range []string{
		"123456", "1234", "01234567", "123", "0xffeeaa", "red", "Hello hello@example.com",
		"Name <hello@example.com> then mail", "hello@localhost", "hello@example..com", "hello@-example.com", "hello@example.com;",
		"1, 2", "90.01, 180.0", "0.0, -180.01", "NaN, 20.0", "31.2, 121.4 then turn left",
		"-180.01, 0.0", "180.01, 0.0", "-122.168861, 90.01", "-122.168861, -90.01",
		"-122, 37.428230", "-122.168861, 37", "-122.168861, 37.428230, 0.0",
		"03/04/2026", "2026-02-30", "2026-13-22", "2026-10-22T25:00:00Z", "2026-10-22 then lunch", "2026-10-22T15:30:00+24:00", "2026-10-22T15:30:00+08:60",
		"123456789", "9999999999", "9999999999999", "-1792645800", "1792645800.0",
		"#ggg", "#abcdefg", "rgb(256, 0, 0)", "rgba(0, 0, 0, 2)", "hsl(0, 120%, 40%)", "hsl(NaN, 50%, 50%)", "rgb(0 0)",
		"example/relative.txt", "file://remote.example/path", "file:///Users/alex/a?query=1", "file:///tmp/a%00b", "/Users/alex/../private", "//remote/file", "/",
		"code123456", "Your verification code is 123456, use it soon.", "OTP: 123", "OTP: 123456789", "OTP: abcd",
		"+0123456789", "+1 (202 555 0148", "+12025550148---", "255.255.255.255", "1234567890123456", "call +1 202 555 0148",
		"ghp_short", "some-very-long-random-ordinary-word", "-----BEGIN PRIVATE KEY-----\ninvalid\n-----END PRIVATE KEY-----",
	} {
		if got := Analyze(value).Type; got != TypeText {
			t.Errorf("%q: got %q, want ordinary text", value, got)
		}
	}
}

func TestCoordinatesRecognizeOrderFromValidRanges(t *testing.T) {
	for _, tc := range []struct {
		input    string
		lat, lon float64
	}{
		{"-122.168861, 37.428230", 37.428230, -122.168861},
		{"37.428230, -122.168861", 37.428230, -122.168861},
		{"  +121.4737,\t31.2304 \n", 31.2304, 121.4737},
		{"151.2093, -33.8688", -33.8688, 151.2093},
		{"+180.0, -90.0", -90, 180},
		{"-180.0, +90.0", 90, -180},
		{"-90.0, +180.0", -90, 180},
		{"45.0, -80.0", 45, -80},
		{"90.0, -90.0", 90, -90},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := Analyze(tc.input)
			if got.Type != TypeCoordinates || got.Coordinates == nil ||
				got.Coordinates.Latitude != tc.lat || got.Coordinates.Longitude != tc.lon {
				t.Fatalf("coordinates = %+v, want latitude %g, longitude %g", got.Coordinates, tc.lat, tc.lon)
			}
			store := NewStore(1)
			item, ok := store.Capture(tc.input, "Fixture Editor", false)
			if !ok || item.Type != TypeCoordinates || item.Text != tc.input {
				t.Fatal("coordinate recognition failed capture or rewrote the original text")
			}
		})
	}
}

func TestSemanticDerivedFields(t *testing.T) {
	email := Analyze("Alex+tag@EXAMPLE.com").Email
	if email.Address != "Alex+tag@EXAMPLE.com" || email.Local != "Alex+tag" || email.Domain != "example.com" {
		t.Fatal("email parts changed")
	}
	phone := Analyze("+1 (202) 555-0148").Phone
	if phone.Number != "+12025550148" || !phone.International || phone.Display != "+1 (202) 555-0148" {
		t.Fatal("phone normalization changed")
	}
	coordinate := Analyze("31.2304,121.4737").Coordinates
	if coordinate.Latitude != 31.2304 || coordinate.Longitude != 121.4737 {
		t.Fatal("coordinates changed")
	}
	date := Analyze("2026-10-22T15:30:00+08:00").Date
	_, offset := date.Time.Zone()
	if !date.HasTime || !date.HasZone || date.Time.Hour() != 15 || offset != 8*3600 {
		t.Fatal("date lost explicit timezone")
	}
	onlyDate := Analyze("2026-10-22").Date
	if onlyDate.HasTime || onlyDate.HasZone || onlyDate.Time.Format(time.DateOnly) != "2026-10-22" {
		t.Fatal("date-only semantics changed")
	}
	timestamp := Analyze("1792645800123").Timestamp
	if timestamp.Unit != "ms" || timestamp.Milliseconds != 1792645800123 || timestamp.Seconds != 1792645800 || timestamp.Time.Format("2006-01-02 15:04:05.000Z") != "2026-10-22 05:10:00.123Z" {
		t.Fatal("millisecond timestamp lost precision")
	}
	file := Analyze("file:///Users/alex/My%20File.pdf").FilePath
	if file.Path != "/Users/alex/My File.pdf" || file.Name != "My File.pdf" || file.Ext != ".pdf" || file.Dir != "/Users/alex" {
		t.Fatal("file URL decoding changed")
	}
	if home := Analyze("~/Downloads/archive.zip").FilePath; !home.HomeRelative || home.Path != "~/Downloads/archive.zip" {
		t.Fatal("model expanded home directory")
	}
	otp := Analyze("OTP 123 456").Secret
	if otp.Value != "123456" || otp.Length != 6 || otp.Masked != "••••••" {
		t.Fatal("OTP display normalization changed")
	}
	for _, kind := range []ContentType{TypeOTP, TypeToken, TypePrivateKey} {
		if !IsSensitiveType(kind) {
			t.Fatalf("%q is not marked sensitive", kind)
		}
	}
	if IsSensitiveType(TypeEmail) || IsSensitiveType(TypeText) {
		t.Fatal("ordinary values marked sensitive")
	}
}

func TestZoneLessDatesRetainCivilFieldsAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	previous := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = previous })
	for _, raw := range []string{"2026-03-08 02:30", "2026-11-01 01:30", "2026-03-08"} {
		date := Analyze(raw).Date
		layout := "2006-01-02 15:04"
		if len(raw) == 10 {
			layout = time.DateOnly
		}
		if date == nil || date.HasZone || date.Time.Format(layout) != raw {
			t.Fatalf("zone-less fields changed in a DST location: %q => %+v", raw, date)
		}
	}
	for _, raw := range []string{"2026-03-08T02:30:00-05:00", "2026-11-01T01:30:00-04:00", "2026-11-01T01:30:00-05:00"} {
		date := Analyze(raw).Date
		want, err := time.Parse(time.RFC3339, raw)
		if err != nil || date == nil || !date.HasZone || !date.Time.Equal(want) {
			t.Fatalf("explicit offset changed: %q => %+v, %v", raw, date, err)
		}
	}
}

func TestColorChannelsAndAlpha(t *testing.T) {
	for _, tc := range []struct {
		input, hex string
		r, g, b, a uint8
	}{
		{"#D78379", "#D78379", 215, 131, 121, 255},
		{"#abc8", "#AABBCC88", 170, 187, 204, 136},
		{"rgb(100% 0% 0% / 50%)", "#FF000080", 255, 0, 0, 128},
		{"hsl(120, 100%, 50%)", "#00FF00", 0, 255, 0, 255},
		{"hsla(-120deg, 100%, 50%, .25)", "#0000FF40", 0, 0, 255, 64},
		{"rgba(0, 0, 0, 0)", "#00000000", 0, 0, 0, 0},
	} {
		color := Analyze(tc.input).Color
		if color == nil || color.HEX != tc.hex || color.R != tc.r || color.G != tc.g || color.B != tc.b || color.A != tc.a {
			t.Errorf("wrong color channels for %q", tc.input)
			continue
		}
		if Analyze(color.RGB).Color.HEX != color.HEX {
			t.Errorf("RGB display did not round trip: %s", color.RGB)
		}
	}
}

func TestSemanticRestoreReclassifiesWithoutChangingRepresentationsOrProtection(t *testing.T) {
	original := "  hello@example.com \n"
	data := Representations{{{Format: TextFormat, Bytes: []byte(original)}, {Format: HTMLFormat, Bytes: []byte("<b>hello@example.com</b>")}}}
	before := data.Clone()
	saved := Item{ID: "item-7", Type: TypeText, Text: original, SourceApp: "Editor", SourceAppID: "com.example.editor", AllowedApps: []string{"com.example.editor"}, CreatedAt: time.Now(), Pinned: true}
	model, err := RestoreRecords([]Record{{Item: saved, Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if _, selected := model.Current(); selected {
		t.Fatal("restoring semantic type acquired clipboard authority")
	}
	item := model.Items()[0]
	if item.Type != TypeEmail || item.ID != saved.ID || item.Text != original || !reflect.DeepEqual(item.AllowedApps, saved.AllowedApps) || !reflect.DeepEqual(data, before) {
		t.Fatal("semantic migration changed representation or scope")
	}
	content, err := ContentFromRepresentations(data)
	if err != nil {
		t.Fatal(err)
	}
	content.SourceAppID = "com.example.other"
	refreshed, ok := model.CaptureProtectedContent(content, "Other", []string{"com.example.editor"})
	if !ok || refreshed.ID != saved.ID || refreshed.SourceApp != saved.SourceApp || !refreshed.Pinned {
		t.Fatal("reclassification broke exact dedup or provenance")
	}
	unrestricted, _ := model.CaptureContent(content, "Other", false)
	if unrestricted.ID == saved.ID || len(model.Items()) != 2 {
		t.Fatal("semantic type merged different protection scopes")
	}
	forged, ok := model.CaptureContent(Content{Type: TypePrivateKey, Text: "ordinary text"}, "Editor", false)
	if !ok || forged.Type != TypeText {
		t.Fatal("caller supplied semantic type was trusted")
	}
	saved.Type = TypeImage
	if _, err := RestoreRecords([]Record{{Item: saved, Data: data}}); err == nil {
		t.Fatal("corrupt image/text migration was accepted")
	}
}

func TestNewTypeSearchTermsCoverAllLanguages(t *testing.T) {
	cases := []struct {
		value   string
		queries []string
	}{
		{"hello@example.com", []string{"email", "邮箱", "電郵", "メール"}},
		{"31.2304,121.4737", []string{"coordinates", "经纬度", "經緯度", "緯度"}},
		{"OTP: 123456", []string{"verification", "验证码", "驗證碼", "認証コード"}},
		{"#D78379", []string{"color", "颜色", "顏色", "カラー"}},
	}
	for _, tc := range cases {
		store := NewStore(1)
		store.Capture(tc.value, "Editor", false)
		for _, query := range tc.queries {
			if len(store.Search(query)) != 1 {
				t.Errorf("type search missing %q", query)
			}
		}
	}
}

func TestOTPRecognitionLabelsCoverSupportedLanguages(t *testing.T) {
	var catalog map[string]map[string][]string
	if err := json.Unmarshal(resources.ContentLabels, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"en", "zh-Hans", "zh-Hant", "ja"} {
		labels := catalog["otp"][language]
		if len(labels) == 0 {
			t.Fatalf("OTP labels missing for %s", language)
		}
		for _, label := range labels {
			for _, separator := range []string{": ", "：", " ", "="} {
				value := label + separator + "123456"
				analysis := Analyze(value)
				if analysis.Type != TypeOTP || analysis.Secret == nil || analysis.Secret.Value != "123456" {
					t.Errorf("OTP label not recognized for %s: %q", language, value)
				}
			}
			if Analyze("prefix "+label+": 123456").Type != TypeText {
				t.Errorf("OTP label matched inside prose: %q", label)
			}
		}
	}
}
