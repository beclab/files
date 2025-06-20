package cache

import (
	"errors"
	"files/pkg/common"
	"files/pkg/drivers/fs/base"
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

type CacheStorage struct {
	Base *base.FSStorage
}

func (s *CacheStorage) CreateFolder(fileParam *models.FileParam) (int, error) {
	var fsType = fileParam.FileType
	var mode = 0775

	var cacheRootPrefix = s.Base.GetRoot(fsType)
	if cacheRootPrefix == "" {
		return 400, errors.New("type root path invalid")
	}

	var pvc, err = s.Base.GetPvc(fsType)
	if err != nil {
		return 400, errors.New("pvc not found")
	}

	fileMode := os.FileMode(mode)

	var basePath = filepath.Join(cacheRootPrefix, pvc)

	klog.Infof("POSIX CACHE create, owner: %s, basepath: %s, param: %s", s.Base.Handler.Owner, basePath, fileParam.Json())

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
