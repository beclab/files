package fileutils

import (
	"fmt"
	"github.com/spf13/afero"
	"io"
	"k8s.io/klog/v2"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

// MoveFile moves file from src to dst.
// By default the rename filesystem system call is used. If src and dst point to different volumes
// the file copy is used as a fallback
func MoveFile(fs afero.Fs, src, dst string) error {
	if fs.Rename(src, dst) == nil {
		return nil
	}
	// fallback
	err := CopyFile(fs, src, dst)
	if err != nil {
		_ = fs.Remove(dst)
		return err
	}
	if err := fs.Remove(src); err != nil {
		return err
	}
	return nil
}

func IoCopyFileWithBufferOs(sourcePath, targetPath string, bufferSize int) error {
	klog.Infoln("***IoCopyFileWithBufferOs")
	klog.Infoln("***sourcePath:", sourcePath)
	klog.Infoln("***targetPath:", targetPath)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	dir := filepath.Dir(targetPath)
	baseName := filepath.Base(targetPath)

	tempFileName := fmt.Sprintf(".uploading_%s", baseName)
	tempFilePath := filepath.Join(dir, tempFileName)
	klog.Infoln("***tempFilePath:", tempFilePath)
	klog.Infoln("***tempFileName:", tempFileName)

	targetFile, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	buf := make([]byte, bufferSize)
	for {
		n, err := sourceFile.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if _, err := targetFile.Write(buf[:n]); err != nil {
			return err
		}
	}

	if err := targetFile.Sync(); err != nil {
		return err
	}
	return os.Rename(tempFilePath, targetPath)
}

func IoCopyFileWithBufferFs(fs afero.Fs, sourcePath, targetPath string, bufferSize int) error {
	klog.Infoln("***IoCopyFileWithBufferFs")
	klog.Infoln("***sourcePath:", sourcePath)
	klog.Infoln("***targetPath:", targetPath)

	sourceFile, err := fs.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	dir := filepath.Dir(targetPath)
	baseName := filepath.Base(targetPath)

	tempFileName := fmt.Sprintf(".uploading_%s", baseName)
	tempFilePath := filepath.Join(dir, tempFileName)
	klog.Infoln("***tempFilePath:", tempFilePath)
	klog.Infoln("***tempFileName:", tempFileName)

	if err = fs.MkdirAll(filepath.Dir(tempFilePath), 0755); err != nil {
		return err
	}
	if err = Chown(fs, filepath.Dir(tempFilePath), 1000, 1000); err != nil {
		klog.Errorf("can't chown directory %s to user %d: %s", filepath.Dir(tempFilePath), 1000, err)
		return err
	}

	targetFile, err := fs.OpenFile(tempFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	buf := make([]byte, bufferSize)
	for {
		n, err := sourceFile.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if _, err := targetFile.Write(buf[:n]); err != nil {
			return err
		}
	}

	if err := targetFile.Sync(); err != nil {
		return err
	}
	return fs.Rename(tempFilePath, targetPath)
}

// CopyFile copies a file from source to dest and returns
// an error if any.
func CopyFile(fs afero.Fs, source, dest string) error {
	err := IoCopyFileWithBufferFs(fs, source, dest, 8*1024*1024)
	if err != nil {
		return err
	}

	// Copy the mode
	info, err := fs.Stat(source)
	if err != nil {
		return err
	}
	err = fs.Chmod(dest, info.Mode())
	if err != nil {
		return err
	}

	return nil
}

// CommonPrefix returns common directory path of provided files
func CommonPrefix(sep byte, paths ...string) string {
	// Handle special cases.
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return path.Clean(paths[0])
	}

	// Note, we treat string as []byte, not []rune as is often
	// done in Go. (And sep as byte, not rune). This is because
	// most/all supported OS' treat paths as string of non-zero
	// bytes. A filename may be displayed as a sequence of Unicode
	// runes (typically encoded as UTF-8) but paths are
	// not required to be valid UTF-8 or in any normalized form
	// (e.g. "é" (U+00C9) and "é" (U+0065,U+0301) are different
	// file names.
	c := []byte(path.Clean(paths[0]))

	// We add a trailing sep to handle the case where the
	// common prefix directory is included in the path list
	// (e.g. /home/user1, /home/user1/foo, /home/user1/bar).
	// path.Clean will have cleaned off trailing / separators with
	// the exception of the root directory, "/" (in which case we
	// make it "//", but this will get fixed up to "/" bellow).
	c = append(c, sep)

	// Ignore the first path since it's already in c
	for _, v := range paths[1:] {
		// Clean up each path before testing it
		v = path.Clean(v) + string(sep)

		// Find the first non-common byte and truncate c
		if len(v) < len(c) {
			c = c[:len(v)]
		}
		for i := 0; i < len(c); i++ {
			if v[i] != c[i] {
				c = c[:i]
				break
			}
		}
	}

	// Remove trailing non-separator characters and the final separator
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == sep {
			c = c[:i]
			break
		}
	}

	return string(c)
}

func Chown(fs afero.Fs, path string, uid, gid int) error {
	start := time.Now()
	klog.Infoln("Function Chown starts at", start)
	defer func() {
		elapsed := time.Since(start)
		klog.Infof("Function Chown execution time: %v\n", elapsed)
	}()

	if fs == nil {
		return os.Chown(path, uid, gid)
	}
	return fs.Chown(path, uid, gid)
}

func GetUID(fs afero.Fs, path string) (int, error) {
	start := time.Now()
	klog.Infoln("Function GetUID starts at", start)
	defer func() {
		elapsed := time.Since(start)
		klog.Infof("Function GetUID execution time: %v\n", elapsed)
	}()

	var fileInfo os.FileInfo
	var err error
	if fs == nil {
		if fileInfo, err = os.Stat(path); err != nil {
			return 0, err
		}
	} else {
		if fileInfo, err = fs.Stat(path); err != nil {
			return 0, err
		}
	}

	statT, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("unable to convert Sys() type to *syscall.Stat_t")
	}

	return int(statT.Uid), nil
}
