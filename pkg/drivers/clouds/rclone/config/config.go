package config

import (
	"context"
	"encoding/json"
	common2 "files/pkg/common"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/utils"
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

type Interface interface {
	Create(param *Config) error
	Delete(configName string) error
	Dump() (map[string]*Config, error)

	GetServeConfigs() map[string]*Config
	SetConfigs(configs map[string]*Config)
	GetConfig(configName string) (*Config, error)
	GetFsPath(configName string) (string, error)
}

type config struct {
	configs map[string]*Config // {owner}_{fileType}_{ACCESS_KEY}
	sync.RWMutex
}

var localConfig = &Config{
	ConfigName: common2.Local,
	Name:       common2.Local,
	Type:       common2.Local,
}

var _ Interface = &config{}

func NewConfig() *config {
	return &config{
		configs: make(map[string]*Config),
	}
}

func (c *config) SetConfigs(configs map[string]*Config) {
	c.configs = configs
}

func (c *config) Create(param *Config) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CreateConfigPath)
	var data = &CreateConfig{
		Name:       param.ConfigName,
		Type:       param.Type,
		Parameters: c.parseCreateConfigParameters(param),
	}

	klog.Infof("[rclone] create config: %s, param: %s", param.Type, common2.ToJson(data))
	_, err := utils.Request(ctx, url, http.MethodPost, nil, []byte(common2.ToJson(data)))
	if err != nil {
		klog.Warningf("[rclone] create config, result: %s", err.Error())
	}

	c.configs[param.ConfigName] = param

	klog.Infof("[rclone] create config success, configName: %s", param.ConfigName)
	return nil

}

func (c *config) Delete(configName string) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeleteConfigPath)
	var data = map[string]string{
		"name": configName,
	}
	_, err := utils.Request(ctx, url, http.MethodPost, nil, common2.ToBytes(data))
	if err != nil {
		return err
	}

	delete(c.configs, configName)

	klog.Infof("[rclone] delete config success, configName: %s", configName)

	return nil
}

func (c *config) Dump() (map[string]*Config, error) {
	var ctx, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DumpConfigPath)
	resp, err := utils.Request(ctx, url, http.MethodPost, nil, []byte("{}"))
	if err != nil {
		klog.Errorf("[rclone] dump error: %v", err)
		return nil, err
	}

	var configs map[string]*Config
	if err := json.Unmarshal(resp, &configs); err != nil {
		klog.Errorf("[rclone] dump unmarshal error: %v", err)
		return nil, err
	}

	if configs != nil && len(configs) > 0 {
		for k, v := range configs {
			v.ConfigName = k

			if v.Type == common2.RcloneTypeDropbox || v.Type == common2.RcloneTypeDrive {
				token, err := c.formatToken(v)
				if err != nil {
					klog.Errorf("[rclone] dump config, format token error: %v, configName: %s", err, k)
					continue
				}

				v.AccessToken = token.AccessToken
				v.RefreshToken = token.RefreshToken
				v.ExpiresAt = token.ExpiresAt
			}
		}
	}

	klog.Infof("[rclone] config dump: %d", len(configs))

	return configs, nil
}

func (c *config) GetServeConfigs() map[string]*Config {
	var result = make(map[string]*Config)

	for k, v := range c.configs {
		result[k] = v
	}

	return result
}

func (c *config) GetConfig(configName string) (*Config, error) {
	c.RLock()
	defer c.RUnlock()

	val, ok := c.configs[configName]
	if !ok {
		return nil, fmt.Errorf("config not found, configName: %s", configName)
	}

	return val, nil
}

func (c *config) GetFsPath(configName string) (string, error) {
	c.RLock()
	defer c.RUnlock()

	val, ok := c.configs[configName]
	if !ok {
		return "", fmt.Errorf("config not found, configName: %s", configName)
	}

	if val.Type == common2.RcloneTypeS3 {
		return val.Bucket, nil
	} else if val.Type == common2.RcloneTypeDropbox || val.Type == common2.RcloneTypeDrive {
		return "", nil
	} else if val.Type == common2.RcloneTypeLocal {
		return "", nil
	}

	return "", fmt.Errorf("config fspath not found, configName: %s", configName)
}

func (c *config) parseCreateConfigParameters(param *Config) *ConfigParameters {
	if param.Type == common2.RcloneTypeS3 {
		return c.parseS3Params(param)
	} else if param.Type == common2.RcloneTypeDropbox || param.Type == common2.RcloneTypeDrive {
		return c.parseDropboxParams(param)
	}

	return &ConfigParameters{}
}

func (c *config) parseGoogleDrive(param *Config) *ConfigParameters {
	return &ConfigParameters{
		ConfigName: param.ConfigName,
		Name:       param.Name,
	}
}

func (c *config) parseDropboxParams(param *Config) *ConfigParameters {
	var eat = time.UnixMilli(param.ExpiresAt)
	var dropboxToken = &DropBoxToken{
		AccessToken:  param.AccessToken,
		RefreshToken: param.RefreshToken,
		TokenType:    "bearer",
		Expiry:       eat.Format(time.RFC3339Nano),
		ExpiresAt:    param.ExpiresAt,
	}

	return &ConfigParameters{
		ConfigName: param.ConfigName,
		Name:       param.Name,
		Token:      common2.ToJson(dropboxToken),
	}
}

func (c *config) parseS3Params(param *Config) *ConfigParameters {
	return &ConfigParameters{
		ConfigName:      param.ConfigName,
		Name:            param.Name,
		Provider:        param.Provider,
		AccessKeyId:     param.Name,
		SecretAccessKey: param.AccessToken,
		Url:             param.Url,
		Endpoint:        param.Endpoint,
		Bucket:          param.Bucket,
	}
}

func (c *config) formatToken(config *Config) (*DropBoxToken, error) {
	var t *DropBoxToken
	if err := json.Unmarshal([]byte(config.Token), &t); err != nil {
		return nil, err
	}

	return t, nil
}
