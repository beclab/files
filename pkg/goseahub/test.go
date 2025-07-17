package goseahub

import (
	"files/pkg/common"
	"k8s.io/klog/v2"
	"net/http"
)

func SeahubUsersGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	//fileParam, _, err := UrlPrep(r, "")
	//if err != nil {
	//	return http.StatusBadRequest, err
	//}
	//
	//uri, err := fileParam.GetResourceUri()
	//if err != nil {
	//	return http.StatusBadRequest, err
	//}
	//urlPath := uri + fileParam.Path
	//dealUrlPath := strings.TrimPrefix(urlPath, "/data")
	//
	//exists, err := afero.Exists(files.DefaultFs, dealUrlPath)
	//if err != nil {
	//	return http.StatusInternalServerError, err
	//}
	//if !exists {
	//	return http.StatusNotFound, nil
	//}
	//
	//responseData := make(map[string]interface{})
	//responseData["uid"], err = fileutils.GetUID(files.DefaultFs, dealUrlPath)
	//if err != nil {
	//	return http.StatusInternalServerError, err
	//}
	responseData, err := ListAllUsers()
	if err != nil {
		klog.Errorf("ListAllUsers failed: %v", err)
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}
