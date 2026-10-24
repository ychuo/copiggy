package clipmodel

import (
	"reflect"
	"testing"
)

func TestHSLAngleUnitsPreserveColorAndOriginal(t *testing.T) {
	for _, tc := range []struct{ input, hex string }{
		{"hsl(0.5turn 100% 50%)", "#00FFFF"},
		{"hsl(200grad, 100%, 50%)", "#00FFFF"},
		{"hsl(3.141592653589793rad 100% 50%)", "#00FFFF"},
		{"hsla(-0.25turn, 100%, 50%, 0.5)", "#8000FF80"},
		{"hsl(1.5turn 100% 50% / 25%)", "#00FFFF40"},
		{"HSL(200GRAD 100% 50%)", "#00FFFF"},
		{"hsl(180deg 100% 50%)", "#00FFFF"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			value := Analyze(tc.input)
			if value.Type != TypeColor || value.Color == nil || value.Color.HEX != tc.hex {
				t.Fatalf("color = %+v, want %s", value.Color, tc.hex)
			}
			store := NewStore(1)
			item, ok := store.Capture(tc.input, "Fixture Editor", false)
			if !ok || item.Text != tc.input {
				t.Fatal("color capture changed the original")
			}
			if text, err := CopyText(item, ColorHEX); err != nil || text != tc.hex {
				t.Fatalf("HEX copy = %q, %v", text, err)
			}
		})
	}
}

func TestHSLAngleUnitsRejectMalformedValues(t *testing.T) {
	for _, input := range []string{
		"hsl(NaNturn 100% 50%)", "hsl(Infgrad 100% 50%)", "hsl(1e999rad 100% 50%)",
		"hsl(0.5turnrad 100% 50%)", "hsl(turn 100% 50%)", "hsl(0.5 turn 100% 50%)",
		"hsl(180degrees 100% 50%)", "hsl(0.5turn 101% 50%)",
	} {
		if got := Analyze(input).Type; got != TypeText {
			t.Errorf("%q: got %s, want text", input, got)
		}
	}
}

func TestHWBColorsPreserveOriginalAndCopyChoices(t *testing.T) {
	for _, tc := range []struct{ input, hex string }{
		{"hwb(150 20% 10%)", "#33E68C"},
		{"hwb(45 40% 80%)", "#555555"},
		{"hwb(0 30% 70%)", "#4D4D4D"},
		{"hwb(0 100% 0%)", "#FFFFFF"},
		{"hwb(0 0% 100%)", "#000000"},
		{"hwb(0 100% 100%)", "#808080"},
		{"hwb(0.5turn 0% 0%)", "#00FFFF"},
		{"hwb(-.25turn 0% 0%)", "#8000FF"},
		{"hwb(200grad 20% 30% / .5)", "#33B3B380"},
		{"HWB(3.141592653589793RAD 0% 0% / 25%)", "#00FFFF40"},
		{"hwb(1e308turn 0% 0%)", "#FF0000"},
		{" \thwb(120deg 0% 0% / 0)\n", "#00FF0000"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			store := NewStore(1)
			item, ok := store.CaptureProtected(tc.input, "fixture.editor", []string{"fixture.editor"})
			if !ok || item.Type != TypeColor || item.Text != tc.input || !item.Allows("fixture.editor") || item.Allows("other.app") {
				t.Fatalf("HWB recognition changed original text or protection: %+v", item)
			}
			before := cloneItem(item)
			for _, format := range []CopyFormat{ColorHEX, ColorRGB, ColorHSL} {
				copied, err := CopyText(item, format)
				if err != nil {
					t.Fatalf("%s copy failed: %v", format, err)
				}
				color := Analyze(copied).Color
				if color == nil || color.HEX != tc.hex {
					t.Fatalf("%s copy = %q, want color %s", format, copied, tc.hex)
				}
			}
			if !reflect.DeepEqual(item, before) {
				t.Fatal("color copy changed the captured source or protection")
			}
		})
	}
}

func TestHWBColorsRejectIncompleteAndMalformedValues(t *testing.T) {
	for _, input := range []string{
		"hwb(0, 0%, 0%)", "hwb(0, 0%, 0%, .5)", "hwb(0 0% 0% .5)",
		"hwb(0 0% 0% /)", "hwb(0 0% 0% / .5 / .5)", "hwb(0 0%)", "hwb(0 0% 0%))",
		"hwb(0 0 0)", "hwb(0 -1% 0%)", "hwb(0 101% 0%)", "hwb(0 0% 101%)",
		"hwb(0 0% 0% / -1)", "hwb(0 0% 0% / 101%)", "hwb(0 0% 0% / 2)",
		"hwb(NaN 0% 0%)", "hwb(Infturn 0% 0%)", "hwb(1e999 0% 0%)", "hwb(0 NaN% 0%)",
		"hwb(none 0% 0%)", "hwb(0 none 0%)", "hwb(0 0% 0% / none)",
		"hwb(from red h w b)", "hwb(var(--hue) 0% 0%)", "hwb(calc(1 + 2) 0% 0%)",
		"hwb(0 0% 0%) extra", "color: hwb(0 0% 0%)", "hwba(0 0% 0% / .5)",
	} {
		if got := Analyze(input).Type; got != TypeText {
			t.Errorf("%q: got %s, want text", input, got)
		}
	}
}

func TestColorNumbersUseCSSDecimalSyntax(t *testing.T) {
	for _, tc := range []struct{ input, hex string }{
		{"rgb(+2.55e2 0e0 .0)", "#FF0000"},
		{"hsl(+5e-1turn 1e2% 5e1% / 5e-1)", "#00FFFF80"},
		{"hwb(+5e-1turn 0e0% 0e0% / +.5)", "#00FFFF80"},
	} {
		if color := Analyze(tc.input).Color; color == nil || color.HEX != tc.hex {
			t.Errorf("%q: color = %+v, want %s", tc.input, color, tc.hex)
		}
	}
	for _, input := range []string{
		"rgb(0xFF 0 0)", "rgb(0xffp0 0 0)", "rgb(2_55 0 0)", "rgb(1. 0 0)",
		"rgb(1e 0 0)", "rgb(. 0 0)", "rgb(NaN 0 0)", "rgb(Inf 0 0)",
		"hsl(0x1p0turn 100% 50%)", "hsl(1_80 100% 50%)", "hsl(0 1_00% 50%)",
		"hwb(0x1p0turn 0% 0%)", "hwb(1_80 0% 0%)", "hwb(0 0x1p0% 0%)", "hwb(0 0% 0% / 0x1p-1)",
	} {
		if got := Analyze(input).Type; got != TypeText {
			t.Errorf("%q: got %s, want text", input, got)
		}
	}
}
