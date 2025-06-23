package upload

import (
	"crypto/md5"
	"encoding/hex"
	"files/pkg/rpc"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strings"
)

const (
	CacheRequestPrefix = "/AppData"
	CachePathPrefix    = "/appcache"
	ExternalPathPrefix = "/data/External/"
)

func GetPVC(r *http.Request) (string, string, string, string, error) {
	bflName := r.Header.Get("X-Bfl-User")
	klog.Info("BFL_NAME: ", bflName)

	userPvc, err := rpc.BflPVCs.GetUserPVCOrCache(bflName)
	if err != nil {
		klog.Info(err)
		return bflName, "", "", "", err
	} else {
		klog.Info("user-space pvc: ", userPvc)
	}

	cachePvc, err := rpc.BflPVCs.GetCachePVCOrCache(bflName)
	if err != nil {
		klog.Info(err)
		return bflName, "", "", "", err
	} else {
		klog.Info("appcache pvc: ", cachePvc)
	}

	var uploadsDir = CachePathPrefix + "/" + cachePvc + "/files/.uploadstemp"

	return bflName, userPvc, cachePvc, uploadsDir, nil
}

func ExtractPart(s string) string {
	if !strings.HasPrefix(s, ExternalPathPrefix) {
		return ""
	}

	s = s[len(ExternalPathPrefix):]

	index := strings.Index(s, "/")

	if index == -1 {
		return s
	} else {
		return s[:index]
	}
}

func CheckDirExist(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		return false
	}
	return false
}

func MakeUid(filePath string) string {
	hash := md5.Sum([]byte(filePath))
	md5String := hex.EncodeToString(hash[:])
	klog.Infof("filePath:%s, uid:%s", filePath, md5String)
	return md5String
}

func PathExistsAndGetLen(path string) (bool, int64) {
	info, err := os.Stat(path)
	if err == nil {
		return true, info.Size()
	}

	if os.IsNotExist(err) {
		return false, 0
	}
	return false, 0
}

func RewriteUrl(path string, pvc string, prefix string, focusPrefix string) string {
	if prefix == "" {
		dealPath := path
		if focusPrefix == "" {
			focusPrefix = "/"
		}
		dealPath = strings.TrimPrefix(path, focusPrefix)
		klog.Infof("Rewriting url for: %s with a focus prefix: %s", dealPath, focusPrefix)
		klog.Infof("pvc: %s", pvc)

		pathSplit := strings.Split(dealPath, "/")
		if len(pathSplit) < 2 {
			return ""
		}

		if pathSplit[0] != pvc {
			switch pathSplit[0] {
			case "external", "External":
				return focusPrefix + "External" + strings.TrimPrefix(dealPath, pathSplit[0])
			case "home", "Home":
				return focusPrefix + pvc + "/Home" + strings.TrimPrefix(dealPath, pathSplit[0])
			case "data", "Data", "application", "Application":
				return focusPrefix + pvc + "/Data" + strings.TrimPrefix(dealPath, pathSplit[0])
			}
		}
	} else {
		pathSuffix := strings.TrimPrefix(path, prefix)
		if strings.HasSuffix(prefix, "/cache") {
			prefix = strings.TrimSuffix(prefix, "/cache") + "/AppData"
		}
		if strings.HasPrefix(pathSuffix, "/"+pvc) {
			return path
		}
		return prefix + "/" + pvc + pathSuffix
	}
	return path
}
