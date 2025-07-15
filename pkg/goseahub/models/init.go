package models

import (
	"files/pkg/postgres"
	"k8s.io/klog/v2"
	"os"
	"strconv"
	"time"
)

func InitGoSeahubModels() {
	if postgres.SeahubDBServer == nil {
		klog.Fatal("PostgreSQL server not initialized")
	}
	GlobalProfileManager = NewProfileManager(postgres.SeahubDBServer)

	if envTimeout := os.Getenv("NICKNAME_CACHE_TIMEOUT"); envTimeout != "" {
		if sec, err := strconv.Atoi(envTimeout); err == nil {
			NICKNAME_CACHE_TIMEOUT = time.Duration(sec) * time.Second
		}
	}

	if envPrefix := os.Getenv("NICKNAME_CACHE_PREFIX"); envPrefix != "" {
		NICKNAME_CACHE_PREFIX = envPrefix
	}

	if envTimeout := os.Getenv("EMAIL_ID_CACHE_TIMEOUT"); envTimeout != "" {
		if sec, err := strconv.Atoi(envTimeout); err == nil {
			EMAIL_ID_CACHE_TIMEOUT = time.Duration(sec) * time.Second
		}
	}

	if envPrefix := os.Getenv("EMAIL_ID_CACHE_PREFIX"); envPrefix != "" {
		EMAIL_ID_CACHE_PREFIX = envPrefix
	}

	if envTimeout := os.Getenv("CONTACT_CACHE_TIMEOUT"); envTimeout != "" {
		if sec, err := strconv.Atoi(envTimeout); err == nil {
			CONTACT_CACHE_TIMEOUT = time.Duration(sec) * time.Second
		}
	}

	if envPrefix := os.Getenv("CONTACT_CACHE_PREFIX"); envPrefix != "" {
		CONTACT_CACHE_PREFIX = envPrefix
	}
}
