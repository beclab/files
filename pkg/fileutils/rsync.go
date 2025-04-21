package fileutils

import (
	"bufio"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

func ExecuteRsyncSimulated(source, dest string) (chan int, error) {
	progressChan := make(chan int, 10) // 使用缓冲通道

	go func() {
		defer close(progressChan)

		// 模拟进度从 0 到 100，每秒增加 1
		for i := 0; i <= 100; i++ {
			klog.Infof("Send progress: %d", i)
			progressChan <- i           // 发送进度到通道
			time.Sleep(1 * time.Second) // 模拟每秒更新一次
		}
	}()

	return progressChan, nil
}

func ExecuteRsync(source, dest string) (chan int, error) {
	progressChan := make(chan int, 100) // 缓冲区大小为100
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
			klog.Infof("Rsync line: %s", line) // 确保这一行被执行
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
					progressChan <- int(p)
					klog.Infof("Send progress: %d", int(p))
				}
			}
		}
		if err := scanner.Err(); err != nil {
			klog.Errorf("Error reading rsync output: %v", err)
		}
	}()

	go func() {
		if err := cmd.Wait(); err != nil {
			klog.Errorf("Rsync command failed: %v", err)
		}
	}()

	// 为了调试，可以添加一个短暂的等待来确保 goroutine 有时间启动（通常不需要，但用于调试）
	// time.Sleep(100 * time.Millisecond)

	return progressChan, nil
}
