package files

import (
	"errors"
	"path/filepath"
	"strings"
)

// CleanResourcePath cleans absPath and rejects anything that escapes root.
func CleanResourcePath(root, absPath string) (string, error) {
	cleaned := filepath.Clean(absPath)
	if cleaned != root && !strings.HasPrefix(cleaned, root+"/") {
		return "", errors.New("path escapes resource root")
	}
	return cleaned, nil
}
