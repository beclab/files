package exec

import (
	"context"
	"errors"
	"files/pkg/utils"
	"strings"

	"k8s.io/klog/v2"
)

func ExecRsync(parentCtx context.Context, name string, args []string) (string, error) {
	var ctx, cancel = context.WithCancel(parentCtx)
	defer cancel()

	var opts = utils.CommandOptions{
		Name:  name,
		Args:  args,
		Print: true,
	}
	c := utils.NewCommand(ctx, opts)

	var errMsg string

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case res, ok := <-c.Ch:
				if !ok {
					return
				}
				if res == nil || len(res) == 0 {
					continue
				}

				result := string(res)
				klog.Infof("[rsync] output: %s", result)

				if strings.Contains(result, "error") {
					errMsg = result
					return
				}

				// todo update task progress
				formatRsyncLogs(result)
			}
		}
	}()

	err := c.Run()
	if err != nil {
		return "", err
	}

	if errMsg != "" {
		return "", errors.New(errMsg)
	}

	return "", nil
}

func formatRsyncLogs(l string) {
	if strings.Contains(l, "%") {
	} else {

	}
}
