package mediaencoding

import (
	"context"
	"io"
	//	"os"
	"sync"
	"time"

	//	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/utils"

	"k8s.io/klog/v2"
)

type TranscodingJob struct {
	PlaySessionID *string
	LiveStreamID  *string
	IsLiveOutput  bool
	MediaSource   *dto.MediaSourceInfo
	Path          *string
	Type          TranscodingJobType
	//Process                *os.Process
	Stdin                     io.WriteCloser
	Process                   *utils.Process
	ActiveRequestCount        int
	DeviceID                  *string
	CancellationTokenSource   *context.Context
	HasExited                 bool
	ExitCode                  int
	IsUserPaused              bool
	ID                        *string
	Framerate                 *float32
	CompletionPercentage      *float64
	BytesDownloaded           int64
	BytesTranscoded           *int64
	BitRate                   *int
	TranscodingPositionTicks  *int64
	DownloadPositionTicks     *int64
	TranscodingThrottler      *TranscodingThrottler
	TranscodingSegmentCleaner *TranscodingSegmentCleaner
	LastPingDate              time.Time
	PingTimeout               time.Duration
	Logger                    *utils.Logger
	processLock               sync.Mutex
	timerLock                 sync.Mutex
	killTimer                 *time.Timer
}

func (t *TranscodingJob) StopKillTimer() {
	t.timerLock.Lock()
	defer t.timerLock.Unlock()

	if t.killTimer != nil {
		t.killTimer.Stop()
		t.killTimer = nil
	}
}

func (t *TranscodingJob) StartKillTimer(callback func(interface{})) {
	t.StartKillTimerWithTimeout(callback, t.PingTimeout)
}

func (t *TranscodingJob) StartKillTimerWithTimeout(callback func(interface{}), intervalMs time.Duration) {
	if t.HasExited {
		return
	}

	t.timerLock.Lock()
	defer t.timerLock.Unlock()

	if t.killTimer == nil {
		t.Logger.Infof("Starting kill timer at %dms. JobId %s PlaySessionId %s", intervalMs, *t.ID, *t.PlaySessionID)
		//	t.killTimer = time.AfterFunc(time.Duration(intervalMs)*time.Millisecond, func() {
		t.killTimer = time.AfterFunc(intervalMs, func() {
			callback(t)
		})
	} else {
		t.Logger.Infof("Changing kill timer to %dms. JobId %s PlaySessionId %s", intervalMs, *t.ID, *t.PlaySessionID)
		t.killTimer.Reset(intervalMs)
	}
}

func (t *TranscodingJob) DisposeKillTimer() {
	t.timerLock.Lock()
	defer t.timerLock.Unlock()

	if t.killTimer != nil {
		t.killTimer.Stop()
		t.killTimer = nil
	}
}

func (t *TranscodingJob) ChangeKillTimerIfStarted() {
	if t.HasExited {
		return
	}

	t.timerLock.Lock()
	defer t.timerLock.Unlock()

	if t.killTimer != nil {
		intervalMs := t.PingTimeout
		t.Logger.Infof("Changing kill timer to %dms. JobId %s PlaySessionId %s", intervalMs, t.ID, t.PlaySessionID)
		t.killTimer.Reset(intervalMs)
	}
}

func (t *TranscodingJob) Stop() {
	klog.Infoln("job stop...........................", *t.ID)
	t.processLock.Lock()
	defer t.processLock.Unlock()

	if t.TranscodingThrottler != nil {
		t.TranscodingThrottler.Stop()
	}

	if t.TranscodingSegmentCleaner != nil {
		t.TranscodingSegmentCleaner.Stop()
	}

	var process = t.Process
	klog.Infof("%+v\n", process)
	if !t.HasExited {
		klog.Infoln(t.Path)
		klog.Infoln(*t.Path)
		t.Logger.Infomation("Stopping ffmpeg process with q command for %s", *t.Path)

		/*
			stdin, err := process.StdinPipe()
			if err != nil {
				log.Fatalf("get stdin error: %v\n", err)
			}
		*/
		_, err := t.Stdin.Write([]byte("q"))
		if err != nil {
			t.Logger.Errorf("Error writing to ffmpeg stdin: %v", err)
			process.Kill()
		}

		// Need to wait because killing is asynchronous.
		if err := process.WaitForExit(5 * time.Second); err != nil {
			t.Logger.Infomation("Killing FFmpeg process for %s %v", *t.Path, err)
			process.Kill()
		}
	}
}

func (job *TranscodingJob) Dispose() {
	/*
	   if job.Process != nil {
	       job.Process.Dispose()
	       job.Process = nil
	   }
	*/
	if job.killTimer != nil {
		job.killTimer.Stop()
		job.killTimer = nil
	}
	/*
	   if job.CancellationTokenSource != nil {
	       job.CancellationTokenSource.Cancel()
	       job.CancellationTokenSource = nil
	   }
	*/
	if job.TranscodingThrottler != nil {
		job.TranscodingThrottler.Dispose()
		job.TranscodingThrottler = nil
	}
	if job.TranscodingSegmentCleaner != nil {
		job.TranscodingSegmentCleaner.Dispose()
		job.TranscodingSegmentCleaner = nil
	}
}
