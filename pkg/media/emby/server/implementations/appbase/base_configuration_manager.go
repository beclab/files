package appbase

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cc "files/pkg/media/mediabrowser/common/configuration"
	"files/pkg/media/mediabrowser/model/configuration"
	"files/pkg/media/mediabrowser/model/serialization"
	"files/pkg/media/utils"
	// "files/pkg/media/emby/server/implementations"

	"k8s.io/klog/v2"
)

// ConfigurationUpdateEventArgs holds configuration update event data
type ConfigurationUpdateEventArgs struct {
	Key           string
	Configuration interface{}
}

// IValidatingConfiguration defines the interface for validating configurations
type IValidatingConfiguration interface {
	Validate(current, newConfig interface{}) error
}

// BaseConfigurationManager manages configurations
type BaseConfigurationManager struct {
	configurations         sync.Map
	configurationSyncLock  sync.Mutex
	configurationStores    []cc.ConfigurationStore
	configurationFactories []cc.IConfigurationFactory
	configuration          *configuration.BaseApplicationConfiguration
	configurationUpdated   atomic.Value // func(*BaseConfigurationManager, struct{})
	namedConfigUpdating    atomic.Value // func(*BaseConfigurationManager, ConfigurationUpdateEventArgs)
	namedConfigUpdated     atomic.Value // func(*BaseConfigurationManager, ConfigurationUpdateEventArgs)
	configurationType      reflect.Type
	logger                 *utils.Logger
	serializer             serialization.ISerializer
	commonApplicationPaths cc.IApplicationPaths
}

// NewBaseConfigurationManager creates a new BaseConfigurationManager
func NewBaseConfigurationManager(
	applicationPaths cc.IApplicationPaths,
	logger *utils.Logger,
	serializer serialization.ISerializer,
	configurationType reflect.Type,
) *BaseConfigurationManager {
	cm := &BaseConfigurationManager{
		commonApplicationPaths: applicationPaths,
		serializer:             serializer,
		logger:                 logger,
		configurationType:      configurationType,
	}
	cm.UpdateCachePath()
	return cm
}

// RegisterConfiguration registers a configuration factory
func (cm *BaseConfigurationManager) RegisterConfiguration(factory cc.IConfigurationFactory) {
	cm.configurationFactories = append(cm.configurationFactories, factory)
	cm.configurationStores = nil
	for _, f := range cm.configurationFactories {
		cm.configurationStores = append(cm.configurationStores, f.GetConfigurations()...)
	}
}

// AddParts adds configuration factories
func (cm *BaseConfigurationManager) AddParts(factories []cc.IConfigurationFactory) {
	cm.configurationFactories = factories
	cm.configurationStores = nil
	for _, f := range cm.configurationFactories {
		cm.configurationStores = append(cm.configurationStores, f.GetConfigurations()...)
	}
}

// SaveConfiguration saves the system configuration
func (cm *BaseConfigurationManager) SaveConfiguration() error {
	cm.logger.Info("Saving system configuration")
	panic("Save")
	path := cm.commonApplicationPaths.SystemConfigurationFilePath()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	cm.configurationSyncLock.Lock()
	defer cm.configurationSyncLock.Unlock()

	if err := cm.serializer.SerializeToFile(cm.CommonConfiguration(), path); err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	cm.OnConfigurationUpdated()
	return nil
}

// OnConfigurationUpdated handles configuration update events
func (cm *BaseConfigurationManager) OnConfigurationUpdated() {
	cm.UpdateCachePath()

	if handler, ok := cm.configurationUpdated.Load().(func(*BaseConfigurationManager, struct{})); ok && handler != nil {
		handler(cm, struct{}{})
	}
}

// ReplaceConfiguration replaces the current configuration
func (cm *BaseConfigurationManager) ReplaceConfiguration(newConfig *configuration.BaseApplicationConfiguration) error {
	if newConfig == nil {
		return errors.New("new configuration cannot be nil")
	}

	if err := cm.ValidateCachePath(newConfig); err != nil {
		return err
	}

	cm.configuration = newConfig
	return cm.SaveConfiguration()
}

// UpdateCachePath updates the cache path
func (cm *BaseConfigurationManager) UpdateCachePath() {
	var cachePath string
	config := cm.CommonConfiguration()
	if config == nil {
		panic("CommonConfiguration")
	}

	if config.CachePath == nil || *config.CachePath == "" {
		basePaths, ok := cm.commonApplicationPaths.(*BaseApplicationPaths)
		if !ok || basePaths.CachePath() == "" {
			cachePath = filepath.Join(cm.commonApplicationPaths.ProgramDataPath(), "cache")
		} else {
			cachePath = basePaths.CachePath()
		}
	} else {
		cachePath = *config.CachePath
	}

	cm.logger.Info("Setting cache path %v", cachePath)
	//      to resolve
	//	cm.commonApplicationPaths.(*BaseApplicationPaths).SetCachePath(cachePath)
	//	cm.commonApplicationPaths.SetCachePath(cachePath)
	cm.commonApplicationPaths.CreateAndCheckMarker(cachePath, "cache", false)
}

// ValidateCachePath validates the cache path
func (cm *BaseConfigurationManager) ValidateCachePath(newConfig *configuration.BaseApplicationConfiguration) error {
	newPath := newConfig.CachePath
	if (newPath != nil || *newPath != "") && newPath != cm.CommonConfiguration().CachePath {
		if _, err := os.Stat(*newPath); os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist", newPath)
		}
		if err := cm.EnsureWriteAccess(*newPath); err != nil {
			return err
		}
	}
	return nil
}

// EnsureWriteAccess ensures write access to a path
func (cm *BaseConfigurationManager) EnsureWriteAccess(path string) error {
	file := filepath.Join(path, fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.WriteFile(file, []byte{}, 0644); err != nil {
		return err
	}
	return os.Remove(file)
}

// GetConfigurationFile gets the configuration file path
func (cm *BaseConfigurationManager) GetConfigurationFile(key string) string {
	return filepath.Join(cm.commonApplicationPaths.ConfigurationDirectoryPath(), strings.ToLower(key)+".xml")
}

// GetConfiguration retrieves a configuration by key
func (cm *BaseConfigurationManager) GetConfiguration(key string) (interface{}, error) {
	klog.Infof("GetConfiguration key: %v\n", key)
	/*
		if val, ok := cm.configurations.Load(key); ok {
			return val, nil
		}
	*/

	/*
		var configurationInfo *cc.ConfigurationStore
		for _, store := range cm.configurationStores {
			if strings.EqualFold(store.Key, key) {
				configurationInfo = &store
				break
			}
		}

		if configurationInfo == nil {
			return nil, fmt.Errorf("configuration with key %s not found", key)
		}
	*/

	cm.configurationSyncLock.Lock()
	defer cm.configurationSyncLock.Unlock()

	var config interface{}
	var err error

	if !utils.IsTestEnv() {
		config, err = GetConfigurationFromConfigMap(reflect.TypeOf(configuration.EncodingOptions{}), "media-server-config", key, cm.Serializer())
	} else {
		file := cm.GetConfigurationFile(key)
		cm.logger.Infof("load configuration from file: ", file)

		//config, err = cm.LoadConfiguration(file, configurationInfo.ConfigurationType)
		config, err = cm.LoadConfiguration(file, reflect.TypeOf(configuration.EncodingOptions{}))
	}
	if err != nil {
		cm.logger.Errorf("failed load configuration from file: %v", err)
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	cm.configurations.Store(key, config)
	return config, nil
}

// LoadConfiguration loads a configuration from file
func (cm *BaseConfigurationManager) LoadConfiguration(path string, configurationType reflect.Type) (interface{}, error) {
	if _, err := os.Stat(path); err == nil {
		config, err := cm.serializer.DeserializeFromFile(configurationType, path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			cm.logger.Error("Error loading configuration file, path: %v", path)
		}
		return config, err
	}

	config := CreateInstance(configurationType)
	if config == nil {
		return nil, errors.New("configuration type cannot be nil")
	}
	return config, nil
}

// SaveConfiguration saves a named configuration
func (cm *BaseConfigurationManager) SaveConfigurationByKey(key string, configuration interface{}) error {
	store, err := cm.GetConfigurationStore(key)
	if err != nil {
		return err
	}
	if configurationType := reflect.TypeOf(configuration).Elem(); configurationType != store.GetConfigurationType() {
		return fmt.Errorf("expected configuration type is %s", store.GetConfigurationType().Name())
	}

	klog.Infof("configuration store %+v %T\n", store, store)
	if validatingStore, ok := store.(IValidatingConfiguration); ok {
		klog.Infoln("Validate....................")
		currentConfig, err := cm.GetConfiguration(key)
		if err != nil {
			return err
		}
		if err := validatingStore.Validate(currentConfig, configuration); err != nil {
			return err
		}
	}

	if handler, ok := cm.namedConfigUpdating.Load().(func(*BaseConfigurationManager, ConfigurationUpdateEventArgs)); ok && handler != nil {
		handler(cm, ConfigurationUpdateEventArgs{Key: key, Configuration: configuration})
	}

	cm.configurations.Store(key, configuration)
	path := cm.GetConfigurationFile(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	cm.configurationSyncLock.Lock()
	defer cm.configurationSyncLock.Unlock()

	if !utils.IsTestEnv() {
		if err := SaveConfigurationToConfigMap(configuration, "media-server-config", key, cm.Serializer()); err != nil {
			return fmt.Errorf("failed to save configuration to configmap: %w", err)
		}
	} else {
		cm.logger.Infof("SaveConfigurationByKey path: %+v configuration: %+v", path, configuration)
		if err := cm.serializer.SerializeToFile(configuration, path); err != nil {
			return fmt.Errorf("failed to serialize configuration: %w", err)
		}
	}

	cm.OnNamedConfigurationUpdated(key, configuration)
	return nil
}

// OnNamedConfigurationUpdated handles named configuration update events
func (cm *BaseConfigurationManager) OnNamedConfigurationUpdated(key string, configuration interface{}) {
	if handler, ok := cm.namedConfigUpdated.Load().(func(*BaseConfigurationManager, ConfigurationUpdateEventArgs)); ok && handler != nil {
		handler(cm, ConfigurationUpdateEventArgs{Key: key, Configuration: configuration})
	}
}

// GetConfigurationStores returns all configuration stores
func (cm *BaseConfigurationManager) GetConfigurationStores() []cc.ConfigurationStore {
	return cm.configurationStores
}

// GetConfigurationType returns the configuration type for a key
func (cm *BaseConfigurationManager) GetConfigurationType(key string) (reflect.Type, error) {
	store, err := cm.GetConfigurationStore(key)
	if err != nil {
		return nil, err
	}
	return store.GetConfigurationType(), nil
}

// GetConfigurationStore gets a configuration store by key
func (cm *BaseConfigurationManager) GetConfigurationStore(key string) (cc.ConfigurationStore, error) {
	for _, store := range cm.configurationStores {
		if strings.EqualFold(store.GetKey(), key) {
			return store, nil
		}
	}
	return nil, fmt.Errorf("configuration store with key %s not found", key)
}

func (cm *BaseConfigurationManager) Logger() *utils.Logger {
	return cm.logger
}

func (cm *BaseConfigurationManager) Serializer() serialization.ISerializer {
	return cm.serializer
}

func (cm *BaseConfigurationManager) setSerializer(serializer serialization.ISerializer) {
	cm.serializer = serializer
}

func (cm *BaseConfigurationManager) CommonApplicationPaths() cc.IApplicationPaths {
	return cm.commonApplicationPaths
}

// CommonConfiguration returns the current configuration
func (cm *BaseConfigurationManager) CommonConfiguration() *configuration.BaseApplicationConfiguration {
	if cm.configuration != nil {
		return cm.configuration
	}

	cm.configurationSyncLock.Lock()
	defer cm.configurationSyncLock.Unlock()

	if cm.configuration != nil {
		return cm.configuration
	}

	var config interface{}
	var err error

	if !utils.IsTestEnv() {
		config, err = GetConfigurationFromConfigMap(
			cm.configurationType,
			"media-server-config",
			"system",
			cm.Serializer(),
		)
	} else {
		// Assuming ConfigurationHelper.GetXmlConfiguration returns a BaseApplicationConfiguration
		config, err = GetXmlConfiguration(
			cm.configurationType,
			cm.CommonApplicationPaths().SystemConfigurationFilePath(),
			cm.Serializer(),
		)
	}
	if err != nil {
		cm.logger.Infof("CommonConfiguration %v", err)
		return nil
	}
	serverConfig, ok := config.(*configuration.ServerConfiguration)
	if !ok {
		klog.Infof("invalid configuration type: expected ServerConfiguration")
		return nil
	}
	cm.configuration = serverConfig.BaseApplicationConfiguration

	return cm.configuration
}

func (cm *BaseConfigurationManager) SetCommonConfiguration(config *configuration.BaseApplicationConfiguration) {
	cm.configurationSyncLock.Lock()
	defer cm.configurationSyncLock.Unlock()
	cm.configuration = config
}
