package common

import (
	"net/url"
	"regexp"
	"strings"
)

func IsURLEscaped(s string) bool {
	escapePattern := `%[0-9A-Fa-f]{2}`
	re := regexp.MustCompile(escapePattern)

	if re.MatchString(s) {
		decodedStr, err := url.QueryUnescape(s)
		if err != nil {
			return false
		}
		return decodedStr != s
	}
	return false
}

func UnescapeURLIfEscaped(s string) (string, error) {
	var result = s
	var err error
	if IsURLEscaped(s) {
		result, err = url.QueryUnescape(s)
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func EscapeURLWithSpace(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func EscapeAndJoin(input string, delimiter string) string {
	segments := strings.Split(input, delimiter)
	for i, segment := range segments {
		segments[i] = EscapeURLWithSpace(segment)
	}
	return strings.Join(segments, delimiter)
}
