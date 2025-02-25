package my_redis

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/go-redis/redis"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var (
	folderPath = diskcache.CacheDir
	zsetKey    = "file_cache_access_times"
	cleanupMux sync.Mutex
)

func DelThumbRedisKey(key string) error {
	cleanupMux.Lock()
	defer cleanupMux.Unlock()

	err := RedisZRem(zsetKey, []string{key})
	if err != nil {
		fmt.Println("Error removing file from Redis:", err)
		return err
	}
	return nil
}

func StartDailyCleanup() {
	cycle := 7 * 24 * time.Hour
	ticker := time.NewTicker(cycle)
	defer ticker.Stop()

	now := time.Now()
	nextCleanupTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Add(cycle)
	duration := nextCleanupTime.Sub(now)
	time.Sleep(duration)

	for range ticker.C {
		cleanupMux.Lock()
		CleanupOldFilesAndRedisEntries(1 * cycle)
		cleanupMux.Unlock()
	}
}

func CleanupOldFilesAndRedisEntries(duration time.Duration) {
	fmt.Printf("Cleaning up old files at %d\n", time.Now().Unix())
	cutoffTime := time.Now().Add(-duration).Unix()
	cutoffTimeStr := strconv.FormatInt(cutoffTime, 10)

	results := RedisZRangeByScore(zsetKey, "-inf", cutoffTimeStr)

	for _, member := range results {
		fileName := member
		filePath := filepath.Join(folderPath, fileName)

		err := os.Remove(filePath)
		if err != nil {
			fmt.Println("Error deleting file:", err)
		}

		err = RedisZRem(zsetKey, []string{fileName})
		if err != nil {
			fmt.Println("Error removing file from Redis:", err)
			continue
		}
	}
}

func GetFileName(key string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return hash
}

func UpdateFileAccessTimeToRedis(fileName string) error {
	key := fileName
	currentTime := time.Now().Unix()
	err := RedisZAdd(zsetKey, key, float64(currentTime))
	return err
}

func InitFolderAndRedis() {
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileName := filepath.Base(path)

			exists, err := RedisZScore(zsetKey, fileName)
			if err == redis.Nil {
				err = UpdateFileAccessTimeToRedis(fileName)
				if err != nil {
					fmt.Println("Error adding file to Redis:", err)
				}
			} else if err != nil {
				fmt.Println("Error checking file in Redis:", err)
			} else {
				fmt.Printf("File %s already exists in Redis with score %f\n", fileName, exists)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error initializing folder and Redis:", err)
	}

	results := RedisZRange(zsetKey, 0, -1)

	for _, member := range results {
		fmt.Println("filename=", member)
		score, err := RedisZScore(zsetKey, member)
		if err != nil {
			fmt.Println("Error fetching file from Redis:", err)
			return
		}
		fmt.Println("score=", score)
	}
	return
}
