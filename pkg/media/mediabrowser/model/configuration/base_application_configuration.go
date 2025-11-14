package configuration

import (
	"github.com/blang/semver/v4"
)

// BaseApplicationConfiguration represents the base application configuration
type BaseApplicationConfiguration struct {
	// LogFileRetentionDays is the number of days to retain log files
	LogFileRetentionDays int

	// IsStartupWizardCompleted indicates whether this is the first run
	IsStartupWizardCompleted bool

	// CachePath is the path for cache storage
	CachePath *string

	// PreviousVersion is the last known version that was run
	PreviousVersion *semver.Version
}

// NewBaseApplicationConfiguration initializes a new BaseApplicationConfiguration
func NewBaseApplicationConfiguration() *BaseApplicationConfiguration {
	return &BaseApplicationConfiguration{
		LogFileRetentionDays: 3,
	}
}

// PreviousVersionStr returns the string representation of PreviousVersion
func (c *BaseApplicationConfiguration) PreviousVersionStr() string {
	if c.PreviousVersion == nil {
		return ""
	}
	return c.PreviousVersion.String()
}

// SetPreviousVersionStr sets PreviousVersion from a string
func (c *BaseApplicationConfiguration) SetPreviousVersionStr(value string) error {
	if value == "" {
		c.PreviousVersion = nil
		return nil
	}
	version, err := semver.Parse(value)
	if err != nil {
		return err
	}
	c.PreviousVersion = &version
	return nil
}
