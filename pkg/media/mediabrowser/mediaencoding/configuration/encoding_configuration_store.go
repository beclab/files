package configuration

import (
	"errors"
	"os"
	"reflect"

	"files/pkg/media/mediabrowser/model/configuration"
)

type IValidatingConfiguration interface {
	Validate(oldConfig, newConfig interface{}) error
}

type EncodingConfigurationStore struct {
	Key               string
	ConfigurationType reflect.Type
}

func NewEncodingConfigurationStore() *EncodingConfigurationStore {
	return &EncodingConfigurationStore{
		ConfigurationType: reflect.TypeOf(configuration.EncodingOptions{}),
		Key:               "encoding",
	}
}

func (s *EncodingConfigurationStore) Validate(oldConfig, newConfig interface{}) error {
	oldOptions, ok := oldConfig.(*configuration.EncodingOptions)
	if !ok {
		return errors.New("invalid oldConfig type")
	}
	newOptions, ok := newConfig.(*configuration.EncodingOptions)
	if !ok {
		return errors.New("invalid newConfig type")
	}

	if newOptions.TranscodingTempPath != "" &&
		oldOptions.TranscodingTempPath != newOptions.TranscodingTempPath {
		// Validate
		if _, err := os.Stat(newOptions.TranscodingTempPath); os.IsNotExist(err) {
			return errors.New(newOptions.TranscodingTempPath + " does not exist")
		}
	}

	return nil
}

func (e *EncodingConfigurationStore) GetKey() string {
	return e.Key
}

func (e *EncodingConfigurationStore) GetConfigurationType() reflect.Type {
	return e.ConfigurationType
}
