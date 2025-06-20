package base

import (
	"errors"
	"files/pkg/common"
	"files/pkg/fileutils"
	"files/pkg/models"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

func (s *FSStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	var fsType = fileParam.FileType
	var mode = 0775

	klog.Infof("POSIX BASE create, owner: %s, param: %s", s.Handler.Owner, fileParam.Json())

	var rootPrefix = s.GetRoot(fsType)
	if rootPrefix == "" {
		return 400, errors.New("type root path invalid")
	}

	var pvc, err = s.GetPvc(fsType)
	if err != nil {
		return 400, errors.New("pvc not found")
	}

	fileMode := os.FileMode(mode)

	var basePath = filepath.Join(rootPrefix, pvc, fileParam.Extend)
	klog.Infof("POSIX BASE create, owner: %s, basepath: %s", s.Handler.Owner, basePath)
	if strings.HasSuffix(fileParam.Path, "/") {
		// files.DefaultFs
		if err = fileutils.MkdirAllWithChown(afero.NewBasePathFs(afero.NewOsFs(), basePath), fileParam.Path, fileMode); err != nil {
			klog.Errorln(err)
			return common.ErrToStatus(err), err
		}
		return http.StatusOK, nil
	}
	return http.StatusBadRequest, fmt.Errorf("%s is not a valid directory path", fileParam.Path)
}
