package configuration

import (
	"reflect"
)

type ConfigurationStore interface {
	GetKey() string
	GetConfigurationType() reflect.Type
}
