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

// Per-job mutexes prevent two invocations of the *same* cron job
// from overlapping (cron.Cron does not synchronize jobs internally).
// Distinct jobs no longer share a single global lock - the previous
// `cleanupMux` covered both the slow daily cleanup and the every-5min
// mount refresh, so a long-running daily run could block mount data
// from updating for the duration of its execution.
var (
	dailyCleanupMux sync.Mutex
	mountRefreshMux sync.Mutex
)

// InitCrontabs registers all scheduled jobs and starts the scheduler. It
// returns the running *cron.Cron so callers can wire its Stop()/ctx into
// the graceful-shutdown coordinator. The returned value is non-nil even
// when AddFunc registrations fail — a nil here would defeat shutdown.
func InitCrontabs() *cron.Cron {
	c := cron.New()

	_, err := c.AddFunc("5 0 * * *", func() {
		// TryLock so a previous slow run (e.g. >24h, however
		// unlikely) doesn't queue up a second concurrent cleanup.
		if !dailyCleanupMux.TryLock() {
			klog.Warning("Crontab: daily cleanup still running, skipping this tick")
			return
		}
		defer dailyCleanupMux.Unlock()

		seahub.ClearAccessTokens()
		redisutils.CleanupOldFilesAndRedisEntries(7 * 24 * time.Hour)

		tasks.TaskManager.ClearTasks()
		tasks.TaskManager.ClearCacheFiles()
	})
	if err != nil {
		klog.Errorf("AddFunc CleanupOldFilesAndRedisEntries err: %v", err)
	} else {
		klog.Info("Crontab task: CleanupOldFilesAndRedisEntries added successfully.")
	}

	_, err = c.AddFunc("*/5 * * * *", func() {
		if !mountRefreshMux.TryLock() {
			klog.Warning("Crontab: mount refresh still running, skipping this tick")
			return
		}
		defer mountRefreshMux.Unlock()

		global.GlobalMounted.Updated()
	})
	if err != nil {
		klog.Errorf("AddFunc GetMountedData err: %v", err)
	} else {
		klog.Info("Crontab task: GetMountedData added successfully.")
	}

	upload.Init(c)

	c.Start()
	return c
}
