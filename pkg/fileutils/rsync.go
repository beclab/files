package fileutils

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
)

func ExecuteRsync(source, dest string) (chan int, error) {
	progressChan := make(chan int)
	bwLimit := 1024

	stderrReader, stderrWriter := io.Pipe()
	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	cmd.Stderr = stderrWriter

	err := cmd.Start()
	if err != nil {
		stderrWriter.Close()
		return nil, fmt.Errorf("failed to start rsync: %v", err)
	}

	go func() {
		defer stderrWriter.Close()
		defer close(progressChan)

		scanner := bufio.NewScanner(stderrReader)
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
					select {
					case progressChan <- int(p): // 确保非阻塞发送
					default: // 防止进度通道阻塞
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading rsync output: %v\n", err)
		}
	}()

	go func() { cmd.Wait() }() // 确保 cmd.Wait() 不会阻塞主逻辑
	return progressChan, nil
}
