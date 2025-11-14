package configuration

import (
	//	"fmt"
	"files/pkg/media/mediabrowser/model/configuration"
	"reflect"
	// cc "files/pkg/media/mediabrowser/common/configuration"
)

// ConfigurationUpdateEventArgs represents the event data for configuration updates
type ConfigurationUpdateEventArgs struct {
	// Define fields as needed
}

// IConfigurationManager defines the interface for configuration management
type IConfigurationManager interface {
	/*
	   // CommonApplicationPaths returns the application paths
	   CommonApplicationPaths() cc.IApplicationPaths

	   // CommonConfiguration returns the base application configuration
	   CommonConfiguration() BaseApplicationConfiguration
	*/

	// SaveConfiguration saves the configuration
	SaveConfiguration() error
	/*

	   // ReplaceConfiguration replaces the current configuration with a new one
	   ReplaceConfiguration(newConfiguration BaseApplicationConfiguration)

	   // RegisterConfiguration registers a configuration factory before system initialization
	   RegisterConfiguration(factory IConfigurationFactory)
	*/
	// GetConfiguration retrieves a configuration by key
	GetConfiguration(key string) (interface{}, error)
	// GetConfigurationStores returns an array of configuration stores
	GetConfigurationStores() []ConfigurationStore

	// GetConfigurationType returns the type of configuration for the given key
	GetConfigurationType(key string) (reflect.Type, error)

	// SaveConfigurationWithKey saves a configuration with the specified key
	SaveConfigurationByKey(key string, configuration interface{}) error

	// AddParts adds multiple configuration factories
	AddParts(factories []IConfigurationFactory)

	/*
	   // Events would typically be handled via channels in Go
	   // NamedConfigurationUpdating channel for configuration updating events
	   NamedConfigurationUpdating() chan ConfigurationUpdateEventArgs

	   // ConfigurationUpdated channel for configuration updated events
	   ConfigurationUpdated() chan struct{}

	   // NamedConfigurationUpdated channel for named configuration updated events
	   NamedConfigurationUpdated() chan ConfigurationUpdateEventArgs
	*/
	GetEncodingOptions() *configuration.EncodingOptions
	GetTranscodePath() string
}

// ConfigurationManagerExtensions provides extension-like functionality
type ConfigurationManagerExtensions struct{}

/*
// GetConfiguration retrieves a typed configuration
func GetConfiguration[T any](manager IConfigurationManager, key string) T {
    config, err := manager.GetConfiguration(key)
    if err != nil {
	    fmt.Println(err)
	    return nil
    }
    return config.(T)
}
*/
