package client

import (
	"sync"

	"k8s.io/klog/v2"
)

var once sync.Once

var clientFactory Factory

func ClientFactory() Factory {
	return clientFactory
}

func Init(logLevel string) (err error) {
	klog.Info("new dynamic client")

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
