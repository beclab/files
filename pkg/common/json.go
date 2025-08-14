package common

import "encoding/json"

func ToJson(v any) string {
	r, _ := json.Marshal(v)
	return string(r)
}

func ToBytes(v any) []byte {
	r, _ := json.Marshal(v)
	return r
}
