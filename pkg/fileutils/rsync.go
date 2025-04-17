package fileutils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

func ExecuteRsync(source, dest string) (chan int, error) {
	progressChan := make(chan int)
	bwLimit := 100

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

	progressChanMade := false // 用于确保 progressChan 只关闭一次

	go func() {
		defer func() {
			if !progressChanMade {
				close(progressChan)
			}
		}()
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
		progressChanMade = true // 标记为已关闭（仅在正常退出时）
	}()

	// 模拟定期检查 stderrBuf（实际上不需要，因为 bufio.Scanner 会自动处理）
	// 但为了演示，我们可以添加一个 ticker 来“模拟”这种行为（实际不推荐这样做）
	ticker := time.NewTicker(100 * time.Millisecond) // 每100ms检查一次（仅用于演示）
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			// 这里实际上不需要做任何事情，因为 bufio.Scanner 已经在处理
			// 但如果需要，可以添加一些调试输出
			fmt.Println("Checking stderrBuf...")
		}
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("Rsync failed: %v\n", err)
			fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
			fmt.Printf("Rsync STDERR: %s\n", stderrBuf.String())
		} else {
			fmt.Printf("Rsync completed successfully.\n")
			fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
			fmt.Printf("Rsync STDERR: %s\n", stderrBuf.String())
		}
		if !progressChanMade { // 确保只关闭一次
			close(progressChan)
			progressChanMade = true
		}
	}()

	return progressChan, nil
}
