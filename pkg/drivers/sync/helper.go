package sync

import "strings"

func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	splitPaths := make([][]string, len(paths))
	for i, p := range paths {
		trimmed := strings.Trim(p, "/")
		if trimmed == "" {
			splitPaths[i] = []string{}
		} else {
			splitPaths[i] = strings.Split(trimmed, "/")
		}
	}

	var common []string
	for idx := 0; ; idx++ {
		if idx >= len(splitPaths[0]) {
			break
		}
		seg := splitPaths[0][idx]
		for j := 1; j < len(splitPaths); j++ {
			if idx >= len(splitPaths[j]) || splitPaths[j][idx] != seg {
				goto DONE
			}
		}
		common = append(common, seg)
	}
DONE:
	if len(common) == 0 {
		return "/"
	}
	return "/" + strings.Join(common, "/") + "/"
}
