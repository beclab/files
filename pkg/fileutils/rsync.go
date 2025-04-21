package fileutils

import (
	"bufio"
	"bytes"
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
	cmd := exec.Command("rsync", "-av", "--info=progress2", "--stats", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	cmd.Stdout = stdoutWriter

	var buf bytes.Buffer
	var lastProgress int

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
		progressRe := regexp.MustCompile(`(\d+\.?\d*)%`)
		statsRe := regexp.MustCompile(`Total.*?(\d+\.?\d*)%`)

		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")

			// 从进度行提取
			if matches := progressRe.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.Atoi(matches[1]); err == nil && p != lastProgress {
					lastProgress = p
					progressChan <- p
					klog.Infof("Progress: %d%%", p)
				}
			}

			// 从统计行提取（最终进度）
			if matches := statsRe.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.Atoi(matches[1]); err == nil && p != lastProgress {
					lastProgress = p
					progressChan <- p
					klog.Infof("Final progress: %d%%", p)
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
		} else {
			progressChan <- 100 // 确保最终进度为100%
		}
	}()

	return progressChan, nil
}
