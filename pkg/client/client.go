package client

import (
	"sync"
)

var once sync.Once

var clientFactory Factory

func ClientFactory() Factory {
	return clientFactory
}

func Init(logLevel string) (err error) {

	f, err := NewFactory()
	if err != nil {
		return err
	}
	once.Do(func() {
		clientFactory = f
	})

	return getNamespaces(logLevel, f)
}

func getNamespaces(logLevel string, f Factory) error {
	return nil
}
