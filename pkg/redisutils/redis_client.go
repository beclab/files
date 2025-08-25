package redisutils

import (
	//"context"
	//"github.com/cloudwego/hertz/pkg/app/client"
	//"github.com/cloudwego/hertz/pkg/app/middlewares/client/sd"
	//"github.com/cloudwego/hertz/pkg/common/config"
	//"github.com/cloudwego/hertz/pkg/common/hlog"
	//"github.com/hertz-contrib/registry/redis"
	"github.com/go-redis/redis"
	"os"
	"strconv"
)

var RedisClient *redis.Client

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

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort, // "localhost:6379",
		Password: redisPassword,               // "difyai123456",
		DB:       redisDB,                     //0,
	})
	//cli, err := client.NewClient()
	//if err != nil {
	//	panic(err)
	//}
	//r := redis.NewRedisResolver(redisHost+":"+redisPort, redis.WithPassword(redisPassword), redis.WithDB(redisDB))
	//cli.Use(sd.Discovery(r))
	//for i := 0; i < 10; i++ {
	//	status, body, err := cli.Get(context.Background(), nil, "http://hertz.test.demo/ping", config.WithSD(true))
	//	if err != nil {
	//		hlog.Fatal(err)
	//	}
	//	hlog.Infof("HERTZ: code=%d,body=%s", status, string(body))
	//}
}
