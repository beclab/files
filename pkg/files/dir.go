package files

import (
	"os"
	"path/filepath"

	"files/pkg/common"
)

func FindExistingDir(targetDir string) string {
	currentDir := filepath.Clean(targetDir)

	rootStop := common.RootPrefix
	if rootStop == "" {
		rootStop = common.ROOT_PREFIX
	}

	for {
		if dirExists(currentDir) {
			return currentDir
		}

		parentDir := filepath.Dir(currentDir)

		if parentDir == currentDir ||
			parentDir == "/" ||
			parentDir == rootStop ||
			parentDir == common.EXTERNAL_PREFIX ||
			parentDir == common.COMMON_PREFIX ||
			parentDir == common.CACHE_PREFIX {
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
