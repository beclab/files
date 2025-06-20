package models

import (
	"encoding/json"
	"errors"
	"files/pkg/utils"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

type FileParam struct {
	FileType string `json:"file_type"` // drive data cache internal usb smb hdd sync cloud
	Extend   string `json:"extend"`    // node repo key deviceId diskId ...
	Path     string `json:"path"`      // path
}

func CreateFileParam(owner string, path string) (*FileParam, error) {

	var data = PathFormatter(path)

	var param = &FileParam{}

	if err := param.ParseTo(data); err != nil {
		return nil, err
	}

	return param, nil
}

func (r *FileParam) ParseTo(path PathFormatter) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()
	err = errors.New("type not found")

	r.FileType = path.Type()
	if r.FileType == "" {
		return err
	}

	klog.Infof("type: %s", r.FileType)

	switch r.FileType {
	case "home":
		r.FileType = "drive"
		r.Path = path.DrivePath()
	case "data":
		r.Path = path.DataPath()
	case "cache":
		r.Extend = path.CacheNode()
		r.Path = path.CachePath()
	case "sync":
		r.FileType = "sync"
		r.Extend = path.SyncRepo()
		r.Path = path.SyncPath()
	case "google", "awss3", "tencent", "dropbox":
		r.Extend = path.CloudExtend()
		r.Path = path.CloudPath()
	case "external":
		r.Extend = path.ExternalExtend()
		r.Path = path.ExternalPath()
	// case "internal", "usb", "smb", "hdd":
	// 	r.Extend = ""
	// 	r.Path = ""
	default:
		return err
	}

	return nil
}

func (r *FileParam) Json() string {
	d, _ := json.Marshal(r)
	return string(d)
}

func (r *FileParam) PrettyJson() string {
	d, _ := json.MarshalIndent(r, "", "    ")
	return string(d)
}

type PathFormatter string

func (p PathFormatter) String() string {
	return string(p)
}

func (p PathFormatter) Type() string {
	var s = p.trimPrefix([]string{"/api/resources/"})
	if s == "" {
		panic(fmt.Errorf("path is invalid: %s", p.String()))
	}
	a := strings.Split(s, "/")
	if a == nil || len(a) < 1 {
		panic(fmt.Errorf("path is invalid: %s", p.String()))
	}

	var fileType = strings.ToLower(a[0])
	if fileType == "cloud" || fileType == "drive" {
		if len(s) < 1 {
			panic(fmt.Errorf("cloud type from path invalid: %s", p.String()))
		}
		return strings.ToLower(a[1])
	}
	return fileType
}

func (p PathFormatter) DrivePath() string {
	s := p.trimPrefix([]string{"/api/resources/home/", "/api/resources/Home/"})
	return utils.JoinWithSlash("/", "Home", s)
}

func (p PathFormatter) DataPath() string {
	s := p.trimPrefix([]string{"/api/resources/data/", "/api/resources/Data/"})
	return utils.JoinWithSlash("/", "Data", s)
}

func (p PathFormatter) CacheNode() string {
	s := p.trimPrefix([]string{"/api/resources/cache", "/api/resources/cache/"})
	s = strings.Trim(s, "/")
	if s == "" {
		return ""
	}
	a := strings.Split(s, "/")
	if a == nil || len(a) < 1 {
		return ""
	}

	return a[0]
}

func (p PathFormatter) CachePath() string {
	s := p.trimPrefix([]string{"/api/resources/cache", "/api/resources/cache/"})
	s = strings.Trim(s, "/")
	if s == "" {
		return "/"
	}
	a := strings.Split(s, "/")
	if a == nil || len(a) < 1 {
		return "/"
	}

	res := utils.JoinWithSlash(a[1:]...)
	if res == "" {
		return "/"
	}
	return res
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
	if s == "" {
		return "/"
	}
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 2 {
		return "/"
	}
	return utils.JoinWithSlash(a[1:]...)
}

func (p PathFormatter) CloudSubType(tag string) string {
	var s = strings.TrimPrefix(p.String(), fmt.Sprintf("/api/resources/%s/", tag))
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	return a[0]
}

func (p PathFormatter) CloudExtend() string {
	s := p.trimPrefix([]string{"/api/resources/cloud/", "/api/resources/drive/"})
	s = strings.Trim(s, "/")
	if s == "" {
		panic(fmt.Sprintf("cloud extend empty: %s", p.String()))
	}

	var a = strings.Split(s, "/")
	if a == nil || len(a) < 2 {
		panic(fmt.Sprintf("cloud extend suffix is empty: %s", p.String()))
	}
	return a[1]
}

func (p PathFormatter) CloudPath() string {
	s := p.trimPrefix([]string{"/api/resources/cloud/", "/api/resources/drive/"})
	s = strings.Trim(s, "/")
	if s == "" {
		panic(fmt.Sprintf("cloud path empty: %s", p.String()))
	}
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 2 {
		panic(fmt.Sprintf("cloud path is invalid: %s", p.String()))
	}

	var t = a[0]
	if t == "google" {
		if len(a) < 3 {
			panic(fmt.Sprintf("google path no root path: %s", p.String()))
		}

		var res = utils.JoinWithSlash(a[2:]...)
		if res == "root/" { // special for google
			res = "/"
		} else {
			res = strings.Trim(res, "/")
		}
		return res
	} else if t == "dropbox" {
		if len(a) < 2 {
			panic(fmt.Sprintf("dropbox path no root path: %s", p.String()))
		}
		return utils.JoinWithSlash(a[2:]...)
	}
	return utils.JoinWithSlash(a[2:]...)
}

func (p PathFormatter) ExternalExtend() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources/external/")
	s = strings.Trim(s, "/")
	var a = strings.Split(s, "/")
	if a == nil || len(a) < 1 {
		return ""
	}

	return ""
}

func (p PathFormatter) ExternalPath() string {
	var s = strings.TrimPrefix(p.String(), "/api/resources/external/")
	s = strings.Trim(s, "/")
	if s == "" {
		return "/"
	}
	var a = strings.Split(s, "/")
	return utils.JoinWithSlash(a[0:]...)
}

func (p PathFormatter) trimPrefix(s []string) string {
	var res string = p.String()
	for _, c := range s {
		res = strings.TrimPrefix(res, c)
	}
	return res
}
