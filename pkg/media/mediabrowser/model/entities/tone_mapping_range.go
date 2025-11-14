package entities

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type TonemappingRange int

const (
	TonemappingRangeAuto TonemappingRange = iota // 0
	TonemappingRangeTV                           // 1
	TonemappingRangePC                           // 2
)

// String converts the TonemappingRange to its string representation.
func (t TonemappingRange) String() string {
	names := [...]string{"auto", "tv", "pc"}
	if t < 0 || int(t) >= len(names) {
		return fmt.Sprintf("invalid(%d)", t)
	}
	return names[t]
}

// MarshalJSON serializes a TonemappingRange to JSON.
func (t *TonemappingRange) MarshalJSON() ([]byte, error) {
	str := t.String()
	if strings.HasPrefix(str, "invalid") {
		return nil, fmt.Errorf("invalid TonemappingRange: %d", *t)
	}
	return json.Marshal(str)
}

// UnmarshalJSON parses a JSON string into a TonemappingRange.
func (t *TonemappingRange) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "auto":
		*t = TonemappingRangeAuto
	case "tv":
		*t = TonemappingRangeTV
	case "pc":
		*t = TonemappingRangePC
	default:
		return fmt.Errorf("invalid TonemappingRange: %s", str)
	}

	return nil
}

// MarshalXML serializes a TonemappingRange to XML.
func (t *TonemappingRange) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	str := t.String()
	if strings.HasPrefix(str, "invalid") {
		return fmt.Errorf("invalid TonemappingRange: %d", *t)
	}
	return e.EncodeElement(str, start)
}

// UnmarshalXML parses an XML element into a TonemappingRange.
func (t *TonemappingRange) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var str string
	if err := dec.DecodeElement(&str, &start); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "auto":
		*t = TonemappingRangeAuto
	case "tv":
		*t = TonemappingRangeTV
	case "pc":
		*t = TonemappingRangePC
	default:
		return fmt.Errorf("invalid TonemappingRange: %s", str)
	}

	return nil
}
