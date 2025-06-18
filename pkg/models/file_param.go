package models

import (
	"encoding/json"
	"files/pkg/constant"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

type FileParam struct {
	FileType     string      `json:"file_type"` // drive data cache internal usb smb hdd sync cloud
	SubType      string      `json:"sub_type"`
	RootPrefix   string      `json:"root_prefix"`
	Extend       string      `json:"extend"`        // node repo key deviceId diskId ...
	Path         string      `json:"path"`          // path
	AppendPrefix string      `json:"append_prefix"` // used for cache,
	Query        url.Values  `json:"query"`
	Header       http.Header `json:"header"`
	UserPvc      string      `json:"user_pvc"`
	CachePvc     string      `json:"cache_pvc"`
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
		r.RootPrefix = constant.ROOT_PREFIX
		r.Extend = r.UserPvc
		r.Path = data.DrivePath(r.UserPvc)
	case "data":
		r.FileType = "data"
		r.RootPrefix = constant.ROOT_PREFIX
		r.Extend = r.UserPvc
		r.Path = data.DataPath(r.UserPvc)
	case "cache":
		r.FileType = "cache"
		r.RootPrefix = constant.CACHE_PREFIX
		r.Extend = data.CacheNode(r.CachePvc)
		r.Path = data.CachePath()
		r.AppendPrefix = "Cache"
	case "sync":
		r.FileType = "sync"
		r.Extend = data.SyncRepo()
		r.Path = data.SyncPath()
	// case "drive": // old cloud format
	// 	r.FileType = "cloud"
	// 	r.SubType = data.CloudSubType("drive")
	// 	r.Extend = data.CloudExtend("drive")
	// 	r.Path = data.CloudPath("drive")
	case "cloud", "drive":
		r.FileType = "cloud"
		r.SubType = data.CloudSubType(routerType)
		r.Extend = data.CloudExtend(routerType)
		r.Path = data.CloudPath(routerType)
	case "external":
		r.FileType = "external"
		r.RootPrefix = constant.EXTERNAL_PREFIX
		r.Extend = ""
		r.Path = ""
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
	s := strings.TrimPrefix(p.String(), "/api/resources/home/") // Home
	return filepath.Join("/", "Home", s)                        // Include the prefix symbol: /
}

func (p PathFormatter) DataPath(userPvc string) string {
	s := strings.TrimPrefix(p.String(), "/api/resources/data/") // Application
	return filepath.Join("/", "Data", s)                        // Include the prefix symbol: /
}

func (p PathFormatter) CacheNode(cachePvc string) string {
	if strings.HasPrefix(p.String(), "/api/resources/cache/") { // AppData
		s := strings.TrimPrefix(p.String(), "/api/resources/cache/")
		s = strings.Trim(s, "/")
		a := strings.Split(s, "/")
		if a == nil || len(a) < 1 {
			return cachePvc
		}
		if a[0] == "olares" {
			return cachePvc
		}
		return filepath.Join(a[0], cachePvc)
	}
	return cachePvc

	// if strings.HasPrefix(p.String(), "/api/cache/") {
	// 	s := strings.TrimPrefix(p.String(), "/api/cache/")
	// 	ss := strings.Split(s, "/")
	// 	fmt.Println("---node / 1---", ss)
	// 	if ss == nil || len(ss) < 3 {
	// 		return ""
	// 	}
	// 	return filepath.Join(ss[0], cachePvc) //ss[0]
	// }

	// return ""
}

func (p PathFormatter) CachePath() string {
	// var pf = "/AppData"
	// var pos int
	if strings.HasPrefix(p.String(), "/api/resources/cache") { // AppData
		// pos = strings.Index(p.String(), pf)
		var tmp = strings.TrimPrefix(p.String(), "/api/resources/cache/")
		tmp = strings.Trim(tmp, "/")
		s := strings.Split(tmp, "/")
		fmt.Println("---1---", s, len(s))
		if s == nil || len(s) < 1 {
			return "/"
		}

		res := filepath.Join(s[1:]...)
		if res == "" {
			return "/"
		}
		return res
	}
	return "/"

	// if strings.HasPrefix(p.String(), "/api/cache/") {
	// 	pos = strings.Index(p.String(), pf)
	// }
	// if pos < 0 {
	// 	return "/"
	// }
	// var res = p.String()[pos+len(pf):]
	// res = strings.TrimLeft(res, "/")
	// return res
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
		return "/"
	}
	return filepath.Join(a[1:]...)
}

func (p PathFormatter) CloudSubType(tag string) string {
	var s = strings.TrimPrefix(p.String(), fmt.Sprintf("/api/resources/%s/", tag))
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	return a[0]
}

func (p PathFormatter) CloudExtend(tag string) string { // cloud Drive
	var s = strings.TrimPrefix(p.String(), fmt.Sprintf("/api/resources/%s/", tag))
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

func (p PathFormatter) CloudPath(tag string) string { // cloud Drive
	var s = strings.TrimPrefix(p.String(), fmt.Sprintf("/api/resources/%s/", tag))
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
