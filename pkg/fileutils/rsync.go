package fileutils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

func ExecuteRsync(source, dest string) (chan int, error) {
	progressChan := make(chan int)
	bwLimit := 1024

	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	fmt.Printf("Starting rsync command: %v\n", cmd.Args)

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
			fmt.Printf("[RSYNC STDERR] %s\n", line) // 输出 rsync 的 stderr 信息
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
			fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String()) // 输出 rsync 的 stdout 信息
			fmt.Printf("Rsync STDERR: %s\n", stderrBuf.String()) // 再次输出 rsync 的 stderr 信息，确保完整性
		} else {
			fmt.Printf("Rsync completed successfully.\n")
			fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
			fmt.Printf("Rsync STDERR: %s\n", stderrBuf.String())
		}
	}()

	return progressChan, nil
}
