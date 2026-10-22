package clipmodel

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ColorValue keeps eight-bit sRGB channels and explicit normalized display
// alternatives. Parsing never changes the original clipboard representation.
type ColorValue struct {
	R, G, B, A    uint8
	HEX, RGB, HSL string
}

func analyzeColor(value string) *ColorValue {
	if len(value) > 128 {
		return nil
	}
	var r, g, b, a float64
	a = 1
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		if len(hex) == 3 || len(hex) == 4 {
			var expanded strings.Builder
			for _, ch := range hex {
				expanded.WriteRune(ch)
				expanded.WriteRune(ch)
			}
			hex = expanded.String()
		}
		if len(hex) != 6 && len(hex) != 8 {
			return nil
		}
		integer, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return nil
		}
		if len(hex) == 6 {
			integer = integer<<8 | 255
		}
		r, g, b, a = float64(integer>>24), float64(integer>>16&255), float64(integer>>8&255), float64(integer&255)/255
	} else {
		function, body, ok := strings.Cut(strings.ToLower(value), "(")
		if !ok || !strings.HasSuffix(body, ")") {
			return nil
		}
		function, body = strings.TrimSpace(function), strings.TrimSuffix(body, ")")
		if function != "rgb" && function != "rgba" && function != "hsl" && function != "hsla" {
			return nil
		}
		parts, ok := colorComponents(body)
		if !ok {
			return nil
		}
		if function == "rgba" || function == "hsla" {
			if len(parts) != 4 {
				return nil
			}
		}
		if len(parts) == 4 {
			if a, ok = colorNumber(parts[3], 1); !ok {
				return nil
			}
		}
		if function == "rgb" || function == "rgba" {
			if r, ok = colorNumber(parts[0], 255); !ok {
				return nil
			}
			if g, ok = colorNumber(parts[1], 255); !ok {
				return nil
			}
			if b, ok = colorNumber(parts[2], 255); !ok {
				return nil
			}
		} else {
			hueText := strings.TrimSuffix(parts[0], "deg")
			h, err := strconv.ParseFloat(hueText, 64)
			if err != nil || math.IsNaN(h) || math.IsInf(h, 0) || !strings.HasSuffix(parts[1], "%") || !strings.HasSuffix(parts[2], "%") {
				return nil
			}
			s, okS := colorNumber(parts[1], 1)
			l, okL := colorNumber(parts[2], 1)
			if !okS || !okL {
				return nil
			}
			r, g, b = hslRGB(math.Mod(math.Mod(h, 360)+360, 360), s, l)
		}
	}
	return describeColor(uint8(math.Round(r)), uint8(math.Round(g)), uint8(math.Round(b)), uint8(math.Round(a*255)))
}

func colorComponents(body string) ([]string, bool) {
	var parts []string
	if strings.Contains(body, ",") {
		if strings.Contains(body, "/") {
			return nil, false
		}
		parts = strings.Split(body, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
	} else {
		channels, alpha, hasAlpha := strings.Cut(body, "/")
		parts = strings.Fields(channels)
		if len(parts) != 3 {
			return nil, false
		}
		if hasAlpha {
			parts = append(parts, strings.TrimSpace(alpha))
		}
	}
	return parts, len(parts) == 3 || len(parts) == 4
}

func colorNumber(value string, maximum float64) (float64, bool) {
	percent := strings.HasSuffix(value, "%")
	parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	if percent {
		parsed = parsed / 100 * maximum
	}
	return parsed, parsed >= 0 && parsed <= maximum
}

func hslRGB(h, s, l float64) (r, g, b float64) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return (r + m) * 255, (g + m) * 255, (b + m) * 255
}

func describeColor(r, g, b, a uint8) *ColorValue {
	value := &ColorValue{R: r, G: g, B: b, A: a, HEX: fmt.Sprintf("#%02X%02X%02X", r, g, b)}
	value.RGB = fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	hi, lo := math.Max(rf, math.Max(gf, bf)), math.Min(rf, math.Min(gf, bf))
	h, s, l := 0.0, 0.0, (hi+lo)/2
	delta := hi - lo
	if delta > 0 {
		s = delta / (1 - math.Abs(2*l-1))
		switch hi {
		case rf:
			h = math.Mod((gf-bf)/delta, 6)
		case gf:
			h = (bf-rf)/delta + 2
		default:
			h = (rf-gf)/delta + 4
		}
		h = math.Mod(h*60+360, 360)
	}
	// Two decimal places keep rounding error below half an eight-bit channel
	// across hue, saturation and lightness. Integer percentages can turn
	// near-black/white swatches into different colors when copied and parsed.
	hue, saturation, lightness := colorDecimal(h), colorDecimal(s*100), colorDecimal(l*100)
	value.HSL = fmt.Sprintf("hsl(%s, %s%%, %s%%)", hue, saturation, lightness)
	if a != 255 {
		alpha := strconv.FormatFloat(float64(a)/255, 'f', 3, 64)
		alpha = strings.TrimRight(strings.TrimRight(alpha, "0"), ".")
		value.HEX += fmt.Sprintf("%02X", a)
		value.RGB = fmt.Sprintf("rgba(%d, %d, %d, %s)", r, g, b, alpha)
		value.HSL = fmt.Sprintf("hsla(%s, %s%%, %s%%, %s)", hue, saturation, lightness, alpha)
	}
	return value
}

func colorDecimal(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 2, 64), "0"), ".")
}
