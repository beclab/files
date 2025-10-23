package integration

import (
	"context"
	"encoding/json"
	"files/pkg/common"
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

func (i *integration) getAccounts(owner string, authToken string) ([]*accountsResponseData, error) {
	var integrationUrl = fmt.Sprintf("%s/api/account/list", common.DefaultIntegrationProviderUrl)
	var data = make(map[string]string)
	data["user"] = owner

	var backoff = wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    1,
	}

	var result *accountsResponse

	if e := retry.OnError(backoff, func(err error) bool {
		return true
	}, func() error {
		resp, err := i.rest.R().
			SetHeader(common.REQUEST_HEADER_AUTHORIZATION, fmt.Sprintf("Bearer %s", authToken)).
			SetBody(data).
			SetResult(&accountsResponse{}).
			Post(integrationUrl)

		if err != nil {
			return err
		}

		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return err
		}

		return nil
	}); e != nil {
		return nil, e
	}

	return result.Data, nil
}

func (i *integration) getToken(owner string, accountName string, accountType string, authToken string) (*accountResponseData, error) {
	var integrationUrl = fmt.Sprintf("%s/api/account/retrieve", common.DefaultIntegrationProviderUrl)
	var data = make(map[string]string)
	data["name"] = accountName
	data["type"] = i.formatUrl(accountType)
	data["user"] = owner

	klog.Infof("fetch integration from settings: %s", integrationUrl)
	resp, err := i.rest.R().SetHeader(restful.HEADER_ContentType, restful.MIME_JSON).
		SetHeader(common.REQUEST_HEADER_AUTHORIZATION, fmt.Sprintf("Bearer %s", authToken)).
		SetBody(data).
		SetResult(&accountResponse{}).
		Post(integrationUrl)

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

func (i *integration) formatUrl(location string) string {
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
	return l
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

	tr := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{"https://kubernetes.default.svc.cluster.local"},
			ExpirationSeconds: ptr.To[int64](86400), // one day
		},
	}

	token, err := i.kubeClient.CoreV1().ServiceAccounts(common.DefaultNamespace).
		CreateToken(context.Background(), common.DefaultServiceAccount, tr, metav1.CreateOptions{})
	if err != nil {
		// klog.Errorf("Failed to create token for user %s in namespace %s: %v", owner, namespace, err)
		return "", fmt.Errorf("failed to create token for user %s in namespace %s: %v", owner, common.DefaultNamespace, err)
	}

	if !ok {
		at = &authToken{}
	}
	at.token = token.Status.Token
	at.expire = time.Now().Add(40000 * time.Second)

	i.authToken[owner] = at

	return at.token, nil
}
