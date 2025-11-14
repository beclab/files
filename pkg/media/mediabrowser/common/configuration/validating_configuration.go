package configuration

// IValidatingConfiguration defines a configuration store that can be validated.
type IValidatingConfiguration interface {
	// Validate is called before saving the configuration.
	// oldConfig is the existing configuration, newConfig is the proposed configuration.
	Validate(oldConfig, newConfig interface{}) error
}
