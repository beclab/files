package config

import (
	"encoding/json"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"net/http"
	"sync"

	"k8s.io/klog/v2"
)

type Interface interface {
	Create(param *Config) error
	Delete(configName string) error
	Dump() (map[string]*Config, error)

	GetServeConfigs() map[string]*Config
	SetConfigs(configs map[string]*Config)
	GetConfig(configName string) (*Config, error)
}

type config struct {
	configs map[string]*Config // {owner}_{fileType}_{ACCESS_KEY}
	sync.RWMutex
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
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CreateConfigPath)
	var data = &CreateConfig{
		Name: param.Name,
		Type: c.parseType(param.Type),
		Parameters: &ConfigParameters{
			Name:            param.AccessKeyId,
			Provider:        c.parseProvider(param.Type),
			AccessKeyId:     param.AccessKeyId,
			SecretAccessKey: param.SecretAccessKey,
			Url:             param.Url,
			Endpoint:        param.Endpoint,
			Bucket:          param.Bucket,
			ClientId:        param.ClientId,
			ClientSecret:    param.RefreshToken,
			Token:           param.SecretAccessKey,
		},
	}

	klog.Infof("[rclone] create config param: %s", commonutils.ToJson(data))
	_, err := utils.Request(url, http.MethodPost, commonutils.ToBytes(data))
	if err != nil {
		return err
	}

	c.configs[param.Name] = param

	klog.Infof("[rclone] create config success, name: %s", param.Name)
	return nil

}

func (c *config) Delete(configName string) error {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DeleteConfigPath)
	var data = map[string]string{
		"name": configName,
	}
	_, err := utils.Request(url, http.MethodPost, commonutils.ToBytes(data))
	if err != nil {
		return err
	}

	delete(c.configs, configName)

	klog.Infof("[rclone] delete config success, name: %s", configName)

	return nil
}

func (c *config) Dump() (map[string]*Config, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, DumpConfigPath)
	resp, err := utils.Request(url, http.MethodPost, []byte("{}"))
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
		return nil, fmt.Errorf("config not found, name: %s", configName)
	}

	return val, nil
}

func (c *config) parseType(s string) string {
	switch s {
	case "awss3", "tencent":
		return "s3"
	case "dropbox":
		return "dropbox"
	case "google":
		return "googledrive"
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
