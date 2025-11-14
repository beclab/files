package configuration

// IApplicationPaths defines the interface for application paths
type IApplicationPaths interface {
	// ProgramDataPath returns the path to the program data folder
	ProgramDataPath() string

	// WebPath returns the path to the web UI resources folder
	// This value is not relevant if the server is configured to not host any static web content
	WebPath() string

	// ProgramSystemPath returns the path to the program system folder
	ProgramSystemPath() string

	// DataPath returns the folder path to the data directory
	DataPath() string

	// ImageCachePath returns the image cache path
	ImageCachePath() string

	// PluginsPath returns the path to the plugin directory
	PluginsPath() string

	// PluginConfigurationsPath returns the path to the plugin configurations directory
	PluginConfigurationsPath() string

	// LogDirectoryPath returns the path to the log directory
	LogDirectoryPath() string

	// ConfigurationDirectoryPath returns the path to the application configuration root directory
	ConfigurationDirectoryPath() string

	// SystemConfigurationFilePath returns the path to the system configuration file
	SystemConfigurationFilePath() string

	// CachePath returns the folder path to the cache directory
	CachePath() string

	// TempDirectory returns the folder path to the temp directory within the cache folder
	TempDirectory() string

	// VirtualDataPath returns the magic string used for virtual path manipulation
	VirtualDataPath() string

	// TrickplayPath returns the path used for storing trickplay files
	TrickplayPath() string

	// BackupPath returns the path used for storing backup archives
	BackupPath() string

	// MakeSanityCheckOrThrow checks and creates all known base paths
	MakeSanityCheckOrThrow() error

	// CreateAndCheckMarker checks and creates the given path and adds it with a marker file if non-existent
	CreateAndCheckMarker(path, markerName string, recursive bool) error
}
