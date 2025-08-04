package utils

import (
	"strconv"
	"time"
)

func ParseInt(s string) (int, error) {
	r, err := strconv.Atoi(s)
	if err != nil {
		return r, err
	}
	return r, nil

}

func ParseInt64(s string) (int64, error) {
	r, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return r, err
	}
	return r, nil
}

func ParseUnixMilli(s int64) time.Time {
	var date = time.UnixMilli(s)

	return date
}
