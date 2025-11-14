package service

import (
	"fmt"
	"log"
	"os"
	//	"reflect"

	"files/pkg/media/api/controllers"
	"files/pkg/media/api/helpers"

	"files/pkg/media/emby/server/implementations"
	"files/pkg/media/emby/server/implementations/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	mc "files/pkg/media/mediabrowser/mediaencoding/configuration"
	"files/pkg/media/mediabrowser/mediaencoding/encoder"
	"files/pkg/media/mediabrowser/mediaencoding/transcoding"
	//	"files/pkg/media/mediabrowser/model/entities"

	iio "files/pkg/media/emby/server/implementations/io"
	"files/pkg/media/emby/server/implementations/session"

	"files/pkg/media/jellyfin/mediaencoding/hls/playlist"

	cc "files/pkg/media/mediabrowser/common/configuration"
	"files/pkg/media/mediabrowser/model/serialization"
	"files/pkg/media/utils"
)

var logger *utils.Logger
var mediaEncoder *encoder.MediaEncoder
var transcodeManager *transcoding.TranscodeManager
var dynamicHlsPlaylistGenerator *playlist.DynamicHlsPlaylistGenerator
var dynamicHlsHelper *helpers.DynamicHlsHelper
var serverConfigurationManager *configuration.ServerConfigurationManager
var encodingHelper mediaencoding.EncodingHelper
var fileSystem *iio.ManagedFileSystem

func Init() {

	CreateApplicationPaths()

	logger = utils.NewLogger("media-server ", log.LstdFlags)
	sessionManager := session.NewSessionManager()
	fileSystem = iio.NewManagedFileSystem( /*[]io.ShortcutHandler{implementations.MbLinkShortcutHandler{}}, */ "./tmp")
	var serializer serialization.ISerializer
	if !utils.IsTestEnv() {
		serializer = serialization.NewMyJsonSerializer()
		logger.Infof("json serializer")
	} else {
		serializer = serialization.NewMyXmlSerializer()
		logger.Infof("xml serializer")
	}
	applicationPaths := implementations.NewServerApplicationPaths("/tmp/", "/tmp/log", "/tmp/config", "/tmp/cache", "/tmp/web")
	serverConfigurationManager = configuration.NewServerConfigurationManager(applicationPaths, logger, serializer)
	//serverConfigurationManager.AddParts(GetExports[cc.IConfigurationFactory](true))
	serverConfigurationManager.AddParts([]cc.IConfigurationFactory{&mc.EncodingConfigurationFactory{}})

	mediaEncoder = encoder.NewMediaEncoder(logger, serverConfigurationManager)
	encodingHelper = mediaencoding.NewEncodingHelper(mediaEncoder, serverConfigurationManager)
	transcodeManager = transcoding.NewTranscodeManager(mediaEncoder, fileSystem, serverConfigurationManager, sessionManager)

	dynamicHlsPlaylistGenerator = playlist.NewDynamicHlsPlaylistGenerator(serverConfigurationManager)

	mediaEncoder.SetFFmpegPath()

	dynamicHlsHelper = helpers.NewDynamicHlsHelper(serverConfigurationManager, mediaEncoder, transcodeManager, logger, encodingHelper)
}

func GetDynamicHlsController() *controllers.DynamicHlsController {
	return controllers.NewDynamicHlsController(logger, mediaEncoder, transcodeManager, encodingHelper, fileSystem, dynamicHlsPlaylistGenerator, serverConfigurationManager, dynamicHlsHelper)
}

func GetCustomPlayController() *controllers.CustomPlayController {
	return controllers.NewCustomPlayController(logger, mediaEncoder)
}

func GetConfigurationController() *controllers.ConfigurationController {
	return controllers.NewConfigurationController(serverConfigurationManager, mediaEncoder)
}

func CreateApplicationPaths() error {
	var logDir string = "/tmp/log"

	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return err
	}

	return nil
}

// ***********************************************************************************************************
/*
// allConcreteTypes is a package-level slice to store concrete types
var allConcreteTypes []reflect.Type = []reflect.Type{
	reflect.TypeOf(mc.EncodingOptions{}),
}

// DiscoverTypes loads and stores concrete types from relevant packages
func DiscoverTypes() {
	log.Println("Loading assemblies")

	// Replace GetComposablePartAssemblies and GetTypes with a Go equivalent
	// This could involve a predefined list of types or plugin loading
	types := getComposablePartTypes()

	// Store types in allConcreteTypes slice
	allConcreteTypes = types
}

// getComposablePartTypes simulates retrieving concrete types
// In a real implementation, this could involve plugin loading or a type registry
func getComposablePartTypes() []reflect.Type {
	// Example: Return a slice of reflect.Type for demonstration
	// In practice, this might involve loading plugins or registering types manually
	return []reflect.Type{
		reflect.TypeOf(struct{}{}), // Placeholder; replace with actual types
	}
}

// Assembly represents a Go equivalent of a .NET Assembly
type Assembly struct {
	Name  string
	Types []reflect.Type // Placeholder for exported types
}

// PluginManager represents a plugin manager with a method to fail plugins
type PluginManager struct{}

// FailPlugin marks a plugin as failed
func (pm *PluginManager) FailPlugin(ass *Assembly) {
	// Placeholder: Log or handle plugin failure
	log.Printf("Plugin %s marked as failed", ass.Name)
}

// getTypes retrieves concrete types from a slice of assemblies
func getTypes(assemblies []*Assembly) []reflect.Type {
	var result []reflect.Type

	for _, ass := range assemblies {
		var exportedTypes []reflect.Type
		// Simulate GetExportedTypes; in practice, this could involve plugin loading
		exportedTypes = ass.Types // Placeholder: Assume Types field contains exported types

		// Error handling for file not found or type loading issues
		if len(exportedTypes) == 0 {
			// Simulate FileNotFoundException or TypeLoadException
			err := fmt.Errorf("error getting exported types from %s", ass.Name)
			log.Printf("Error: %v", err)
			pluginManager := &PluginManager{}
			pluginManager.FailPlugin(ass)
			continue
		}

		// Filter for concrete types (non-abstract, non-interface, non-generic classes)
		for _, t := range exportedTypes {
			// In Go, reflect.Type has limited introspection; assume a type registry
			// provides this information. For simplicity, include all types here.
			// In a real implementation, filter based on type properties or metadata.
			if t.Kind() == reflect.Struct {
				// Placeholder: Assume struct types are equivalent to concrete classes
				result = append(result, t)
			}
		}
	}

	return result
}

// Disposable interface, equivalent to C#'s IDisposable
type Disposable interface {
	Dispose()
}

// _disposableParts tracks disposable instances
var _disposableParts []Disposable

// GetExportTypes returns a slice of types that implement or are assignable to T
func GetExportTypes[T any]() []reflect.Type {
	var result []reflect.Type
	tType := reflect.TypeOf((*T)(nil)).Elem()
	for _, t := range allConcreteTypes {
		if t.AssignableTo(tType) {
			result = append(result, t)
		}
	}
	return result
}

// CreateInstanceSafe is a placeholder for creating instances of a type
func CreateInstanceSafe(t reflect.Type) any {
	// Placeholder: actual implementation depends on your needs
	// Could use reflect.New or a factory function
	return reflect.New(t).Interface()
}

// GetExports returns a slice of instances of type T, with optional lifetime management
func GetExports[T any](manageLifetime bool) []T {
	// Get types and create instances
	parts := make([]T, 0)
	for _, t := range GetExportTypes[T]() {
		instance := CreateInstanceSafe(t)
		if instance != nil {
			if typedInstance, ok := instance.(T); ok {
				parts = append(parts, typedInstance)
			}
		}
	}

	// Manage lifetime for disposable parts
	if manageLifetime {
		for _, part := range parts {
			if disposable, ok := any(part).(Disposable); ok {
				_disposableParts = append(_disposableParts, disposable)
			}
		}
	}

	return parts
}
*/
