package fileutils

import (
	"bufio"
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
	bwLimit := 1024

	stdoutReader, stdoutWriter := io.Pipe()
	cmd := exec.Command("rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", bwLimit), source, dest)
	//cmd := exec.Command("rsync", "-av", "--info=progress2", source, dest)
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

//func ExecuteRsyncWithContext(ctx context.Context, source, dest string) (chan int, chan error, error) {
//	progressChan := make(chan int, 100)
//	errChan := make(chan error, 1)
//
//	stdoutReader, stdoutWriter := io.Pipe()
//	cmd := exec.CommandContext(ctx, "rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", 1024), source, dest)
//	cmd.Stdout = stdoutWriter
//
//	go func() {
//		err := cmd.Start()
//		if err != nil {
//			stdoutWriter.Close()
//			errChan <- err
//			return
//		}
//	}()
//
//	go func() {
//		defer stdoutWriter.Close()
//		defer close(progressChan)
//
//		reader := bufio.NewReader(stdoutReader)
//		buffer := make([]byte, 4096)
//		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
//
//		for {
//			n, err := reader.Read(buffer)
//			if n > 0 {
//				output := string(buffer[:n])
//				klog.Infoln("Rsync output:", output)
//
//				lines := strings.Split(output, "\n")
//				for i, line := range lines {
//					if line != "" {
//						if i == len(lines)-1 && !strings.HasSuffix(line, "\n") {
//							line = strings.TrimSuffix(line, "\r")
//						}
//
//						var matched bool
//
//						matches := re.FindAllStringSubmatch(line, -1)
//						if len(matches) > 0 {
//							for _, match := range matches {
//								if len(match) > 1 {
//									p := int(math.Floor(parseFloat(match[1])))
//									matched = true
//									progressChan <- p
//									fmt.Printf("Progress: %d%%\n", p)
//									klog.Infof("Send progress: %d", p)
//								}
//							}
//						}
//
//						if !matched {
//							klog.Infof("No percent info in: %s", line)
//						}
//					}
//				}
//			}
//			if err != nil {
//				if err == io.EOF {
//					break
//				}
//				klog.Errorf("Error reading rsync output: %v", err)
//				errChan <- err
//				break
//			}
//		}
//	}()
//
//	go func() {
//		if err := cmd.Wait(); err != nil {
//			klog.Errorf("Rsync command failed: %v", err)
//			errChan <- err
//		}
//	}()
//
//	return progressChan, errChan, nil
//}

//func ExecuteRsyncWithContext(ctx context.Context, source, dest string) (chan int, chan error, error) {
//	progressChan := make(chan int, 100)
//	errChan := make(chan error, 1)
//	stdoutReader, stdoutWriter := io.Pipe()
//
//	cmd := exec.CommandContext(ctx, "rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", 256), source, dest)
//	cmd.Stdout = stdoutWriter
//
//	var wg sync.WaitGroup
//	var mu sync.Mutex
//	var firstErr error
//
//	// 启动命令
//	go func() {
//		if err := cmd.Start(); err != nil {
//			stdoutWriter.Close()
//			return
//		}
//	}()
//
//	// Goroutine: 处理 stdout 并解析进度
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		defer stdoutWriter.Close()
//		defer close(progressChan)
//
//		reader := bufio.NewReader(stdoutReader)
//		buffer := make([]byte, 4096)
//		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
//
//		for {
//			n, err := reader.Read(buffer)
//			if n > 0 {
//				output := string(buffer[:n])
//				klog.Infoln("Rsync output:", output)
//
//				lines := strings.Split(output, "\n")
//				for i, line := range lines {
//					if line != "" {
//						if i == len(lines)-1 && !strings.HasSuffix(line, "\n") {
//							line = strings.TrimSuffix(line, "\r")
//						}
//
//						var matched bool
//
//						matches := re.FindAllStringSubmatch(line, -1)
//						if len(matches) > 0 {
//							for _, match := range matches {
//								if len(match) > 1 {
//									p := int(math.Floor(parseFloat(match[1])))
//									matched = true
//									progressChan <- p
//									fmt.Printf("Progress: %d%%\n", p)
//									klog.Infof("Send progress: %d", p)
//								}
//							}
//						}
//
//						if !matched {
//							klog.Infof("No percent info in: %s", line)
//						}
//					}
//				}
//			}
//			if err != nil {
//				klog.Errorf("Error reading rsync output: %v", err)
//				if err == io.EOF {
//					break
//				}
//				mu.Lock()
//				if firstErr == nil {
//					firstErr = err
//				}
//				mu.Unlock()
//				break
//			}
//		}
//	}()
//
//	// Goroutine: 等待命令完成并处理错误
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		if err := cmd.Wait(); err != nil {
//			mu.Lock()
//			if firstErr == nil {
//				firstErr = err
//			}
//			mu.Unlock()
//		}
//	}()
//
//	// Goroutine: 确保在 context 取消时终止命令
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		<-ctx.Done()
//		if cmd.Process != nil {
//			cmd.Process.Signal(os.Interrupt) // 尝试优雅地终止进程
//			done := make(chan error)
//			go func() {
//				done <- cmd.Wait()
//			}()
//			select {
//			case err := <-done:
//				mu.Lock()
//				if firstErr == nil {
//					firstErr = err
//				}
//				mu.Unlock()
//			case <-time.After(5 * time.Second): // 等待一段时间后强制终止
//				cmd.Process.Kill()
//				<-done // 确保等待进程退出
//			}
//		}
//	}()
//
//	// 启动一个 goroutine 来将错误发送到 errChan（如果存在）
//	go func() {
//		wg.Wait()
//		mu.Lock()
//		if firstErr != nil {
//			errChan <- firstErr
//		}
//		close(errChan)
//		mu.Unlock()
//	}()
//
//	return progressChan, errChan, nil
//}

// func ExecuteRsyncWithContext(ctx context.Context, source, dest string) (chan int, chan string, chan error, error) {
func ExecuteRsync(task *pool.Task, progressLeft, progressRight int) error {
	klog.Infoln("Starting ExecuteRsync function")

	stdoutReader, stdoutWriter := io.Pipe()

	cmd := exec.CommandContext(task.Ctx, "rsync", "-av", "--info=progress2", fmt.Sprintf("--bwlimit=%d", 1024),
		RootPrefix+task.Source, RootPrefix+task.Dest)
	cmd.Stdout = stdoutWriter

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	var cmdDone = make(chan struct{})

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

		for {
			select {
			case <-task.Ctx.Done():
				klog.Infoln("Stdout processing goroutine exiting due to cancellation")
				return
			case <-cmdDone:
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
				}
				if err != nil {
					if err != io.EOF {
						mu.Lock()
						if firstErr == nil {
							firstErr = err
							klog.Errorf("Error reading stdout: %v", firstErr)
						}
						mu.Unlock()
					}
					klog.Infoln("Finished reading stdout")
					return
				}
			}
		}
	}()

	// 取消处理goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infoln("Starting cancellation handling goroutine")
		defer klog.Infoln("Cancellation handling goroutine completed")

		<-task.Ctx.Done()
		if cmd.Process != nil {
			klog.Infoln("Sending interrupt signal to rsync command")
			cmd.Process.Signal(os.Interrupt)
			done := make(chan error)
			go func() {
				done <- cmd.Wait()
			}()
			select {
			case <-done:
				klog.Infoln("Rsync command interrupted successfully")
			case <-time.After(5 * time.Second):
				klog.Infoln("Killing rsync command after timeout")
				cmd.Process.Kill()
				<-done
			}
		}
		stdoutWriter.Close()
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

						select {
						case task.ErrChan <- firstErr:
							firstErr = nil
							klog.Infoln("Error sent to ErrChan")
						default:
							klog.Errorf("Rsync command failed: %v (channel full)", firstErr)
						}
					}()
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
		klog.Errorf("ExecuteRsync failed with error: %v", firstErr)
		return firstErr
	}

	return nil
}
