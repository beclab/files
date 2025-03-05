package crontab

import (
	"files/pkg/backend/redisutils"
	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

var cleanupMux sync.Mutex

func InitCrontabs() {
	c := cron.New()

	_, err := c.AddFunc("5 0 * * *", func() {
		cleanupMux.Lock()
		defer cleanupMux.Unlock()
		redisutils.CleanupOldFilesAndRedisEntries(7 * 24 * time.Hour)
		klog.Info("Crontab task: CleanupOldFilesAndRedisEntries added successfully.")
	})
	if err != nil {
		klog.Warningf("AddFunc CleanupOldFilesAndRedisEntries err:%v", err)
	}

	c.Start()
}
