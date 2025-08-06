package integration

import (
	"files/pkg/drivers/clouds/rclone"
	"files/pkg/drivers/clouds/rclone/config"
	"files/pkg/utils"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
)

var UserGVR = schema.GroupVersionResource{Group: "iam.kubesphere.io", Version: "v1alpha2", Resource: "users"}

var IntegrationService *integration

type integration struct {
	client *dynamic.DynamicClient
	rest   *resty.Client
	tokens map[string]*Integrations

	sync.RWMutex
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

	IntegrationService = &integration{
		client: client,
		rest:   resty.New().SetTimeout(60 * time.Second).SetDebug(true),
		tokens: make(map[string]*Integrations),
	}

	IntegrationService.watch()
}

func IntegrationManager() *integration {
	return IntegrationService
}

func (i *integration) watch() {
	go func() {
		for range time.NewTicker(60 * time.Second).C {
			// i.GetIntegrations()
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
				klog.Infof("ingegrateion add func")
				if err := i.GetIntegrations(); err != nil {
					klog.Errorf("get users error: %v", err)
				}
			},
			DeleteFunc: func(obj interface{}) {
				klog.Infof("ingegrateion delete func")
				if err := i.GetIntegrations(); err != nil {
					klog.Errorf("get users error: %v", err)
				}
			},
		},
	}
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

	klog.Infof("integration get users: %s", utils.ToJson(users))

	var configs []*config.Config

	for _, user := range users {
		accounts, err := i.getAccounts(user.Name)
		if err != nil {
			klog.Errorf("get user accounts error: %v, user: %s", err, user.Name)
			continue
		}

		if accounts == nil || len(accounts) == 0 {
			continue
		}

		klog.Infof("integration get accounts, user: %s, data: %s", user.Name, utils.ToJson(accounts))

		for _, acc := range accounts {
			token, err := i.getToken(user.Name, acc.Name, acc.Type)
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
					Type:         token.Type,
					AccessToken:  token.RawData.AccessToken,
					RefreshToken: token.RawData.RefreshToken,
					Url:          token.RawData.Endpoint,
					Endpoint:     i.parseEndpoint(token.RawData.Endpoint),
					Bucket:       i.parseBucket(token.RawData.Endpoint),
					ClientId:     token.ClientId, // only for dropbox
				}

				configs = append(configs, config)
			}
		}
	}

	if len(configs) == 0 {
		return nil
	}

	klog.Infof("integration new configs: %s", utils.ToJson(configs))

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
