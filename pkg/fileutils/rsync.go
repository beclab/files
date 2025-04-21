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
	progressChan := make(chan int) // 无缓冲通道，确保每次发送都有接收方处理
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
			klog.Infof("Rsync line: %s", line)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
					// 使用 select 确保非阻塞发送
					select {
					case progressChan <- int(p): // 成功发送进度
						klog.Infof("Send progress: %d", int(p))
					default:
						// 如果通道已满，可以选择丢弃数据或记录日志（这里选择丢弃）
						// fmt.Println("Progress channel is full, dropping progress update")
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading rsync output: %v\n", err)
		}
	}()

	// 确保 cmd.Wait() 不会阻塞主逻辑
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Printf("Rsync command failed: %v\n", err)
		}
	}()

	return progressChan, nil
}
