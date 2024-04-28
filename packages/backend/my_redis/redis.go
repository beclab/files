package my_redis

import (
	"fmt"
	"github.com/go-redis/redis"
	"os"
	"strconv"
	"time"
)

var redisClient *redis.Client

func InitRedis() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "difyai123456"
	}
	redisDBStr := os.Getenv("REDIS_DB")
	if redisDBStr == "" {
		redisDBStr = "0"
	}
	redisDB, _ := strconv.Atoi(redisDBStr)
	// 创建一个Redis客户端实例
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort, // "localhost:6379", // Redis服务器地址和端口
		Password: redisPassword,               // "difyai123456",   // Redis服务器密码，如果有的话
		DB:       redisDB,                     //0,                // 使用的Redis数据库编号
	})
}

func RedisSet(key string, value interface{}, expire time.Duration) {
	// 设置键值对
	err := redisClient.Set(key, value, expire).Err()
	if err != nil {
		fmt.Println("设置键值对失败:", err)
		return
	}
}

func RedisGet(key string) string {
	// 获取键的值
	value, err := redisClient.Get(key).Result()
	if err != nil {
		if err != redis.Nil {
			fmt.Println("获取键值失败:", err)
			return ""
		}
		fmt.Println("键", key, "不存在")
		return ""
	}
	fmt.Println("键值:", value)
	return value
}

func RedisGetAllKeys() ([]string, error) {
	var keys []string
	iter := redisClient.Scan(0, "*", 0).Iterator()
	for iter.Next() {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	// Print all keys
	for _, key := range keys {
		fmt.Println(key)
	}

	return keys, nil
}

func RedisGetKeys(keys string) []string {
	var cursor uint64
	var keyResults []string

	iter := redisClient.Scan(cursor, keys, 100).Iterator()
	for iter.Next() {
		keyResults = append(keyResults, iter.Val())
	}
	if err := iter.Err(); err != nil {
		fmt.Println("查询 Redis 中的 keys 失败:", err)
		return []string{}
	}

	return keyResults
}

func RedisAddInt(key string, value int, expire time.Duration) int {
	origin, _ := strconv.Atoi(RedisGet(key))
	fmt.Println(origin)
	fmt.Println(origin + value)
	RedisSet(key, origin+value, expire)
	return origin + value
}
