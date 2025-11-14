package entities

import (
	"encoding/json"
	"encoding/xml"
	"errors"
)

type DownMixStereoAlgorithms int

const (
	// None indicates no special algorithm.
	None DownMixStereoAlgorithms = 0

	// Dave750 algorithm, sourced from https://superuser.com/questions/852400/properly-downmix-5-1-to-stereo-using-ffmpeg/1410620#1410620.
	Dave750 DownMixStereoAlgorithms = 1

	// NightmodeDialogue algorithm, sourced from https://superuser.com/questions/852400/properly-downmix-5-1-to-stereo-using-ffmpeg/1410620#1410620.
	NightmodeDialogue DownMixStereoAlgorithms = 2

	// Rfc7845 algorithm, defined in RFC7845 Section 5.1.1.5.
	Rfc7845 DownMixStereoAlgorithms = 3

	// Ac4 standard algorithm with its default gain values, defined in ETSI TS 103 190 Section 6.2.17.
	Ac4 DownMixStereoAlgorithms = 4
)

var downMixStereoAlgorithmsString = map[DownMixStereoAlgorithms]string{
	None:              "None",
	Dave750:           "Dave750",
	NightmodeDialogue: "NightmodeDialogue",
	Rfc7845:           "Rfc7845",
	Ac4:               "Ac4",
}

var downMixStereoAlgorithmsValue = map[string]DownMixStereoAlgorithms{
	"None":              None,
	"Dave750":           Dave750,
	"NightmodeDialogue": NightmodeDialogue,
	"Rfc7845":           Rfc7845,
	"Ac4":               Ac4,
}

func (d DownMixStereoAlgorithms) String() string {
	if s, ok := downMixStereoAlgorithmsString[d]; ok {
		return s
	}
	return "Unknown"
}

func (d *DownMixStereoAlgorithms) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	if value, ok := downMixStereoAlgorithmsValue[s]; ok {
		*d = value
		return nil
	}

	return errors.New("invalid DownMixStereoAlgorithms value: " + s)
}

func (d *DownMixStereoAlgorithms) MarshalJSON() ([]byte, error) {
	if s, ok := downMixStereoAlgorithmsString[*d]; ok {
		return json.Marshal(s)
	}
	return nil, errors.New("invalid DownMixStereoAlgorithms value")
}

func (d *DownMixStereoAlgorithms) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := dec.DecodeElement(&s, &start); err != nil {
		return err
	}

	if value, ok := downMixStereoAlgorithmsValue[s]; ok {
		*d = value
		return nil
	}

	return errors.New("invalid DownMixStereoAlgorithms value: " + s)
}

func (d *DownMixStereoAlgorithms) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if s, ok := downMixStereoAlgorithmsString[*d]; ok {
		return e.EncodeElement(s, start)
	}
	return errors.New("invalid DownMixStereoAlgorithms value")
}
