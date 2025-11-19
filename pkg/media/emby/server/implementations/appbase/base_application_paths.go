package appbase

import (
	"fmt"
	"os"
	"path/filepath"
)

// BaseApplicationPaths implements the ApplicationPaths interface.
type BaseApplicationPaths struct {
	programDataPath             string
	webPath                     string
	programSystemPath           string
	dataPath                    string
	virtualDataPath             string
	imageCachePath              string
	pluginsPath                 string
	pluginConfigurationsPath    string
	logDirectoryPath            string
	configurationDirectoryPath  string
	systemConfigurationFilePath string
	cachePath                   string
	tempDirectory               string
	trickplayPath               string
	backupPath                  string
}

// NewBaseApplicationPaths initializes a new BaseApplicationPaths instance.
func NewBaseApplicationPaths(programDataPath, logDirectoryPath, configurationDirectoryPath, cacheDirectoryPath, webDirectoryPath string) *BaseApplicationPaths {
	dataPath := filepath.Join(programDataPath, "data")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		panic(fmt.Errorf("failed to create data directory: %w", err))
	}

	return &BaseApplicationPaths{
		programDataPath:             programDataPath,
		webPath:                     webDirectoryPath,
		programSystemPath:           os.Args[0], // Equivalent to AppContext.BaseDirectory
		dataPath:                    dataPath,
		virtualDataPath:             "%AppDataPath%",
		imageCachePath:              filepath.Join(cacheDirectoryPath, "images"),
		pluginsPath:                 filepath.Join(programDataPath, "plugins"),
		pluginConfigurationsPath:    filepath.Join(programDataPath, "plugins", "configurations"),
		logDirectoryPath:            logDirectoryPath,
		configurationDirectoryPath:  configurationDirectoryPath,
		systemConfigurationFilePath: filepath.Join(configurationDirectoryPath, "system.xml"),
		cachePath:                   cacheDirectoryPath,
		tempDirectory:               filepath.Join(os.TempDir(), "jellyfin"),
		trickplayPath:               filepath.Join(dataPath, "trickplay"),
		backupPath:                  filepath.Join(dataPath, "backups"),
	}
}

// ProgramDataPath returns the program data path.
func (b *BaseApplicationPaths) ProgramDataPath() string {
	return b.programDataPath
}

// WebPath returns the web directory path.
func (b *BaseApplicationPaths) WebPath() string {
	return b.webPath
}

// ProgramSystemPath returns the program system path.
func (b *BaseApplicationPaths) ProgramSystemPath() string {
	return b.programSystemPath
}

// DataPath returns the data path.
func (b *BaseApplicationPaths) DataPath() string {
	return b.dataPath
}

// VirtualDataPath returns the virtual data path.
func (b *BaseApplicationPaths) VirtualDataPath() string {
	return b.virtualDataPath
}

// ImageCachePath returns the image cache path.
func (b *BaseApplicationPaths) ImageCachePath() string {
	return b.imageCachePath
}

// PluginsPath returns the plugins path.
func (b *BaseApplicationPaths) PluginsPath() string {
	return b.pluginsPath
}

// PluginConfigurationsPath returns the plugin configurations path.
func (b *BaseApplicationPaths) PluginConfigurationsPath() string {
	return b.pluginConfigurationsPath
}

// LogDirectoryPath returns the log directory path.
func (b *BaseApplicationPaths) LogDirectoryPath() string {
	return b.logDirectoryPath
}

// ConfigurationDirectoryPath returns the configuration directory path.
func (b *BaseApplicationPaths) ConfigurationDirectoryPath() string {
	return b.configurationDirectoryPath
}

// SystemConfigurationFilePath returns the system configuration file path.
func (b *BaseApplicationPaths) SystemConfigurationFilePath() string {
	return b.systemConfigurationFilePath
}

// CachePath returns the cache path.
func (b *BaseApplicationPaths) CachePath() string {
	return b.cachePath
}

// SetCachePath sets the cache path.
func (b *BaseApplicationPaths) SetCachePath(path string) {
	b.cachePath = path
	b.imageCachePath = filepath.Join(path, "images")
}

// TempDirectory returns the temporary directory path.
func (b *BaseApplicationPaths) TempDirectory() string {
	return b.tempDirectory
}

// TrickplayPath returns the trickplay path.
func (b *BaseApplicationPaths) TrickplayPath() string {
	return b.trickplayPath
}

// BackupPath returns the backup path.
func (b *BaseApplicationPaths) BackupPath() string {
	return b.backupPath
}

// MakeSanityCheckOrThrow performs sanity checks on critical directories.
func (b *BaseApplicationPaths) MakeSanityCheckOrThrow() error {
	if err := b.CreateAndCheckMarker(b.ConfigurationDirectoryPath(), "config", false); err != nil {
		return err
	}
	if err := b.CreateAndCheckMarker(b.LogDirectoryPath(), "log", false); err != nil {
		return err
	}
	if err := b.CreateAndCheckMarker(b.PluginsPath(), "plugin", false); err != nil {
		return err
	}
	if err := b.CreateAndCheckMarker(b.ProgramDataPath(), "data", false); err != nil {
		return err
	}
	if err := b.CreateAndCheckMarker(b.CachePath(), "cache", false); err != nil {
		return err
	}
	if err := b.CreateAndCheckMarker(b.DataPath(), "data", false); err != nil {
		return err
	}
	return nil
}

// CreateAndCheckMarker creates a directory and ensures a marker file exists.
func (b *BaseApplicationPaths) CreateAndCheckMarker(path, markerName string, recursive bool) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return b.checkOrCreateMarker(path, ".media-server-"+markerName, recursive)
}

// getMarkers retrieves marker files in the given path.
func (b *BaseApplicationPaths) getMarkers(path string, recursive bool) ([]string, error) {
	var markers []string
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Base(p)[:9] == ".media-server-" {
			markers = append(markers, p)
		}
		if !recursive && p != path {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate files in %s: %w", path, err)
	}
	return markers, nil
}

// checkOrCreateMarker checks for a specific marker file and creates it if absent.
func (b *BaseApplicationPaths) checkOrCreateMarker(path, markerName string, recursive bool) error {
	markers, err := b.getMarkers(path, recursive)
	if err != nil {
		return err
	}

	for _, marker := range markers {
		if filepath.Base(marker) != markerName {
			return fmt.Errorf("expected to find only %s but found marker for %s", markerName, marker)
		}
	}

	markerPath := filepath.Join(path, markerName)
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		file, err := os.Create(markerPath)
		if err != nil {
			return fmt.Errorf("failed to create marker file %s: %w", markerPath, err)
		}
		file.Close()
	}
	return nil
}
