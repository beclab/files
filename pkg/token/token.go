package token

import (
	"crypto/rand"
	"files/pkg/redisutils"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const keyPrefix = "tk:secrect:"

type customClaims struct {
	Data []byte `json:"data"`
	jwt.RegisteredClaims
}

func InitTokenSecret(Key string) error {
	secret, err := generateHS256Key()
	if err != nil {
		return err
	}
	_, err = redisutils.RedisClient.SetNX(keyPrefix+Key, secret, time.Hour*24).Result()
	return err
}

func GenerateToken(userData []byte, secretKey string, expireTime time.Duration) (string, error) {
	claims := customClaims{
		Data: userData,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "olares",
			Subject:   "user_token",
		},
	}

	// 创建Token，使用HS256算法
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret, err := getTokenSecret(secretKey)
	if err != nil {
		return "", err
	}
	expireTokenSecret(secretKey, expireTime)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		fmt.Println("创建令牌失败:", err)
		return "", err
	}
	return tokenString, nil
}

func generateHS256Key() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	return key, err
}

func getTokenSecret(Key string) ([]byte, error) {
	secret, err := redisutils.RedisClient.Get(keyPrefix + Key).Bytes()
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func expireTokenSecret(Key string, expireTime time.Duration) error {
	_, err := redisutils.RedisClient.Expire(keyPrefix+Key, time.Hour*24+expireTime).Result()
	if err != nil {
		return err
	}
	return nil
}
