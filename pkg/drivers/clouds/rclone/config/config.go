package config

import (
	"context"
	"encoding/json"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"net/http"
	"strings"
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
	ConfigName: "local",
	Name:       "local",
	Type:       "local",
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
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CreateConfigPath)
	var t = c.parseType(param.Type)
	var data = &CreateConfig{
		Name:       param.ConfigName,
		Type:       c.parseType(param.Type),
		Parameters: c.parseCreateConfigParameters(param),
	}

	klog.Infof("[rclone] create config: %s, param: %s", t, commonutils.ToJson(data))
	_, err := utils.Request(ctx, url, http.MethodPost, nil, []byte(commonutils.ToJson(data)))
	if err != nil {
		klog.Warningf("[rclone] create config, result: %s", err.Error())
		if !strings.Contains(err.Error(), "failed to start auth webserver") {
			return err
		}
	}

	param.Type = c.parseType(param.Type)

	c.configs[param.ConfigName] = param

	klog.Infof("[rclone] create config success, configName: %s", param.ConfigName)
	return nil

}

func (c *config) Delete(configName string) error {
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeleteConfigPath)
	var data = map[string]string{
		"name": configName,
	}
	_, err := utils.Request(ctx, url, http.MethodPost, nil, commonutils.ToBytes(data))
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

	klog.Infof("[rclone] config dump: %s", commonutils.ToJson(configs))

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

	if val.Type == "awss3" || val.Type == "tencent" {
		return val.Bucket, nil
	} else if val.Type == "dropbox" || val.Type == "google" {
		return "", nil
	}

	return "", fmt.Errorf("config not found, configName: %s", configName)
}

func (c *config) parseType(s string) string {
	switch s {
	case "awss3", "tencent":
		return "s3"
	case "dropbox":
		return "dropbox"
	case "google":
		return "drive"
	case "local":
		return "local"
	default:
		return ""
	}
}

func (c *config) parseProvider(s string) string {
	switch s {
	case "awss3":
		return "AWS"
	case "tencent":
		return "TencentCOS"
	default:
		return ""
	}
}

func (c *config) parseCreateConfigParameters(param *Config) *ConfigParameters {
	if param.Type == "awss3" {
		return c.parseS3Params(param)
	} else if param.Type == "dropbox" || param.Type == "google" {
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

	var dropboxToken = &DropBoxToken{
		AccessToken:  param.AccessToken,
		RefreshToken: param.RefreshToken,
		TokenType:    "Bearer",
		Expiry:       "0001-01-01T00:00:00Z", //commonutils.ParseUnixMilli(param.ExpiresAt).String(),
		ExpiresIn:    param.ExpiresIn,
	}

	return &ConfigParameters{
		ConfigName: param.ConfigName,
		Name:       param.Name,
		Token:      commonutils.ToJson(dropboxToken),
	}
}

func (c *config) parseS3Params(param *Config) *ConfigParameters {
	return &ConfigParameters{
		ConfigName:      param.ConfigName,
		Name:            param.Name,
		Provider:        c.parseProvider(param.Type),
		AccessKeyId:     param.Name,
		SecretAccessKey: param.AccessToken,
		Url:             param.Url,
		Endpoint:        param.Endpoint,
		Bucket:          param.Bucket,
	}
}
