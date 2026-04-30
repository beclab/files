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
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

func (i *integration) getAccounts(owner string, authToken string) ([]*accountsResponseData, error) {
	var integrationUrl = fmt.Sprintf("%s/api/account/list", common.DefaultIntegrationProviderUrl)
	var data = make(map[string]string)
	data["user"] = owner

	var header = make(map[string]string)
	header[common.REQUEST_HEADER_AUTHORIZATION] = fmt.Sprintf("Bearer %s", authToken)

	resp, err := common.Request(integrationUrl, http.MethodPost, header, data, false)
	if err != nil {
		return nil, err
	}

	var result *accountsResponse

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
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

	var header = make(map[string]string)
	header[restful.HEADER_ContentType] = restful.MIME_JSON
	header[common.REQUEST_HEADER_AUTHORIZATION] = fmt.Sprintf("Bearer %s", authToken)

	resp, err := common.Request(integrationUrl, http.MethodPost, header, data, false)

	if err != nil {
		return nil, err
	}

	var result *accountResponse

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
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
	// Fast path: cache hit under the dedicated mutex. Release before any
	// network IO so a slow K8s call cannot block other owners.
	i.authTokenMu.Lock()
	if at, ok := i.authToken[owner]; ok && time.Now().Before(at.expire) {
		token := at.token
		i.authTokenMu.Unlock()
		return token, nil
	}
	i.authTokenMu.Unlock()

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

	// Two goroutines may have raced into CreateToken; whoever runs second
	// just overwrites with its own freshly-issued token. The duplicated
	// signing call is acceptable (every owner only goes through this every
	// ~11h) and keeps the implementation free of singleflight.
	i.authTokenMu.Lock()
	defer i.authTokenMu.Unlock()
	i.authToken[owner] = &authToken{
		token:  token.Status.Token,
		expire: time.Now().Add(40000 * time.Second),
	}
	return token.Status.Token, nil
}
