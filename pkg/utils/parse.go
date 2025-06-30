package utils

import "strconv"

func ParseInt(s string) (int, error) {
	r, err := strconv.Atoi(s)
	if err != nil {
		return r, err
	}
	return r, nil

}
