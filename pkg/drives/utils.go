package drives

import (
	"compress/gzip"
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/models"
	"files/pkg/pool"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mholt/archiver/v3"
	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

func PasteAddVersionSuffix(source string, fileParam *models.FileParam, isDir bool, fs afero.Fs, w http.ResponseWriter, r *http.Request) string {
	if strings.HasSuffix(source, "/") {
		source = strings.TrimSuffix(source, "/")
	}

	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	renamed := ""
	bubble := ""
	if fileParam.FileType == SrcTypeSync {
		bubble = " "
	}

	var err error
	handler, err := GetResourceService(fileParam.FileType)
	if err != nil {
		return ""
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return ""
	}

	for {
		//statSource := source
		//if isDir {
		//	statSource += "/"
		//}
		if _, _, _, _, err = handler.GetStat(fs, fileParam, w, r); err != nil {
			break
		}
		if !isDir {
			renamed = fmt.Sprintf("%s%s(%d)%s", base, bubble, counter, ext)
		} else {
			renamed = fmt.Sprintf("%s%s(%d)", name, bubble, counter)
		}
		source = path.Join(dir, renamed)
		fileParam.Path = strings.TrimPrefix(source, uri)
		counter++
	}

	if isDir {
		source += "/"
	}

	fileParam.Path = strings.TrimPrefix(source, uri)

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
		bufferFolderPath = "/data/buffer/" + bflName + "/" + fmt.Sprintf("%d", timestamp)
	} else {
		bufferFileName = fmt.Sprintf("%d_%s.bin", timestamp, originalFileName)
		bufferFolderPath = "/data/buffer/" + bflName
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
	bufferFolderPath := "/data/buffer/" + bflName + "/" + bufferPathName
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

func RemoveDiskBuffer(task *pool.Task, filePath string, srcType string) {
	if task != nil {
		defer task.RemoveBuffer(filePath)
	}
	TaskLog(task, "info", "Removing buffer file:", filePath)

	var err error
	if IsThridPartyDrives(srcType) {
		dir := filepath.Dir(filePath)
		err = os.RemoveAll(dir)
		if err != nil {
			TaskLog(task, "warning", "Failed to delete buffer file dir:", err)
			return
		}
	} else {
		err = os.Remove(filePath)
		if err != nil {
			TaskLog(task, "warning", "Failed to delete buffer file:", err)
			return
		}
	}
	TaskLog(task, "info", fmt.Sprintf("Buffer file %s deleted.", filePath))
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
		if value, exists := ValidSrcTypes[pathSplit[1]]; exists && value {
			return pathSplit[1], nil
		}
		return "", errors.New("invalid path type")
	case "sync":
		return SrcTypeSync, nil
	case "appdata", "cache": // AppData
		return SrcTypeCache, nil
	case "application": // Application
		if !rewritten {
			return SrcTypeData, nil
		}
	case "home": // Home
		if !rewritten {
			return SrcTypeDrive, nil
		}
	case "data":
		if !rewritten {
			return SrcTypeData, nil
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

	if r == nil {
		return "", errors.New("invalid path type")
	}

	// use src/src_type/dst_type for the last try and compatible
	if isDst {
		if value, exists := ValidSrcTypes[r.URL.Query().Get("dst_type")]; exists && value {
			return r.URL.Query().Get("dst_type"), nil
		}
	}
	if value, exists := ValidSrcTypes[r.URL.Query().Get("src")]; exists && value {
		return r.URL.Query().Get("src"), nil
	}
	if value, exists := ValidSrcTypes[r.URL.Query().Get("src_type")]; exists && value {
		return r.URL.Query().Get("src_type"), nil
	}
	return "", errors.New("invalid path type")
}

func callSendS3MultiFiles(fileInfos []os.FileInfo) {
}

func SuitableResponseReader(resp *http.Response) io.ReadCloser {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			klog.Errorf("unzip response failed: %v\n", err)
			return nil
		}
		return &autoCloseReader{
			Reader: gzipReader,
			closer: resp.Body,
		}
	}
	return resp.Body
}

type autoCloseReader struct {
	io.Reader
	closer io.Closer
}

func (a *autoCloseReader) Close() error {
	return a.closer.Close()
}

func TaskCancellable(srcType, dstType string, same bool) bool {
	if srcType == SrcTypeSync && dstType == SrcTypeSync {
		return false
	}
	if IsCloudDrives(srcType) && same {
		return false
	}
	if srcType == SrcTypeGoogle && same {
		return false
	}
	return true
}

func TaskLog(task *pool.Task, level string, args ...interface{}) {
	switch level {
	case "info":
		klog.Infoln(args...)
	case "warning":
		klog.Warningln(args...)
	case "error":
		klog.Errorln(args...)
	default:
		klog.Infoln(args...)
	}

	if task != nil && task.LogChan != nil {
		logMsg := fmt.Sprintln(args...)

		select {
		case <-task.Ctx.Done():
			klog.Warningln("Task context has been cancelled, only logging to klog")
		default:
			switch level {
			case "info", "warning":
				task.Logging(logMsg)
			case "error":
				task.LoggingError(logMsg)
			default:
			}
		}
	}
}

func CalculateProgressRange(task *pool.Task, currentFileSize int64) (left, mid, right int) {
	klog.Infof("taskProgress=%d, totalFileSize=%d, currentFileSize=%d, transferred=%d",
		task.Progress, task.TotalFileSize, currentFileSize, task.Transferred)

	if task.TotalFileSize <= 0 {
		return 0, 0, 0
	}
	if task.Progress >= 99 {
		return 99, 99, 99
	}

	sum := task.Transferred + currentFileSize
	if sum > task.TotalFileSize {
		sum = task.TotalFileSize
	}

	right = int(math.Floor((float64(sum) / float64(task.TotalFileSize)) * 100))

	if right > 99 {
		right = 99
	}

	left = task.Progress
	if left > right {
		left = right
	}

	mid = (left + right) / 2

	return left, mid, right
}

func MapProgress(progress float64, left, right int) int {
	if progress <= 0.0 {
		return left
	}
	if progress >= 100.0 {
		return right
	}

	// Calculate the percentage of progress between 0.0 and 100.0
	percentage := progress / 100.0

	// Map this percentage to the range [left, right]
	formattedProgress := int(float64(left) + percentage*float64(right-left))

	return formattedProgress
}

func RemoveAdditionalHeaders(header *http.Header) {
	header.Del("Traceparent")
	header.Del("Tracestate")
	return
}

func MapProgressByTime(left, right int, size, speed int64, usedTime int) int {
	transferredBytes := int64(usedTime) * speed

	var progressPercentage int64
	if size > 0 {
		progress := transferredBytes * 10000 / size
		progressPercentage = progress / 100 // Keep all calculations in int64
	} else {
		progressPercentage = 0
	}

	if progressPercentage < 0 {
		progressPercentage = 0
	} else if progressPercentage > 100 {
		progressPercentage = 100
	}

	// Convert progressPercentage to int for the final mapping
	mappedProgress := left + (right-left)*int(progressPercentage)/100

	if mappedProgress < left {
		mappedProgress = left
	} else if mappedProgress > right {
		mappedProgress = right
	}

	return mappedProgress
}

// SimulateProgress simulates the progress over time by calling MapProgress every second.
func SimulateProgress(ctx context.Context, left, right int, size, speed int64, task *pool.Task) {
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Simulate progress update
			usedTime := int(time.Now().Sub(startTime).Seconds())
			progress := MapProgressByTime(left, right, size, speed, usedTime)
			if task.Status == "running" {
				task.Progress = progress
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func CommonPrefix(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	i := 0
	for i < minLen && a[i] == b[i] {
		i++
	}

	return a[:i]
}

func CommonPrefixMultiple(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	prefix := paths[0]

	for _, s := range paths[1:] {
		prefix = CommonPrefix(prefix, s)
		if prefix == "" {
			break
		}
	}

	return prefix
}
