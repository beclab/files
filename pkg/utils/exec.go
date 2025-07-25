package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

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
