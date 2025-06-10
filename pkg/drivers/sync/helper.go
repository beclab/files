package sync

import (
	"strings"

	"k8s.io/klog/v2"
)

func ParseSyncPath(src string) (string, string, string) {
	// prefix is with suffix "/" and prefix "/" while repo_id and filename don't have any prefix and suffix
	src = strings.TrimPrefix(src, "/")

	firstSlashIdx := strings.Index(src, "/")

	repoID := src[:firstSlashIdx]

	lastSlashIdx := strings.LastIndex(src, "/")

	filename := src[lastSlashIdx+1:]

	prefix := ""
	if firstSlashIdx != lastSlashIdx {
		prefix = src[firstSlashIdx+1 : lastSlashIdx+1]
	}
	if prefix == "" {
		prefix = "/"
	}

	// for url with additional "/" or lack of "/"
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	klog.Infoln("repo-id:", repoID)
	klog.Infoln("prefix:", prefix)
	klog.Infoln("filename:", filename)
	return repoID, prefix, filename
}
