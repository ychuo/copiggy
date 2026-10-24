package clipmodel

import (
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestTimestampExplicitUnitsPreserveInstantOriginalAndCopyScope(t *testing.T) {
	for _, tc := range []struct {
		input, unit, instant  string
		seconds, milliseconds int64
	}{
		{"1792645800s", "s", "2026-10-22T05:10:00Z", 1792645800, 1792645800000},
		{"1792645800 s", "s", "2026-10-22T05:10:00Z", 1792645800, 1792645800000},
		{"1792645800123ms", "ms", "2026-10-22T05:10:00.123Z", 1792645800, 1792645800123},
		{"  1792645800123 ms \n", "ms", "2026-10-22T05:10:00.123Z", 1792645800, 1792645800123},
		{"946684800s", "s", "2000-01-01T00:00:00Z", 946684800, 946684800000},
		{"946684800000 ms", "ms", "2000-01-01T00:00:00Z", 946684800, 946684800000},
		{"4102444799 s", "s", "2099-12-31T23:59:59Z", 4102444799, 4102444799000},
		{"4102444799999ms", "ms", "2099-12-31T23:59:59.999Z", 4102444799, 4102444799999},
	} {
		t.Run(tc.input, func(t *testing.T) {
			analysis := Analyze(tc.input)
			stamp := analysis.Timestamp
			if analysis.Type != TypeTimestamp || stamp == nil || stamp.Unit != tc.unit || stamp.Seconds != tc.seconds || stamp.Milliseconds != tc.milliseconds || stamp.Time.Format(time.RFC3339Nano) != tc.instant {
				t.Fatalf("timestamp = %+v, want %q", stamp, tc.instant)
			}
			store := NewStore(1)
			allowed := []string{"com.example.editor"}
			item, ok := store.CaptureProtectedContent(Content{Text: tc.input}, "Fixture Editor", allowed)
			if !ok || item.Type != TypeTimestamp || item.Text != tc.input || !reflect.DeepEqual(item.AllowedApps, allowed) {
				t.Fatal("timestamp recognition rewrote original text or protection")
			}
			before := cloneItem(item)
			for _, copy := range []struct {
				format CopyFormat
				value  int64
			}{{TimestampSeconds, tc.seconds}, {TimestampMilliseconds, tc.milliseconds}} {
				got, err := CopyText(item, copy.format)
				if err != nil || got != strconv.FormatInt(copy.value, 10) || !reflect.DeepEqual(item, before) {
					t.Fatalf("copy %s changed source or converted incorrectly: %q, %v", copy.format, got, err)
				}
			}
		})
	}
}

func TestTimestampUnitsDoNotBroadenBareOrAmbiguousNumbers(t *testing.T) {
	if got := AnalyzeInRegion("2025550148", "US").Type; got != TypeText {
		t.Fatalf("ambiguous bare phone/epoch became %q", got)
	}
	for _, input := range []string{"2025550148s", "2025550148 s"} {
		if got := AnalyzeInRegion(input, "US"); got.Type != TypeTimestamp || got.Timestamp.Seconds != 2025550148 {
			t.Fatalf("explicit seconds were treated as a phone: %q => %+v", input, got)
		}
	}
	for _, input := range []string{"1792645800", "1792645800123"} {
		if got := Analyze(input).Type; got != TypeTimestamp {
			t.Fatalf("existing bare epoch became %q", got)
		}
	}
	for _, input := range []string{
		"946684800", "946684800000", "946684799s", "946684799999ms", "4102444800s", "4102444800000ms",
		"1792645800ms", "1792645800123s", "1792645800.5s", "1792645800123.5ms", "+1792645800s", "-1792645800s",
		"1792645800sec", "1792645800 seconds", "1792645800S", "1792645800123MS", "1792645800 s ms",
		"1792645800  s", "1792645800\ts", "1 792645800s", "1792645800s ago", "epoch 1792645800s", "9223372036854775807s",
	} {
		t.Run(input, func(t *testing.T) {
			if got := Analyze(input); got.Type != TypeText || got.Timestamp != nil {
				t.Fatalf("malformed or unsupported epoch recognized: %+v", got)
			}
		})
	}
}
