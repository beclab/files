package probing

import (
	"strconv"
	"strings"
	"time"
)

func NormalizeFFProbeResult(result *InternalMediaInfoResult) {
	if result == nil {
		panic("result must not be nil")
	}

	if result.Format != nil && result.Format.Tags != nil {
		result.Format.Tags = ConvertDictionaryToCaseInsensitive(result.Format.Tags)
	}

	if result.Streams != nil {
		for _, stream := range result.Streams {
			if stream.Tags != nil {
				stream.Tags = ConvertDictionaryToCaseInsensitive(stream.Tags)
			}
		}
	}
}

func GetDictionaryNumericValue(tags map[string]string, key string) *int {
	if val, ok := tags[key]; ok {
		i, err := strconv.Atoi(val)
		if err == nil {
			return &i
		}
	}
	return nil
}

func GetDictionaryDateTime(tags map[string]string, key string) *time.Time {
	if val, ok := tags[key]; ok {
		t, err := time.Parse(time.RFC3339, val)
		if err == nil {
			return &t
		}
		t, err = time.Parse("2006", val)
		if err == nil {
			return &t
		}
	}
	return nil
}

func ConvertDictionaryToCaseInsensitive(dict map[string]string) map[string]string {
	result := make(map[string]string, len(dict))
	for k, v := range dict {
		result[strings.ToLower(k)] = v
	}
	return result
}
