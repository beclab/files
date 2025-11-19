package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"files/pkg/hertz/biz/handler"
	"files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"

	"github.com/cloudwego/hertz/pkg/app"

	"k8s.io/klog/v2"
)

type ConfigurationController struct {
	configurationManager configuration.IServerConfigurationManager
	mediaEncoder         mediaencoding.IMediaEncoder
}

func NewConfigurationController(configurationManager configuration.IServerConfigurationManager, mediaEncoder mediaencoding.IMediaEncoder) *ConfigurationController {
	return &ConfigurationController{
		configurationManager: configurationManager,
		mediaEncoder:         mediaEncoder,
	}
}

func (c *ConfigurationController) GetNamedConfiguration(ctx context.Context, r *app.RequestContext) {
	key := r.Param("key")
	klog.Infof("[media] GetNamedConfiguration key: %s", key)

	configuration, err := c.configurationManager.GetConfiguration(key)
	if err != nil {
		klog.Errorf("[media] GetNamedConfiguration, get config error: %v", err)
		handler.RespStatusInternalServerError(r, "failed to get configuration")
		return
	}

	jsonData, err := json.Marshal(configuration)
	if err != nil {
		klog.Errorf("[media] GetNamedConfiguration, error marshaling to JSON: %v", err)
		handler.RespStatusInternalServerError(r, "error marshaling to JSON")
		return
	}

	r.Response.Header.Set("Content-Type", "application/json")
	r.JSON(200, string(jsonData))
}

func (c *ConfigurationController) UpdateNamedConfiguration(ctx context.Context, r *app.RequestContext) {
	key := r.Param("key")
	klog.Infof("[media] UpdateNamedConfiguration key: %s", key)

	var err error
	var config json.RawMessage
	err = r.BindAndValidate(&config)
	if err != nil {
		handler.RespBadRequest(r, "invalid configuration body")
		return
	}

	configType, err := c.configurationManager.GetConfigurationType(key)
	if err != nil {
		handler.RespStatusInternalServerError(r, "failed to get configuration type")
		return
	}

	klog.Info("[media] UpdateNamedConfiguration config type:", configType)
	// Create a new instance of the configuration type
	configInstance := reflect.New(configType).Interface()

	// Deserialize the JSON into the configuration type
	if err := json.Unmarshal(config, configInstance); err != nil {
		klog.Errorf("[media] UpdateNamedConfiguration, err: %v", err)
		handler.RespBadRequest(r, "failed to deserialize configuration")
		return
	}

	err = c.configurationManager.SaveConfigurationByKey(key, configInstance)
	if err != nil {
		klog.Errorf("[media] UpdateNamedConfiguration, save config by key error: %v", err)
		handler.RespStatusInternalServerError(r, "failed to save configuration")
		return
	}

	r.SetStatusCode(http.StatusNoContent)
}
