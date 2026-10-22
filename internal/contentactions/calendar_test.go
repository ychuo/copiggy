package contentactions

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"copiggy/internal/clipmodel"
)

func TestCalendarDocumentPreservesDateAndZoneSemantics(t *testing.T) {
	for _, tc := range []struct{ text, start, end string }{
		{"2026-10-22", "DTSTART;VALUE=DATE:20261022", "DTEND;VALUE=DATE:20261023"},
		{"2024-02-29", "DTSTART;VALUE=DATE:20240229", "DTEND;VALUE=DATE:20240301"},
		{"2026-12-31", "DTSTART;VALUE=DATE:20261231", "DTEND;VALUE=DATE:20270101"},
		{"2026-10-22 15:30", "DTSTART:20261022T153000", "DTEND:20261022T163000"},
		{"2026-10-22T15:30:00+08:00", "DTSTART:20261022T073000Z", "DTEND:20261022T083000Z"},
		{"2026-10-22T23:30:00-07:00", "DTSTART:20261023T063000Z", "DTEND:20261023T073000Z"},
	} {
		value := clipmodel.Analyze(tc.text).Date
		if value == nil {
			t.Fatal("fixture date not recognized")
		}
		document, err := CalendarDocument(*value)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(document), "\r\n")
		contains := func(want string) bool {
			for _, line := range lines {
				if line == want {
					return true
				}
			}
			return false
		}
		if !contains(tc.start) || !contains(tc.end) || !contains("VERSION:2.0") || !contains("BEGIN:VEVENT") || !contains("END:VEVENT") {
			t.Fatalf("wrong calendar for %q:\n%s", tc.text, document)
		}
		for _, line := range lines {
			if strings.ContainsAny(line, "\r\n") || len(line) > 75 {
				t.Fatalf("invalid ICS line %q", line)
			}
		}
		again, err := CalendarDocument(*value)
		if err != nil || !bytes.Equal(document, again) {
			t.Fatal("calendar generation uses live clock or randomness")
		}
		if value.HasZone {
			start, _ := time.Parse("20060102T150405Z", strings.TrimPrefix(tc.start, "DTSTART:"))
			if !start.Equal(value.Time) {
				t.Fatal("calendar changed zoned instant")
			}
		}
	}
}

func TestCalendarDocumentRejectsInvalidPublicValues(t *testing.T) {
	for _, value := range []clipmodel.DateValue{
		{}, {Time: time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), HasZone: true},
	} {
		if _, err := CalendarDocument(value); !errors.Is(err, ErrInvalidDate) {
			t.Fatal("invalid calendar value accepted")
		}
	}
}

func TestCalendarFloatingTimesUseCivilArithmeticAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	previous := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = previous })
	for _, tc := range []struct{ raw, start, end string }{
		{"2026-03-08 02:30", "20260308T023000", "20260308T033000"},
		{"2026-03-08 01:30", "20260308T013000", "20260308T023000"},
		{"2026-11-01 01:30", "20261101T013000", "20261101T023000"},
	} {
		target, err := OpenTarget(tc.raw)
		if err != nil || !bytes.Contains(target.Calendar, []byte("DTSTART:"+tc.start+"\r\n")) || !bytes.Contains(target.Calendar, []byte("DTEND:"+tc.end+"\r\n")) {
			t.Fatalf("floating calendar fields changed: %q => %s, %v", tc.raw, target.Calendar, err)
		}
	}
	// The public constructor also treats supplied Time fields as floating
	// when HasZone is false, even if that Time carries a local DST location.
	date := clipmodel.DateValue{Time: time.Date(2026, 3, 8, 1, 30, 0, 0, location), HasTime: true}
	document, err := CalendarDocument(date)
	if err != nil || !bytes.Contains(document, []byte("DTEND:20260308T023000\r\n")) {
		t.Fatalf("local offset changed a floating event's end: %s, %v", document, err)
	}
}
