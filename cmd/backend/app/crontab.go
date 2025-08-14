package app

import (
	"files/pkg/drivers/posix/upload"
	"files/pkg/drives"
	"files/pkg/redisutils"
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
	})
	if err != nil {
		klog.Fatalf("AddFunc CleanupOldFilesAndRedisEntries err:%v", err)
	} else {
		klog.Info("Crontab task: CleanupOldFilesAndRedisEntries added successfully.")
	}

	_, err = c.AddFunc("*/5 * * * *", func() {
		cleanupMux.Lock()
		defer cleanupMux.Unlock()
		drives.GetMountedData(nil)
	})
	if err != nil {
		klog.Fatalf("AddFunc GetMountedData err:%v", err)
	} else {
		klog.Info("Crontab task: GetMountedData added successfully.")
	}

	upload.Init(c)

	c.Start()
}
