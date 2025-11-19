package entities

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

// EncoderPreset defines the encoder preset levels.
type EncoderPreset int

const (
	// Auto preset.
	Auto EncoderPreset = 0
	// Placebo preset.
	Placebo EncoderPreset = 1
	// Veryslow preset.
	VerySlow EncoderPreset = 2
	// Slower preset.
	Slower EncoderPreset = 3
	// Slow preset.
	Slow EncoderPreset = 4
	// Medium preset.
	Medium EncoderPreset = 5
	// Fast preset.
	Fast EncoderPreset = 6
	// Faster preset.
	Faster EncoderPreset = 7
	// Veryfast preset.
	VeryFast EncoderPreset = 8
	// Superfast preset.
	SuperFast EncoderPreset = 9
	// Ultrafast preset.
	UltraFast EncoderPreset = 10
)

// String returns the string representation of the EncoderPreset.
func (e EncoderPreset) String() string {
	switch e {
	case Auto:
		return "auto"
	case Placebo:
		return "placebo"
	case VerySlow:
		return "veryslow"
	case Slower:
		return "slower"
	case Slow:
		return "slow"
	case Medium:
		return "medium"
	case Fast:
		return "fast"
	case Faster:
		return "faster"
	case VeryFast:
		return "veryfast"
	case SuperFast:
		return "superfast"
	case UltraFast:
		return "ultrafast"
	default:
		return "unknown"
	}
}

// MarshalJSON serializes an EncoderPreset to JSON.
func (e *EncoderPreset) MarshalJSON() ([]byte, error) {
	str := e.String()
	if str == "unknown" {
		return nil, fmt.Errorf("invalid EncoderPreset: %d", *e)
	}
	return json.Marshal(str)
}

// UnmarshalJSON implements the json.Unmarshaler interface for EncoderPreset.
func (e *EncoderPreset) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "auto":
		*e = Auto
	case "placebo":
		*e = Placebo
	case "veryslow":
		*e = VerySlow
	case "slower":
		*e = Slower
	case "slow":
		*e = Slow
	case "medium":
		*e = Medium
	case "fast":
		*e = Fast
	case "faster":
		*e = Faster
	case "veryfast":
		*e = VeryFast
	case "superfast":
		*e = SuperFast
	case "ultrafast":
		*e = UltraFast
	default:
		*e = Auto // Default to Auto for unknown values
	}
	return nil
}

// MarshalXML serializes an EncoderPreset to XML.
func (e *EncoderPreset) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	str := e.String()
	if str == "unknown" {
		return fmt.Errorf("invalid EncoderPreset: %d", *e)
	}
	return enc.EncodeElement(str, start)
}

// UnmarshalXML implements the xml.Unmarshaler interface for EncoderPreset.
func (e *EncoderPreset) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}

	switch s {
	case "auto":
		*e = Auto
	case "placebo":
		*e = Placebo
	case "veryslow":
		*e = VerySlow
	case "slower":
		*e = Slower
	case "slow":
		*e = Slow
	case "medium":
		*e = Medium
	case "fast":
		*e = Fast
	case "faster":
		*e = Faster
	case "veryfast":
		*e = VeryFast
	case "superfast":
		*e = SuperFast
	case "ultrafast":
		*e = UltraFast
	default:
		*e = Auto // Default to Auto for unknown values
	}
	return nil
}
