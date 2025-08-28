package integration

import (
	"files/pkg/common"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/emicklei/go-restful/v3"
	"k8s.io/klog/v2"
)

var authTokenPatn = "/var/run/secrets/kubernetes.io/serviceaccount/token"

func (i *integration) getAccounts(owner string) ([]*accountsResponseData, error) {
	settingsUrl := fmt.Sprintf("http://settings.user-system-%s:28080/api/account/all", owner)

	// klog.Infof("fetch integration from settings: %s", settingsUrl)
	resp, err := i.rest.SetDebug(false).R().SetHeader(common.REQUEST_HEADER_AUTHORIZATION, fmt.Sprintf("Bearer %s", i.authToken)).
		SetResult(&accountsResponse{}).
		Get(settingsUrl)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("request status invalid, status code: %d, msg: %s", resp.StatusCode(), string(resp.Body()))
	}

	accountsResp := resp.Result().(*accountsResponse)

	if accountsResp.Code != 0 {
		return nil, fmt.Errorf("get accounts failed, code:  %d, msg: %s", accountsResp.Code, string(resp.Body()))
	}

	return accountsResp.Data, nil
}

func (i *integration) getToken(owner string, accountName string, accountType string) (*accountResponseData, error) {
	settingsUrl := fmt.Sprintf("http://settings.user-system-%s:28080/api/account/retrieve", owner)

	var data = make(map[string]string)
	data["name"] = i.formatUrl(accountType, accountName)
	klog.Infof("fetch integration from settings: %s", settingsUrl)
	resp, err := i.rest.R().SetHeader(restful.HEADER_ContentType, restful.MIME_JSON).
		SetHeader(common.REQUEST_HEADER_AUTHORIZATION, fmt.Sprintf("Bearer %s", i.authToken)).
		SetBody(data).
		SetResult(&accountResponse{}).
		Post(settingsUrl)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("request status invalid, status code: %d", resp.StatusCode())
	}

	accountResp := resp.Result().(*accountResponse)

	if accountResp.Code != 0 {
		return nil, fmt.Errorf("get account failed, code: %d, msg: %s", accountResp.Code, accountResp.Message)
	}

	return accountResp.Data, nil
}

func (i *integration) formatUrl(location, name string) string {
	var l string
	switch location {
	case common.AwsS3:
		l = common.AwsS3
	case common.DropBox:
		l = common.DropBox
	case common.GoogleDrive:
		l = common.GoogleDrive
	case common.TencentCos: // from settings api
		l = common.TencentCos
	}
	return fmt.Sprintf("integration-account:%s:%s", l, name)
}

func (i *integration) checkExpired(expiredAt int64) bool {
	adjustedTimestamp := expiredAt - (15 * 1000)
	currentTimestamp := time.Now().UnixMilli()
	return adjustedTimestamp < currentTimestamp
}

func (i *integration) getAuthToken() error {
	token, err := os.ReadFile(authTokenPatn)
	if err != nil {
		return fmt.Errorf("read auth token file error: %v", err)
	}

	if token == nil || len(token) == 0 {
		return fmt.Errorf("auth token invalid")
	}

	i.authToken = string(token)

	return nil
}
