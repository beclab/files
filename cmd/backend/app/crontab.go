package app

import (
	"files/pkg/drivers/posix/upload"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/global"
	"files/pkg/redisutils"
	"files/pkg/tasks"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
)

var cleanupMux sync.Mutex

// InitCrontabs registers all scheduled jobs and starts the scheduler. It
// returns the running *cron.Cron so callers can wire its Stop()/ctx into
// the graceful-shutdown coordinator. The returned value is non-nil even
// when AddFunc registrations fail — a nil here would defeat shutdown.
func InitCrontabs() *cron.Cron {
	c := cron.New()

	_, err := c.AddFunc("5 0 * * *", func() {
		cleanupMux.Lock()
		defer cleanupMux.Unlock()
		seahub.AccessTokenMap = make(map[string]string)
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
	return c
}
