package diskcache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"files/pkg/common"
	"files/pkg/global"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Interface interface {
	Store(ctx context.Context, owner string, key string, tag string, value []byte) error
	Load(ctx context.Context, owner string, key string, tag string) (value []byte, exist bool, err error)
	Delete(ctx context.Context, owner string, key string, tag string) error
}

func GenerateCacheBufferPath(owner string, filePath string) string {
	timeStamp := time.Now().UnixNano()
	var fileName, prefixPath string
	if !strings.Contains(filePath, "/") {
		fileName = filePath
		prefixPath = "/"
	} else {
		var pos = strings.LastIndex(filePath, "/")
		fileName = filePath[pos+1:]
		prefixPath = filePath[:pos]
		if prefixPath == "" {
			prefixPath = "/"
		}
	}

	var fileNamePathMapping string = fmt.Sprintf("%d_%s", timeStamp, strings.Trim(fileName, "/"))

	bufferFolderPath := filepath.Join(common.CACHE_PREFIX, global.GlobalData.GetPvcCache(owner), common.DefaultLocalFileCachePath, common.CacheBuffer, fileNamePathMapping, prefixPath)
	if !strings.HasSuffix(bufferFolderPath, "/") {
		bufferFolderPath += "/"
	}

	return bufferFolderPath
}

func GenerateCacheKey(s string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(s))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return hash
}
