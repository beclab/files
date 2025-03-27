package fileutils

import (
	"errors"
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// CopyDir copies a directory from source to dest and all
// of its sub-directories. It doesn't stop if it finds an error
// during the copy. Returns an error if any.
func CopyDir(fs afero.Fs, source, dest string) error {
	// Get properties of source.
	srcinfo, err := fs.Stat(source)
	if err != nil {
		return err
	}

	// Create the destination directory.
	if err = MkdirAllWithChown(fs, dest, srcinfo.Mode()); err != nil {
		klog.Errorln(err)
		return err
	}

	dir, _ := fs.Open(source)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		fsource := filepath.Join(source, obj.Name())
		fdest := filepath.Join(dest, obj.Name())

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = CopyDir(fs, fsource, fdest)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = CopyFile(fs, fsource, fdest)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	var errString string
	for _, err := range errs {
		errString += err.Error() + "\n"
	}

	if errString != "" {
		return errors.New(errString)
	}

	return nil
}

func extractBaseDirectory(fullPath string) (string, error) {
	if !strings.HasPrefix(fullPath, RootPrefix) {
		fullPath = RootPrefix + fullPath
	}
	if !strings.HasPrefix(fullPath, basePrefix) {
		return "", fmt.Errorf("path %s is not start with %s", fullPath, basePrefix)
	}

	start := len(basePrefix)

	if end := strings.Index(fullPath[start:], "/"); end != -1 {
		return fullPath[:start+end], nil
	}

	return fullPath, nil
}

func checkDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	return nil
}

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
