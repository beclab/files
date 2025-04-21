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
	progressChan := make(chan int, 100)
	bwLimit := 1024

	stdoutReader, stdoutWriter := io.Pipe()
	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	cmd.Stdout = stdoutWriter

	go func() {
		err := cmd.Start()
		if err != nil {
			stdoutWriter.Close()
			klog.Errorf("Error starting rsync: %v", err)
			return
		}
	}()

	go func() {
		defer stdoutWriter.Close()
		defer close(progressChan)

		scanner := bufio.NewScanner(stdoutReader)
		// 改进的正则表达式：匹配整数百分比（如"7%"）
		re := regexp.MustCompile(`\b(\d+)%\b`)

		for scanner.Scan() {
			line := scanner.Text()
			klog.Infof("Rsync line: %s", line)

			// 查找所有匹配的百分比（progress2可能有多个进度行）
			for _, match := range re.FindAllStringSubmatch(line, -1) {
				if len(match) > 1 {
					if p, err := strconv.Atoi(match[1]); err == nil {
						progressChan <- p
						klog.Infof("Send progress: %d", p)
					}
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

	return progressChan, nil
}
