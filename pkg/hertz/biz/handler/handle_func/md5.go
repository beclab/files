package handle_func

import (
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/models"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
	"strings"
)

func md5FileHandler(file *files.FileInfo) ([]byte, int, error) {
	var err error
	responseData := make(map[string]interface{})
	responseData["md5"], err = common.Md5File(file.Path)
	if err != nil {
		return nil, consts.StatusInternalServerError, err
	}
	klog.Infof("~~~Debug log: responseData: %v", responseData)
	return common.ToBytes(responseData), consts.StatusOK, nil
}

func Md5Handler(c *app.RequestContext, _ interface{}, prefix string) ([]byte, int, error) {
	var p = string(c.Path())
	klog.Infof("~~~Debug log: p=%s", p)
	var path = strings.TrimPrefix(p, prefix)
	if path == "" {
		return nil, consts.StatusBadRequest, errors.New("path invalid")
	}

	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		return nil, consts.StatusBadRequest, errors.New("user not found")
	}
	var fileParam, err = models.CreateFileParam(owner, path)
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}

	uri, err := fileParam.GetResourceUri()
	if err != nil {
		return nil, consts.StatusBadRequest, err
	}
	urlPath := uri + fileParam.Path
	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         files.DefaultFs,
		Path:       strings.TrimPrefix(urlPath, "/data"),
		Modify:     true,
		Expand:     false,
		ReadHeader: true,
	})
	if err != nil {
		return nil, common.ErrToStatus(err), err
	}

	if file.IsDir {
		err = errors.New("only support md5 for file")
		return nil, consts.StatusForbidden, err
	}

	return md5FileHandler(file)
}
