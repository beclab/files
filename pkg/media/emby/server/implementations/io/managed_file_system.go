package implementations

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	ioo "files/pkg/media/mediabrowser/model/io"
	"k8s.io/klog/v2"
)

type ManagedFileSystem struct {
	//    logger           *log.Logger
	shortcutHandlers []ioo.ShortcutHandler
	tempPath         string
}

var invalidPathCharacters = []rune{
	'"', '<', '>', '|', '\x00',
	'\x01', '\x02', '\x03', '\x04', '\x05', '\x06', '\x07', '\x08', '\x09', '\x0a',
	'\x0b', '\x0c', '\x0d', '\x0e', '\x0f', '\x10', '\x11', '\x12', '\x13', '\x14',
	'\x15', '\x16', '\x17', '\x18', '\x19', '\x1a', '\x1b', '\x1c', '\x1d', '\x1e',
	'\x1f', ':', '*', '?', '\\', '/',
}

func NewManagedFileSystem( /*logger *log.Logger, shortcutHandlers []ShortcutHandler, */ tempPath string) *ManagedFileSystem {
	return &ManagedFileSystem{
		//        logger:           logger,
		//        shortcutHandlers: shortcutHandlers,
		tempPath: tempPath,
	}
}

func (fs *ManagedFileSystem) IsEnvironmentCaseInsensitive() bool {
	return runtime.GOOS == "windows"
}

func (fs *ManagedFileSystem) ContainsInvalidPathCharacters(path string) bool {
	for _, c := range path {
		for _, invalid := range invalidPathCharacters {
			if c == invalid {
				return true
			}
		}
	}
	return false
}

/*
func (fs *ManagedFileSystem) HandleShortcut(path string) (string, error) {
    for _, handler := range fs.shortcutHandlers {
        resolved, err := handler.HandleShortcut(path)
        if err == nil {
            return resolved, nil
        }
    }
    return path, nil
}
*/

func (fs *ManagedFileSystem) GetTempPath() string {
	return fs.tempPath
}

/*
func (fs *ManagedFileSystem) GetFileSystemInfo(path string) (FileSystemMetadata, error) {
    // Take a guess to try and avoid two file system hits, but we'll double-check by calling Exists
    if filepath.Ext(path) != "" {
        fileInfo, err := os.Stat(path)
        if err != nil {
            if os.IsNotExist(err) {
                // Try to get directory info instead
                dirInfo, err := os.Stat(path)
                if err != nil {
                    return FileSystemMetadata{}, err
                }
                return fs.GetFileSystemMetadata(dirInfo), nil
            }
            return FileSystemMetadata{}, err
        }
        return fs.GetFileSystemMetadata(fileInfo), nil
    } else {
        dirInfo, err := os.Stat(path)
        if err != nil {
            if os.IsNotExist(err) {
                // Try to get file info instead
                fileInfo, err := os.Stat(path)
                if err != nil {
                    return FileSystemMetadata{}, err
                }
                return fs.GetFileSystemMetadata(fileInfo), nil
            }
            return FileSystemMetadata{}, err
        }
        return fs.GetFileSystemMetadata(dirInfo), nil
    }
}

/*
func (fs *ManagedFileSystem) GetFileSystemMetadata(info os.DirEntry) FileSystemMetadata {
    isDir := info.IsDir()
    size := int64(0)
    if !isDir {
        fileInfo, _ := info.Info()
        size = fileInfo.Size()
    }
    return FileSystemMetadata{
        Path:     info.Name(),
        IsDir:    isDir,
        Size:     size,
        LastModified: info.ModTime(),
    }
}
*/

func (fs *ManagedFileSystem) GetFiles(path string, recursive bool) ([]ioo.FileSystemMetadata, error) {
	return fs.GetFilesWithFilter(path, []string{}, false, recursive)
}

func (fs *ManagedFileSystem) GetFilesWithFilter(path string, extensions []string, enableCaseSensitiveExtensions bool, recursive bool) ([]ioo.FileSystemMetadata, error) {
	var files []ioo.FileSystemMetadata
	err := filepath.WalkDir(path, func(p string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if !recursive && p != path {
				return filepath.SkipDir
			}
			return nil
		}

		if len(extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(p))
			if !containsString(extensions, ext[1:], enableCaseSensitiveExtensions) {
				return nil
			}
		}

		files = append(files, ioo.FileSystemMetadata{
			Name:        info.Name(),
			IsDirectory: info.IsDir(),
		})
		return nil
	})
	if err != nil {
		klog.Infof("Error walking the path %s: %v\n", path, err)
	}
	return files, nil
}

func containsString(slice []string, str string, caseSensitive bool) bool {
	for _, s := range slice {
		if caseSensitive {
			if s == str {
				return true
			}
		} else {
			if strings.ToLower(s) == strings.ToLower(str) {
				return true
			}
		}
	}
	return false
}

func (fs *ManagedFileSystem) GetLastWriteTimeUtc(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		//h.logger.LogError(err, "Error getting FileSystemInfo for %s", path)
		klog.Errorf("Error getting FileSystemInfo for %s", path)
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

/*
func (fs *ManagedFileSystem) GetLastWriteTimeUtc2(info FileSystemInfo) time.Time {
    // This could throw an error on some file systems that have dates out of range
    defer func() {
        if r := recover(); r != nil {
            //fs.logger.LogError(fmt.Errorf("%v", r), "Error determining LastAccessTimeUtc for %s", info.FullName)
            fmt.Errorf("%v", r)
	    fmt.Errorf("Error determining LastAccessTimeUtc for %s", info.FullName)
        }
    }()
    return info.LastWriteTimeUtc
}
*/

/*
	func (fs *ManagedFileSystem) GetFileSystemMetadata(info os.DirEntry) FileSystemMetadata {
		result := FileSystemMetadata{
			FullName: info.Name(),
			Name:     filepath.Base(info.Name()),
			Extension: filepath.Ext(info.Name()),
		}

		stat, err := info.Info()
		if err != nil {
			return result
		}

		result.Exists = true
		result.IsDirectory = info.IsDir()

		if !result.IsDirectory {
			result.IsHidden = (stat.Mode() & os.ModeHidden) == os.ModeHidden
			result.Length = stat.Size()
		}

		result.CreationTimeUtc = stat.ModTime()
		result.LastWriteTimeUtc = stat.ModTime()

		return result
	}
*/
func (m *ManagedFileSystem) IsPathFile(path string) bool {
	if strings.Contains(strings.ToLower(path), "://") && !strings.HasPrefix(strings.ToLower(path), "file://") {
		return false
	}
	return true
}

func (m *ManagedFileSystem) DeleteFile(path string) error {
	if err := m.SetAttributes(path, false, false); err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	return nil
}

func (m *ManagedFileSystem) SetAttributes(path string, isHidden, readOnly bool) error {
	if !m.isWindows() {
		return nil
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	if (!readOnly == (fileInfo.Mode()&0400 == 0400)) &&
		(!isHidden == (fileInfo.Mode()&0200 == 0200)) {
		return nil
	}

	mode := fileInfo.Mode()
	if readOnly {
		mode |= 0400
	} else {
		mode &^= 0400
	}

	if isHidden {
		mode |= 0200
	} else {
		mode &^= 0200
	}

	return os.Chmod(path, mode)
}

func (m *ManagedFileSystem) isWindows() bool {
	return runtime.GOOS == "windows"
}

func (m *ManagedFileSystem) GetFilePaths(path string, recursive bool) ([]string, error) {
	return m.GetFilePaths2(path, nil, false, recursive)
}

func (m *ManagedFileSystem) GetFilePaths2(path string, extensions []string, enableCaseSensitiveExtensions bool, recursive bool) ([]string, error) {
	var result []string

	var walkFunc filepath.WalkFunc
	if recursive {
		walkFunc = func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				result = append(result, filePath)
			}
			return nil
		}
	} else {
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !file.IsDir() {
				result = append(result, filepath.Join(path, file.Name()))
			}
		}
	}

	if recursive {
		err := filepath.Walk(path, walkFunc)
		if err != nil {
			return nil, err
		}
	}

	if extensions != nil && len(extensions) > 0 {
		result = filterFilesByExtensions(result, extensions, enableCaseSensitiveExtensions)
	}

	return result, nil
}

func filterFilesByExtensions(files []string, extensions []string, caseSensitive bool) []string {
	var filteredFiles []string

	for _, file := range files {
		ext := filepath.Ext(file)
		if ext == "" {
			continue
		}

		found := false
		for _, extension := range extensions {
			if caseSensitive {
				if ext == extension {
					found = true
					break
				}
			} else {
				if strings.EqualFold(ext, extension) {
					found = true
					break
				}
			}
		}

		if found {
			filteredFiles = append(filteredFiles, file)
		}
	}

	return filteredFiles
}

/*
func main() {
	path := "/path/to/directory"
	extensions := []string{".txt", ".go"}
	enableCaseSensitiveExtensions := false
	recursive := true

	files, err := GetFilePaths(path, extensions, enableCaseSensitiveExtensions, recursive)
	if err != nil {
		klog.Infoln("Error:", err)
		return
	}

	for _, file := range files {
		klog.Infoln(file)
	}
}
*/

/*
func (c *ManagedFileSystem) GetFilePaths2(path string, extensions []string, enableCaseSensitiveExtensions bool, recursive bool) ([]string, error) {
    opts := c.getEnumerationOptions(recursive)

    // On Linux and macOS, the search pattern is case-sensitive.
    // If we're OK with case-sensitivity, and we're only filtering for one extension, then use the native method.
    if (enableCaseSensitiveExtensions || c.isEnvironmentCaseInsensitive) && len(extensions) == 1 {
        return c.getFilePathsWithSingleExtension(path, extensions[0], opts)
    }

    var files []string
    err := c.getFilePathsWithFiltering(path, extensions, opts, &files)
    return files, err
}

func (m *ManagedFileSystem) getEnumerationOptions(recursive bool) *fs.DirEntryOptions {
    return &fs.DirEntryOptions{
        Recursive: recursive,
    }
}

func (m *ManagedFileSystem) getFilePathsWithSingleExtension(path, ext string, opts *fs.DirEntryOptions) ([]string, error) {
    var files []string
    entries, err := os.ReadDir(path, opts)
    if err != nil {
        return nil, err
    }

    for _, entry := range entries {
        if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
            files = append(files, filepath.Join(path, entry.Name()))
        }
    }

    return files, nil
}

func (m *ManagedFileSystem) getFilePathsWithFiltering(path string, extensions []string, opts *fs.DirEntryOptions, files *[]string) error {
    entries, err := os.ReadDir(path, opts)
    if err != nil {
        return err
    }

    for _, entry := range entries {
        if !entry.IsDir() {
            if len(extensions) == 0 || m.matchesExtension(entry.Name(), extensions) {
                *files = append(*files, filepath.Join(path, entry.Name()))
            }
        }
    }

    return nil
}

func (m *ManagedFileSystem) matchesExtension(filename string, extensions []string) bool {
    ext := filepath.Ext(filename)
    if ext == "" {
        return false
    }

    for _, e := range extensions {
        if strings.EqualFold(ext[1:], e) {
            return true
        }
    }

    return false
}
*/

func (fs *ManagedFileSystem) DirectoryExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return true
	}
	return true
}

func (fs *ManagedFileSystem) FileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return true
	}
	return true
}
