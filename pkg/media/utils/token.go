package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	restful "github.com/emicklei/go-restful/v3"
	rest "github.com/go-resty/resty/v2"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ptr "k8s.io/utils/ptr"
)

var authToken = make(map[string]authv1.TokenRequestStatus)

func GetAuthToken(owner string) (string, error) {
	at, ok := authToken[owner]
	if ok {
		if time.Now().Before(at.ExpirationTimestamp.Time.Add(-5 * time.Minute)) {
			return at.Token, nil
		}
	}

	namespace := fmt.Sprintf("user-system-%s", owner)
	tr := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{"https://kubernetes.default.svc.cluster.local"},
			ExpirationSeconds: ptr.To[int64](86400), // one day
		},
	}

	clientset, err := GetKubernetesClient()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes client: %v", err)
	}

	token, err := clientset.CoreV1().ServiceAccounts(namespace).
		CreateToken(context.Background(), "user-backend", tr, metav1.CreateOptions{})
	if err != nil {
		// klog.Errorf("Failed to create token for user %s in namespace %s: %v", owner, namespace, err)
		return "", fmt.Errorf("failed to create token for user %s in namespace %s: %v", owner, namespace, err)
	}
	fmt.Printf("%+v\n", token)

	if !ok {
		at = authv1.TokenRequestStatus{}
	}
	at = token.Status

	authToken[owner] = at

	return at.Token, nil
}

func GetToken(owner string, accountName string, accountType string, authToken string) (*accountResponseData, error) {
	settingsUrl := fmt.Sprintf("http://settings.user-system-%s:28080/api/account/retrieve", owner)

	var data = make(map[string]string)
	data["name"] = formatUrl(accountType, accountName)
	klog.Infof("fetch integration from settings: %s %s", settingsUrl, data["name"])
	resp, err := rest.New().R().SetHeader(restful.HEADER_ContentType, restful.MIME_JSON).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", authToken)).
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

	if accountResp.Data == nil || accountResp.Data.RawData == nil {
		return nil, errors.New("account response data or raw data is nil")
	}
	fmt.Printf("accountResp.Data: %+v\n", accountResp.Data)
	fmt.Printf("accountResp.Data.RawData: %+v\n", accountResp.Data.RawData)
	return accountResp.Data, nil
}

func formatUrl(location, name string) string {
	var l string
	switch location {
	case "awss3":
		l = "awss3"
	case "dropbox":
		l = "dropbox"
	case "google":
		l = "google"
	case "tencent": // from settings api
		l = "tencent"
	}
	return fmt.Sprintf("integration-account:%s:%s", l, name)
}

type Header struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type accountResponse struct {
	Header
	Data *accountResponseData `json:"data,omitempty"`
}

type accountResponseData struct {
	Name     string                  `json:"name"`
	Type     string                  `json:"type"`
	RawData  *accountResponseRawData `json:"raw_data"`
	CloudUrl string                  `json:"cloudUrl"`
	ClientId string                  `json:"client_id"`
}

type accountResponseRawData struct {
	ExpiresAt    int64  `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	Endpoint     string `json:"endpoint"`
	Bucket       string `json:"bucket"`
	UserId       string `json:"userid"`
	Available    bool   `json:"available"`
	CreateAt     int64  `json:"create_at"`
	Scope        string `json:"scope"`
	IdToken      string `json:"id_token"`
	ClientId     string `json:"client_id"`
}
