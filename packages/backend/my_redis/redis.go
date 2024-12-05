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

func RedisHSet(key string, field string, value interface{}) {
	// 设置键值对
	err := redisClient.HSet(key, field, value).Err()
	if err != nil {
		fmt.Println("设置Hash表键值对失败:", err)
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

func RedisHGet(key, field string) string {
	// 获取键的值
	value, err := redisClient.HGet(key, field).Result()
	if err != nil {
		if err != redis.Nil {
			fmt.Println("获取Hash表键值失败:", err)
			return ""
		}
		fmt.Println("Hash表", key, "和键", field, "不存在")
		return ""
	}
	fmt.Println("值:", value)
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

func RedisDelKey(key string) {
	redisClient.Del(key)
	return
}

// RedisZAdd 添加成员到有序集合
func RedisZAdd(key string, members map[string]float64) error {
	// 将 map 转换为 []redis.Z
	zMembers := make([]redis.Z, 0, len(members))
	for member, score := range members {
		zMembers = append(zMembers, redis.Z{Score: score, Member: member})
	}

	// 添加到有序集合
	err := redisClient.ZAdd(key, zMembers...).Err()
	if err != nil {
		fmt.Println("添加成员到有序集合失败:", err)
		return err
	}
	fmt.Println("成员已成功添加到有序集合")
	return nil
}

// ZScore 获取成员的分数
func RedisZScore(key, member string) (*float64, error) {
	score, err := redisClient.ZScore(key, member).Result()
	if err != nil {
		if err == redis.Nil {
			fmt.Println("成员", member, "在有序集合", key, "中不存在")
		} else {
			fmt.Println("获取成员分数失败:", err)
		}
		return nil, err
	}
	fmt.Println("成员", member, "的分数是:", score)
	return &score, nil
}

// ZIncrBy 增加成员的分数
func RedisZIncrBy(key, member string, increment float64) {
	err := redisClient.ZIncrBy(key, increment, member).Err()
	if err != nil {
		fmt.Println("增加成员分数失败:", err)
		return
	}
	fmt.Println("成员的分数已成功增加")
}

// ZRank 获取成员的排名（按分数从小到大）
func RedisZRank(key, member string) *int64 {
	rank, err := redisClient.ZRank(key, member).Result()
	if err != nil {
		if err == redis.Nil {
			fmt.Println("成员", member, "在有序集合", key, "中不存在")
		} else {
			fmt.Println("获取成员排名失败:", err)
		}
		return nil
	}
	fmt.Println("成员", member, "的排名是:", rank)
	return &rank
}

// ZRem 删除有序集合中的成员
func RedisZRem(key, member string) error {
	err := redisClient.ZRem(key, member).Err()
	if err != nil {
		fmt.Println("删除有序集合中的成员失败:", err)
		return err
	}
	fmt.Println("成员已成功删除")
	return nil
}

// ZRange 获取有序集合中指定范围的成员（按分数从小到大）
func RedisZRange(key string, start, stop int64) ([]string, error) {
	members, err := redisClient.ZRange(key, start, stop).Result()
	if err != nil {
		fmt.Println("获取有序集合中指定范围的成员失败:", err)
		return nil, err
	}
	fmt.Println("有序集合中指定范围的成员是:", members)
	return members, nil
}

func RedisZRangeByScore(key, minScore, maxScore string, withScores bool) ([]string, error) {
	// 准备 ZRangeByScore 的选项
	zRangeBy := redis.ZRangeBy{
		Min:    minScore,
		Max:    maxScore,
		Offset: 0,
		Count:  -1,
	}

	// 根据是否需要分数来调用 ZRangeByScore
	var members []string
	var err error
	if withScores {
		// 如果需要分数，我们将获取成员和分数的映射
		memberScores, err := redisClient.ZRangeByScoreWithScores(key, zRangeBy).Result()
		if err != nil {
			fmt.Println("获取有序集合中指定分数范围的成员及其分数失败:", err)
			return nil, err
		}

		// 只提取成员部分
		for _, memberScore := range memberScores {
			members = append(members, memberScore.Member.(string))
		}
	} else {
		// 如果不需要分数，我们直接获取成员列表
		members, err = redisClient.ZRangeByScore(key, zRangeBy).Result()
		if err != nil {
			fmt.Println("获取有序集合中指定分数范围的成员失败:", err)
			return nil, err
		}
	}

	// 打印获取到的成员列表（可选）
	fmt.Println("有序集合中指定分数范围的成员是:", members)

	// 返回成员列表和 nil 错误
	return members, nil
}

// ZCard 获取有序集合的基数（成员数量）
func RedisZCard(key string) int64 {
	card, err := redisClient.ZCard(key).Result()
	if err != nil {
		fmt.Println("获取有序集合的基数失败:", err)
		return 0
	}
	fmt.Println("有序集合的基数是:", card)
	return card
}

// RedisZGetMaxMember 获取有序集合中分数最大的成员及其分数
func RedisZGetMaxMember(key string) (string, float64, error) {
	// 使用 ZREVRANGE 命令，从高到低获取成员，只取第一个
	result, err := redisClient.ZRevRangeWithScores(key, 0, 0).Result()
	if err != nil {
		return "", 0, fmt.Errorf("获取有序集合中分数最大的成员失败: %v", err)
	}

	// 检查结果是否为空
	if len(result) == 0 {
		return "", 0, fmt.Errorf("有序集合 %s 中没有成员", key)
	}

	// 返回分数最大的成员及其分数
	member := result[0].Member.(string)
	score := result[0].Score
	return member, score, nil
}

// RedisZGetMinMember 获取有序集合中分数最小的成员及其分数
func RedisZGetMinMember(key string) (string, float64, error) {
	// 使用 ZRANGE 命令，从低到高获取成员，只取第一个
	result, err := redisClient.ZRangeWithScores(key, 0, 0).Result()
	if err != nil {
		return "", 0, fmt.Errorf("获取有序集合中分数最小的成员失败: %v", err)
	}

	// 检查结果是否为空
	if len(result) == 0 {
		return "", 0, fmt.Errorf("有序集合 %s 中没有成员", key)
	}

	// 返回分数最小的成员及其分数
	member := result[0].Member.(string)
	score := result[0].Score
	return member, score, nil
}
