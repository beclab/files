package google

import (
	"strings"

	"k8s.io/klog/v2"
)

func ParseGoogleDrivePath(path string) (drive, name, dir, filename string) {
	if strings.HasPrefix(path, "/Drive/google") || strings.HasPrefix(path, "/drive/google") {
		path = path[13:]
		drive = "google"
	}

	slashes := []int{}
	for i, char := range path {
		if char == '/' {
			slashes = append(slashes, i)
		}
	}

	if len(slashes) < 2 {
		klog.Infoln("Path does not contain enough slashes.")
		return drive, "", "", ""
	}

	name = path[1:slashes[1]]

	if len(slashes) == 2 {
		return drive, name, "/", path[slashes[1]+1:]
	}

	dir = path[slashes[1]+1 : slashes[2]]
	filename = strings.TrimPrefix(path[slashes[2]:], "/")

	if dir == "root" {
		dir = "/"
	}

	return drive, name, dir, filename
}
