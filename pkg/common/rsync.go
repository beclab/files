package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
)

var (
	completeMsgs = []string{"sent", "received", "total size is", "speedup is"}
)

func GetCommand(c string) (string, error) {
	return exec.LookPath(c)
}

func ExecRsync(ctx context.Context, name string, args []string, callbackup func(p int, t int64)) (string, error) {
	var opts = CommandOptions{
		Name:   name,
		Args:   args,
		Print:  true,
		Reaper: true,
	}

	c := NewCommand(ctx, opts)

	var errMsg string
	var errChan = make(chan error, 1)

	go func() {
		defer close(errChan)
		for {
			select {
			case result, ok := <-c.Ch:
				if !ok {
					errChan <- errors.New("chan is closed")
					return
				}
				if result == "" {
					continue
				}

				if strings.Contains(result, "error") || strings.Contains(result, "failed:") {
					errChan <- errors.New(formatFailed(result))
					return
				}

				if progress, trans, err := formatProgress(result); err == nil {
					callbackup(progress, trans)
					continue
				} else {
					errChan <- err
					return
				}

				if formatFinished(result) {
					return
				}
			}
		}
	}()

	if err := c.Run(); err != nil {
		errMsg = err.Error()
		if strings.Contains(errMsg, "error") || strings.Contains(errMsg, "failed:") {
			errMsg = formatFailed(errMsg)
			return "", errors.New(errMsg)
		}
		return "", err
	}

	if e, ok := <-errChan; ok && e != nil {
		return "", e
	}

	return "", nil
}

func formatFailed(l string) string {
	var msgs = strings.Split(l, "\n")
	var msg = msgs[len(msgs)-1]
	var result string
	if strings.Contains(msg, " failed: ") {
		// rsync: [receiver] mkstemp "/{path}/.{filename}.tar.gz.FHtToQ" failed: No such file or directory (2)
		result = msg[strings.LastIndex(msg, "failed:")+7:]
		result = strings.TrimSpace(result)
	} else {
		result = l // msg
	}

	return result
}

func formatFinished(l string) bool {
	for _, m := range completeMsgs {
		if !strings.Contains(l, m) {
			return false
		}
	}
	return true
}

func formatProgress(l string) (int, int64, error) {
	// 441,505,944  87%    7.82MB/s    0:00:07
	// sent 479,087,779 bytes  received 184 bytes  8,189,537.83 bytes/sec
	var lines = strings.Split(l, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "% ") {
			continue
		}
		var transfer int64
		var s = strings.Fields(line)
		if len(s) == 4 {
			var tr = strings.ReplaceAll(s[0], ",", "")
			transfer, _ = ParseInt64(tr)
			if strings.HasSuffix(s[1], "%") {
				var ps = strings.TrimSuffix(s[1], "%")
				var p, err = strconv.Atoi(ps)
				if err == nil {
					return p, transfer, nil
				}
				return 0, 0, err
			}
		}
	}

	return 0, 0, fmt.Errorf("not the progress")
}

type Command struct {
	options CommandOptions
	ctx     context.Context
	cmd     *exec.Cmd
	Ch      chan string
}

type CommandOptions struct {
	Name   string
	Args   []string
	Envs   map[string]string
	Print  bool
	Reaper bool
}

func NewCommand(ctx context.Context, opts CommandOptions) *Command {
	return &Command{
		options: opts,
		ctx:     ctx,
		Ch:      make(chan string, 50),
	}
}

func (c *Command) GetCmd() *exec.Cmd {
	return c.cmd
}

func (c *Command) Run() error {
	c.cmd = exec.Command(c.options.Name, c.options.Args...)
	c.cmd.Env = append(os.Environ(), c.cmd.Env...)
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	for k, v := range c.options.Envs {
		c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "stdout pipe error")
	}
	c.cmd.Stderr = c.cmd.Stdout

	klog.Infof("[Cmd] %s", c.cmd.String())
	if err := c.cmd.Start(); err != nil {
		return errors.Wrap(err, "cmd start error")
	}

	pgid, err := syscall.Getpgid(c.cmd.Process.Pid)
	if err != nil {
		pgid = c.cmd.Process.Pid
	}

	sigc := make(chan os.Signal, 1)
	waitDone := make(chan struct{})
	reaperDone := make(chan struct{})

	signal.Notify(sigc, syscall.SIGCHLD)
	go func() {
		defer func() {
			signal.Stop(sigc)
			close(reaperDone)
		}()

		var ws syscall.WaitStatus

		for {
			select {
			case <-sigc:
				if !c.options.Reaper {
					continue
				}
				for {
					pid, err := syscall.Wait4(-pgid, &ws, syscall.WNOHANG, nil)
					if pid <= 0 || err != nil {
						break
					}
				}
			case <-waitDone:
				if c.options.Reaper {
					for {
						pid, err := syscall.Wait4(-pgid, &ws, 0, nil)
						if pid <= 0 || err != nil {
							break
						}
					}
				}
				return
			}
		}
	}()

	var gErr error
	go func() {
		select {
		case <-c.ctx.Done():
			gErr = c.ctx.Err()
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		case <-waitDone:
		}
	}()

	go func() {
		defer close(c.Ch)
		var reader = bufio.NewReader(stdout)
		for {
			var n int
			buffer := make([]byte, 4096)
			n, err = reader.Read(buffer)
			if err != nil {
				if err == io.EOF || errors.Is(err, os.ErrClosed) {
					return
				}
				gErr = err
				return
			}

			if n <= 0 {
				return
			}

			if n > 0 {
				content := string(buffer[:n])
				if strings.Contains(content, "error") || strings.Contains(content, "failed:") {
					gErr = errors.Errorf(content)
					return
				} else {
					c.Ch <- content
				}
			}

		}
	}()

	err = c.cmd.Wait()

	close(waitDone)
	<-reaperDone

	if err != nil {
		if errors.Is(err, syscall.ECHILD) {
			err = nil
		}
		if gErr != nil {
			return gErr
		}
		return err
	}

	if gErr != nil {
		return gErr
	}

	return nil
}
