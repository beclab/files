package configuration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	//	"strings"
	"reflect"
	"sync"

	//"go.uber.org/zap"

	"files/pkg/media/emby/server/implementations/appbase"
	"files/pkg/media/mediabrowser/controller"
	"files/pkg/media/mediabrowser/model/configuration"
	"files/pkg/media/utils"

	cc "files/pkg/media/mediabrowser/common/configuration"
	"files/pkg/media/mediabrowser/model/serialization"
)

// GenericEventArgs represents a generic event argument
type GenericEventArgs[T any] struct {
	Value T
}

// EventHandler represents an event handler function
type EventHandler[T any] func(sender interface{}, args GenericEventArgs[T])

// ServerConfigurationManager implements IServerConfigurationManager
type ServerConfigurationManager struct {
	*appbase.BaseConfigurationManager
	ConfigurationUpdating []EventHandler[*configuration.ServerConfiguration]
	mu                    sync.RWMutex
}

// NewServerConfigurationManager initializes a new ServerConfigurationManager
func NewServerConfigurationManager(
	applicationPaths cc.IApplicationPaths,
	loggerFactory *utils.Logger,
	serializer serialization.ISerializer,
) *ServerConfigurationManager {
	scm := &ServerConfigurationManager{
		BaseConfigurationManager: appbase.NewBaseConfigurationManager(applicationPaths, loggerFactory, serializer, reflect.TypeOf(configuration.ServerConfiguration{})),
	}
	scm.UpdateMetadataPath()
	return scm
}

// ConfigurationType returns the type of the configuration
//func (scm *ServerConfigurationManager) ConfigurationType() string {
//	return "ServerConfiguration"
//}

// ConfigurationType returns the type of the configuration
func (scm *ServerConfigurationManager) ConfigurationType() reflect.Type {
	fmt.Println("who calllllllllllllllllllllllllllllllllllllllllllllll")
	return reflect.TypeOf(configuration.ServerConfiguration{})
}

// ApplicationPaths returns the server application paths
func (scm *ServerConfigurationManager) ApplicationPaths() controller.IServerApplicationPaths {
	return scm.BaseConfigurationManager.CommonApplicationPaths().(controller.IServerApplicationPaths)
}

// Configuration returns the server configuration
func (scm *ServerConfigurationManager) Configuration() *configuration.ServerConfiguration {
	return &configuration.ServerConfiguration{
		BaseApplicationConfiguration: scm.BaseConfigurationManager.CommonConfiguration(),
	}
	//return scm.BaseConfigurationManager.CommonConfiguration().(*configuration.ServerConfiguration)
}

// OnConfigurationUpdated is called when configuration is updated
func (scm *ServerConfigurationManager) OnConfigurationUpdated() {
	scm.UpdateMetadataPath()
	scm.BaseConfigurationManager.OnConfigurationUpdated()
}

// UpdateMetadataPath updates the metadata path
func (scm *ServerConfigurationManager) UpdateMetadataPath() error {
	/*
		paths := scm.ApplicationPaths().(controller.IServerApplicationPaths)
		config := scm.Configuration()

		metadataPath := config.MetadataPath
		if metadataPath == "" {
			metadataPath = paths.DefaultInternalMetadataPath()
		}

		paths.InternalMetadataPath = metadataPath

		// Create directory if it doesn't exist
		err := os.MkdirAll(paths.InternalMetadataPath(), 0755)
		if err != nil {
			return errors.New("failed to create metadata directory: " + err.Error())
		}
	*/

	return nil
}

// ReplaceConfiguration replaces the current configuration with a new one
func (scm *ServerConfigurationManager) ReplaceConfiguration(newConfiguration *configuration.BaseApplicationConfiguration) error {
	newConfig := &configuration.ServerConfiguration{
		BaseApplicationConfiguration: scm.BaseConfigurationManager.CommonConfiguration(),
	}
	/*
		newConfig, ok := newConfiguration.(*ServerConfiguration)
		if !ok {
			return errors.New("invalid configuration type")
		}
	*/

	if err := scm.ValidateMetadataPath(newConfig); err != nil {
		return err
	}

	scm.mu.Lock()
	defer scm.mu.Unlock()

	// Trigger ConfigurationUpdating event
	for _, handler := range scm.ConfigurationUpdating {
		handler(scm, GenericEventArgs[*configuration.ServerConfiguration]{Value: newConfig})
	}

	return scm.BaseConfigurationManager.ReplaceConfiguration(newConfiguration)
}

// ValidateMetadataPath validates the metadata path in the new configuration
func (scm *ServerConfigurationManager) ValidateMetadataPath(newConfig *configuration.ServerConfiguration) error {
	newPath := newConfig.MetadataPath
	currentPath := scm.Configuration().MetadataPath

	if newPath != "" && newPath != currentPath {
		// Check if directory exists
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return errors.New("metadata path does not exist: " + newPath)
		}

		// Check write access
		if err := ensureWriteAccess(newPath); err != nil {
			return err
		}
	}

	return nil
}

// ensureWriteAccess checks if the path is writable
func ensureWriteAccess(path string) error {
	// Attempt to create a temporary file to test write access
	testFile := filepath.Join(path, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return errors.New("no write access to metadata path: " + err.Error())
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

func (scm *ServerConfigurationManager) GetEncodingOptions() *configuration.EncodingOptions {
	options, err := scm.GetConfiguration("encoding")
	if err != nil {
		scm.Logger().Infof("GetEncodingOptions: %v", err)
		return nil
	}
	scm.Logger().Infof("ServerConfigurationManager GetEncodingOptions: %+v", options)
	return options.(*configuration.EncodingOptions)
}

func (scm *ServerConfigurationManager) _GetTranscodePath() string {
	transcodingTempPath := "/tmp/cache/transcodes"
	// Make sure the directory exists
	err := os.MkdirAll(transcodingTempPath, 0755)
	if err != nil {
		fmt.Println(err)
	}

	return transcodingTempPath
}

func (scm *ServerConfigurationManager) GetTranscodePath() string {
	transcodingTempPath := scm.GetEncodingOptions().TranscodingTempPath
	if transcodingTempPath == "" {
		transcodingTempPath = filepath.Join(scm.BaseConfigurationManager.CommonApplicationPaths().CachePath(), "transcodes")
	}

	err := scm.BaseConfigurationManager.CommonApplicationPaths().CreateAndCheckMarker(transcodingTempPath, "transcode", true)
	if err != nil {
		fmt.Println(err)
	}

	return transcodingTempPath
}
