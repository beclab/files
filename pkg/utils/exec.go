package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	Name  string
	Args  []string
	Envs  map[string]string
	Print bool
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
	var err error
	c.cmd = exec.CommandContext(c.ctx, c.options.Name, c.options.Args...)
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

	defer func() error {
		klog.Infof("[command] run exec defer")
		if errWait := c.cmd.Wait(); errWait != nil {
			return errors.Wrapf(errWait, fmt.Sprintf("wait error for command: %s, exec error %v", c.cmd.String(), err))
		}
		return err
	}()

	reader := bufio.NewReader(stdout)

	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
			var n int
			buffer := make([]byte, 4096)
			n, err = reader.Read(buffer)

			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

			if n <= 0 {
				return nil
			}

			chunk := string(buffer[:n])
			chunk = strings.TrimSpace(chunk)

			if c.options.Print {
				klog.Infof("[Cmd] run output: %s", chunk)
			}

			if strings.Contains(chunk, "error") || strings.Contains(chunk, "failed:") {
				return errors.New(chunk)
			}

			c.Ch <- chunk
		}
	}
}
