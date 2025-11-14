package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"

	"files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"github.com/gorilla/mux"
)

type ConfigurationController struct {
	configurationManager configuration.IServerConfigurationManager
	mediaEncoder         mediaencoding.IMediaEncoder
}

func NewConfigurationController(configurationManager configuration.IServerConfigurationManager, mediaEncoder mediaencoding.IMediaEncoder) *ConfigurationController {
	return &ConfigurationController{
		configurationManager: configurationManager,
		mediaEncoder:         mediaEncoder,
	}
}

/*
func (c *ConfigurationController) GetConfiguration(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, c.configurationManager.Configuration)
}

func (c *ConfigurationController) UpdateConfiguration(ctx *gin.Context) {
	var configuration model_configuration.ServerConfiguration
	if err := ctx.ShouldBindJSON(&configuration); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	c.configurationManager.ReplaceConfiguration(configuration)
	ctx.Status(http.StatusNoContent)
}
*/

func (c *ConfigurationController) GetNamedConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]
	fmt.Println(key)

	configuration, err := c.configurationManager.GetConfiguration(key)
	if err != nil {
		http.Error(w, "failed to get configuration", http.StatusInternalServerError)
		return
	}

	jsonData, err := json.Marshal(configuration)
	if err != nil {
		log.Fatalf("Error marshaling to JSON: %v", err)
		http.Error(w, "error marshaling to JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, string(jsonData))
}

func (c *ConfigurationController) UpdateNamedConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]
	fmt.Println(key)

	var config json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid configuration body", http.StatusBadRequest)
		return
	}

	configType, err := c.configurationManager.GetConfigurationType(key)
	if err != nil {
		http.Error(w, "failed to get configuration type", http.StatusInternalServerError)
		return
	}

	log.Println("config type:", configType)
	// Create a new instance of the configuration type
	configInstance := reflect.New(configType).Interface()

	// Deserialize the JSON into the configuration type
	if err := json.Unmarshal(config, configInstance); err != nil {
		log.Printf("err: %+v\n", err)
		http.Error(w, "failed to deserialize configuration", http.StatusBadRequest)
		return
	}

	err = c.configurationManager.SaveConfigurationByKey(key, configInstance)
	if err != nil {
		log.Printf("err: %+v\n", err)
		http.Error(w, "failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

/*
func (c *ConfigurationController) GetDefaultMetadataOptions(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, model_configuration.MetadataOptions{})
}

func (c *ConfigurationController) UpdateMediaEncoderPath(ctx *gin.Context) {
	var mediaEncoderPath configurationdtos.MediaEncoderPathDto
	if err := ctx.ShouldBindJSON(&mediaEncoderPath); err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// API ENDPOINT DISABLED (NOOP) FOR SECURITY PURPOSES
	// c.mediaEncoder.UpdateEncoderPath(mediaEncoderPath.Path, mediaEncoderPath.PathType)
	ctx.Status(http.StatusNoContent)
}
*/
