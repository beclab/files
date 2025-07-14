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
				klog.Errorf("Start failed: %v", err)
			}
			mu.Unlock()
			if stdoutWriter != nil {
				stdoutWriter.Close()
			}
			return
		}

		done := make(chan error, 1)
		go func() {
			defer close(done)
			done <- cmd.Wait()
		}()

		select {
		case <-task.Ctx.Done():
			return
		case err, ok := <-done:
			if !ok {
				return
			}

			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					klog.Errorf("Wait failed: %v", err)
				}
				mu.Unlock()
			}
		}

		if stdoutWriter != nil {
			stdoutWriter.Close()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infoln("Starting stdout processing goroutine")
		defer klog.Infoln("Stdout processing goroutine completed")

		reader := bufio.NewReader(stdoutReader)
		buffer := make([]byte, 4096)
		re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

		eofCount := 0
		const maxEOFCount = 10

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
					eofCount = 0
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

	go func() {
		klog.Infoln("Starting cancellation handling goroutine")
		defer klog.Infoln("Cancellation handling goroutine completed")

		totalTimeoutCtx, cancelTotalTimeout := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancelTotalTimeout()

		for {
			select {
			case <-totalTimeoutCtx.Done():
				klog.Errorln("Total timeout (1h) reached! Forcing exit...")
				if cmd.Process != nil {
					klog.Infoln("Killing rsync command due to total timeout")
					if err := cmd.Process.Kill(); err != nil {
						klog.Errorf("Kill failed: %v", err)
					}
				}
				if stdoutWriter != nil {
					stdoutWriter.Close()
				}
				return

			case <-task.Ctx.Done():
				if cmd.Process == nil {
					klog.Warningln("Rsync process already exited, skipping cancellation")
					return
				}

				klog.Infoln("Sending interrupt signal to rsync command")
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					klog.Errorf("Failed to send interrupt: %v", err)
				}

				done := make(chan error, 1)
				go func() {
					done <- cmd.Wait()
				}()

				select {
				case err := <-done:
					if err != nil {
						klog.Errorf("Rsync exited with error: %v", err)
					} else {
						klog.Infoln("Rsync interrupted successfully")
					}
				case <-time.After(5 * time.Second):
					klog.Infoln("Killing rsync command after 5s timeout")
					if cmd.Process != nil {
						if err := cmd.Process.Kill(); err != nil {
							klog.Errorf("Kill failed: %v", err)
						}
					}
					<-done
				}

				if stdoutWriter != nil {
					stdoutWriter.Close()
				}
				return
			}
		}
	}()

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
					}()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		klog.Infoln("Starting command execution state monitoring goroutine")
		defer klog.Infoln("Command execution state monitoring goroutine completed")

		var (
			lastProgress    int
			lastLogLength   int
			noChangeCounter int
		)

		lastProgress = task.Progress
		lastLogLength = len(task.Log)

		checkInterval := 1 * time.Second
		maxStaleChecks := 5

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-cmdDone:
				klog.Infoln("Rsync state monitored")
				if stdoutWriter != nil {
					stdoutWriter.Close()
				}
				return

			case <-ticker.C:
				currentProgress := task.Progress
				currentLogLength := len(task.Log)

				if currentProgress == lastProgress && currentLogLength == lastLogLength {
					noChangeCounter++
					klog.Warningf("No progress/logs for %d seconds", noChangeCounter*int(checkInterval.Seconds()))
				} else {
					noChangeCounter = 0
					lastProgress = currentProgress
					lastLogLength = currentLogLength
					klog.Infoln("Progress/logs updated, resetting counter")
				}

				if noChangeCounter >= maxStaleChecks {
					klog.Errorln("Force exiting due to 5 seconds of inactivity")
					if cmd.Process != nil {
						klog.Infoln("Killing rsync process")
						if err := cmd.Process.Kill(); err != nil {
							klog.Errorf("Kill failed: %v", err)
						}
					}
					if stdoutWriter != nil {
						stdoutWriter.Close()
					}
					task.Cancel()
					return
				}
			}
		}
	}()

	klog.Infoln("Waiting for all goroutines to complete")
	wg.Wait()
	klog.Infoln("All goroutines completed")

	if firstErr != nil {
		if task.Status != "failed" {
			errMsg := parseRsyncError(firstErr, task)
			task.Log = append(task.Log, "["+time.Now().Format("2006-01-02 15:04:05")+"]"+errMsg)
			task.FailedReason = errMsg
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
