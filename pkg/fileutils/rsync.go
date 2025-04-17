package fileutils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

func ProcessProgress(progress, progressType int) int {
	// TODO: define progressType
	return progress
}

func ExecuteRsync(source, dest string) (chan int, error) {
	progressChan := make(chan int)

	cmd := exec.Command("rsync", "-av", "--info=progress2", source, dest)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Start()
	if err != nil {
		close(progressChan)
		return nil, fmt.Errorf("failed to start rsync: %v", err)
	}

	go func() {
		defer close(progressChan)
		scanner := bufio.NewScanner(&stderrBuf)
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
					progressChan <- int(p)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading rsync output: %v\n", err)
		}
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("Rsync failed: %v\n", err)
		}
	}()

	return progressChan, nil
}
