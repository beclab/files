package utils

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

type Process struct {
	cmd *exec.Cmd
}

func NewProcess(name string, args ...string) *Process {
	return &Process{
		cmd: exec.Command(name, args...),
	}
}

func (p *Process) Start() error {
	klog.Infoln("Starting process")
	klog.Infoln(p.cmd)
	return p.cmd.Start()
}

func (p *Process) Wait() error {
	return p.cmd.Wait()
}

func (p *Process) Kill() error {
	klog.Infoln(p)
	klog.Infoln(p.cmd)
	klog.Infoln(p.cmd.Process)
	if p.cmd.Process != nil {
		return p.cmd.Process.Signal(syscall.SIGKILL)
	}
	return nil
}

func (p *Process) CloseMainWindow() error {
	return p.cmd.Process.Signal(syscall.SIGTERM)
}

func (p *Process) Refresh() {
}

func (p *Process) WaitForExit(timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- p.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("process did not exit within %s", timeout)
	}
}

func (p *Process) WaitForInputIdle() error {
	return fmt.Errorf("WaitForInputIdle is not implemented")
}

func (p *Process) StdinPipe() (io.WriteCloser, error) {
	return p.cmd.StdinPipe()
}

func (p *Process) StdoutPipe() (io.Reader, error) {
	return p.cmd.StdoutPipe()
}

func (p *Process) StderrPipe() (io.Reader, error) {
	return p.cmd.StderrPipe()
}

func (p *Process) Id() int {
	if p.cmd.Process == nil {
		return -1
	}
	return p.cmd.Process.Pid
}

func (p *Process) ExitCode() int {
	if p.cmd.ProcessState == nil {
		return -1
	}
	return p.cmd.ProcessState.ExitCode()
}

func (p *Process) ProcessName() string {
	return p.cmd.Path
}
