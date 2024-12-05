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

	err := RedisZRem(zsetKey, key)
	if err != nil {
		fmt.Println("Error removing file from Redis:", err)
		return err
	}
	return nil
}

// 每天定时清理过期文件和Redis ZSET成员
func StartDailyCleanup() {
	cycle := time.Minute * 5
	ticker := time.NewTicker(cycle)
	defer ticker.Stop()

	// 在下一次整点时触发
	//now := time.Now()
	//nextCleanupTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Add(cycle)
	//duration := nextCleanupTime.Sub(now)
	//time.Sleep(duration)

	for range ticker.C {
		cleanupMux.Lock()
		CleanupOldFilesAndRedisEntries(cycle)
		cleanupMux.Unlock()
	}
}

// 清理过期文件和Redis ZSET成员
func CleanupOldFilesAndRedisEntries(duration time.Duration) {
	fmt.Printf("Cleaning up old files at %d\n", time.Now().Unix())
	cutoffTime := time.Now().Add(-duration).Unix()
	//cutoffTime := time.Now().Add(-7 * 24 * time.Hour).Unix()
	cutoffTimeStr := strconv.FormatInt(cutoffTime, 10)

	// 获取所有成员及其分数
	results, err := RedisZRangeByScore(zsetKey, "-inf", cutoffTimeStr, false)
	if err != nil {
		fmt.Println("Error fetching files from Redis:", err)
		return
	}

	for _, member := range results {
		fileName := member
		filePath := filepath.Join(folderPath, fileName)

		// 删除文件
		err = os.Remove(filePath)
		if err != nil {
			fmt.Println("Error deleting file:", err)
			continue
		}

		// 从Redis ZSET中删除成员
		err = RedisZRem(zsetKey, fileName)
		if err != nil {
			fmt.Println("Error removing file from Redis:", err)
			continue
		}
	}
}

func GetFileName(key string) string {
	hasher := sha1.New() //nolint:gosec
	_, _ = hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	//return fmt.Sprintf("%s/%s/%s", hash[:1], hash[1:3], hash)
	return hash
}

// 更新文件访问时间
func UpdateFileAccessTimeToRedis(fileName string) error {
	key := fileName
	currentTime := time.Now().Unix()
	member := make(map[string]float64)
	member[key] = float64(currentTime)
	err := RedisZAdd(zsetKey, member)
	return err
}

// 程序启动时初始化文件夹和Redis ZSET
func InitFolderAndRedis() {
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileName := filepath.Base(path)

			// 检查Redis ZSET中是否存在该文件
			exists, err := RedisZScore(zsetKey, fileName)
			if err == redis.Nil {
				// 如果不存在，则添加该文件并设置访问时间为当前时间
				err = UpdateFileAccessTimeToRedis(fileName)
				if err != nil {
					fmt.Println("Error adding file to Redis:", err)
				}
			} else if err != nil {
				fmt.Println("Error checking file in Redis:", err)
			} else {
				// 如果存在，则保持分数不变
				fmt.Printf("File %s already exists in Redis with score %f\n", fileName, exists)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error initializing folder and Redis:", err)
	}

	results, err := RedisZRange(zsetKey, 0, -1)
	if err != nil {
		fmt.Println("Error fetching files from Redis:", err)
		return
	}

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
