package clipmodel

import (
	"regexp"
	"strconv"
	"strings"
)

var coordinatePattern = regexp.MustCompile(`^([+-]?[0-9]{1,3}(?:\.[0-9]+)?)\s*,\s*([+-]?[0-9]{1,3}(?:\.[0-9]+)?)$`)

// Directional values carry their own axis and sign, so either order is safe.
// Match the complete pair before parsing, without extracting from prose or
// accepting contradictory signs, duplicate directions or incomplete units.
const coordinateMagnitudePattern = `(?:[0-9]{1,3}\s*°\s*[0-9]{1,2}\s*['′]\s*[0-9]{1,2}(?:\.[0-9]+)?\s*["″]|[0-9]{1,3}\s*°\s*[0-9]{1,2}(?:\.[0-9]+)?\s*['′]|[0-9]{1,3}(?:\.[0-9]+)?\s*°?)`
const directionalCoordinatePattern = `(?:[NSEW]\s*` + coordinateMagnitudePattern + `|` + coordinateMagnitudePattern + `\s*[NSEW])`

var directionalCoordinatePairPattern = regexp.MustCompile(`(?i)^(` + directionalCoordinatePattern + `)(?:\s*,\s*|\s+)(` + directionalCoordinatePattern + `)$`)
var coordinateDMSPattern = regexp.MustCompile(`^([0-9]{1,3})\s*°\s*([0-9]{1,2})\s*['′]\s*([0-9]{1,2}(?:\.[0-9]+)?)\s*["″]$`)
var coordinateDegreesMinutesPattern = regexp.MustCompile(`^([0-9]{1,3})\s*°\s*([0-9]{1,2}(?:\.[0-9]+)?)\s*['′]$`)

func analyzeCoordinates(value string) *CoordinatesValue {
	parts := coordinatePattern.FindStringSubmatch(value)
	if parts == nil {
		return analyzeDirectionalCoordinates(value)
	}
	// Keep latitude first when both orders are valid. A first value outside
	// latitude's range can only be longitude. Decide from the original decimal
	// text, before floating-point rounding can erase an overflow at 90 or 180.
	latText, lonText := parts[1], parts[2]
	if !coordinateMagnitudeAtMost(latText, 90) {
		latText, lonText = lonText, latText
	}
	if !coordinateMagnitudeAtMost(latText, 90) || !coordinateMagnitudeAtMost(lonText, 180) {
		return nil
	}
	// Requiring decimal precision avoids turning ordinary pairs/counts into maps.
	if !strings.Contains(parts[1], ".") || !strings.Contains(parts[2], ".") {
		return nil
	}
	lat, errLat := strconv.ParseFloat(latText, 64)
	lon, errLon := strconv.ParseFloat(lonText, 64)
	if errLat != nil || errLon != nil {
		return nil
	}
	return &CoordinatesValue{Latitude: lat, Longitude: lon}
}

// coordinateMagnitudeAtMost compares a regex-validated decimal magnitude to an
// integer limit without losing significant fractional digits at the boundary.
func coordinateMagnitudeAtMost(value string, limit int) bool {
	value = strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
	whole, fraction, _ := strings.Cut(value, ".")
	integer, err := strconv.Atoi(whole)
	return err == nil && (integer < limit || integer == limit && strings.Trim(fraction, "0") == "")
}

func analyzeDirectionalCoordinates(value string) *CoordinatesValue {
	if len(value) > 256 {
		return nil
	}
	parts := directionalCoordinatePairPattern.FindStringSubmatch(value)
	if parts == nil {
		return nil
	}
	first, firstLatitude, firstOK := directionalCoordinate(parts[1])
	second, secondLatitude, secondOK := directionalCoordinate(parts[2])
	if !firstOK || !secondOK || firstLatitude == secondLatitude {
		return nil
	}
	if firstLatitude {
		return &CoordinatesValue{Latitude: first, Longitude: second}
	}
	return &CoordinatesValue{Latitude: second, Longitude: first}
}

func directionalCoordinate(value string) (magnitude float64, latitude, ok bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	direction := value[0]
	if strings.ContainsRune("NSEW", rune(direction)) {
		value = value[1:]
	} else {
		direction, value = value[len(value)-1], value[:len(value)-1]
	}
	latitude = direction == 'N' || direction == 'S'
	limit := 180
	if latitude {
		limit = 90
	}
	value = strings.TrimSpace(value)
	if dms := coordinateDMSPattern.FindStringSubmatch(value); dms != nil {
		degrees, degreeErr := strconv.Atoi(dms[1])
		minutes, minuteErr := strconv.Atoi(dms[2])
		secondWhole, _, _ := strings.Cut(dms[3], ".")
		wholeSeconds, secondErr := strconv.Atoi(secondWhole)
		if degreeErr != nil || minuteErr != nil || secondErr != nil || degrees > limit || minutes >= 60 || wholeSeconds >= 60 ||
			degrees == limit && (minutes != 0 || !coordinateMagnitudeAtMost(dms[3], 0)) {
			return 0, latitude, false
		}
		seconds, err := strconv.ParseFloat(dms[3], 64)
		if err != nil {
			return 0, latitude, false
		}
		magnitude = float64(degrees) + float64(minutes)/60 + seconds/3600
	} else if dm := coordinateDegreesMinutesPattern.FindStringSubmatch(value); dm != nil {
		degrees, degreeErr := strconv.Atoi(dm[1])
		minuteWhole, _, _ := strings.Cut(dm[2], ".")
		wholeMinutes, minuteErr := strconv.Atoi(minuteWhole)
		if degreeErr != nil || minuteErr != nil || degrees > limit || wholeMinutes >= 60 ||
			degrees == limit && !coordinateMagnitudeAtMost(dm[2], 0) {
			return 0, latitude, false
		}
		minutes, err := strconv.ParseFloat(dm[2], 64)
		if err != nil {
			return 0, latitude, false
		}
		magnitude = float64(degrees) + minutes/60
	} else {
		value = strings.TrimSpace(strings.TrimSuffix(value, "°"))
		if !coordinateMagnitudeAtMost(value, limit) {
			return 0, latitude, false
		}
		var err error
		magnitude, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, latitude, false
		}
	}
	if direction == 'S' || direction == 'W' {
		magnitude = -magnitude
	}
	return magnitude, latitude, true
}
