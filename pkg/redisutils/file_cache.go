package redisutils

import (
	"files/pkg/common"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"k8s.io/klog/v2"
)

var (
	folderPath = common.CACHE_PREFIX
	zsetKey    = "file_cache_access_times"
)

func CleanupOldFilesAndRedisEntries(duration time.Duration) {
	klog.Infof("Cleaning up old files at %d\n", time.Now().Unix())
	cutoffTime := time.Now().Add(-duration).Unix()
	cutoffTimeStr := strconv.FormatInt(cutoffTime, 10)

	results, err := RedisClient.ZRangeByScore(zsetKey, redis.ZRangeBy{
		Min: "-inf",
		Max: cutoffTimeStr,
	}).Result()
	if err != nil {
		klog.Errorln("get members in a given range from zset failed: ", err)
		return
	}

	for _, member := range results {
		fileName := member
		filePath := filepath.Join(folderPath, fileName)

		err = os.Remove(filePath)
		if err != nil {
			klog.Errorln("Error deleting file:", err)
		}

		err = RedisClient.ZRem(zsetKey, []string{fileName}).Err()
		if err != nil {
			klog.Errorln("Error removing file from Redis:", err)
			continue
		}
	}
}
