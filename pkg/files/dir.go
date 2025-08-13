package files

import (
	"os"
	"path/filepath"
)

func FindExistingDir(targetDir string) string {
	currentDir := filepath.Clean(targetDir)

	for {
		if dirExists(currentDir) {
			return currentDir
		}

		parentDir := filepath.Dir(currentDir)

		if parentDir == currentDir || parentDir == "/" || parentDir == "/data" || parentDir == "/data/External" {
			break
		}

		currentDir = parentDir
	}

	return ""
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}
