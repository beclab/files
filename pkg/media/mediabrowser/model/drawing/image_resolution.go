package drawing

import (
	//	"encoding/json"
	//	"encoding/xml"
	"fmt"
)

// ImageResolution represents different image resolution options.
type ImageResolution int

const (
	// MatchSource matches the source resolution.
	MatchSource ImageResolution = 0
	// P144 represents 144p resolution.
	P144 ImageResolution = 1
	// P240 represents 240p resolution.
	P240 ImageResolution = 2
	// P360 represents 360p resolution.
	P360 ImageResolution = 3
	// P480 represents 480p resolution.
	P480 ImageResolution = 4
	// P720 represents 720p resolution.
	P720 ImageResolution = 5
	// P1080 represents 1080p resolution.
	P1080 ImageResolution = 6
	// P1440 represents 1440p resolution.
	P1440 ImageResolution = 7
	// P2160 represents 2160p resolution.
	P2160 ImageResolution = 8
)

// String returns the string representation of the ImageResolution.
func (ir ImageResolution) String() string {
	switch ir {
	case MatchSource:
		return "MatchSource"
	case P144:
		return "P144"
	case P240:
		return "P240"
	case P360:
		return "P360"
	case P480:
		return "P480"
	case P720:
		return "P720"
	case P1080:
		return "P1080"
	case P1440:
		return "P1440"
	case P2160:
		return "P2160"
	default:
		return fmt.Sprintf("ImageResolution(%d)", ir)
	}
}

/*
// UnmarshalJSON implements the json.Unmarshaler interface for ImageResolution.
func (ir *ImageResolution) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "MatchSource":
		*ir = MatchSource
	case "P144":
		*ir = P144
	case "P240":
		*ir = P240
	case "P360":
		*ir = P360
	case "P480":
		*ir = P480
	case "P720":
		*ir = P720
	case "P1080":
		*ir = P1080
	case "P1440":
		*ir = P1440
	case "P2160":
		*ir = P2160
	default:
		return fmt.Errorf("invalid ImageResolution value: %s", s)
	}
	return nil
}

// UnmarshalXML implements the xml.Unmarshaler interface for ImageResolution.
func (ir *ImageResolution) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}

	switch s {
	case "MatchSource":
		*ir = MatchSource
	case "P144":
		*ir = P144
	case "P240":
		*ir = P240
	case "P360":
		*ir = P360
	case "P480":
		*ir = P480
	case "P720":
		*ir = P720
	case "P1080":
		*ir = P1080
	case "P1440":
		*ir = P1440
	case "P2160":
		*ir = P2160
	default:
		return fmt.Errorf("invalid ImageResolution value: %s", s)
	}
	return nil
}
*/
