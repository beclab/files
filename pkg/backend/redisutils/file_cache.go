package redisutils

import (
	"crypto/sha1"
	"encoding/hex"
	"files/pkg/backend/diskcache"
	"fmt"
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

	err := RedisClient.ZRem(key, []string{key}).Err()
	if err != nil {
		fmt.Println("Error removing file from Redis:", err)
		return err
	}
	return nil
}

func CleanupOldFilesAndRedisEntries(duration time.Duration) {
	if diskcache.CacheDir == "" {
		//fmt.Println("Cache dir not set, nothing to clean up")
		return
	}

	fmt.Printf("Cleaning up old files at %d\n", time.Now().Unix())
	cutoffTime := time.Now().Add(-duration).Unix()
	cutoffTimeStr := strconv.FormatInt(cutoffTime, 10)

	results, err := RedisClient.ZRangeByScore(zsetKey, redis.ZRangeBy{
		Min: "-inf",
		Max: cutoffTimeStr,
	}).Result()
	if err != nil {
		fmt.Println("get members in a given range from zset failed: ", err)
		return
	}

	for _, member := range results {
		fileName := member
		filePath := filepath.Join(folderPath, fileName)

		err = os.Remove(filePath)
		if err != nil {
			fmt.Println("Error deleting file:", err)
		}

		err = RedisClient.ZRem(zsetKey, []string{fileName}).Err()
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
	err := RedisClient.ZAdd(zsetKey, redis.Z{Score: float64(currentTime), Member: key}).Err()
	return err
}

func InitFolderAndRedis() {
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileName := filepath.Base(path)

			var exists float64
			exists, err = RedisClient.ZScore(zsetKey, fileName).Result()
			if err == redis.Nil {
				err = UpdateFileAccessTimeToRedis(fileName)
				if err != nil {
					fmt.Println("Error adding file to Redis:", err)
					return err
				}
			} else if err != nil {
				fmt.Println("Error checking file in Redis:", err)
				return err
			} else {
				fmt.Printf("File %s already exists in Redis with score %f\n", fileName, exists)
				return nil
			}
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error initializing folder and Redis:", err)
		return
	}

	var results []string
	results, err = RedisClient.ZRange(zsetKey, 0, -1).Result()
	if err != nil {
		fmt.Println("get range member of zset failed: ", err)
		return
	}

	for _, member := range results {
		fmt.Println("filename=", member)
		var score float64
		score, err = RedisClient.ZScore(zsetKey, member).Result()
		if err != nil {
			if err == redis.Nil {
				fmt.Errorf("member %s doesn't exist in zset %s", member, zsetKey)
				return
			}
			fmt.Errorf("get score for member from zset failed: %v", err)
			return
		}
		fmt.Println("score=", score)
	}
	return
}
