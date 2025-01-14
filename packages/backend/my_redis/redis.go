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

	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort, // "localhost:6379", // Redis服务器地址和端口
		Password: redisPassword,               // "difyai123456",   // Redis服务器密码，如果有的话
		DB:       redisDB,                     //0,                // 使用的Redis数据库编号
	})
}

func RedisSet(key string, value interface{}, expire time.Duration) {
	err := redisClient.Set(key, value, expire).Err()
	if err != nil {
		fmt.Println("Set key value failed: ", err)
		return
	}
}

func RedisHSet(key string, field string, value interface{}) {
	err := redisClient.HSet(key, field, value).Err()
	if err != nil {
		fmt.Println("Set key value of Hash table failed: ", err)
		return
	}
}

func RedisGet(key string) string {
	value, err := redisClient.Get(key).Result()
	if err != nil {
		if err != redis.Nil {
			fmt.Println("get key value failed: ", err)
			return ""
		}
		fmt.Println("key ", key, "doesn't exist")
		return ""
	}
	fmt.Println("value:", value)
	return value
}

func RedisHGet(key, field string) string {
	value, err := redisClient.HGet(key, field).Result()
	if err != nil {
		if err != redis.Nil {
			fmt.Println("get key value of Hash table failed: ", err)
			return ""
		}
		fmt.Println("Hash table ", key, " and field ", field, "doesn't exist")
		return ""
	}
	fmt.Println("value:", value)
	return value
}

func RedisHGetAll(key string) (map[string]string, error) {
	result, err := redisClient.HGetAll(key).Result()
	if err != nil {
		fmt.Println("get all key-value pairs of Hash table failed: ", err)
		return nil, err
	}

	data := make(map[string]string)
	for k, v := range result {
		data[k] = v
	}

	fmt.Println("Hash table data:", data)
	return data, nil
}

func RedisHMSet(key string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields provided for HMSet")
	}

	for field, value := range fields {
		_, err := redisClient.HSet(key, field, value).Result()
		if err != nil {
			fmt.Printf("set hash field '%s' failed: %v\n", field, err)
			return err
		}
	}

	return nil
}

func RedisHDel(key string, fields ...string) error {
	result, err := redisClient.HDel(key, fields...).Result()
	if err != nil {
		fmt.Printf("Delete fields from Hash table failed: %v\n", err)
		return err
	}
	fmt.Printf("Successfully deleted %d fields from hash %s\n", result, key)
	return nil
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
		fmt.Println("get keys in redis failed: ", err)
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

func RedisDelKey(key string) error {
	result, err := redisClient.Del(key).Result()
	if err != nil {
		fmt.Printf("Delete key failed: %v\n", err)
		return err
	}
	fmt.Printf("Successfully deleted %d keys\n", result)
	return nil
}

func RedisZAdd(key string, member interface{}, score float64) error {
	err := redisClient.ZAdd(key, redis.Z{Score: score, Member: member}).Err()
	if err != nil {
		fmt.Println("add new member to zset failed: ", err)
		return err
	}
	return nil
}

func RedisZRange(key string, start, stop int64) []string {
	members, err := redisClient.ZRange(key, start, stop).Result()
	if err != nil {
		fmt.Println("get range member of zset failed: ", err)
		return []string{}
	}
	return members
}

func RedisZRangeWithScores(key string, start, stop int64) (map[string]float64, error) {
	result, err := redisClient.ZRangeWithScores(key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("get range member with scores failed: %v", err)
	}

	membersWithScores := make(map[string]float64)
	for _, z := range result {
		membersWithScores[z.Member.(string)] = z.Score
	}

	return membersWithScores, nil
}

func RedisZScore(key, member string) (float64, error) {
	score, err := redisClient.ZScore(key, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("member %s doesn't exist in zset %s", member, key)
		}
		return 0, fmt.Errorf("get score for member from zset failed: %v", err)
	}
	return score, nil
}

// score from min to max
func RedisZRank(key, member string) (int64, error) {
	rank, err := redisClient.ZRank(key, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("member %s doesn't exist in zset %s", member, key)
		}
		return 0, fmt.Errorf("get rank for zset member failed: %v", err)
	}
	return rank, nil
}

// score from max to min
func RedisZRevRank(key, member string) (int64, error) {
	rank, err := redisClient.ZRevRank(key, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("member %s doesn't exist in zset %s", member, key)
		}
		return 0, fmt.Errorf("get reverse rank for zset member failed: %v", err)
	}
	return rank, nil
}

func RedisZRem(key string, members []string) error {
	err := redisClient.ZRem(key, members).Err()
	if err != nil {
		fmt.Println("remove member for zset failed: ", err)
		return err
	}
	return nil
}

func RedisZCard(key string) int64 {
	card, err := redisClient.ZCard(key).Result()
	if err != nil {
		fmt.Println("get member count for zset failed: ", err)
		return 0
	}
	return card
}

func RedisZIncrBy(key, member string, increment float64) float64 {
	newScore, err := redisClient.ZIncrBy(key, increment, member).Result()
	if err != nil {
		fmt.Println("increase score for zset member failed: ", err)
		return 0
	}
	return newScore
}

func RedisZRangeByScore(key, min, max string) []string {
	members, err := redisClient.ZRangeByScore(key, redis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
	if err != nil {
		fmt.Println("get members in a given range from zset failed: ", err)
		return []string{}
	}
	return members
}

type ZSetEntry struct {
	Value string  `json:"value"`
	Score float64 `json:"score"`
}

func RedisZRevRangeWithScores(key string, start, stop int64) ([]ZSetEntry, error) {
	// Retrieve the zset entries in reverse order with scores
	zset, err := redisClient.ZRevRangeWithScores(key, start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("get reverse range with scores from zset failed: %v", err)
	}

	// Convert the result to a slice of ZSetEntry
	var result []ZSetEntry
	for _, entry := range zset {
		result = append(result, ZSetEntry{
			Value: entry.Member.(string),
			Score: float64(entry.Score),
		})
	}

	return result, nil
}
