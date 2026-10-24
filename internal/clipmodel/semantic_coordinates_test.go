package clipmodel

import (
	"math"
	"strings"
	"testing"
)

func TestCoordinatesRecognizeExplicitDirectionsAndDegreeUnits(t *testing.T) {
	for _, tc := range []struct {
		input    string
		lat, lon float64
	}{
		{"37.428230 N, 122.168861 W", 37.428230, -122.168861},
		{"W 122.168861, N 37.428230", 37.428230, -122.168861},
		{"37.428230N 122.168861W", 37.428230, -122.168861},
		{"  w122.168861\tn37.428230 \n", 37.428230, -122.168861},
		{"S33.8688 151.2093E", -33.8688, 151.2093},
		{"N 37.428230°, W 122.168861°", 37.428230, -122.168861},
		{"37.428230° N, 122.168861° W", 37.428230, -122.168861},
		{"37 N, 122 W", 37, -122},
		{"90N, 180E", 90, 180},
		{"180W 90S", -90, -180},
		{"0.0S, 0.0W", 0, 0},
		{`37°25'41.628"N 122°10'7.8996"W`, 37.428230, -122.168861},
		{`W 122° 10' 7.8996", N 37° 25' 41.628"`, 37.428230, -122.168861},
		{"37°25′41.628″n, 122°10′7.8996″w", 37.428230, -122.168861},
		{`S 33°52'7.68", E 151°12'33.48"`, -33.8688, 151.2093},
		{`90°0'0"N, 180°0'0"E`, 90, 180},
		{`90°0'0.000000000000000"N, 180°0'0.000000000000000"E`, 90, 180},
		{`37°25'59.999999999999999"N, 122°10'59.999999999999999"W`, 37 + 26.0/60, -122 - 11.0/60},
		{`89°59'59.999999999999999"N, 179°59'59.999999999999999"E`, 90, 180},
		{`0°0'0"S, 0°0'0"W`, 0, 0},
		{`37°25'41.628"N, 122.168861W`, 37.428230, -122.168861},
		{`37°25.6938'N 122°10.13166'W`, 37.428230, -122.168861},
		{`W 122° 10.13166', N 37° 25.6938'`, 37.428230, -122.168861},
		{"37°25.6938′n, 122°10.13166′w", 37.428230, -122.168861},
		{`S 33°52.128', E 151°12.558'`, -33.8688, 151.2093},
		{`37°25'N 122°10'W`, 37 + 25.0/60, -122 - 10.0/60},
		{`37°25'41"N, 122°10'W`, 37 + 25.0/60 + 41.0/3600, -122 - 10.0/60},
		{`37°25.6938'N, 122.168861W`, 37.428230, -122.168861},
		{`90°0.000'N, 180°0.000'E`, 90, 180},
		{`0°0.000'S, 0°0.000'W`, 0, 0},
		{`89°59.999999999999999999'N, 179°59.999999999999999999'E`, 90, 180},
	} {
		t.Run(tc.input, func(t *testing.T) {
			analysis := Analyze(tc.input)
			if analysis.Type != TypeCoordinates || analysis.Coordinates == nil ||
				math.Abs(analysis.Coordinates.Latitude-tc.lat) > 1e-9 || math.Abs(analysis.Coordinates.Longitude-tc.lon) > 1e-9 {
				t.Fatalf("coordinates = %+v, want latitude %g, longitude %g", analysis.Coordinates, tc.lat, tc.lon)
			}
			store := NewStore(1)
			item, ok := store.CaptureProtected(tc.input, "fixture.editor", []string{"fixture.editor"})
			if !ok || item.Type != TypeCoordinates || item.Text != tc.input || !item.Allows("fixture.editor") || item.Allows("other.app") {
				t.Fatalf("coordinate recognition changed the saved source or protection: %+v", item)
			}
		})
	}
}

func TestCoordinatesRejectContradictoryAndIncompleteDirections(t *testing.T) {
	for _, raw := range []string{
		"N37.4 N122.1", "37.4N, 122.1S", "37.4E, 122.1W",
		"N37.4S, W122.1", "N37.4N, W122.1", "37.4NE, 122.1W",
		"-37.4N, 122.1W", "+37.4N, 122.1W", "N37.4, W-122.1", "37.4S, -122.1E",
		"91N, 122W", "37N, 181W", "90.000000000000001N, 180E", "90N, 180.000000000000001E",
		"37.4, 122.1W", "37.4N, 122.1", "N37.4W122.1", "N37.4, W122.1, E12.3",
		"Latitude: 37.4N, longitude: 122.1W", "37.4N, 122.1W then turn left", "Meet at N37.4 W122.1",
		"N37.4\nW122.1", "NaNN, 122.1W", "N1e1, E1e2", "37, 122", "37N 122",
		`37°60'0"N, 122°10'0"W`, `37°25'60"N, 122°10'0"W`,
		`90°0'0.01"N, 180°0'0"E`, `90°1'0"S, 180°0'0"W`, `90°0'0"N, 180°0'0.01"W`,
		`90°0'0.000000000000001"N, 180°0'0"E`, `90°0'0"N, 180°0'0.000000000000001"E`,
		`37°25'60.000000000000001"N, 122°10'0"W`, `37°25'0"N, 122°10'60.000000000000001"W`,
		`37.1°25'0"N, 122°10'0"W`, `37°25.1'0"N, 122°10'0"W`,
		`37°-25'0"N, 122°10'0"W`, `37°25'-1"N, 122°10'0"W`,
		`37°25'41"N, 122°10'7"`, `37°25'41", 122°10'7"W`,
		`37°25'41N, 122°10'7"W`,
		`37°25'41"N, 122°10'7"W extra`, `37 25 41 N, 122 10 7 W`,
		`37°60'N, 122°10'W`, `37°60.000000000000001'N, 122°10'W`, `37°25'N, 122°60'W`,
		`90°0.000000000000001'N, 180°0'E`, `90°0'N, 180°0.000000000000001'E`,
		`91°0'N, 122°0'W`, `37°0'N, 181°0'W`,
		`37.5°25'N, 122°10'W`, `37°-25'N, 122°10'W`, `N-37°25', W122°10'`,
		`37°25.5N, 122°10'W`, `37 25.5'N, 122°10'W`, `37°25.5'', 122°10'W`,
		`37°25.5', 122°10.5'`, `37°25.5'N, 122°10.5'`,
		`37°25.5'41"N, 122°10'7"W`, `37°25.5'N 122°10.5'N`,
		"N37." + strings.Repeat("1", 256) + ", W122.1",
	} {
		t.Run(raw, func(t *testing.T) {
			if got := Analyze(raw).Type; got != TypeText {
				t.Fatalf("%q: got %q, want ordinary text", raw, got)
			}
		})
	}
}

func TestDirectionalCoordinatesRejectUnderflowingPoleOverflow(t *testing.T) {
	// Exercise the numeric parser directly so this exact range check remains
	// independent of the separate 256-byte public-recognition limit.
	seconds := "0." + strings.Repeat("0", 400) + "1"
	for _, value := range []string{`90°0'` + seconds + `"N`, `180°0'` + seconds + `"E`} {
		if _, _, ok := directionalCoordinate(value); ok {
			t.Fatal("nonzero pole seconds must not become valid through float underflow")
		}
	}
}

func TestCoordinatesCompareExactDecimalsBeforeChoosingAxes(t *testing.T) {
	for _, tc := range []struct {
		input    string
		lat, lon float64
	}{
		{"90.0000, 180.0000", 90, 180},
		{"-90.0000, -180.0000", -90, -180},
		{"180.0000, +90.0000", 90, 180},
		{"+090.0000, -045.0000", 90, -45},
		{"90.000000000000001, 45.0", 45, 90},
		{"-90.000000000000001, -45.0", -45, -90},
		{"90.000000000000001, 90.000000000000000", 90, 90},
		{"89.999999999999999, 179.999999999999999", 90, 180},
		{"45.0, 90.000000000000001", 45, 90},
	} {
		t.Run(tc.input, func(t *testing.T) {
			analysis := Analyze(tc.input)
			if analysis.Type != TypeCoordinates || analysis.Coordinates == nil ||
				analysis.Coordinates.Latitude != tc.lat || analysis.Coordinates.Longitude != tc.lon {
				t.Fatalf("coordinates = %+v, want latitude %g, longitude %g", analysis.Coordinates, tc.lat, tc.lon)
			}
			store := NewStore(1)
			item, ok := store.Capture(tc.input, "Fixture Editor", false)
			if !ok || item.Text != tc.input {
				t.Fatal("coordinate recognition rewrote the original decimal text")
			}
		})
	}
	for _, input := range []string{
		"90.000000000000001, 180.000000000000001",
		"90.000000000000001, 90.000000000000001",
		"-90.000000000000001, -90.000000000000001",
		"90.000000000000001, 122.168861",
		"180.000000000000001, 45.0", "-180.000000000000001, 45.0",
		"45.0, 180.000000000000001", "45.0, -180.000000000000001",
		"180.000000000000001, 90.0000", "90.0000, 180.000000000000001",
	} {
		if got := Analyze(input).Type; got != TypeText {
			t.Errorf("%q: got %s, want text", input, got)
		}
	}
}
