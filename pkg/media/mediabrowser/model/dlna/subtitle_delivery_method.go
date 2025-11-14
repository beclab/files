package dlna

import (
	"fmt"
)

type SubtitleDeliveryMethod int

const (
	Encode SubtitleDeliveryMethod = iota
	Embed
	External
	Hls
	Drop
)

var subtitleDeliveryMethodFromString = map[string]SubtitleDeliveryMethod{
	"Encode":   Encode,
	"Embed":    Embed,
	"External": External,
	"Hls":      Hls,
	"Drop":     Drop,
}

func ParseSubtitleDeliveryMethod(s string) (SubtitleDeliveryMethod, error) {
	fmt.Println("9528 ..................... ", s, "aaaaaaaaaaaaaaaaaa")
	if method, ok := subtitleDeliveryMethodFromString[s]; ok {
		return method, nil
	}
	return 0, fmt.Errorf("invalid subtitle delivery method: %s", s)
}
