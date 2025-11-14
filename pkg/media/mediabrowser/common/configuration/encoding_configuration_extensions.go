//package encodingconfigurationextensions

package configuration

/*
import (
 "fmt"
 "os"
 "path/filepath"

 "files/pkg/media/mediabrowser/model/configuration"
)


type EncodingConfigurationExtensions struct {
    ConfigurationManager ConfigurationManager
}

func (e *EncodingConfigurationExtensions) GetEncodingOptions() (*configuration.EncodingOptions, error) {
    return e.ConfigurationManager.GetConfiguration[*configuration.EncodingOptions]("encoding")
}

func (e *EncodingConfigurationExtensions) GetTranscodePath() string {
    // Get the configured path and fall back to a default
    transcodingTempPath := e.ConfigurationManager.GetEncodingOptions().TranscodingTempPath
    if transcodingTempPath == "" {
        transcodingTempPath = filepath.Join(e.ConfigurationManager.CommonApplicationPaths().CachePath, "transcodes")
    }

    // Make sure the directory exists
    err := os.MkdirAll(transcodingTempPath, 0755)
    if err != nil {
        klog.Infoln(err)
    }

    return transcodingTempPath
}
*/
