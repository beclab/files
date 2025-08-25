package handle_func

import (
	"files/pkg/drivers/sync/seahub"
	"files/pkg/hertz/biz/model/callback"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
	"strings"
)

func CallbackCreateHandler(c *app.RequestContext, req interface{}, _ string) ([]byte, int, error) {
	data := req.(callback.CallbackCreateReq)
	bflName := strings.TrimSpace(data.Name)
	if bflName != "" {
		newUsername := bflName + "@auth.local"
		klog.Infof("Try to create user for %s", newUsername)

		isNew, err := seahub.CreateUser(newUsername)
		if err != nil {
			klog.Infof("Error creating user: %v", err)
			return nil, consts.StatusInternalServerError, err
		}

		if isNew {
			repoId, err := seahub.CreateDefaultLibrary(newUsername)
			if err != nil {
				klog.Infof("Create default library for %s failed: %v", newUsername, err)
			} else {
				klog.Infof("Create default library %s for %s successfully!", repoId, newUsername)
			}
		}
	}

	return nil, 0, nil
}

func CallbackDeleteHandler(c *app.RequestContext, req interface{}, _ string) ([]byte, int, error) {
	requestData := req.(callback.CallbackDeleteReq)

	bflName := strings.TrimSpace(requestData.Name)
	username := bflName + "@auth.local"

	err := seahub.RemoveUser(username)
	if err != nil {
		return nil, consts.StatusInternalServerError, err
	}
	return nil, 0, nil
}
