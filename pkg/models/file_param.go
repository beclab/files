package models

import (
	"encoding/json"
	"errors"
	"files/pkg/constant"
	"files/pkg/global"
	"fmt"
	"path/filepath"
	"strings"
)

type FileParam struct {
	Owner    string `json:"owner"`
	FileType string `json:"file_type,omitempty"` // drive data cache internal usb smb hdd sync cloud
	Extend   string `json:"extend,omitempty"`    // node repo key deviceId diskId ...
	Path     string `json:"path,omitempty"`      // path
}

func CreateFileParam(owner string, path string) (*FileParam, error) {
	var param = &FileParam{
		Owner: owner,
	}

	if err := param.convert(path); err != nil {
		return nil, err
	}

	return param, nil
}

func (p *FileParam) convert(url string) (err error) {
	if url == "" {
		return fmt.Errorf("url invalid, url: %s", url)
	}

	var u = strings.TrimLeft(url, "/")
	if u == "" {
		return fmt.Errorf("url invalid, %s", url)
	}

	var s = strings.Split(u, "/")
	var fileType = strings.ToLower(s[0])

	if len(s) < 3 {
		return fmt.Errorf("url invalid, %s", url)
	}

	var extend = s[1]
	var subPath string = ""
	for i := 2; i < len(s); i++ {
		subPath = subPath + "/" + s[i]
	}

	if fileType == constant.Drive {

		if extend != "Home" && extend != "Data" {
			return fmt.Errorf("invalid drive type: %s", extend)
		}
		p.FileType = constant.Drive
		p.Extend = extend
		p.Path = subPath

	} else if fileType == constant.Cache {

		if !global.GlobalNode.CheckNodeExists(extend) {
			return fmt.Errorf("node %s not found", extend)
		}
		p.FileType = constant.Cache
		p.Extend = extend
		p.Path = subPath

	} else if fileType == constant.Sync {

		p.FileType = constant.Sync
		p.Extend = extend
		p.Path = subPath

	} else if fileType == constant.AwsS3 || fileType == constant.DropBox {

		p.FileType = fileType
		p.Extend = extend
		p.Path = subPath

	} else if fileType == constant.GoogleDrive {

		if subPath != "/" {
			subPath = strings.Trim(subPath, "/")
		}
		p.FileType = constant.GoogleDrive
		p.Extend = extend
		p.Path = subPath

	} else if fileType == constant.External {
		if !global.GlobalNode.CheckNodeExists(extend) {
			return fmt.Errorf("node %s not found", extend)
		}

		p.FileType = constant.External
		// don't check file type is exists
		p.Extend = extend
		p.Path = subPath

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

func (r *FileParam) GetResourceUri() (string, error) {
	if r.FileType == "drive" {
		var pvc = global.GlobalData.GetPvcUser(r.Owner)
		if pvc == "" {
			return "", errors.New("pvc user not found")
		}
		return filepath.Join(constant.ROOT_PREFIX, pvc, r.Extend), nil
	} else if r.FileType == "cache" {
		var pvc = global.GlobalData.GetPvcCache(r.Owner)
		if pvc == "" {
			return "", errors.New("pvc cache not found")
		}
		return filepath.Join(constant.CACHE_PREFIX, pvc), nil
	} else if r.FileType == "external" {
		return filepath.Join(constant.EXTERNAL_PREFIX), nil
	} else if r.FileType == "internal" || r.FileType == "smb" || r.FileType == "usb" || r.FileType == "hdd" {
		return filepath.Join(constant.EXTERNAL_PREFIX), nil
	} else if r.FileType == "sync" {
		return filepath.Join("/", r.FileType, r.Extend), nil
	} else if r.FileType == "google" || r.FileType == "dropbox" || r.FileType == "awss3" {
		return filepath.Join("/", "drive", r.FileType, r.Extend), nil
	}

	return "", fmt.Errorf("invalid file type: %s", r.FileType)

}

func (r *FileParam) GetFileParam(uri string) error {
	var u = strings.TrimLeft(uri, "/")

	var s = strings.Split(u, "/")
	if len(s) < 2 {
		return errors.New("url invalid")
	}

	if s[0] == constant.AwsS3 || s[0] == constant.DropBox || s[0] == constant.GoogleDrive {
		r.Owner = ""
		r.FileType = s[0]
		r.Extend = s[1]
		r.Path = r.joinPath(2, s)
		return nil

	}

	if s[0] == constant.Sync {
		r.Owner = ""
		r.FileType = s[0]
		r.Extend = s[1]
		r.Path = r.joinPath(2, s)
		return nil

	}

	if strings.HasPrefix(uri, constant.ROOT_PREFIX+"/") {
		var p = strings.TrimPrefix(uri, constant.ROOT_PREFIX+"/")
		s = strings.Split(p, "/")
		pvcUser, err := global.GlobalData.GetPvcUserName(s[0])
		if err == nil {
			r.Owner = pvcUser
			r.FileType = constant.Drive
			r.Extend = s[1]
			r.Path = r.joinPath(2, s)
			return nil
		}
	}

	if strings.HasPrefix(uri, constant.CACHE_PREFIX+"/") {
		var p = strings.TrimPrefix(uri, constant.CACHE_PREFIX+"/")
		s = strings.Split(p, "/")
		pvcCache, err := global.GlobalData.GetPvcCacheName(s[0])
		if err == nil {
			r.Owner = pvcCache
			r.FileType = constant.Cache
			r.Extend = global.CurrentNodeName
			r.Path = r.joinPath(1, s)
			return nil
		}
	}

	if strings.HasPrefix(uri, constant.EXTERNAL_PREFIX+"/") {
		var p = strings.TrimPrefix(uri, constant.EXTERNAL_PREFIX+"/")
		s = strings.Split(p, "/")
		r.Owner = ""
		r.FileType = constant.External
		r.Extend = global.CurrentNodeName
		r.Path = r.joinPath(0, s)
		return nil
	}
	return nil
}

func (r *FileParam) IsFile() (string, bool) {
	if ok := strings.HasSuffix(r.Path, "/"); !ok {
		return "", false
	}
	return r.Path[strings.LastIndex(r.Path, "/")+1:], true
}

func (r *FileParam) GetName() string {
	fileName, isFile := r.IsFile()
	if isFile {
		return fileName
	}

	var tmp = strings.TrimSuffix(r.Path, "/")
	var p = strings.LastIndex(tmp, "/")
	return r.Path[p:]
}

func (r *FileParam) joinPath(pos int, s []string) string {
	var str string
	for i := pos; i < len(s); i++ {
		str = str + "/" + s[i]
	}
	if str == "" {
		str = "/"
	}
	return str
}
