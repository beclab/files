package rpc

import (
	"fmt"
	"os"
	"strings"
)

func isInvalidDir(dirPath string) bool {
	fileInfo, err := os.Stat(dirPath)

	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("directory %s doesn't exist\n", dirPath)
		} else {
			fmt.Printf("cannot access directory %s：%v\n", dirPath, err)
		}
		return true
	}

	if fileInfo.IsDir() {
		fmt.Printf("%s is a directory\n", dirPath)
		return false
	} else {
		fmt.Printf("%s is not a directory\n", dirPath)
		return true
	}
}

func isSubdir(subPath string, path string) bool {
	return strings.HasPrefix(subPath, path) && strings.HasPrefix(strings.TrimPrefix(subPath, path), "/")
}

func dedupArray(paths []string, prefix string) []string {
	uniqueMap := make(map[string]bool)
	for _, path := range paths {
		if isInvalidDir(path) {
			continue
		}

		if prefix != "" && !isSubdir(path, prefix) {
			continue
		}
		uniqueMap[path] = true
	}

	uniquePaths := make([]string, 0, len(uniqueMap))
	for path := range uniqueMap {
		uniquePaths = append(uniquePaths, path)
	}

	var result []string
	for _, subPath := range uniquePaths {
		nodup := true
		for _, path := range uniquePaths {
			if (path != subPath) && isSubdir(subPath, path) {
				nodup = false
				break
			}
		}
		if nodup {
			result = append(result, subPath)
		}
	}
	return result
}
