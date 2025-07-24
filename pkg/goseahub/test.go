package goseahub

import (
	"files/pkg/common"
	"k8s.io/klog/v2"
	"net/http"
)

func SeahubUsersGetHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	//ctx := r.Context()
	//user, exists := ctx.Value("user").(*models.Profile)

	//if !exists || user == nil {
	//	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	//	return http.StatusUnauthorized, nil
	//}
	//klog.Infof("Full user info: %+v", *user)

	responseData, err := ListAllUsers()
	if err != nil {
		klog.Errorf("ListAllUsers failed: %v", err)
		return http.StatusInternalServerError, err
	}
	return common.RenderJSON(w, r, responseData)
}
