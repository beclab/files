package net

import (
// "github.com/your-package-name/configuration"
)

// NetworkConfigurationFactory implements the IConfigurationFactory interface
type NetworkConfigurationFactory struct{}

// GetConfigurations returns a slice of configuration stores
func (n *NetworkConfigurationFactory) GetConfigurations() []configuration.ConfigurationStore {
	return []configuration.ConfigurationStore{
		configuration.ConfigurationStore{},
	}
}
