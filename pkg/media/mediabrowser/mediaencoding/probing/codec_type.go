package probing

import (
	"encoding/json"
	"fmt"
)

type CodecType int

const (
	Video CodecType = iota
	Audio
	Data
	Subtitle
	Attachment
)

func (c CodecType) String() string {
	switch c {
	case Video:
		return "Video"
	case Audio:
		return "Audio"
	case Data:
		return "Data"
	case Subtitle:
		return "Subtitle"
	case Attachment:
		return "Attachment"
	default:
		return "Unknown"
	}
}

func (c *CodecType) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch v.(type) {
	case string:
		switch v.(string) {
		case "video":
			*c = Video
		case "audio":
			*c = Audio
		case "data":
			*c = Data
		case "subtitle":
			*c = Subtitle
		case "attachment":
			*c = Attachment
		default:
			return fmt.Errorf("invalid CodecType: %s", v.(string))
		}
	default:
		return fmt.Errorf("invalid CodecType type: %T", v)
	}
	return nil
}
