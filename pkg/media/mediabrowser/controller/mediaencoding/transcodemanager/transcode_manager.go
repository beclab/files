package transcodemanager

import (
	"context"
	"time"

	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/controller/streaming"
)

type ITranscodeManager interface {
	GetTranscodingJob(playSessionID string) *mediaencoding.TranscodingJob
	GetTranscodingJob2(path string, jobType mediaencoding.TranscodingJobType) *mediaencoding.TranscodingJob

	PingTranscodingJob(playSessionID string, isUserPaused *bool)

	KillTranscodingJobs(deviceID, playSessionID string, deleteFiles func(string) bool) error

	ReportTranscodingProgress(job *mediaencoding.TranscodingJob, state *streaming.StreamState, transcodingPosition *time.Duration, framerate *float32, percentComplete *float64, bytesTranscoded *int64, bitRate *int)

	StartFfMpeg(state streaming.StreamState, outputPath, commandLineArgs string, userID string, jobType mediaencoding.TranscodingJobType, cancellationTokenSource context.Context, workingDir string) (*mediaencoding.TranscodingJob, error)

	OnTranscodeBeginRequest(path string, jobType mediaencoding.TranscodingJobType) *mediaencoding.TranscodingJob

	OnTranscodeEndRequest(job *mediaencoding.TranscodingJob)

	//LockAsync(outputPath string, cancellationToken context.Context) (IDisposable, error)
	LockAsync(outputPath string, cancellationToken context.Context) (func(), error)
}
