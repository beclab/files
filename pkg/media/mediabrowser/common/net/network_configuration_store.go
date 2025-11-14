package net

import (
	"reflect"
	// "github.com/your-package-name/configuration"
)

// NetworkConfigurationStore represents a configuration that stores network-related settings
type NetworkConfigurationStore struct {
	configuration.ConfigurationStore
}

// StoreKey is the name of the configuration in the storage
const StoreKey = "network"

// NewNetworkConfigurationStore initializes a new NetworkConfigurationStore
func NewNetworkConfigurationStore() *NetworkConfigurationStore {
	return &NetworkConfigurationStore{
		ConfigurationStore: configuration.ConfigurationStore{
			Key:               StoreKey,
			ConfigurationType: reflect.TypeOf(NetworkConfiguration{}),
		},
	}
}
