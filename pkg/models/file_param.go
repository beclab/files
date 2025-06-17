package models

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type FileParam struct {
	FileType string      `json:"file_type"` // drive data cache internal usb smb hdd sync cloud
	Extend   string      `json:"extend"`    // node repo key deviceId diskId ...
	Path     string      `json:"path"`      // path
	Query    url.Values  `json:"query"`
	Header   http.Header `json:"header"`
	UserPvc  string      `json:"user_pvc"`
	CachePvc string      `json:"cache_pvc"`
}

func (r *FileParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}

func (r *FileParam) ConvertToBackendUrl(data PathFormatter) {
	var routerType = data.Type()
	routerType = strings.ToLower(routerType)

	klog.Infof("router type: %s", routerType)

	switch routerType {
	case "home":
		r.FileType = "drive"
		r.Path = data.DrivePath(r.UserPvc)
	case "application":
		r.FileType = "data"
		r.Path = data.DataPath(r.UserPvc)
	case "appdata":
		r.FileType = "cache"
		r.Extend = data.CacheNode()
		r.Path = data.CachePath(r.CachePvc)
	case "sync":
		r.FileType = "sync"
		r.Extend = data.SyncRepo()
		r.Path = data.SyncPath()
	case "cloud":
		r.FileType = "cloud"
		r.Extend = data.CloudExtend()
		r.Path = data.CloudPath()
	case "internal", "usb", "smb", "hdd":
		// r.FileType = routerType
	// r.Extend = ""
	// r.Path = ""
	default:
		panic("router type not found")
	}
}

type PathFormatter string

func (p PathFormatter) String() string {
	return string(p)
}

func (p PathFormatter) Suffix() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources")
	return strings.Trim(s, "/")
}

func (p PathFormatter) Type() string {
	s := strings.Split(p.Suffix(), "/")
	if s == nil || len(s) < 1 {
		return ""
	}
	return s[0]
}

func (p PathFormatter) DrivePath(userPvc string) string {
	s := strings.TrimPrefix(p.String(), "/api/resources/Home/")
	return filepath.Join(userPvc, "Home", s)
}

func (p PathFormatter) DataPath(userPvc string) string {
	s := strings.TrimPrefix(p.String(), "/api/resources/Application/")
	return filepath.Join(userPvc, "Data", s)
}

func (p PathFormatter) CacheNode() string {
	if strings.HasPrefix(p.String(), "/api/resources/AppData") {
		return ""
	}

	if strings.HasPrefix(p.String(), "/api/cache/") {
		s := strings.TrimPrefix(p.String(), "/api/cache/")
		ss := strings.Split(s, "/")
		if ss == nil || len(ss) < 3 {
			return ""
		}
		return ss[0]
	}

	return ""
}

func (p PathFormatter) CachePath(cachePvc string) string {
	var pf = "/AppData"
	var pos int
	if strings.HasPrefix(p.String(), "/api/resources/AppData") {
		pos = strings.Index(p.String(), pf)
	}
	if strings.HasPrefix(p.String(), "/api/cache/") {
		pos = strings.Index(p.String(), pf)
	}
	if pos < 0 {
		return ""
	}
	var res = p.String()[pos+len(pf):]
	res = strings.TrimLeft(res, "/")
	return filepath.Join(cachePvc, res)
}

func (p PathFormatter) SyncRepo() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources/sync/")
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 1 {
		return ""
	}
	return a[0]
}

func (p PathFormatter) SyncPath() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources/sync/")
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 2 {
		return ""
	}
	return filepath.Join(a[1:]...)
}

func (p PathFormatter) CloudExtend() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources/cloud/")
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 2 {
		return ""
	}
	if a[0] == "google" {
		return filepath.Join(a[1:]...)
	}

	return a[1]
}

func (p PathFormatter) CloudPath() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources/cloud/")
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 2 {
		return ""
	}
	if a[0] == "google" {
		return ""
	}

	return filepath.Join(a[1:]...)
}
