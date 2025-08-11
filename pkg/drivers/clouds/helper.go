package clouds

import (
	"files/pkg/common"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"time"
)

func CreateFileDownloadFolder(owner, f string) string {
	timestamp := time.Now().Unix()

	rand.Seed(time.Now().UnixNano())
	randomNumber := rand.Intn(10000000000)
	randomNumberString := fmt.Sprintf("%010d", randomNumber)

	timestampPlus := fmt.Sprintf("%d%s", timestamp, randomNumberString)

	originalPathName := filepath.Base(strings.TrimSuffix(f, "/"))
	extension := filepath.Ext(originalPathName)
	if len(extension) > 0 {
		originalPathName = strings.TrimSuffix(originalPathName, extension) + "_" + extension[1:]
	}

	bufferPathName := fmt.Sprintf("%s_%s", timestampPlus, originalPathName) // as parent folder
	bufferPathName = common.RemoveSlash(bufferPathName)
	bufferFolderPath := "/data/buffer/" + owner + "/" + bufferPathName

	if !strings.HasSuffix(bufferFolderPath, "/") {
		bufferFolderPath = bufferFolderPath + "/"
	}

	return bufferFolderPath

	// var baseDir = path.Join("/", "data", "buffer", owner, time.Now().Format("2006-01-02"))
	// var filePathDir = path.Dir(filePath)
	// var filePathSplits = strings.Split(filePathDir, "/")
	// var downloadPath = path.Join(baseDir, path.Join(filePathSplits[2:]...))

	// if !strings.HasSuffix(downloadPath, "/") {
	// 	downloadPath = downloadPath + "/"
	// }

	// return downloadPath
}

func getRenamedSrcName(s string) (string, bool) {

	var isFile = strings.HasSuffix(s, "/")
	var tmp = strings.TrimSuffix(s, "/")
	var p = strings.LastIndex(tmp, "/")
	var r = tmp[p:]
	r = strings.Trim(r, "/")

	return r, !isFile
}

func getRenamedSrcPrefixPath(s string) string {
	if s == "/" {
		return s
	}

	var r = strings.TrimSuffix(s, "/")
	var p = strings.LastIndex(r, "/")
	return r[:p+1]
}
