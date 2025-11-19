package session

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type HardwareEncodingType int

const (
	AMF HardwareEncodingType = iota
	QSV
	NVENC
	V4L2M2M
	VAAPI
	VideoToolBox
	RKMPP
)

// String converts the HardwareEncodingType to its string representation.
func (h HardwareEncodingType) String() string {
	names := [...]string{"AMF", "QSV", "NVENC", "V4L2M2M", "VAAPI", "VideoToolBox", "RKMPP"}
	if h < 0 || int(h) >= len(names) {
		return fmt.Sprintf("invalid(%d)", h)
	}
	return names[h]
}

// ParseHardwareEncodingType parses a string into a HardwareEncodingType.
func ParseHardwareEncodingType(s string) (HardwareEncodingType, error) {
	switch strings.ToLower(s) {
	case "amf":
		return AMF, nil
	case "qsv":
		return QSV, nil
	case "nvenc":
		return NVENC, nil
	case "v4l2m2m":
		return V4L2M2M, nil
	case "vaapi":
		return VAAPI, nil
	case "videotoolbox":
		return VideoToolBox, nil
	case "rkmpp":
		return RKMPP, nil
	default:
		return -1, fmt.Errorf("invalid hardware encoding type: %s", s)
	}
}

// MarshalJSON serializes a HardwareEncodingType to JSON.
func (h *HardwareEncodingType) MarshalJSON() ([]byte, error) {
	str := h.String()
	if strings.HasPrefix(str, "invalid") {
		return nil, fmt.Errorf("invalid HardwareEncodingType: %d", *h)
	}
	return json.Marshal(str)
}

// UnmarshalJSON parses a JSON string into a HardwareEncodingType.
func (h *HardwareEncodingType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	value, err := ParseHardwareEncodingType(str)
	if err != nil {
		return err
	}
	*h = value
	return nil
}

// MarshalXML serializes a HardwareEncodingType to XML.
func (h *HardwareEncodingType) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	str := h.String()
	if strings.HasPrefix(str, "invalid") {
		return fmt.Errorf("invalid HardwareEncodingType: %d", *h)
	}
	return e.EncodeElement(str, start)
}

// UnmarshalXML parses an XML element into a HardwareEncodingType.
func (h *HardwareEncodingType) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var str string
	if err := dec.DecodeElement(&str, &start); err != nil {
		return err
	}

	value, err := ParseHardwareEncodingType(str)
	if err != nil {
		return err
	}
	*h = value
	return nil
}
