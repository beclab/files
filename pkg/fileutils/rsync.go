package fileutils

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

func ExecuteRsyncSimulated(source, dest string) (chan int, error) {
	progressChan := make(chan int)

	go func() {
		defer close(progressChan)

		// 模拟进度从 0 到 100，每秒增加 1
		for i := 0; i <= 100; i++ {
			select {
			case progressChan <- i: // 发送进度到通道
			case <-time.After(1 * time.Second): // 模拟每秒更新一次（实际上循环本身已经控制了节奏，这里 `time.After` 是冗余保险）
			}
		}
	}()

	// 模拟任务完成后返回（虽然这里没有实际使用 source 和 dest，但函数签名保留了它们）
	time.Sleep(101 * time.Second) // 确保模拟的任务运行足够长的时间（大于 100 秒）
	return progressChan, nil
}

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
