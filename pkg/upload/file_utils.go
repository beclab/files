package upload

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"files/pkg/drives"
	"files/pkg/files"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	expireTime         = time.Duration(24) * time.Hour
	DefaultMaxFileSize = 4 * 1024 * 1024 * 1024 // 4G
)

var UploadsFiles map[string]string = map[string]string{}

func removeTempFile(uid string, uploadsDir string) {
	filePath := filepath.Join(uploadsDir, uid)
	err := os.Remove(filePath)
	if err != nil {
		klog.Warningf("remove %s err:%v", filePath, err)
	}
}

func removeInfoFile(uid string, uploadsDir string) {
	infoPath := filepath.Join(uploadsDir, uid+".info")
	err := os.Remove(infoPath)
	if err != nil {
		klog.Warningf("remove %s err:%v", infoPath, err)
	}
}

func RemoveTempFileAndInfoFile(uid string, uploadsDir string) {
	removeTempFile(uid, uploadsDir)
	removeInfoFile(uid, uploadsDir)
}

func ParseContentRange(ranges string) (int64, int64, bool) {
	start := strings.Index(ranges, "bytes")
	end := strings.Index(ranges, "-")
	slash := strings.Index(ranges, "/")

	if start < 0 || end < 0 || slash < 0 {
		return -1, -1, false
	}

	startStr := strings.TrimLeft(ranges[start+len("bytes"):end], " ")
	firstByte, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return -1, -1, false
	}

	lastByte, err := strconv.ParseInt(ranges[end+1:slash], 10, 64)
	if err != nil {
		return -1, -1, false
	}

	fileSize, err := strconv.ParseInt(ranges[slash+1:], 10, 64)
	if err != nil {
		return -1, -1, false
	}

	if firstByte > lastByte || lastByte >= fileSize {
		return -1, -1, false
	}

	return firstByte, lastByte, true
}

func CalculateMD5(fileHeader *multipart.FileHeader) (string, error) {
	start := time.Now()
	klog.Infoln("Function CalculateMD5 starts at", start)
	defer func() {
		elapsed := time.Since(start)
		klog.Infof("Function CalculateMD5 execution time: %v\n", elapsed)
	}()

	// Open the file
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create an MD5 hash object
	hash := md5.New()

	// Copy the file content to the hash object
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Compute the hash and get the checksum
	hashInBytes := hash.Sum(nil)[:16]

	// Convert the byte array to a hexadecimal string
	md5String := hex.EncodeToString(hashInBytes)

	return md5String, nil
}

func SaveFile(fileHeader *multipart.FileHeader, filePath string, newFile bool, offset int64) (int64, error) {
	startTime := time.Now()
	klog.Infof("--- Function SaveFile started at: %s\n", startTime)

	defer func() {
		endTime := time.Now()
		klog.Infof("--- Function SaveFile ended at: %s\n", endTime)
	}()

	localStartTime := time.Now()
	klog.Infof("------ fileHeader.Open() started at: %s\n", localStartTime)
	// Open source file
	file, err := fileHeader.Open()
	if err != nil {
		return 0, err
	}
	defer file.Close()
	localEndTime := time.Now()
	klog.Infof("------ fileHeader.Open() ended at: %s\n", localEndTime)

	// Determine file open flags based on newFile parameter
	var flags int
	if newFile {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	} else {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	localStartTime = time.Now()
	klog.Infof("------ OpenFile() started at: %s\n", localStartTime)
	// Create target file with appropriate flags
	dstFile, err := os.OpenFile(filePath, flags, 0644)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()
	localEndTime = time.Now()
	klog.Infof("------ OpenFile() ended at: %s\n", localEndTime)

	localStartTime = time.Now()
	klog.Infof("------ Seek() started at: %s\n", localStartTime)
	// Seek to the specified offset if not creating a new file
	if !newFile {
		_, err = dstFile.Seek(offset, io.SeekStart)
		if err != nil {
			return 0, err
		}
	}
	localEndTime = time.Now()
	klog.Infof("------ Seek() ended at: %s\n", localEndTime)

	localStartTime = time.Now()
	klog.Infof("------ io.Copy() started at: %s\n", localStartTime)
	// Write the contents of the source file to the target file
	_, err = io.Copy(dstFile, file)
	if err != nil {
		return 0, err
	}
	localEndTime = time.Now()
	klog.Infof("------ io.Copy() ended at: %s\n", localEndTime)

	localStartTime = time.Now()
	klog.Infof("------ getFileSize started at: %s\n", localStartTime)
	// Get new file size
	fileInfo, err := dstFile.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := fileInfo.Size()
	localEndTime = time.Now()
	klog.Infof("------ getFileSize ended at: %s\n", localEndTime)

	return fileSize, nil
}

func UpdateFileInfo(fileInfo FileInfo, uploadsDir string) error {
	// Construct file information path
	infoPath := filepath.Join(uploadsDir, fileInfo.ID+".info")

	// Convert file information to JSON string
	infoJSON, err := json.Marshal(fileInfo)
	if err != nil {
		return err
	}

	// Write file information
	err = ioutil.WriteFile(infoPath, infoJSON, 0644)
	if err != nil {
		return err
	}

	return nil
}

func MoveFileByInfo(fileInfo FileInfo, uploadsDir string) error {
	// Construct file path
	filePath := filepath.Join(uploadsDir, fileInfo.ID)

	// Construct target path
	destinationPath := drives.AddVersionSuffix(fileInfo.FullPath, nil, false)

	// Move files to target path
	err := files.MoveFileOs(filePath, destinationPath)
	if err != nil {
		return err
	}

	// Remove info file
	removeInfoFile(fileInfo.ID, uploadsDir)

	return nil
}
