package entities

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type TonemappingMode int

const (
	TonemappingModeAuto TonemappingMode = iota // 0
	TonemappingModeMAX                         // 1
	TonemappingModeRGB                         // 2
	TonemappingModeLUM                         // 3
	TonemappingModeITP                         // 4
)

// String converts the TonemappingMode to its string representation.
func (t TonemappingMode) String() string {
	names := [...]string{"auto", "max", "rgb", "lum", "itp"}
	if t < 0 || int(t) >= len(names) {
		return fmt.Sprintf("invalid(%d)", t)
	}
	return names[t]
}

// MarshalJSON serializes a TonemappingMode to JSON.
func (t *TonemappingMode) MarshalJSON() ([]byte, error) {
	str := t.String()
	if str == "" {
		return nil, fmt.Errorf("invalid TonemappingMode: %d", *t)
	}
	return json.Marshal(str)
}

// UnmarshalJSON parses a JSON string into a TonemappingMode.
func (t *TonemappingMode) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "auto":
		*t = TonemappingModeAuto
	case "max":
		*t = TonemappingModeMAX
	case "rgb":
		*t = TonemappingModeRGB
	case "lum":
		*t = TonemappingModeLUM
	case "itp":
		*t = TonemappingModeITP
	default:
		return fmt.Errorf("invalid TonemappingMode: %s", str)
	}

	return nil
}

// MarshalXML serializes a TonemappingMode to XML.
func (t *TonemappingMode) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	str := t.String()
	if str == "" {
		return fmt.Errorf("invalid TonemappingMode: %d", *t)
	}
	return e.EncodeElement(str, start)
}

// UnmarshalXML parses an XML element into a TonemappingMode.
func (t *TonemappingMode) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var str string
	if err := dec.DecodeElement(&str, &start); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)
	fmt.Println("->", str)

	switch str {
	case "auto":
		*t = TonemappingModeAuto
	case "max":
		*t = TonemappingModeMAX
	case "rgb":
		*t = TonemappingModeRGB
	case "lum":
		*t = TonemappingModeLUM
	case "itp":
		*t = TonemappingModeITP
	default:
		return fmt.Errorf("invalid TonemappingMode: %s", str)
	}

	return nil
}
