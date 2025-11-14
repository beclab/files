package utils

import (
	"fmt"
	"os"
	"strings"
)

func Contains(slice []string, value string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}
func IsTestEnv() bool {
	msDebug := os.Getenv("MS_DEBUG")
	if strings.EqualFold(msDebug, "magic") {
		fmt.Println("test env")
		return true
	}

	return false
}
