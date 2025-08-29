package integration

import (
	"context"
	"files/pkg/common"
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

func (i *integration) getAccounts(owner string, authToken string) ([]*accountsResponseData, error) {
	settingsUrl := fmt.Sprintf("http://settings.user-system-%s:28080/api/account/all", owner)

	// klog.Infof("fetch integration from settings: %s", settingsUrl)
	resp, err := i.rest.SetDebug(false).R().SetHeader(common.REQUEST_HEADER_AUTHORIZATION, fmt.Sprintf("Bearer %s", authToken)).
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

func (i *integration) getToken(owner string, accountName string, accountType string, authToken string) (*accountResponseData, error) {
	settingsUrl := fmt.Sprintf("http://settings.user-system-%s:28080/api/account/retrieve", owner)

	var data = make(map[string]string)
	data["name"] = i.formatUrl(accountType, accountName)
	klog.Infof("fetch integration from settings: %s", settingsUrl)
	resp, err := i.rest.R().SetHeader(restful.HEADER_ContentType, restful.MIME_JSON).
		SetHeader(common.REQUEST_HEADER_AUTHORIZATION, fmt.Sprintf("Bearer %s", authToken)).
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

func (i *integration) getAuthToken(owner string) (string, error) {
	at, ok := i.authToken[owner]
	if ok {
		if time.Now().Before(at.expire) {
			return at.token, nil
		}
	}

	namespace := fmt.Sprintf("user-system-%s", owner)
	tr := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{"https://kubernetes.default.svc.cluster.local"},
			ExpirationSeconds: ptr.To[int64](86400), // one day
		},
	}

	token, err := i.kubeClient.CoreV1().ServiceAccounts(namespace).
		CreateToken(context.Background(), "user-backend", tr, metav1.CreateOptions{})
	if err != nil {
		// klog.Errorf("Failed to create token for user %s in namespace %s: %v", owner, namespace, err)
		return "", fmt.Errorf("failed to create token for user %s in namespace %s: %v", owner, namespace, err)
	}

	if !ok {
		at = &authToken{}
	}
	at.token = token.Status.Token
	at.expire = time.Now().Add(82800 * time.Second)

	i.authToken[owner] = at

	return at.token, nil
}
