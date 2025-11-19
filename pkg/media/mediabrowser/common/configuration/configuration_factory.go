package configuration

// IConfigurationFactory defines the interface for retrieving configuration stores
type IConfigurationFactory interface {
	// GetConfigurations returns a slice of configuration stores for this module
	GetConfigurations() []ConfigurationStore
	//GetConfigurations() []interface{}
}
