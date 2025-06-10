package utils

import "encoding/json"

func ToJson(v any) string {
	r, _ := json.Marshal(v)
	return string(r)
}
