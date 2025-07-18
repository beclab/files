package utils

import "os/exec"

func GetCommand(c string) (string, error) {
	return exec.LookPath(c)
}
