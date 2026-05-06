package files

import (
	"os"
	"path/filepath"

	"files/pkg/common"
)

func FindExistingDir(targetDir string) string {
	currentDir := filepath.Clean(targetDir)

	// Stop walking up the chain at "/", at the configurable
	// data root (RootPrefix, default "/data"), or at the External
	// mount point under that root. Previously the comparison was
	// against the literal "/data" / "/data/External", which
	// silently never triggered when ROOT_PREFIX was set to
	// anything else and the loop would walk all the way up.
	rootStop := common.RootPrefix
	externalStop := filepath.Join(common.RootPrefix, "External")

	for {
		if dirExists(currentDir) {
			return currentDir
		}

		parentDir := filepath.Dir(currentDir)

		if parentDir == currentDir || parentDir == "/" || parentDir == rootStop || parentDir == externalStop {
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
