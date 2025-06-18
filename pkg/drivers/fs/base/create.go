package base

import (
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/models"
	"fmt"
	"net/http"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

func (s *FSStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	var err error
	var mode = 0775

	klog.Infof("Posix drive create, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	fileMode := os.FileMode(mode)

	if strings.HasSuffix(fileParam.Path, "/") {
		if err = fileutils.MkdirAllWithChown(files.DefaultFs, fileParam.Path, fileMode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
		return http.StatusOK, nil
	}
	return http.StatusBadRequest, fmt.Errorf("%s is not a valid directory path", fileParam.Path)
}
