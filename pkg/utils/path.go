package utils

import (
	"os"
	"path/filepath"
)

func JoinWithSlash(elem ...string) string {
	p := filepath.Join(elem...)
	if p != string(os.PathSeparator) {
		p += string(os.PathSeparator)
	}
	return p
}
