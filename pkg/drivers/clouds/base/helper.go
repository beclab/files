package base

import (
	"strings"

	"k8s.io/klog/v2"
)

func ParseCloudDrivePath(src string) (drive, name, path string) {
	if strings.HasPrefix(src, "/Drive/") || strings.HasPrefix(src, "/drive/") {
		src = src[7:]
	}
	parts := strings.SplitN(src, "/", 2)
	drive = parts[0]

	trimSuffix := true
	if drive == "awss3" {
		trimSuffix = false
	}

	src = "/"
	if len(parts) > 1 {
		src += parts[1]
	}

	slashes := []int{}
	for i, char := range src {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		klog.Infoln("Path does not contain enough slashes.")
		return drive, "", ""
	}

	name = src[1:slashes[1]]
	path = src[slashes[1]:]
	if trimSuffix && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	return drive, name, path
}

func CloudDriveNormalizationPath(path, srcType string, same, addSuffix bool) string {
	if same != (srcType == "awss3") {
		return path
	}

	if addSuffix && !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	if !addSuffix && strings.HasSuffix(path, "/") {
		return strings.TrimSuffix(path, "/")
	}

	return path
}
