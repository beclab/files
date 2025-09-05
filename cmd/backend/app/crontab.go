package app

import (
	"files/pkg/drivers/posix/upload"
	"files/pkg/global"
	"files/pkg/redisutils"
	"files/pkg/tasks"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
)

var cleanupMux sync.Mutex

func InitCrontabs() {
	c := cron.New()

	_, err := c.AddFunc("5 0 * * *", func() {
		cleanupMux.Lock()
		defer cleanupMux.Unlock()
		redisutils.CleanupOldFilesAndRedisEntries(7 * 24 * time.Hour)

		tasks.TaskManager.ClearTasks()
		tasks.TaskManager.ClearCacheFiles()
	})
	if err != nil {
		klog.Fatalf("AddFunc CleanupOldFilesAndRedisEntries err:%v", err)
	} else {
		klog.Info("Crontab task: CleanupOldFilesAndRedisEntries added successfully.")
	}

	_, err = c.AddFunc("*/5 * * * *", func() {
		cleanupMux.Lock()
		defer cleanupMux.Unlock()

		global.GlobalMounted.Updated()
	})
	if err != nil {
		klog.Fatalf("AddFunc GetMountedData err:%v", err)
	} else {
		klog.Info("Crontab task: GetMountedData added successfully.")
	}

	upload.Init(c)

	c.Start()
}
