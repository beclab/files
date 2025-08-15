package files

import (
	"crypto/rand"
	"encoding/base64"
	e "errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

type PathMeta struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Ext   string `json:"ext"`
	Size  int64  `json:"size"`
	Nums  int64  `json:"nums"`
	IsDir bool   `json:"isDir"`
}

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

func MoveFileOs(src, dst string) error {
	if os.Rename(src, dst) == nil {
		return nil
	}

	// fallback
	err := CopyFileOs(src, dst)
	if err != nil {
		_ = os.Remove(dst)
		return err
	}
	if err := os.Remove(src); err != nil {
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

	if err = MkdirAllWithChown(nil, dir, 0755); err != nil {
		klog.Errorln(err)
		return err
	}

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

	uid, err := GetUID(nil, dir)
	if err != nil {
		return err
	}
	if err = Chown(nil, tempFilePath, uid, uid); err != nil {
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

	if err = MkdirAllWithChown(fs, filepath.Dir(tempFilePath), 0755); err != nil {
		klog.Errorln(err)
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

	uid, err := GetUID(fs, dir)
	if err != nil {
		return err
	}
	if err = Chown(fs, tempFilePath, uid, uid); err != nil {
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

func CopyFileOs(source, dest string) error {
	err := IoCopyFileWithBufferOs(source, dest, 8*1024*1024)
	if err != nil {
		return err
	}

	// Copy the mode
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	err = os.Chmod(dest, info.Mode())
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

func ChownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if err := Chown(nil, name, uid, gid); err != nil {
			klog.Errorf("Failed to chown %s: %v\n", name, err)
			return err
		}

		klog.Infof("Chowned %s\n", name)
		return nil
	})
}

func Chown(fs afero.Fs, path string, uid, gid int) error {
	start := time.Now()
	klog.Infoln("Function Chown starts at", start)
	defer func() {
		elapsed := time.Since(start)
		klog.Infof("Function Chown execution time: %v\n", elapsed)
	}()

	var err error = nil
	if fs == nil {
		err = os.Chown(path, uid, gid)
	} else {
		err = fs.Chown(path, uid, gid)
	}
	if err != nil {
		klog.Errorf("can't chown directory %s to user %d: %s", path, uid, err)
	}
	return err
}

func createAndChownDir(fs afero.Fs, path string, mode os.FileMode, uid, gid int) error {
	if fs == nil {
		if err := os.Mkdir(path, mode); err != nil {
			return err
		}
	} else {
		if err := fs.Mkdir(path, mode); err != nil {
			return err
		}
	}
	return Chown(fs, path, uid, gid)
}

func MkdirAllWithChown(fs afero.Fs, path string, mode os.FileMode) error {
	if path == "" {
		return nil
	}

	var info os.FileInfo
	var err error
	var uid int
	var subErr error

	parts := strings.Split(path, "/")
	vol := ""
	found := false
	for _, part := range parts {
		if part == "" {
			continue
		}
		vol = filepath.Join(vol, part)

		if fs == nil {
			info, err = os.Stat(vol)
		} else {
			info, err = fs.Stat(vol)
		}
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("path %s is not a directory", vol)
			}
			continue
		}

		if os.IsNotExist(err) {
			if !found {
				if filepath.Dir(vol) == "/" {
					uid = 1000
				} else {
					uid, subErr = GetUID(fs, filepath.Dir(vol))
					if subErr != nil {
						return subErr
					}
				}
				found = true
			}

			if subErr = createAndChownDir(fs, vol, mode, uid, uid); subErr != nil {
				return subErr
			}
		} else {
			return err
		}
	}
	return nil
}

func GetUID(fs afero.Fs, path string) (int, error) {
	if path == "/" {
		return 1000, nil
	}

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

func WriteFile(fs afero.Fs, dst string, in io.Reader) (os.FileInfo, error) {
	klog.Infoln("Before open ", dst)
	dir, _ := path.Split(dst)
	if err := MkdirAllWithChown(fs, dir, 0755); err != nil {
		klog.Errorln(err)
		return nil, err
	}

	klog.Infoln("Open ", dst)
	file, err := fs.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	klog.Infoln("Copy file!")
	_, err = io.Copy(file, in)
	if err != nil {
		return nil, err
	}

	klog.Infoln("Get stat")
	// Gets the info about the file.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	klog.Infoln(info)
	return info, nil
}

// FilePathExists returns a boolean, whether the file or directory is exists
func FilePathExists(name string) bool {
	_, err := os.Stat(name)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func GetSpaceSize(filePath string) (uint64, float64, error) {
	usage, err := disk.Usage(filePath)
	if err != nil {
		return 0, 0, err
	}

	return usage.Free, usage.UsedPercent, nil

}

func GetFileInfo(filePath string) (*PathMeta, error) {
	var meta = new(PathMeta)
	var afs = afero.NewOsFs()

	obj, err := NewFileInfo(FileOptions{
		Fs:      afs,
		Path:    filePath,
		Expand:  false,
		Content: false,
	})
	if err != nil {
		return nil, err
	}

	if !obj.IsDir {
		meta.Name = obj.Name
		meta.Path = obj.Path
		meta.IsDir = false
		meta.Ext = obj.Extension
		meta.Size = obj.Size
		meta.Nums = 1

		return meta, nil
	}

	var fileCount int64
	var totalSize int64
	if err := afero.Walk(afs, filePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
			totalSize = totalSize + info.Size()
		}

		return nil
	}); err != nil {
		return nil, err
	}

	var tmp = strings.TrimSuffix(filePath, "/")
	var pos = strings.LastIndex(tmp, "/")
	var name = strings.Trim(tmp[pos:], "/")

	meta.IsDir = true
	meta.Name = name
	meta.Path = filePath
	meta.Ext = ""
	meta.Nums = fileCount
	meta.Size = totalSize

	return meta, nil
}

func UpdatePathName(oldPath string, newName string, isDir bool) string {
	if isDir {
		var tmp = strings.TrimSuffix(oldPath, "/")
		var pos = strings.LastIndex(tmp, "/")
		var p = tmp[:pos] + "/" + newName + "/"
		return p
	}

	var pos = strings.LastIndex(oldPath, "/")
	var p = oldPath[:pos] + "/" + newName
	return p
}

func GetPathName(p string) string {
	if strings.HasSuffix(p, "/") {
		var tmp = strings.TrimSuffix(p, "/")
		var pos = strings.LastIndex(tmp, "/")
		tmp = p[pos:]
		tmp = strings.Trim(tmp, "/")
		return tmp
	} else {
		var pos = strings.LastIndex(p, "/")
		var tmp = p[pos:]
		tmp = strings.Trim(tmp, "/")
		return tmp
	}
}

func WriteTempFile(dstPath string) error {
	dir := FindExistingDir(dstPath)
	if dir == "" {
		return fmt.Errorf("no writable directory found in path hierarchy of %q", dstPath)
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("failed to generate random bytes: %v", err)
	}
	randomStr := base64.URLEncoding.EncodeToString(randomBytes)[:8]
	filename := fmt.Sprintf("temp_%s_%s.testwriting", timestamp, randomStr)
	filePath := filepath.Join(dir, filename)

	defer func() {
		_ = os.Remove(filePath)
		klog.Infof("Cleaned up temporary file %s", filePath)
	}()

	klog.Infof("Creating temporary file %s", filePath)

	if err := os.WriteFile(filePath, []byte{0}, 0o644); err != nil {
		var pathErr *fs.PathError
		if e.As(err, &pathErr) {
			if pathErr.Err == syscall.EACCES || pathErr.Err == syscall.EPERM {
				return fmt.Errorf("permission denied: failed to create file: %v", err)
			} else if pathErr.Err == syscall.EROFS {
				return fmt.Errorf("read-only file system: failed to create file: %v", err)
			}
		}
		return fmt.Errorf("failed to create file: %v", err)
	}

	return nil
}

//

func CollectDupNames(p string, prefixName string, ext string, isDir bool) ([]string, error) {
	// p = strings.Split(p,"/")[:len(x)-2]
	var result []string
	var afs = afero.NewOsFs()
	entries, err := afero.ReadDir(afs, p)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// if entry.IsDir() != isDir {
		// 	continue
		// }

		infoName := entry.Name()
		if isDir {
			if strings.Contains(infoName, prefixName) {
				result = append(result, infoName)
			}
		} else {
			infoName = strings.TrimSuffix(infoName, ext)
			if strings.Contains(infoName, prefixName) {
				result = append(result, infoName)
			}
		}
	}

	return result, nil
}

func GenerateDupCommonName(existsName []string, prefixName string, existSamePathName string) string {
	if existSamePathName == "" {
		return prefixName
	}
	var filePrefixName = prefixName

	var count = 0
	var matchedCount int

	var searchName = prefixName

	for {
		var find bool
		for _, name := range existsName {
			if strings.TrimSpace(name) == searchName {
				find = true
				break
			}
		}

		if find {
			count++
			searchName = fmt.Sprintf("%s(%d)", prefixName, count)
			continue
		} else {
			matchedCount = count
			break
		}

	}

	var newFileName string
	if matchedCount == 0 {
		newFileName = filePrefixName
	} else {
		newFileName = fmt.Sprintf("%s(%d)", filePrefixName, matchedCount)
	}

	return newFileName
}

func GetFileNameFromPath(s string) (string, bool) {

	var isFile = strings.HasSuffix(s, "/")
	var tmp = strings.TrimSuffix(s, "/")
	var p = strings.LastIndex(tmp, "/")
	var r = tmp[p:]
	r = strings.Trim(r, "/")

	return r, !isFile
}

func GetPrefixPath(s string) string {
	// /a/b/hello.txt   > /a/b/
	// /a/b/c/          > /a/b/
	if s == "/" {
		return s
	}

	var r = strings.TrimSuffix(s, "/")
	var p = strings.LastIndex(r, "/")
	return r[:p+1]
}
