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
	"strings"
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

	var buf bytes.Buffer // 新增缓冲区

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
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line) // 累积输出到缓冲区

			// 尝试从缓冲区提取完整进度行
			if fullLine := extractFullProgressLine(&buf, re); fullLine != "" {
				if p, err := parseProgress(fullLine, re); err == nil {
					progressChan <- p
					klog.Infof("Send progress: %d", p)
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

// extractFullProgressLine 尝试从缓冲区提取完整的进度行
func extractFullProgressLine(buf *bytes.Buffer, re *regexp.Regexp) string {
	// 查找最后一个换行符
	lastNewline := bytes.LastIndex(buf.Bytes(), []byte("\n"))
	if lastNewline == -1 {
		return "" // 没有完整行
	}

	// 提取最后一行
	line := buf.Bytes()[lastNewline+1:]

	// 检查是否包含完整进度信息
	if re.Match(line) {
		return string(line)
	}
	return ""
}

// parseProgress 从完整行解析进度值
func parseProgress(line string, re *regexp.Regexp) (int, error) {
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return strconv.Atoi(strings.TrimSuffix(matches[1], "%"))
	}
	return 0, fmt.Errorf("no progress found in line: %s", line)
}
