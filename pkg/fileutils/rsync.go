package fileutils

import (
	"bufio"
	"context"
	e "errors"
	"files/pkg/pool"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	basePrefix      = "/data/External/"
	SrcTypeExternal = "external"
)

var RootPrefix = os.Getenv("ROOT_PREFIX")

func init() {
	if RootPrefix == "" {
		RootPrefix = "/data"
	}
}

func ExecuteRsyncSimulated(source, dest string) (chan int, error) {
	progressChan := make(chan int, 10)

	go func() {
		defer close(progressChan)

		for i := 0; i <= 100; i++ {
			klog.Infof("Send progress: %d", i)
			progressChan <- i
			time.Sleep(1 * time.Second)
		}
	}()

	return progressChan, nil
}

func ExecuteRsyncBase(source, dest string) (chan int, error) {
	progressChan := make(chan int, 100)
	//bwLimit := 1024

	stdoutReader, stdoutWriter := io.Pipe()
	//cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	cmd := exec.Command("rsync", "-av", "--info=progress2", source, dest)
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

		reader := bufio.NewReader(stdoutReader)
		buffer := make([]byte, 4096)
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

		for {
			n, err := reader.Read(buffer)
			if n > 0 {
				output := string(buffer[:n])
				klog.Infoln("Rsync output:", output)

				lines := strings.Split(output, "\n")
				for i, line := range lines {
					if line != "" {
						if i == len(lines)-1 && !strings.HasSuffix(line, "\n") {
							line = strings.TrimSuffix(line, "\r")
						}

						var matched bool

						matches := re.FindAllStringSubmatch(line, -1)
						if len(matches) > 0 {
							for _, match := range matches {
								if len(match) > 1 {
									p := int(math.Floor(parseFloat(match[1])))
									matched = true
									progressChan <- p
									fmt.Printf("Progress: %d%%\n", p)
									klog.Infof("Send progress: %d", p)
								}
							}
						}

						if !matched {
							klog.Infof("No percent info in: %s", line)
						}
					}
				}
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				klog.Errorf("Error reading rsync output: %v", err)
				break
			}
		}
	}()

	go func() {
		if err := cmd.Wait(); err != nil {
			klog.Errorf("Rsync command failed: %v", err)
		}
	}()

	return progressChan, nil
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func externalCheckHeartBeat(cmdDone chan struct{}, task *pool.Task, path string, interval time.Duration) {
	srcPath, err := extractBaseDirectory(path)
	if err != nil {
		task.ErrChan <- fmt.Errorf("[TASK EXECUTE ERROR]: %w", err)
		pool.FailTask(task.ID)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-task.Ctx.Done():
			klog.Infoln("External destination check goroutine exit")
			return
		case <-cmdDone:
			return
		default:
			if err = checkDirectory(srcPath); err != nil {
				task.ErrChan <- fmt.Errorf("not found: %w", err)
				pool.FailTask(task.ID)
				return
			}
		}
	}
}

// func ExecuteRsyncWithContext(ctx context.Context, source, dest string) (chan int, chan string, chan error, error) {
func ExecuteRsync(task *pool.Task, src, dst string, progressLeft, progressRight int) error {
	klog.Infoln("Starting ExecuteRsync function")

	stdoutReader, stdoutWriter := io.Pipe()

	if src == "" {
		src = RootPrefix + task.Source
	}
	if dst == "" {
		dst = RootPrefix + task.Dest
	}

	//cmd := exec.CommandContext(task.Ctx, "rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", 1024), src, dst)
	//cmd := exec.CommandContext(task.Ctx, "rsync", "-av", "--info=progress2", "--debug=ALL", src, dst)
	cmd := exec.CommandContext(task.Ctx, "rsync", "-av", "--info=progress2", src, dst)
	cmd.Stdout = stdoutWriter

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	var cmdDone = make(chan struct{})

	if task.SrcType == SrcTypeExternal {
		wg.Add(1)
		go func() {
			defer wg.Done()
			externalCheckHeartBeat(cmdDone, task, task.Source, 1*time.Minute)
		}()
	}

	if task.DstType == SrcTypeExternal {
		wg.Add(1)
		go func() {
			defer wg.Done()
			externalCheckHeartBeat(cmdDone, task, task.Dest, 1*time.Minute)
		}()
	}

	// 启动命令
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(cmdDone)
		klog.Infoln("Starting rsync command goroutine")
		defer klog.Infoln("Rsync command goroutine completed")

		if err := cmd.Start(); err != nil {
			mu.Lock()
			if firstErr == nil {
				firstErr = err
				klog.Errorf("Failed to start rsync command: %v", firstErr)
			}
			mu.Unlock()
			stdoutWriter.Close()
			return
		}

		if err := cmd.Wait(); err != nil {
			mu.Lock()
			if firstErr == nil {
				firstErr = err
				klog.Errorf("Rsync command failed with error: %v", firstErr)
			}
			mu.Unlock()
		}

		stdoutWriter.Close()
	}()

	// 处理stdout的goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infoln("Starting stdout processing goroutine")
		defer klog.Infoln("Stdout processing goroutine completed")

		reader := bufio.NewReader(stdoutReader)
		buffer := make([]byte, 4096)
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

		eofCount := 0
		const maxEOFCount = 10 // 连续读取到 10 个 EOF 后退出

		for {
			select {
			case <-task.Ctx.Done():
				klog.Infoln("Stdout processing goroutine exiting due to cancellation")
				return
			default:
				n, err := reader.Read(buffer)
				if n > 0 {
					output := string(buffer[:n])
					klog.Infoln("Rsync output:", output)

					lines := strings.Split(output, "\n")
					for i, line := range lines {
						if line != "" {
							if i == len(lines)-1 && !strings.HasSuffix(line, "\n") {
								line = strings.TrimSuffix(line, "\r")
							}
							select {
							case task.LogChan <- line:
								klog.Infof("Send Log: %s", line)
							default:
								klog.Warningf("Log channel full, dropping %s%%", line)
							}

							var matched bool
							matches := re.FindAllStringSubmatch(line, -1)
							if len(matches) > 0 {
								for _, match := range matches {
									if len(match) > 1 {
										p := int(math.Floor(parseFloat(match[1])))
										p = pool.ProcessProgress(p, progressLeft, progressRight)
										matched = true
										select {
										case task.ProgressChan <- p:
											fmt.Printf("Progress: %d%%\n", p)
											klog.Infof("Send progress: %d", p)
										default:
											klog.Warningf("Progress channel full, dropping %d%%", p)
										}
									}
								}
							}

							if !matched {
								klog.Infof("No percent info in: %s", line)
							}
						}
					}
					eofCount = 0 // 重置 EOF 计数器
				}
				if err != nil {
					if err != io.EOF {
						mu.Lock()
						if firstErr == nil {
							firstErr = err
							klog.Errorf("Error reading stdout: %v", firstErr)
						}
						mu.Unlock()
					} else {
						eofCount++
						klog.Infof("Have read %d EOFs", eofCount)
						//select {
						//case task.LogChan <- "":
						//	klog.Infof("Send Log: %s", "EOF")
						//default:
						//	klog.Warningf("Log channel full, dropping %s%%", "EOF")
						//}
						time.Sleep(100 * time.Millisecond)
						if eofCount >= maxEOFCount {
							klog.Infoln("Finished reading stdout after multiple EOFs")
							return
						}
					}
				}
			}
		}
	}()

	// 取消处理goroutine
	go func() {
		klog.Infoln("Starting cancellation handling goroutine")
		defer klog.Infoln("Cancellation handling goroutine completed")

		// 创建总超时上下文（1小时）
		totalTimeoutCtx, cancelTotalTimeout := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancelTotalTimeout() // 确保释放资源

		select {
		case <-totalTimeoutCtx.Done():
			// 总超时触发：强制清理
			klog.Errorln("Total timeout (1h) reached! Forcing exit...")
			if cmd.Process != nil {
				klog.Infoln("Killing rsync command due to total timeout")
				cmd.Process.Kill() // 立即强制终止
			}
			if stdoutWriter != nil {
				stdoutWriter.Close() // 关闭输出流
			}
			return // 直接退出goroutine

		case <-task.Ctx.Done():
			// 正常取消信号处理逻辑
			if cmd.Process != nil {
				klog.Infoln("Sending interrupt signal to rsync command")
				cmd.Process.Signal(os.Interrupt)

				done := make(chan error)
				go func() {
					done <- cmd.Wait()
				}()

				// 保留原有的5秒超时逻辑
				select {
				case <-done:
					klog.Infoln("Rsync command interrupted successfully")
				case <-time.After(5 * time.Second):
					klog.Infoln("Killing rsync command after 5s timeout")
					cmd.Process.Kill()
					<-done // 等待实际退出
				}
			}
			// 关闭资源（总超时情况也会执行到这里）
			if stdoutWriter != nil {
				stdoutWriter.Close()
			}
		}
	}()

	// 错误处理goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infoln("Starting error handling goroutine")
		defer klog.Infoln("Error handling goroutine completed")

		for {
			select {
			case <-task.Ctx.Done():
				if firstErr != nil {
					klog.Errorf("Operation cancelled with error: %v", firstErr)
				}
				return
			case <-cmdDone:
				return
			default:
				if firstErr != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								klog.Errorf("Rsync command failed: %v (channel closed)", firstErr)
							}
						}()

						klog.Errorf("ExecuteRsync failed with error: %v", firstErr)
						errMsg := parseRsyncError(firstErr, task)
						task.ErrChan <- e.New(errMsg)
						pool.FailTask(task.ID)
						return

						//select {
						//case task.ErrChan <- firstErr:
						//	firstErr = nil
						//	klog.Infoln("Error sent to ErrChan")
						//default:
						//	klog.Errorf("Rsync command failed: %v (channel full)", firstErr)
						//}
					}()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// 命令执行状态监控
	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infoln("Starting command execution state monitoring goroutine")
		defer klog.Infoln("Command execution state monitoring goroutine completed")

		<-cmdDone
		klog.Infoln("Rsync command execution state monitored")
		stdoutWriter.Close()
	}()

	klog.Infoln("Waiting for all goroutines to complete")
	wg.Wait()
	klog.Infoln("All goroutines completed")

	if firstErr != nil {
		if task.Status != "failed" {
			errMsg := parseRsyncError(firstErr, task)
			task.Mu.Lock()
			task.Log = append(task.Log, errMsg)
			task.FailedReason = errMsg
			task.Mu.Unlock()
			pool.FailTask(task.ID)
		}
	}

	return firstErr
}

func parseRsyncError(err error, task *pool.Task) string {
	errStr := err.Error()
	task.Logging(errStr)
	if errStr == "exit status 11" {
		time.Sleep(100 * time.Millisecond)
		targetDir := FindExistingDir(RootPrefix + task.Dest)
		task.Logging(fmt.Sprintf("checking dir %s with problem", targetDir))
		if targetDir == "" {
			return fmt.Sprintf("write failed on %s: no such file or directory", task.Dest)
		} else if _, err = os.Stat(targetDir); err == nil {
			return fmt.Sprintf("write failed on %s: no space left on device", task.Dest)
		} else {
			return fmt.Sprintf("write failed on %s: %v", task.Dest, err)
		}
	} else if strings.Contains(errStr, "No space left on device") {
		return fmt.Sprintf("write failed on %s: no space left on device", task.Dest)
	} else if errStr == "exit status 13" || strings.Contains(errStr, "Permission denied") {
		return fmt.Sprintf("write failed on %s: permission denied", task.Dest)
	} else if errStr == "exit status 23" || strings.Contains(errStr, "No such file or directory") {
		return fmt.Sprintf("write failed on %s: no such file or directory", task.Dest)
	} else {
		return fmt.Sprintf("write failed on %s with error: %v", task.Dest, err)
	}
}
