package clipmodel

import (
	"errors"
	"strconv"
)

// CopyFormat names a supported local representation. A caller chooses a format,
// never supplies replacement text or protection policy.
type CopyFormat string

const (
	ColorHEX              CopyFormat = "color-hex"
	ColorRGB              CopyFormat = "color-rgb"
	ColorHSL              CopyFormat = "color-hsl"
	TimestampSeconds      CopyFormat = "timestamp-seconds"
	TimestampMilliseconds CopyFormat = "timestamp-milliseconds"
	PhoneDialing          CopyFormat = "phone-dialing"
)

var ErrInvalidCopyFormat = errors.New("copy format is unavailable for this content")

// CopyText derives one plain-text representation from the saved item. Analysis
// must still agree with its type; stale annotations cannot reinterpret content.
// The exact captured item and its representations are never modified.
func CopyText(item Item, format CopyFormat) (string, error) {
	value := AnalyzeItem(item)
	if value.Type != item.Type {
		return "", ErrInvalidCopyFormat
	}
	switch format {
	case ColorHEX, ColorRGB, ColorHSL:
		if item.Type != TypeColor || value.Color == nil {
			return "", ErrInvalidCopyFormat
		}
		switch format {
		case ColorHEX:
			return value.Color.HEX, nil
		case ColorRGB:
			return value.Color.RGB, nil
		default:
			return value.Color.HSL, nil
		}
	case TimestampSeconds, TimestampMilliseconds:
		if item.Type != TypeTimestamp || value.Timestamp == nil {
			return "", ErrInvalidCopyFormat
		}
		number := value.Timestamp.Seconds
		if format == TimestampMilliseconds {
			number = value.Timestamp.Milliseconds
		}
		return strconv.FormatInt(number, 10), nil
	case PhoneDialing:
		if item.Type != TypePhone || value.Phone == nil {
			return "", ErrInvalidCopyFormat
		}
		return value.Phone.DialingNumber(), nil
	default:
		return "", ErrInvalidCopyFormat
	}
}
