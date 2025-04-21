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
	// 使用带缓冲的通道，避免因接收方处理不及时导致的阻塞
	progressChan := make(chan int, 100) // 缓冲区大小为100
	bwLimit := 1024

	stderrReader, stderrWriter := io.Pipe()
	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	cmd.Stderr = stderrWriter

	go func() {
		err := cmd.Start()
		if err != nil {
			stderrWriter.Close()
			klog.Errorf("Error starting rsync: %v", err)
		}
	}()

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
					// 发送进度到通道（带缓冲通道通常不会阻塞）
					progressChan <- int(p)
					klog.Infof("Send progress: %d", int(p))
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
