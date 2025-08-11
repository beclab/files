package token

import (
	"crypto/rand"
	"files/pkg/redisutils"
	"fmt"
	"k8s.io/klog/v2"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const keyPrefix = "tk:secret:"

type customClaims struct {
	Data []byte `json:"data"`
	jwt.RegisteredClaims
}

func (c *customClaims) VerifyIssuer(issuer string) bool {
	if c.Issuer != issuer {
		klog.Infof("~~~Debug log: Issuer mismatch. Expected: %s, Actual: %s", issuer, c.Issuer)
		return false
	}
	klog.Infof("~~~Debug log: Issuer verification succeeded")
	return true
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
	klog.Infof("~~~Debug log: Entering ParseToken with tokenString: %s", tokenString)
	defer klog.Infof("~~~Debug log: Exiting ParseToken")

	secret, err := getTokenSecret(secretKey)
	if err != nil {
		klog.Infof("~~~Debug log: Failed to get token secret: %v", err)
		return nil, err
	}
	klog.Infof("~~~Debug log: Successfully retrieved secret for key: %s", secretKey)

	claims := &customClaims{}
	klog.Infof("~~~Debug log: Created customClaims instance: %+v", claims)

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		klog.Infof("~~~Debug log: Validating signing method for token: %+v", token.Header)

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			errMsg := fmt.Sprintf("Unexpected signing method: %v", token.Header["alg"])
			klog.Infof("~~~Debug log: %s", errMsg)
			return nil, fmt.Errorf(errMsg)
		}

		klog.Infof("~~~Debug log: Using secret of length %d for validation", len(secret))
		return secret, nil
	}

	klog.Infof("~~~Debug log: Parsing token with claims type: %T", claims)
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		keyFunc,
		jwt.WithIssuer("olares"),
	)

	if err != nil {
		klog.Infof("~~~Debug log: Token parsing failed with error: %v", err)
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	klog.Infof("~~~Debug log: Successfully parsed token: %+v", token)
	klog.Infof("~~~Debug log: Token headers: %+v", token.Header)
	klog.Infof("~~~Debug log: Token claims: %+v", claims)

	if token.Valid {
		klog.Infof("~~~Debug log: Token is valid")
	} else {
		klog.Infof("~~~Debug log: Token is invalid. Validation errors: %+v", token.Valid)
	}

	if customClaims, ok := token.Claims.(*customClaims); ok {
		klog.Infof("~~~Debug log: Successfully cast claims to customClaims type")
		if customClaims.VerifyIssuer("olares") {
			klog.Infof("~~~Debug log: Issuer verification passed")
		} else {
			klog.Infof("~~~Debug log: Issuer verification failed")
		}
	} else {
		klog.Infof("~~~Debug log: Failed to cast claims to customClaims type")
	}

	if claims, ok := token.Claims.(*customClaims); ok && token.Valid {
		klog.Infof("~~~Debug log: All validations passed. Returning claims data")
		return claims.Data, nil
	}

	klog.Infof("~~~Debug log: Token validation failed at final check stage")
	return nil, fmt.Errorf("invalid token")
}

//func ParseToken(tokenString string, secretKey string) ([]byte, error) {
//
//	secret, err := getTokenSecret(secretKey)
//	if err != nil {
//		return nil, err
//	}
//	token, err := jwt.ParseWithClaims(
//		tokenString,
//		&customClaims{},
//		func(token *jwt.Token) (interface{}, error) {
//
//			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
//				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
//			}
//			return secret, nil
//		},
//		jwt.WithIssuer("olares"),
//	)
//
//	if err != nil {
//		return nil, fmt.Errorf("failed to parse token: %v", err)
//	}
//
//	if claims, ok := token.Claims.(*customClaims); ok && token.Valid {
//		return claims.Data, nil
//	}
//	return nil, fmt.Errorf("invalid token")
//}

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
