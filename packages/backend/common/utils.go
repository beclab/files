package common

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/rs/zerolog/log"
	"io"
)

func Md5File(f io.Reader) string {
	hasher := md5.New()
	if _, err := io.Copy(hasher, f); err != nil {
		log.Error().Msgf("Md5 file error %v", err)
		return ""
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
