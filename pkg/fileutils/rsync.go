package fileutils

import (
	"bufio"
	"fmt"
	"k8s.io/klog/v2"
	"os"
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

	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	stdoutReader, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动rsync失败: %v", err)
	}

	// 同时处理输出和进度提取
	go func() {
		defer stdoutReader.Close()
		defer close(progressChan)

		scanner := bufio.NewScanner(stdoutReader)
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

		for scanner.Scan() {
			line := scanner.Text()

			// 实时输出到stdout
			fmt.Println(line)
			os.Stdout.Sync() // 强制刷新

			// 提取百分比进度
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if p, err := strconv.ParseFloat(matches[1], 64); err == nil {
					progressChan <- int(p)
				}
			}
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("rsync执行失败: %v", err)
	}

	return progressChan, nil
}
