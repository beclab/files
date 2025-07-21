package utils

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type Command struct {
	options CommandOptions
	ctx     context.Context
	cancel  context.CancelFunc
	cmd     *exec.Cmd
	Ch      chan []byte
}

type CommandOptions struct {
	Name  string
	Args  []string
	Envs  map[string]string
	Print bool
}

func NewCommand(ctx context.Context, opts CommandOptions) *Command {
	var cmdCtx, cancel = context.WithCancel(ctx)
	return &Command{
		options: opts,
		ctx:     cmdCtx,
		cancel:  cancel,
		Ch:      make(chan []byte, 50),
	}
}

func (c *Command) Cancel() {
	c.cancel()
}

func (c *Command) GetCmd() *exec.Cmd {
	return c.cmd
}

func (c *Command) Run() error {
	var err error
	c.cmd = exec.CommandContext(c.ctx, c.options.Name, c.options.Args...)
	c.cmd.Env = append(os.Environ(), c.cmd.Env...)

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
		close(c.Ch)
		if errWait := c.cmd.Wait(); errWait != nil {
			return errors.Wrapf(errWait, fmt.Sprintf("wait error for command: %s, exec error %v", c.cmd.String(), err))
		}
		return err
	}()

	reader := bufio.NewReader(stdout)

	for {
		select {
		case <-c.ctx.Done():
			if c.cmd.Process != nil {
				_ = c.cmd.Process.Kill()
			}
			return c.ctx.Err()
		default:
			var n int
			buffer := make([]byte, 4096)
			n, err = reader.Read(buffer)

			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			if n <= 0 {
				break
			}

			if n > 0 {
				chunk := buffer[:n]
				chunk = bytes.TrimSpace(chunk)
				c.Ch <- chunk
			}
		}
	}
}
