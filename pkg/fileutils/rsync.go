package fileutils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
)

func ExecuteRsync(source, dest string) (chan int, error) {
	progressChan := make(chan int)
	bwLimit := 1024

	// 创建管道用于实时读取 rsync 的 stderr
	stderrReader, stderrWriter := io.Pipe()

	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	cmd.Stderr = stderrWriter // 将 rsync 的 stderr 重定向到管道

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	fmt.Printf("Starting rsync command: %v\n", cmd.Args)

	err := cmd.Start()
	if err != nil {
		stderrWriter.Close() // 确保管道关闭
		return nil, fmt.Errorf("failed to start rsync: %v", err)
	}

	// 启动一个协程读取 rsync 的 stderr 并解析进度
	go func() {
		defer func() {
			stderrWriter.Close() // 关闭管道写入端
			close(progressChan)  // 关闭进度通道
		}()

		scanner := bufio.NewScanner(stderrReader)
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

	// 等待 rsync 命令完成
	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("Rsync failed: %v\n", err)
			fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
		} else {
			fmt.Printf("Rsync completed successfully.\n")
			fmt.Printf("Rsync STDOUT: %s\n", stdoutBuf.String())
		}
	}()

	return progressChan, nil
}
