package job

import (
	"context"
	"files/pkg/drivers/clouds/rclone/common"
	"files/pkg/drivers/clouds/rclone/utils"
	commonutils "files/pkg/utils"
	"fmt"
	"net/http"
	"time"
)

type Interface interface {
	Status(jobId int) ([]byte, error) // job/status
	Stop(jobId int) ([]byte, error)   // job/stop
	Stats(jobId int) ([]byte, error)  // core/stats with progress
}

type job struct {
}

var _ Interface = &job{}

func NewJob() *job {
	return &job{}
}

func (j *job) Status(jobId int) ([]byte, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, JobStatusPath)
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var data = &JobStatusReq{
		JobId: jobId,
	}

	resp, err := utils.Request(ctx, url, http.MethodPost, nil, []byte(commonutils.ToJson(data)))
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (j *job) Stats(jobId int) ([]byte, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, CoreStats)
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var data = &CoreStatsReq{
		Group: fmt.Sprintf("job/%d", jobId),
	}

	resp, err := utils.Request(ctx, url, http.MethodPost, nil, []byte(commonutils.ToJson(data)))
	if err != nil {
		return nil, err
	}

	return resp, nil

}

func (j *job) Stop(jobId int) ([]byte, error) {
	var url = fmt.Sprintf("%s/%s", common.ServeAddr, JobStopPath)
	var ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var data = &JobStatusReq{
		JobId: jobId,
	}

	resp, err := utils.Request(ctx, url, http.MethodPost, nil, []byte(commonutils.ToJson(data)))
	if err != nil {
		return nil, err
	}

	return resp, nil
}
