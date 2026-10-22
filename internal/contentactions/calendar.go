package contentactions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"copiggy/internal/clipmodel"
)

var ErrInvalidDate = errors.New("invalid calendar date")

// CalendarDocument creates a single event without accessing the system clock.
// Zone-less dates remain floating local values, while an explicit zone becomes
// its equivalent UTC instant. All-day events end at the next calendar date.
func CalendarDocument(date clipmodel.DateValue) ([]byte, error) {
	if date.Time.IsZero() || date.Time.Year() < 1900 || date.Time.Year() >= 2200 || date.HasZone && !date.HasTime {
		return nil, ErrInvalidDate
	}
	if !date.HasZone {
		// Floating fields are not an instant in Time's location. Preserve them
		// and do civil arithmetic even if a caller supplies a DST-aware zone.
		year, month, day := date.Time.Date()
		hour, minute, second := date.Time.Clock()
		date.Time = time.Date(year, month, day, hour, minute, second, date.Time.Nanosecond(), time.UTC)
	}
	start, end, summary := "", "", ""
	if !date.HasTime {
		start = "DTSTART;VALUE=DATE:" + date.Time.Format("20060102")
		end = "DTEND;VALUE=DATE:" + date.Time.AddDate(0, 0, 1).Format("20060102")
		summary = date.Time.Format(time.DateOnly)
	} else if date.HasZone {
		start = "DTSTART:" + date.Time.UTC().Format("20060102T150405Z")
		end = "DTEND:" + date.Time.Add(time.Hour).UTC().Format("20060102T150405Z")
		summary = date.Time.Format("2006-01-02 15:04 -07:00")
	} else {
		start = "DTSTART:" + date.Time.Format("20060102T150405")
		end = "DTEND:" + date.Time.Add(time.Hour).Format("20060102T150405")
		summary = date.Time.Format("2006-01-02 15:04")
	}
	digest := sha256.Sum256([]byte(start))
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Copiggy//Calendar//EN", "CALSCALE:GREGORIAN",
		"BEGIN:VEVENT", "UID:" + hex.EncodeToString(digest[:16]) + "@copiggy.local",
		"DTSTAMP:" + date.Time.UTC().Format("20060102T150405Z"),
		start, end, "SUMMARY:" + summary,
		"END:VEVENT", "END:VCALENDAR", "",
	}
	return []byte(strings.Join(lines, "\r\n")), nil
}
