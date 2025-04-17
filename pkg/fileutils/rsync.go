package fileutils

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
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
		for i := 0; i < 100; i++ {
			time.Sleep(100 * time.Millisecond) // 模拟进度更新
			progressChan <- i
		}
	}()
	return progressChan, nil

	//progressChanMade := false // 用于确保 progressChan 只关闭一次

	//go func() {
	//	defer func() {
	//		if !progressChanMade {
	//			close(progressChan)
	//		}
	//	}()
	//	scanner := bufio.NewScanner(&stderrBuf)
	//	re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
	//	for scanner.Scan() {
	//		line := scanner.Text()
	//		fmt.Printf("[RSYNC STDERR] %s\n", line) // 输出 rsync 的 stderr 信息
	//		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
	//			if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
	//				progressChan <- int(p)
	//			}
	//		}
	//	}
	//	if err := scanner.Err(); err != nil {
	//		fmt.Printf("Error reading rsync output: %v\n", err)
	//	}
	//	progressChanMade = true // 标记为已关闭（仅在正常退出时）
	//}()
	//
	//go func() {
	//	err := cmd.Wait()
	//	if err != nil {
	//		fmt.Printf("Rsync failed: %v\n", err)
	//		fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
	//		fmt.Printf("Rsync STDERR: %s\n", stderrBuf.String())
	//	} else {
	//		fmt.Printf("Rsync completed successfully.\n")
	//		fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
	//		fmt.Printf("Rsync STDERR: %s\n", stderrBuf.String())
	//	}
	//	if !progressChanMade { // 确保只关闭一次
	//		close(progressChan)
	//		progressChanMade = true
	//	}
	//}()

	//return progressChan, nil
}
