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

func initTokenSecret(Key string) error {
	secret, err := generateHS256Key()
	if err != nil {
		return err
	}
	_, err = redisutils.RedisClient.SetNX(keyPrefix+Key, secret, time.Hour*24).Result()
	return err
}

func GenerateToken(userData []byte, secretKey string, expireTime time.Duration) (string, error) {
	err := initTokenSecret(secretKey)
	if err != nil {
		return "", err
	}
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

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret, err := getTokenSecret(secretKey)
	if err != nil {
		return "", err
	}
	expireTokenSecret(secretKey, expireTime)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func ParseToken(tokenString string, secretKey string) ([]byte, error) {

	secret, err := getTokenSecret(secretKey)
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&customClaims{},
		func(token *jwt.Token) (interface{}, error) {

			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		},
		jwt.WithIssuer("olares"),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	if claims, ok := token.Claims.(*customClaims); ok && token.Valid {
		return claims.Data, nil
	}
	return nil, fmt.Errorf("invalid token")
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

func expireTokenSecret(Key string, addExpireTime time.Duration) error {
	_, err := redisutils.RedisClient.Expire(keyPrefix+Key, time.Hour*24+addExpireTime).Result()
	if err != nil {
		return err
	}
	return nil
}
