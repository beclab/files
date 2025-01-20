package common

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

var RootPrefix = os.Getenv("ROOT_PREFIX")

func Md5File(filepath string) (string, error) {
	if !strings.HasPrefix(filepath, RootPrefix) {
		filepath = RootPrefix + filepath
	}

	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	buf := make([]byte, 8192)

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

//func Md5File(f io.Reader) string {
//	hasher := md5.New()
//	if _, err := io.Copy(hasher, f); err != nil {
//		log.Error().Msgf("Md5 file error %v", err)
//		return ""
//	}
//	return hex.EncodeToString(hasher.Sum(nil))
//}

func Md5String(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}
