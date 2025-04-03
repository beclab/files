package drives

import (
	"bytes"
	"errors"
	"files/pkg/common"
	"files/pkg/diskcache"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/img"
	"files/pkg/preview"
	"files/pkg/redisutils"
	"fmt"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func PasteAddVersionSuffix(source string, dstType string, isDir bool, fs afero.Fs, w http.ResponseWriter, r *http.Request) string {
	if strings.HasSuffix(source, "/") {
		source = strings.TrimSuffix(source, "/")
	}

	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	renamed := ""

	var err error
	handler, err := GetResourceService(dstType)
	if err != nil {
		return ""
	}

	for {
		statSource := source
		if isDir {
			statSource += "/"
		}
		if _, _, _, _, err = handler.GetStat(fs, statSource, w, r); err != nil {
			break
		}
		if !isDir {
			renamed = fmt.Sprintf("%s(%d)%s", base, counter, ext)
		} else {
			renamed = fmt.Sprintf("%s(%d)", name, counter)
		}
		source = path.Join(dir, renamed)
		counter++
	}

	if isDir {
		source += "/"
	}

	return source
}

func CheckBufferDiskSpace(diskSize int64) (bool, error) {
	spaceOk, needs, avails, reserved, err := common.CheckDiskSpace("/data", diskSize)
	if err != nil {
		return false, err
	}
	needsStr := common.FormatBytes(needs)
	availsStr := common.FormatBytes(avails)
	reservedStr := common.FormatBytes(reserved)
	if spaceOk {
		return true, nil
	} else {
		errorMessage := fmt.Sprintf("Insufficient disk space available. This file still requires: %s, but only %s is available (with an additional %s reserved for the system).",
			needsStr, availsStr, reservedStr)
		return false, errors.New(errorMessage)
	}
}

func GenerateBufferFileName(originalFilePath, bflName string, extRemains bool) (string, error) {
	timestamp := time.Now().Unix()

	extension := filepath.Ext(originalFilePath)

	originalFileName := strings.TrimSuffix(filepath.Base(originalFilePath), extension)

	var bufferFileName string
	var bufferFolderPath string
	if extRemains {
		bufferFileName = originalFileName + extension
		bufferFolderPath = "/data/" + bflName + "/buffer/" + fmt.Sprintf("%d", timestamp)
	} else {
		bufferFileName = fmt.Sprintf("%d_%s.bin", timestamp, originalFileName)
		bufferFolderPath = "/data/" + bflName + "/buffer"
	}

	if err := fileutils.MkdirAllWithChown(nil, bufferFolderPath, 0755); err != nil {
		klog.Errorln(err)
		return "", err
	}
	bufferFilePath := filepath.Join(bufferFolderPath, bufferFileName)

	return bufferFilePath, nil
}

func GenerateBufferFolder(originalFilePath, bflName string) (string, error) {
	timestamp := time.Now().Unix()

	rand.Seed(time.Now().UnixNano())
	randomNumber := rand.Intn(10000000000)
	randomNumberString := fmt.Sprintf("%010d", randomNumber)

	timestampPlus := fmt.Sprintf("%d%s", timestamp, randomNumberString)

	originalPathName := filepath.Base(strings.TrimSuffix(originalFilePath, "/"))
	extension := filepath.Ext(originalPathName)
	if len(extension) > 0 {
		originalPathName = strings.TrimSuffix(originalPathName, extension) + "_" + extension[1:]
	}

	bufferPathName := fmt.Sprintf("%s_%s", timestampPlus, originalPathName) // as parent folder
	bufferPathName = common.RemoveSlash(bufferPathName)
	bufferFolderPath := "/data/" + bflName + "/buffer/" + bufferPathName
	if err := fileutils.MkdirAllWithChown(nil, bufferFolderPath, 0755); err != nil {
		klog.Errorln(err)
		return "", err
	}
	return bufferFolderPath, nil
}

func MakeDiskBuffer(filePath string, bufferSize int64, delete bool) error {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		klog.Errorln("Failed to create buffer file:", err)
		return err
	}
	defer file.Close()

	if err = file.Truncate(bufferSize); err != nil {
		klog.Errorln("Failed to truncate buffer file:", err)
		return err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		klog.Errorln("Failed to get buffer file info:", err)
		return err
	}
	klog.Infoln("Buffer file size:", fileInfo.Size(), "bytes")

	if delete {
		err = os.Remove(filePath)
		if err != nil {
			klog.Errorf("Error removing test buffer: %v\n", err)
			return err
		}

		klog.Infoln("Test buffer removed successfully")
	}
	return nil
}

func RemoveDiskBuffer(filePath string, srcType string) {
	klog.Infoln("Removing buffer file:", filePath)
	err := os.Remove(filePath)
	if err != nil {
		klog.Errorln("Failed to delete buffer file:", err)
		return
	}
	if IsThridPartyDrives(srcType) {
		dir := filepath.Dir(filePath)
		err = os.Remove(dir)
		if err != nil {
			klog.Errorln("Failed to delete buffer file dir:", err)
			return
		}
	}

	klog.Infoln("Buffer file deleted.")
}

func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return path.Clean(name)
}

func parseQueryFiles(r *http.Request, f *files.FileInfo) ([]string, error) {
	var fileSlice []string
	names := strings.Split(r.URL.Query().Get("files"), ",")

	if len(names) == 0 {
		fileSlice = append(fileSlice, f.Path)
	} else {
		for _, name := range names {
			name, err := url.QueryUnescape(strings.Replace(name, "+", "%2B", -1))
			if err != nil {
				return nil, err
			}

			name = slashClean(name)
			fileSlice = append(fileSlice, filepath.Join(f.Path, name))
		}
	}

	return fileSlice, nil
}

func parseQueryAlgorithm(r *http.Request) (string, archiver.Writer, error) {
	switch r.URL.Query().Get("algo") {
	case "zip", "true", "":
		return ".zip", archiver.NewZip(), nil
	case "tar":
		return ".tar", archiver.NewTar(), nil
	case "targz":
		return ".tar.gz", archiver.NewTarGz(), nil
	case "tarbz2":
		return ".tar.bz2", archiver.NewTarBz2(), nil
	case "tarxz":
		return ".tar.xz", archiver.NewTarXz(), nil
	case "tarlz4":
		return ".tar.lz4", archiver.NewTarLz4(), nil
	case "tarsz":
		return ".tar.sz", archiver.NewTarSz(), nil
	default:
		return "", nil, errors.New("format not implemented")
	}
}

func SetContentDisposition(w http.ResponseWriter, r *http.Request, file *files.FileInfo) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(file.Name))
	}
}

func AddFile(ar archiver.Writer, d *common.Data, path, commonPath string) error {
	info, err := files.DefaultFs.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil
	}

	file, err := files.DefaultFs.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if path != commonPath {
		filename := strings.TrimPrefix(path, commonPath)
		filename = strings.TrimPrefix(filename, string(filepath.Separator))
		err = ar.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   info,
				CustomName: filename,
			},
			ReadCloser: file,
		})
		if err != nil {
			return err
		}
	}

	if info.IsDir() {
		names, err := file.Readdirnames(0)
		if err != nil {
			return err
		}

		for _, name := range names {
			fPath := filepath.Join(path, name)
			err = AddFile(ar, d, fPath, commonPath)
			if err != nil {
				klog.Errorf("Failed to archive %s: %v", fPath, err)
			}
		}
	}

	return nil
}

func RawDirHandler(w http.ResponseWriter, r *http.Request, d *common.Data, file *files.FileInfo) (int, error) {
	filenames, err := parseQueryFiles(r, file)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	extension, ar, err := parseQueryAlgorithm(r)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	err = ar.Create(w)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer ar.Close()

	commonDir := fileutils.CommonPrefix(filepath.Separator, filenames...)

	name := filepath.Base(commonDir)
	if name == "." || name == "" || name == string(filepath.Separator) {
		name = file.Name
	}
	// Prefix used to distinguish a filelist generated
	// archive from the full directory archive
	if len(filenames) > 1 {
		name = "_" + name
	}
	name += extension
	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))

	for _, fname := range filenames {
		err = AddFile(ar, d, fname, commonDir)
		if err != nil {
			klog.Errorf("Failed to archive %s: %v", fname, err)
		}
	}

	return 0, nil
}

func RawFileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	SetContentDisposition(w, r, file)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}

func HandleImagePreview(
	w http.ResponseWriter,
	r *http.Request,
	imgSvc preview.ImgService,
	fileCache fileutils.FileCache,
	file *files.FileInfo,
	previewSize preview.PreviewSize,
	enableThumbnails, resizePreview bool,
) (int, error) {
	if (previewSize == preview.PreviewSizeBig && !resizePreview) ||
		(previewSize == preview.PreviewSizeThumb && !enableThumbnails) {
		return RawFileHandler(w, r, file)
	}

	format, err := imgSvc.FormatFromExtension(file.Extension)
	// Unsupported extensions directly return the raw data
	if err == img.ErrUnsupportedFormat || format == img.FormatGif {
		return RawFileHandler(w, r, file)
	}
	if err != nil {
		return common.ErrToStatus(err), err
	}

	cacheKey := preview.PreviewCacheKey(file, previewSize)
	klog.Infoln("cacheKey:", cacheKey)
	klog.Infoln("f.RealPath:", file.RealPath())
	resizedImage, ok, err := fileCache.Load(r.Context(), cacheKey)
	if err != nil {
		return common.ErrToStatus(err), err
	}
	if !ok {
		resizedImage, err = preview.CreatePreview(imgSvc, fileCache, file, previewSize, 1)
		if err != nil {
			klog.Infoln("first method failed!")
			resizedImage, err = preview.CreatePreview(imgSvc, fileCache, file, previewSize, 2)
			if err != nil {
				klog.Infoln("second method failed!")
				return RawFileHandler(w, r, file)
			}
		}
	}

	if diskcache.CacheDir != "" {
		redisutils.UpdateFileAccessTimeToRedis(redisutils.GetFileName(cacheKey))
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, bytes.NewReader(resizedImage))

	return 0, nil
}

func ParsePathType(path string, r *http.Request, isDst, rewritten bool) (string, error) {
	if path == "" && !isDst {
		path = r.URL.Path
	}
	if path == "" {
		return "", errors.New("path is empty")
	}

	pathSplit := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(pathSplit) < 2 {
		return "", errors.New("invalid path type")
	}

	switch strings.ToLower(pathSplit[0]) {
	case "drive": // "Drive" and "drive" both are OK, for compatible
		if ValidSrcTypes[pathSplit[1]] {
			return pathSplit[1], nil
		}
		return "", errors.New("invalid path type")
	case "sync":
		return SrcTypeSync, nil
	case "appdata": // AppData
		return SrcTypeCache, nil
	case "application": // Application
		if !rewritten {
			return SrcTypeData, nil
		}
	case "home": // Home
		if !rewritten {
			return SrcTypeDrive, nil
		}
	case "external": // External
		return SrcTypeExternal, nil
	}

	if rewritten {
		switch pathSplit[1] {
		case "Data":
			return SrcTypeData, nil
		case "Home":
			return SrcTypeDrive, nil
		}
	}

	// use src/src_type/dst_type for the last try and compatible
	if isDst {
		if ValidSrcTypes[r.URL.Query().Get("dst_type")] {
			return r.URL.Query().Get("dst_type"), nil
		}
	}
	if ValidSrcTypes[r.URL.Query().Get("src")] {
		return r.URL.Query().Get("src"), nil
	}
	if ValidSrcTypes[r.URL.Query().Get("src_type")] {
		return r.URL.Query().Get("src_type"), nil
	}
	return "", errors.New("invalid path type")
}
