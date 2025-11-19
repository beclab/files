package entities

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type TonemappingAlgorithm int

const (
	TonemappingAlgorithmNone     TonemappingAlgorithm = iota // 0
	TonemappingAlgorithmClip                                 // 1
	TonemappingAlgorithmLinear                               // 2
	TonemappingAlgorithmGamma                                // 3
	TonemappingAlgorithmReinhard                             // 4
	TonemappingAlgorithmHable                                // 5
	TonemappingAlgorithmMobius                               // 6
	TonemappingAlgorithmBT2390                               // 7
)

func (t TonemappingAlgorithm) String() string {
	names := [...]string{"none", "clip", "linear", "gamma", "reinhard", "hable", "mobius", "bt2390"}
	if t < 0 || int(t) >= len(names) {
		return fmt.Sprintf("invalid(%d)", t)
	}
	return names[t]
}

// MarshalJSON serializes a TonemappingAlgorithm to JSON.
func (t *TonemappingAlgorithm) MarshalJSON() ([]byte, error) {
	str := t.String()
	if str == "" || str == "Unknown" {
		return nil, fmt.Errorf("invalid TonemappingAlgorithm: %d", *t)
	}
	return json.Marshal(str)
}

// UnmarshalJSON parses a JSON string into a TonemappingAlgorithm.
func (t *TonemappingAlgorithm) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "none":
		*t = TonemappingAlgorithmNone
	case "clip":
		*t = TonemappingAlgorithmClip
	case "linear":
		*t = TonemappingAlgorithmLinear
	case "gamma":
		*t = TonemappingAlgorithmGamma
	case "reinhard":
		*t = TonemappingAlgorithmReinhard
	case "hable":
		*t = TonemappingAlgorithmHable
	case "mobius":
		*t = TonemappingAlgorithmMobius
	case "bt2390":
		*t = TonemappingAlgorithmBT2390
	default:
		return fmt.Errorf("invalid TonemappingAlgorithm: %s", str)
	}

	return nil
}

// MarshalXML serializes a TonemappingAlgorithm to XML.
func (t *TonemappingAlgorithm) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	str := t.String()
	if str == "" || str == "Unknown" {
		return fmt.Errorf("invalid TonemappingAlgorithm: %d", *t)
	}
	return e.EncodeElement(str, start)
}

// UnmarshalXML parses an XML element into a TonemappingAlgorithm.
func (t *TonemappingAlgorithm) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var str string
	if err := dec.DecodeElement(&str, &start); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "none":
		*t = TonemappingAlgorithmNone
	case "clip":
		*t = TonemappingAlgorithmClip
	case "linear":
		*t = TonemappingAlgorithmLinear
	case "gamma":
		*t = TonemappingAlgorithmGamma
	case "reinhard":
		*t = TonemappingAlgorithmReinhard
	case "hable":
		*t = TonemappingAlgorithmHable
	case "mobius":
		*t = TonemappingAlgorithmMobius
	case "bt2390":
		*t = TonemappingAlgorithmBT2390
	default:
		return fmt.Errorf("invalid TonemappingAlgorithm: %s", str)
	}

	return nil
}
