package integration

import (
	"files/pkg/common"
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/hertz/biz/model/api/external"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
)

var UserGVR = schema.GroupVersionResource{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "users"}

var IntegrationService *integration

type integration struct {
	client     *dynamic.DynamicClient
	kubeClient *kubernetes.Clientset
	rest       *resty.Client
	tokens     map[string]*Integrations
	authToken  map[string]*authToken

	sync.RWMutex
}

type authToken struct {
	token  string
	expire time.Time
}

func NewIntegrationManager() {
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("Failed to create kube client: %v", err)
		panic(err)
	}

	IntegrationService = &integration{
		client:     client,
		kubeClient: kubeClient,
		rest:       resty.New().SetTimeout(60 * time.Second).SetDebug(true),
		tokens:     make(map[string]*Integrations),
		authToken:  make(map[string]*authToken),
	}

	IntegrationService.watch()
}

func IntegrationManager() *integration {
	return IntegrationService
}

func (i *integration) watch() {
	go func() {
		for range time.NewTicker(15 * time.Second).C {
			i.GetIntegrations()
		}
	}()
}

func (i *integration) HandlerEvent() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// klog.Infof("ingegrateion add func: %s", common.ToJson(obj))
				if err := i.GetIntegrations(); err != nil {
					klog.Errorf("get users error: %v", err)
				}
			},
			DeleteFunc: func(obj interface{}) {
				// klog.Infof("ingegrateion delete func: %s", common.ToJson(obj))
				if err := i.GetIntegrations(); err != nil {
					klog.Errorf("get users error: %v", err)
				}
			},
		},
	}
}

func (i *integration) GetAccounts(owner string) ([]*external.AccountInfo, error) {
	at, err := i.getAuthToken(owner)
	if err != nil {
		klog.Errorf("get auth token error: %v", err)
		return nil, err
	}

	accounts, err := i.getAccounts(owner, at)
	if err != nil {
		return nil, err
	}

	var result []*external.AccountInfo

	for _, ac := range accounts {
		var ai = &external.AccountInfo{
			Name:      ac.Name,
			Type:      ac.Type,
			Available: ac.Available,
			CreateAt:  ac.CreateAt,
			ExpiresAt: ac.ExpiresAt,
		}
		result = append(result, ai)
	}

	return result, nil
}

func (i *integration) GetIntegrations() error {
	i.Lock()
	defer i.Unlock()

	users, err := i.getUsers()
	if err != nil {
		return err
	}

	if users == nil || len(users) == 0 {
		return fmt.Errorf("users not exists")
	}

	var configs []*config.Config

	for _, user := range users {
		at, err := i.getAuthToken(user.Name)
		if err != nil {
			klog.Errorf("get auth token error: %v", err)
			continue
		}

		accounts, err := i.getAccounts(user.Name, at)

		if err != nil {
			klog.Errorf("get user accounts failed, user: %s, error: %s", user.Name, err)
			continue
		}

		if accounts == nil || len(accounts) == 0 {
			continue
		}

		klog.Infof("integration get accounts, user: %s, count: %d", user.Name, len(accounts))

		for _, acc := range accounts {
			flag, existsToken, err := i.checkTokenExpired(user.Name, acc.Name)

			if flag {
				klog.Infof("integration, check expired, name: %s, accName: %s, expired: %v, err: %v", user.Name, acc.Name, flag, err)
			}

			if err == nil && !flag {
				var config = &config.Config{
					ConfigName:   fmt.Sprintf("%s_%s_%s", user.Name, existsToken.Type, existsToken.Name),
					Name:         existsToken.Name,
					Type:         i.parseToRcloneType(existsToken.Type),
					Provider:     i.parseToRcloneProvider(existsToken.Type),
					AccessToken:  existsToken.AccessKey,
					RefreshToken: existsToken.SecretKey,
					Url:          existsToken.Endpoint,
					Endpoint:     i.parseEndpoint(existsToken.Endpoint),
					Bucket:       i.parseBucket(existsToken.Endpoint),
					ClientId:     existsToken.ClientId, // only for dropbox
					ExpiresAt:    existsToken.ExpiresAt,
				}

				configs = append(configs, config)
			} else {
				token, err := i.getToken(user.Name, acc.Name, acc.Type, at)
				if err != nil {
					klog.Errorf("get token error: %v, user: %s, name: %s", err, user.Name, acc.Name)
					continue
				}

				if token == nil || token.RawData == nil {
					klog.Infof("token not exists, skip, user: %s, name: %s", user.Name, acc.Name)
					continue
				}

				var newToken bool
				var getToken = &IntegrationToken{
					Owner:     user.Name,
					Name:      token.Name,
					Type:      token.Type,
					AccessKey: token.RawData.AccessToken,
					SecretKey: token.RawData.RefreshToken,
					Endpoint:  token.RawData.Endpoint,
					Bucket:    token.RawData.Bucket,
					ExpiresAt: token.RawData.ExpiresAt,
					Available: token.RawData.Available,
					Scope:     token.RawData.Scope,
					IdToken:   token.RawData.IdToken,
					ClientId:  token.ClientId, // or ?token.RawData.ClientId,
				}

				userTokens, userExists := i.tokens[user.Name]
				if !userExists {
					newToken = true
					var tokenSets = make(map[string]*IntegrationToken)
					tokenSets[token.Name] = getToken
					userTokens = &Integrations{
						Tokens: tokenSets,
					}
					i.tokens[user.Name] = userTokens
				} else {
					val, tokenExists := userTokens.Tokens[token.Name]
					if !tokenExists {
						newToken = true
						userTokens.Tokens[token.Name] = getToken
					} else {
						if e := reflect.DeepEqual(val, getToken); !e {
							newToken = true
							userTokens.Tokens[token.Name] = getToken
						}
					}
				}

				if newToken {
					var config = &config.Config{
						ConfigName:   fmt.Sprintf("%s_%s_%s", user.Name, token.Type, token.Name),
						Name:         token.Name,
						Type:         i.parseToRcloneType(token.Type),
						Provider:     i.parseToRcloneProvider(token.Type),
						AccessToken:  token.RawData.AccessToken,
						RefreshToken: token.RawData.RefreshToken,
						Url:          token.RawData.Endpoint,
						Endpoint:     i.parseEndpoint(token.RawData.Endpoint),
						Bucket:       i.parseBucket(token.RawData.Endpoint),
						ClientId:     token.ClientId, // only for dropbox
						ExpiresAt:    token.RawData.ExpiresAt,
					}

					configs = append(configs, config)
				}
			}

		}
	}

	if len(configs) == 0 {
		return nil
	}

	// klog.Infof("integration new configs: %d", len(configs))

	rclone.Command.StartHttp(configs)

	return nil
}

func (i *integration) parseEndpoint(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}

	var hosts = strings.Split(u.Host, ".")
	if len(hosts) < 4 || len(hosts) > 5 {
		return ""
	}

	if len(hosts) == 4 {
		return u.Host
	}

	return strings.Join(hosts[1:], ".")

}

func (i *integration) parseBucket(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}

	var hosts = strings.Split(u.Host, ".")
	var paths = strings.Split(strings.TrimLeft(u.Path, "/"), "/")

	if len(hosts) < 4 || len(hosts) > 5 {
		return ""
	}

	var bucket string
	if len(hosts) == 4 {
		bucket = paths[0]
	} else {
		bucket = hosts[0]
	}

	return bucket
}

func (i *integration) parseToRcloneType(s string) string {
	if s == common.AwsS3 || s == common.TencentCos {
		return common.RcloneTypeS3
	} else if s == common.DropBox {
		return common.RcloneTypeDropbox
	} else if s == common.GoogleDrive {
		return common.RcloneTypeDrive
	}
	return common.RcloneTypeLocal
}

func (i *integration) parseToRcloneProvider(s string) string {
	if s == common.AwsS3 {
		return common.ProviderAWS
	} else if s == common.TencentCos {
		return common.ProviderTencentCOS
	}
	return ""
}

func (i *integration) checkTokenExpired(user string, tokenName string) (bool, *IntegrationToken, error) {
	v, ok := i.tokens[user]
	if !ok {
		return false, nil, fmt.Errorf("user not found")
	}

	t, ok := v.Tokens[tokenName]
	if !ok {
		return false, nil, fmt.Errorf("token not found")
	}

	if t.ExpiresAt == 0 {
		return false, t, nil
	}

	return i.checkExpired(t.ExpiresAt), t, nil

}
