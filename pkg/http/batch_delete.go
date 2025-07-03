package http

import (
	"encoding/json"
	"files/pkg/common"
	"files/pkg/drives"
	"files/pkg/fileutils"
	"files/pkg/models"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
)

type BatchDeleteRequest struct {
	Dirents []string `json:"dirents"`
}

func batchDeleteHandler(fileCache fileutils.FileCache) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
		var reqBody BatchDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode request body: %v", err)
		}
		defer r.Body.Close()

		dirents := reqBody.Dirents
		if len(dirents) == 0 {
			return 0, nil
		}

		klog.Infof("dirents: %v", dirents)

		var handler drives.ResourceService
		var err error
		fileParams := []*models.FileParam{}
		for _, dirent := range dirents {
			var fileParam *models.FileParam
			fileParam, handler, err = UrlPrep(r, dirent)
			if err != nil {
				return http.StatusBadRequest, err
			}
			fileParams = append(fileParams, fileParam)
		}

		//srcType, err := drives.ParsePathType(dirents[0], r, false, true)
		//if err != nil {
		//	return http.StatusBadRequest, err
		//}
		//
		//handler, err := drives.GetResourceService(srcType)
		//if err != nil {
		//	return http.StatusBadRequest, err
		//}

		return handler.BatchDeleteHandler(fileCache, fileParams)(w, r, d)
	}
}
