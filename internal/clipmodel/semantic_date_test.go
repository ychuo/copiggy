package clipmodel

import (
	"testing"
	"time"
)

func TestYearFirstDatesPreserveCivilFieldsAndOriginalText(t *testing.T) {
	for _, tc := range []struct {
		input, civil string
		hasTime      bool
	}{
		{"2026/10/3", "2026-10-03 00:00:00", false},
		{"2026.10.3", "2026-10-03 00:00:00", false},
		{"2026-1-3", "2026-01-03 00:00:00", false},
		{"2026/01/03", "2026-01-03 00:00:00", false},
		{"2026.1.03", "2026-01-03 00:00:00", false},
		{"2026-01-3", "2026-01-03 00:00:00", false},
		{"2026/10/3 15:04", "2026-10-03 15:04:00", true},
		{"2026.10.3 15:04:05", "2026-10-03 15:04:05", true},
		{"2026-1-3T15:04", "2026-01-03 15:04:00", true},
		{"2026/1/3T15:04:05", "2026-01-03 15:04:05", true},
		{"  2026.3.8 02:30 \n", "2026-03-08 02:30:00", true},
		{"2026/11/1 01:30", "2026-11-01 01:30:00", true},
		{"1900/1/1", "1900-01-01 00:00:00", false},
		{"2199.12.31 23:59:59", "2199-12-31 23:59:59", true},
		{"2000/2/29", "2000-02-29 00:00:00", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			analysis := Analyze(tc.input)
			date := analysis.Date
			if analysis.Type != TypeDate || date == nil || date.HasZone || date.HasTime != tc.hasTime ||
				date.Time.Location() != time.UTC || date.Time.Format("2006-01-02 15:04:05") != tc.civil {
				t.Fatalf("date = %+v, want civil %q, hasTime=%t", date, tc.civil, tc.hasTime)
			}
			store := NewStore(1)
			item, ok := store.Capture(tc.input, "Fixture Editor", false)
			if !ok || item.Type != TypeDate || item.Text != tc.input {
				t.Fatal("date recognition failed capture or rewrote original text")
			}
		})
	}
}

func TestDatesKeepRFC3339ZonesAndFractionalSeconds(t *testing.T) {
	for _, input := range []string{
		"2026-10-03T15:04:05Z",
		"2026-10-03T15:04:05+08:00",
		"2026-10-03T15:04:05-05:30",
		"2026-10-03T15:04:05.123456789+08:00",
	} {
		want, err := time.Parse(time.RFC3339Nano, input)
		if err != nil {
			t.Fatal(err)
		}
		date := Analyze(input).Date
		if date == nil || !date.HasTime || !date.HasZone || date.Time.Format(time.RFC3339Nano) != want.Format(time.RFC3339Nano) {
			t.Fatalf("zoned date changed: %q => %+v", input, date)
		}
	}
	for _, input := range []string{"2026-10-03 15:04:05.123", "2026/10/3 15:04:05.123"} {
		date := Analyze(input).Date
		if date == nil || !date.HasTime || date.HasZone || date.Time.Nanosecond() != 123000000 {
			t.Fatalf("civil fractional seconds changed: %q => %+v", input, date)
		}
	}
}

func TestYearFirstDatesRejectAmbiguousAndMalformedValues(t *testing.T) {
	for _, input := range []string{
		"10/3/2026", "3.10.2026", "10-03-2026", "26/10/3", "02026/10/3",
		"2026/10.3", "2026.10-3", "2026-10/3", "2026 /10/3", "2026/10/ 3",
		"2026/0/3", "2026/13/3", "2026/10/0", "2026/10/32", "2026/4/31",
		"2026/2/29", "1900/2/29", "2100.2.29", "1899/12/31", "2200.1.1",
		"2026/10/3 24:00", "2026.10.3 15:60", "2026-1-3T15:04:60",
		"2026/10/3T15:04:05Z", "2026.10.3T15:04:05+08:00", "2026-1-3T15:04:05Z",
		"2026-10-03T15:04:05+24:00", "2026-10-03T15:04:05-24:00", "2026-10-03T15:04:05+08:60",
		"2026-10-03T15:04:05+8:00", "2026-10-03T15:04:05+08:0",
		"2026/10/3 then lunch", "on 2026/10/3", "2026/10/3\n15:04", "2026/10/3 15:04 UTC",
	} {
		t.Run(input, func(t *testing.T) {
			if got := Analyze(input); got.Type == TypeDate || got.Date != nil {
				t.Fatalf("malformed date recognized: %+v", got)
			}
		})
	}
}
