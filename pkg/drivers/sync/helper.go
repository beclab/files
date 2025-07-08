package sync

import (
	"strings"
)

func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	parts := make([][]string, len(paths))
	var minLen int
	for i, p := range paths {
		trimmed := strings.Trim(p, "/")
		if trimmed == "" {
			parts[i] = []string{}
		} else {
			parts[i] = strings.Split(trimmed, "/")
		}
		if i == 0 || len(parts[i]) < minLen {
			minLen = len(parts[i])
		}
	}

	var common []string
	for i := 0; i < minLen; i++ {
		seg := parts[0][i]
		same := true
		for j := 1; j < len(parts); j++ {
			if parts[j][i] != seg {
				same = false
				break
			}
		}
		if !same {
			break
		}
		common = append(common, seg)
	}

	if len(common) == 0 {
		return ""
	}
	return "/" + strings.Join(common, "/") + "/"
}
