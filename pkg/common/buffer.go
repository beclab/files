package common

import (
	"errors"
	"files/pkg/fileutils"
	"fmt"
	"k8s.io/klog/v2"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CheckBufferDiskSpace(diskSize int64) (bool, error) {
	spaceOk, needs, avails, reserved, err := checkDiskSpace("/data", diskSize)
	if err != nil {
		return false, err
	}
	needsStr := formatBytes(needs)
	availsStr := formatBytes(avails)
	reservedStr := formatBytes(reserved)
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

	if err := os.MkdirAll(bufferFolderPath, 0755); err != nil {
		return "", err
	}
	if err := fileutils.Chown(nil, bufferFolderPath, 1000, 1000); err != nil {
		klog.Errorf("can't chown directory %s to user %d: %s", bufferFolderPath, 1000, err)
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
	bufferPathName = RemoveSlash(bufferPathName)
	bufferFolderPath := "/data/" + bflName + "/buffer" + "/" + bufferPathName
	if err := os.MkdirAll(bufferFolderPath, 0755); err != nil {
		return "", err
	}
	if err := fileutils.Chown(nil, bufferFolderPath, 1000, 1000); err != nil {
		klog.Errorf("can't chown directory %s to user %d: %s", bufferFolderPath, 1000, err)
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
	if srcType == "google" || srcType == "cloud" || srcType == "awss3" || srcType == "tencent" || srcType == "dropbox" {
		dir := filepath.Dir(filePath)
		err = os.Remove(dir)
		if err != nil {
			klog.Errorln("Failed to delete buffer file dir:", err)
			return
		}
	}

	klog.Infoln("Buffer file deleted.")
}
