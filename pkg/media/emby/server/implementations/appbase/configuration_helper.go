package appbase

import (
	"bytes"
	"fmt"
	//	"io"
	"os"
	"path/filepath"
	"reflect"

	"files/pkg/media/mediabrowser/model/configuration"
	"files/pkg/media/mediabrowser/model/serialization"
	"files/pkg/media/utils"
)

func CreateInstance(t reflect.Type) interface{} {
	if t == reflect.TypeOf(configuration.ServerConfiguration{}) {
		return configuration.NewServerConfiguration()
	} else if t == reflect.TypeOf(configuration.EncodingOptions{}) {
		return configuration.NewEncodingOptions()
	} else {
		return reflect.New(t).Interface()
	}
}

// ConfigurationHelper provides configuration loading functionality
func GetXmlConfiguration(t reflect.Type, path string, xmlSerializer serialization.ISerializer) (interface{}, error) {
	var configuration interface{}
	var buffer []byte

	// Try to read the file
	buffer, err := os.ReadFile(path)
	if err != nil {
		// Create new instance if file doesn't exist or error occurs
		configuration = CreateInstance(t)
	} else {
		// Deserialize the configuration
		config, err := xmlSerializer.DeserializeFromBytes(t, buffer)
		if err != nil {
			return nil, err
		}
		configuration = config
	}

	// Serialize configuration to bytes
	var buf bytes.Buffer
	if err := xmlSerializer.SerializeToStream(configuration, &buf); err != nil {
		return nil, err
	}
	newBytes := buf.Bytes()

	// If file didn't exist or content has changed, save the new configuration
	if err != nil || !bytes.Equal(newBytes, buffer) {
		// Ensure directory exists
		dir := filepath.Dir(path)
		if dir == "" {
			return nil, fmt.Errorf("invalid path: %s", path)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}

		// Save the configuration
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if _, err := f.Write(newBytes); err != nil {
			return nil, err
		}
	}

	return configuration, nil
}

// GetConfigMapConfiguration reads configuration from a Kubernetes ConfigMap
func GetConfigurationFromConfigMap(t reflect.Type, configMapName, configMapKey string, jsonSerializer serialization.IJsonSerializer) (interface{}, error) {
	// Get Kubernetes clientset
	clientset, err := utils.GetKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes client: %v", err)
	}

	namespace, err := utils.GetServiceAccountNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get service account namespace: %v", err)
	}

	var configuration interface{}
	var buffer []byte

	// Read ConfigMap
	configMap, err := utils.ReadConfigMap(clientset, namespace, configMapName)
	if err != nil {
		// Create new instance if ConfigMap doesn't exist
		configuration = CreateInstance(t)
	} else {
		// Get data from ConfigMap
		data, ok := configMap.Data[configMapKey]
		if !ok {
			// Create new instance if key doesn't exist
			configuration = CreateInstance(t)
		} else {
			// Deserialize the configuration from ConfigMap data
			buffer = []byte(data)
			config, err := jsonSerializer.DeserializeFromBytes(t, buffer)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize ConfigMap data: %v", err)
			}
			configuration = config
		}
	}

	// Serialize configuration to bytes
	var buf bytes.Buffer
	if err := jsonSerializer.SerializeToStream(configuration, &buf); err != nil {
		return nil, fmt.Errorf("failed to serialize configuration: %v", err)
	}
	newBytes := buf.Bytes()

	// If ConfigMap didn't exist or content has changed, save the new configuration
	if err != nil || !bytes.Equal(newBytes, buffer) {
		// Create or update ConfigMap
		configMapData := map[string]string{
			configMapKey: string(newBytes),
		}
		if err := utils.WriteConfigMap(clientset, namespace, configMapName, configMapData); err != nil {
			return nil, fmt.Errorf("failed to write ConfigMap: %v", err)
		}
	}

	return configuration, nil
}
func SaveConfigurationToConfigMap(configuration interface{}, configMapName, configMapKey string, jsonSerializer serialization.IJsonSerializer) error {
	clientset, err := utils.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("Failed to create clientset: %v", err)
	}

	namespace, err := utils.GetServiceAccountNamespace()
	if err != nil {
		return fmt.Errorf("failed to get service account namespace: %v", err)
	}

	// Serialize configuration to bytes
	var buf bytes.Buffer
	if err := jsonSerializer.SerializeToStream(configuration, &buf); err != nil {
		return fmt.Errorf("failed to serialize configuration: %v", err)
	}

	// Create or update ConfigMap
	configMapData := map[string]string{
		configMapKey: string(buf.Bytes()),
	}

	if err := utils.WriteConfigMap(clientset, namespace, configMapName, configMapData); err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	return nil
}
