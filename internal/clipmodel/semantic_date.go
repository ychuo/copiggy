package clipmodel

import (
	"regexp"
	"strconv"
	"time"
)

var civilDatePattern = regexp.MustCompile(`^([0-9]{4})([-/.])([0-9]{1,2})([-/.])([0-9]{1,2})([T ].+)?$`)

func analyzeDate(value string) *DateValue {
	if len(value) < 8 || len(value) > 40 {
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
	// Explicit zones retain the existing RFC 3339 syntax and instant semantics.
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil && parsed.Year() >= 1900 && parsed.Year() <= 2199 {
		return &DateValue{Time: parsed, HasTime: true, HasZone: true}
	}
	parts := civilDatePattern.FindStringSubmatch(value)
	if parts == nil || parts[2] != parts[4] {
		return nil
	}
	// A four-digit leading year makes the order explicit. Normalize only the
	// parser input; the captured text and its exact representations stay intact.
	month, day := parts[3], parts[5]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}
	civil := parts[1] + "-" + month + "-" + day + parts[6]
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		// A zone-less value describes the fields the user copied, including
		// times in a local DST gap. Do not infer a location and change them.
		parsed, err := time.Parse(layout, civil)
		if err == nil && parsed.Year() >= 1900 && parsed.Year() <= 2199 {
			return &DateValue{Time: parsed, HasTime: parts[6] != ""}
		}
	}
	return nil
}
