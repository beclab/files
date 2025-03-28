package app

import (
	"files/pkg/fileutils"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
)

func checkErr(err error) {
	if err != nil {
		klog.Fatal(err)
	}
}

type cobraFunc func(cmd *cobra.Command, args []string)
type pythonFunc func(cmd *cobra.Command, args []string)

type pythonConfig struct {
	noDB      bool
	allowNoDB bool
}

func dbExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil {
		return stat.Size() != 0, nil
	}

	if os.IsNotExist(err) {
		d := filepath.Dir(path)
		_, err = os.Stat(d)
		if os.IsNotExist(err) {
			// forced 1000
			if err = os.MkdirAll(d, 0700); err != nil {
				return false, err
			}
			if err = fileutils.Chown(nil, d, 1000, 1000); err != nil {
				klog.Errorf("can't chown directory %s to user %d: %s", d, 1000, err)
				return false, err
			}
			return false, nil
		}
	}

	return false, err
}

func python(fn pythonFunc, cfg pythonConfig) cobraFunc {
	return func(cmd *cobra.Command, args []string) {
		path := getParam(cmd.Flags(), "database")
		exists, err := dbExists(path)

		if err != nil {
			panic(err)
		} else if exists && cfg.noDB {
			klog.Fatal(path + " already exists")
		} else if !exists && !cfg.noDB && !cfg.allowNoDB {
			klog.Fatal(path + " does not exist. Please run 'filebrowser config init' first.")
		}

		fn(cmd, args)
	}
}

func cleanUpInterfaceMap(in map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range in {
		result[fmt.Sprintf("%v", k)] = cleanUpMapValue(v)
	}
	return result
}

func cleanUpInterfaceArray(in []interface{}) []interface{} {
	result := make([]interface{}, len(in))
	for i, v := range in {
		result[i] = cleanUpMapValue(v)
	}
	return result
}

func cleanUpMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanUpInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanUpInterfaceMap(v)
	default:
		return v
	}
}
