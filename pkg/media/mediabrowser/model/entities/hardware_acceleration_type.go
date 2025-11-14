package entities

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type HardwareAccelerationType int

const (
	HardwareAccelerationType_None HardwareAccelerationType = iota // 0
	// AMD AMF.
	HardwareAccelerationType_AMF // 1
	// Intel Quick Sync Video.
	HardwareAccelerationType_QSV // 2
	// NVIDIA NVENC.
	HardwareAccelerationType_NVENC // 3
	// Video4Linux2 V4L2M2M.
	HardwareAccelerationType_V4L2M2M // 4
	// Video Acceleration API (VAAPI).
	HardwareAccelerationType_VAAPI // 5
	// Video ToolBox.
	HardwareAccelerationType_VideoToolbox // 6
	// Rockchip Media Process Platform (RKMPP).
	HardwareAccelerationType_RKMPP // 7
)

// String converts the HardwareAccelerationType to its string representation.
func (h HardwareAccelerationType) String() string {
	names := [...]string{"none", "amf", "qsv", "nvenc", "v4l2m2m", "vaapi", "videotoolbox", "rkmpp"}
	if h < 0 || int(h) >= len(names) {
		return fmt.Sprintf("invalid(%d)", h)
	}
	return names[h]
}

// MarshalJSON serializes a HardwareAccelerationType to JSON.
func (h *HardwareAccelerationType) MarshalJSON() ([]byte, error) {
	str := h.String()
	if strings.HasPrefix(str, "invalid") {
		return nil, fmt.Errorf("invalid HardwareAccelerationType: %d", *h)
	}
	return json.Marshal(str)
}

// UnmarshalJSON parses a JSON string into a HardwareAccelerationType.
func (h *HardwareAccelerationType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "none":
		*h = HardwareAccelerationType_None
	case "amf":
		*h = HardwareAccelerationType_AMF
	case "qsv":
		*h = HardwareAccelerationType_QSV
	case "nvenc":
		*h = HardwareAccelerationType_NVENC
	case "v4l2m2m":
		*h = HardwareAccelerationType_V4L2M2M
	case "vaapi":
		*h = HardwareAccelerationType_VAAPI
	case "videotoolbox":
		*h = HardwareAccelerationType_VideoToolbox
	case "rkmpp":
		*h = HardwareAccelerationType_RKMPP
	default:
		return fmt.Errorf("invalid HardwareAccelerationType: %s", str)
	}

	return nil
}

// MarshalXML serializes a HardwareAccelerationType to XML.
func (h *HardwareAccelerationType) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	str := h.String()
	if strings.HasPrefix(str, "invalid") {
		return fmt.Errorf("invalid HardwareAccelerationType: %d", *h)
	}
	return e.EncodeElement(str, start)
}

// UnmarshalXML parses an XML element into a HardwareAccelerationType.
func (h *HardwareAccelerationType) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var str string
	if err := dec.DecodeElement(&str, &start); err != nil {
		return err
	}

	// Convert the string to lowercase for case-insensitive matching
	str = strings.ToLower(str)

	switch str {
	case "none":
		*h = HardwareAccelerationType_None
	case "amf":
		*h = HardwareAccelerationType_AMF
	case "qsv":
		*h = HardwareAccelerationType_QSV
	case "nvenc":
		*h = HardwareAccelerationType_NVENC
	case "v4l2m2m":
		*h = HardwareAccelerationType_V4L2M2M
	case "vaapi":
		*h = HardwareAccelerationType_VAAPI
	case "videotoolbox":
		*h = HardwareAccelerationType_VideoToolbox
	case "rkmpp":
		*h = HardwareAccelerationType_RKMPP
	default:
		return fmt.Errorf("invalid HardwareAccelerationType: %s", str)
	}

	return nil
}
