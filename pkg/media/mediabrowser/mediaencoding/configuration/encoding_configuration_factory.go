package configuration

import (
	cc "files/pkg/media/mediabrowser/common/configuration"
)

type EncodingConfigurationFactory struct{}

func (f *EncodingConfigurationFactory) GetConfigurations() []cc.ConfigurationStore {
	return []cc.ConfigurationStore{
		NewEncodingConfigurationStore(),
	}
}
