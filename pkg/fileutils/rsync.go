package fileutils

import (
	"bufio"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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

func ExecuteRsync(source, dest string) (chan int, error) {
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
