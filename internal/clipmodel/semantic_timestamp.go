package clipmodel

import (
	"strconv"
	"strings"
	"time"
)

func analyzeTimestamp(value string) *TimestampValue {
	number, unit := value, ""
	for _, suffix := range []string{"ms", "s"} {
		if strings.HasSuffix(value, suffix) {
			unit = suffix
			number = strings.TrimSuffix(strings.TrimSuffix(value, suffix), " ")
			break
		}
	}
	if unit == "" {
		// Keep the established bare-number rule; only an explicit unit admits
		// the shorter epoch values at the beginning of the supported range.
		switch len(number) {
		case 10:
			unit = "s"
		case 13:
			unit = "ms"
		default:
			return nil
		}
	} else if unit == "s" && (len(number) < 9 || len(number) > 10) ||
		unit == "ms" && (len(number) < 12 || len(number) > 13) {
		return nil
	}
	if !asciiDigits(number) {
		return nil
	}
	integer, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return nil
	}
	milliseconds := integer
	if unit == "s" {
		milliseconds *= 1000
	}
	instant := time.UnixMilli(milliseconds).UTC()
	// A fixed practical range avoids treating arbitrary IDs as dates. It never
	// depends on today's date, so restoring history does not change its type.
	if instant.Year() < 2000 || instant.Year() >= 2100 {
		return nil
	}
	return &TimestampValue{Time: instant, Seconds: instant.Unix(), Milliseconds: milliseconds, Unit: unit}
}
