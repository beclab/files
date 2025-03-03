package cmd

import (
	"fmt"
	"github.com/asdine/storm/v3"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"

	"github.com/filebrowser/filebrowser/v2/storage"
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type cobraFunc func(cmd *cobra.Command, args []string)
type pythonFunc func(cmd *cobra.Command, args []string, data pythonData)

type pythonConfig struct {
	noDB      bool
	allowNoDB bool
}

type pythonData struct {
	hadDB bool
	store *storage.Storage
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
			if err := os.MkdirAll(d, 0700); err != nil {
				return false, err
			}
			return false, nil
		}
	}

	return false, err
}

func python(fn pythonFunc, cfg pythonConfig) cobraFunc {
	return func(cmd *cobra.Command, args []string) {
		data := pythonData{hadDB: true}

		path := getParam(cmd.Flags(), "database")
		exists, err := dbExists(path)

		if err != nil {
			panic(err)
		} else if exists && cfg.noDB {
			log.Fatal(path + " already exists")
		} else if !exists && !cfg.noDB && !cfg.allowNoDB {
			log.Fatal(path + " does not exist. Please run 'filebrowser config init' first.")
		}

		data.hadDB = exists
		db, err := storm.Open(path)
		checkErr(err)
		defer db.Close()
		data.store, err = bolt.NewStorage(db)
		checkErr(err)
		fn(cmd, args, data)
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
