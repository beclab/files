package exec

import (
	"context"
	"errors"
	"files/pkg/utils"
	"fmt"
	"strconv"
	"strings"
)

var (
	completeMsgs = []string{"sent", "received", "total size is", "speedup is"}
)

func ExecRsync(ctx context.Context, name string, args []string, callbackup func(p int, t int64)) (string, error) {
	var opts = utils.CommandOptions{
		Name:   name,
		Args:   args,
		Print:  true,
		Reaper: true,
	}

	c := utils.NewCommand(ctx, opts)

	var errMsg string
	var errChan = make(chan error, 1)

	go func() {
		defer close(errChan)
		for {
			select {
			case result, ok := <-c.Ch:
				if !ok {
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
		result = msg
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
			transfer, _ = utils.ParseInt64(tr)
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
