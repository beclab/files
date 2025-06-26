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
	FileType string `json:"file_type"` // drive data cache internal usb smb hdd sync cloud
	Extend   string `json:"extend"`    // node repo key deviceId diskId ...
	Path     string `json:"path"`      // path
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

	} else if fileType == constant.GoogleDrive || fileType == constant.DropBox || fileType == constant.AwsS3 {

		p.FileType = fileType
		p.Extend = extend
		p.Path = subPath

	} else if fileType == constant.External {
		var externalType = s[2]
		if !global.GlobalNode.CheckNodeExists(extend) {
			return fmt.Errorf("node %s not found", extend)
		}

		p.FileType = global.GlobalMounted.CheckExternalType(externalType)
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
	}

	return "", fmt.Errorf("invalid file type: %s", r.FileType)

}
